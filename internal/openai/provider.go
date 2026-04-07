package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge-sdk/pkg/retry"
	openailib "github.com/sashabaranov/go-openai"
)

// classifyError translates a go-openai error into a retry.RetryableError so
// the retry package can decide whether to back off or fail fast.
// 429 and 5xx are retryable; 4xx (except 429) fail immediately.
// Plain network errors (no APIError wrapper) are always retried.
//
// go-openai returns *APIError when the provider sends a well-formed JSON error
// body, and *RequestError when the body is non-JSON (e.g. a plain-text "504
// Gateway Timeout" from an upstream proxy). Both carry HTTPStatusCode.
func classifyError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *openailib.APIError
	if errors.As(err, &apiErr) {
		return &retry.RetryableError{
			StatusCode: apiErr.HTTPStatusCode,
			Err:        err,
		}
	}
	var reqErr *openailib.RequestError
	if errors.As(err, &reqErr) {
		return &retry.RetryableError{
			StatusCode: reqErr.HTTPStatusCode,
			Err:        err,
		}
	}
	return err
}

// OpenAIProviderPlugin implements ProviderPlugin for OpenAI-compatible endpoints.
type OpenAIProviderPlugin struct {
	plugins.UnimplementedProviderPlugin
	driver *OpenAIDriver
}

func (p *OpenAIProviderPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *OpenAIProviderPlugin) Chat(ctx context.Context, messages []plugins.ChatMessage, tools []plugins.ToolCall, model *plugins.Model) (plugins.ChatStream, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}
	if model == nil || model.ModelName == "" {
		return nil, fmt.Errorf("'model' is undefined or invalid")
	}

	req := openailib.ChatCompletionRequest{
		Model:  model.ModelName,
		Stream: true,
		StreamOptions: &openailib.StreamOptions{
			IncludeUsage: true,
		},
	}

	if p.driver.config.Seed != nil {
		req.Seed = p.driver.config.Seed
	}

	var costPerInputToken, costPerOutputToken float64
	var reasoning bool

	alias, ok := p.driver.config.Models[model.ModelName]
	if ok {
		if alias.BaseModel != "" {
			req.Model = alias.BaseModel
		}
		costPerInputToken = alias.CostPerInputToken
		costPerOutputToken = alias.CostPerOutputToken
		reasoning = alias.Reasoning

		if alias.Options != nil {
			if alias.Options.Temperature != nil {
				req.Temperature = float32(*alias.Options.Temperature)
			}
			if alias.Options.TopP != nil {
				req.TopP = float32(*alias.Options.TopP)
			}
			if alias.Options.MaxTokens != nil {
				req.MaxTokens = *alias.Options.MaxTokens
			}
		}

		if alias.System != "" {
			hasSystem := false
			for _, m := range messages {
				if m.Role == "system" {
					hasSystem = true
					break
				}
			}
			if !hasSystem {
				system, err := renderSystemPrompt(alias.System)
				if err != nil {
					p.driver.log.Warn("Failed to render system prompt template", "error", err)
					system = alias.System
				}
				req.Messages = append(req.Messages, openailib.ChatCompletionMessage{
					Role:    openailib.ChatMessageRoleSystem,
					Content: system,
				})
			}
		}
	}

	// Caller-supplied temperature wins over alias defaults.
	if model.Temperature != 0 {
		req.Temperature = float32(model.Temperature)
	}

	for _, msg := range messages {
		cm := openailib.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Assistant messages that carry only tool calls must have empty content.
		if msg.Role == "assistant" && msg.ToolCalls != nil {
			if msg.Content == "" {
				cm.Content = ""
			}
			for _, tc := range msg.ToolCalls.ToolCalls {
				argsJSON, err := json.Marshal(tc.Arguments)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool call arguments for %q: %w", tc.Name, err)
				}
				cm.ToolCalls = append(cm.ToolCalls, openailib.ToolCall{
					ID:   tc.ID,
					Type: openailib.ToolTypeFunction,
					Function: openailib.FunctionCall{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}

		// Tool result messages carry the call ID for correlation.
		if msg.Role == "tool" && msg.ToolCalls != nil {
			cm.ToolCallID = msg.ToolCalls.ID
		}

		req.Messages = append(req.Messages, cm)
	}

	for _, tool := range tools {
		params, err := toolParametersToMap(tool.Parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameters for tool %q: %w", tool.Name, err)
		}
		req.Tools = append(req.Tools, openailib.Tool{
			Type: openailib.ToolTypeFunction,
			Function: &openailib.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		})
	}

	// Retry only the stream *creation*. Once the stream is open and Recv()
	// has started delivering chunks, retrying would re-send the full request
	// and duplicate already-emitted content — so that is left to the caller.
	var stream *openailib.ChatCompletionStream
	if err := retry.Do(ctx, retry.DefaultConfig, func(ctx context.Context) error {
		var callErr error
		stream, callErr = p.driver.client.CreateChatCompletionStream(ctx, req)
		return classifyError(callErr)
	}); err != nil {
		return nil, fmt.Errorf("failed to create chat stream: %w", err)
	}

	return NewChatStream(p.driver.log, stream, reasoning, costPerInputToken, costPerOutputToken), nil
}

// toolParametersToMap converts a typed ToolParameters into the map[string]any
// that the OpenAI "parameters" field expects (a plain JSON Schema object).
func toolParametersToMap(p plugins.ToolParameters) (map[string]any, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(b, &m)
}

// --- Embed ---

func (p *OpenAIProviderPlugin) Embed(ctx context.Context, content string, model *plugins.Model) ([][]float32, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}
	if model == nil || model.ModelName == "" {
		return nil, fmt.Errorf("'model' is undefined or invalid")
	}

	modelName := model.ModelName
	if alias, ok := p.driver.config.Models[modelName]; ok && alias.BaseModel != "" {
		modelName = alias.BaseModel
	}

	var resp openailib.EmbeddingResponse
	if err := retry.Do(ctx, retry.DefaultConfig, func(ctx context.Context) error {
		var callErr error
		resp, callErr = p.driver.client.CreateEmbeddings(ctx, openailib.EmbeddingRequestStrings{
			Model: openailib.EmbeddingModel(modelName),
			Input: []string{content},
		})
		return classifyError(callErr)
	}); err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		embeddings[i] = d.Embedding
	}
	return embeddings, nil
}

// --- Models ---

func (p *OpenAIProviderPlugin) ListModels(ctx context.Context) ([]*plugins.Model, error) {
	if p.driver.config == nil {
		return nil, fmt.Errorf("driver not configured")
	}

	var resp openailib.ModelsList
	if err := retry.Do(ctx, retry.DefaultConfig, func(ctx context.Context) error {
		var callErr error
		resp, callErr = p.driver.client.ListModels(ctx)
		return classifyError(callErr)
	}); err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	seen := make(map[string]struct{}, len(resp.Models))
	models := make([]*plugins.Model, len(resp.Models))
	for i, m := range resp.Models {
		seen[m.ID] = struct{}{}
		models[i] = &plugins.Model{
			ModelName: m.ID,
			Metadata: map[string]any{
				"model_type": "model",
				"owned_by":   m.OwnedBy,
				"created":    m.CreatedAt,
			},
		}
	}

	// Append configured aliases that are not already present in the provider list.
	// Aliases use forge-local names (e.g. "assistant") that map to an underlying
	// base_model, so they will never collide with raw provider model IDs.
	for name, tmpl := range p.driver.config.Models {
		if _, ok := seen[name]; ok {
			continue
		}
		m := &plugins.Model{
			ModelName:          name,
			CostPerInputToken:  tmpl.CostPerInputToken,
			CostPerOutputToken: tmpl.CostPerOutputToken,
			Metadata: map[string]any{
				"model_type": "template",
				"base_model": tmpl.BaseModel,
			},
		}
		models = append(models, m)
	}

	return models, nil
}
