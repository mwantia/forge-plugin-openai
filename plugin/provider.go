package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/sashabaranov/go-openai"
)

type OpenAIProviderPlugin struct {
	plugins.UnimplementedProviderPlugin
	driver *OpenAIDriver
}

func (p *OpenAIProviderPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *OpenAIProviderPlugin) Chat(ctx context.Context, messages []plugins.ChatMessage, tools []plugins.ToolCall, model *plugins.Model) (plugins.ChatStream, error) {
	if p.driver.streamer == nil {
		return nil, fmt.Errorf("driver not configured")
	}
	if model == nil || model.ModelName == "" {
		return nil, fmt.Errorf("model is required")
	}

	req := openai.ChatCompletionRequest{
		Model:  model.ModelName,
		Stream: true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}

	if p.driver.config.Seed != nil {
		req.Seed = p.driver.config.Seed
	}
	if model.Temperature != 0 {
		req.Temperature = float32(model.Temperature)
	}

	for _, msg := range messages {
		cm := openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Role == "assistant" && msg.ToolCalls != nil {
			for _, tc := range msg.ToolCalls.ToolCalls {
				argsJSON, err := json.Marshal(tc.Arguments)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool call arguments for %q: %w", tc.Name, err)
				}
				cm.ToolCalls = append(cm.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}
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
		req.Tools = append(req.Tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		})
	}

	stream, err := p.driver.streamer.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat stream: %w", err)
	}

	return newChatStream(p.driver.log, stream, model.CostPerInputToken, model.CostPerOutputToken, model.CostPerCachedInputToken), nil
}

func (p *OpenAIProviderPlugin) Embed(ctx context.Context, content string, model *plugins.Model) ([][]float32, error) {
	if p.driver.client == nil {
		return nil, fmt.Errorf("driver not configured")
	}
	if model == nil || model.ModelName == "" {
		return nil, fmt.Errorf("model is required")
	}

	resp, err := p.driver.client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Model: openai.EmbeddingModel(model.ModelName),
		Input: []string{content},
	})
	if err != nil {
		return nil, fmt.Errorf("embed failed: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		embeddings[i] = d.Embedding
	}
	return embeddings, nil
}

func (p *OpenAIProviderPlugin) ListModels(ctx context.Context) ([]*plugins.Model, error) {
	if p.driver.client == nil {
		return nil, fmt.Errorf("driver not configured")
	}

	resp, err := p.driver.client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list models failed: %w", err)
	}

	models := make([]*plugins.Model, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = &plugins.Model{
			ModelName: m.ID,
			Metadata: map[string]any{
				"owned_by": m.OwnedBy,
				"created":  m.CreatedAt,
			},
		}
	}
	return models, nil
}

// toolParametersToMap converts ToolParameters into the map[string]any that the
// OpenAI "parameters" field expects (a plain JSON Schema object).
func toolParametersToMap(p plugins.ToolParameters) (map[string]any, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(b, &m)
}
