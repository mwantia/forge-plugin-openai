package plugin

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	openailib "github.com/sashabaranov/go-openai"
)

// toolCallAccum accumulates streamed tool call fragments for a single index.
type toolCallAccum struct {
	id        string
	name      string
	arguments strings.Builder
}

// ChatStream wraps an openai.ChatCompletionStream and implements plugins.ChatStream.
// Tool calls are accumulated across delta fragments and returned together on the
// final Done chunk.
type ChatStream struct {
	logger hclog.Logger
	stream *openailib.ChatCompletionStream
	done   bool

	costPerInputToken       float64
	costPerOutputToken      float64
	costPerCachedInputToken float64

	toolCalls map[int]*toolCallAccum
}

func newChatStream(logger hclog.Logger, stream *openailib.ChatCompletionStream, costIn, costOut, costCached float64) *ChatStream {
	return &ChatStream{
		logger:                  logger.Named("stream"),
		stream:                  stream,
		costPerInputToken:       costIn,
		costPerOutputToken:      costOut,
		costPerCachedInputToken: costCached,
		toolCalls:               make(map[int]*toolCallAccum),
	}
}

func (s *ChatStream) Recv() (*plugins.ChatChunk, error) {
	if s.done {
		return nil, io.EOF
	}

	for {
		resp, err := s.stream.Recv()
		if err == io.EOF {
			s.done = true
			return s.buildDoneChunk(nil), nil
		}
		if err != nil {
			return nil, err
		}

		// Usage-only chunk (no choices) — treat as terminal.
		if len(resp.Choices) == 0 {
			if resp.Usage != nil {
				s.done = true
				return s.buildDoneChunk(resp.Usage), nil
			}
			continue
		}

		choice := resp.Choices[0]

		// Accumulate tool call delta fragments.
		for _, tc := range choice.Delta.ToolCalls {
			idx := tc.Index
			if idx == nil {
				continue
			}
			acc, ok := s.toolCalls[*idx]
			if !ok {
				acc = &toolCallAccum{}
				s.toolCalls[*idx] = acc
			}
			if tc.ID != "" {
				acc.id = tc.ID
			}
			if tc.Function.Name != "" {
				acc.name = tc.Function.Name
			}
			acc.arguments.WriteString(tc.Function.Arguments)
		}

		if choice.FinishReason != "" {
			s.done = true
			return s.buildDoneChunk(resp.Usage), nil
		}

		if choice.Delta.Content == "" {
			continue
		}

		return &plugins.ChatChunk{
			Role:  choice.Delta.Role,
			Delta: choice.Delta.Content,
		}, nil
	}
}

func (s *ChatStream) buildDoneChunk(usage *openailib.Usage) *plugins.ChatChunk {
	chunk := &plugins.ChatChunk{Done: true}

	for _, acc := range s.toolCalls {
		var args map[string]any
		if raw := acc.arguments.String(); raw != "" {
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				s.logger.Warn("Failed to parse tool call arguments", "tool", acc.name, "error", err)
			}
		}
		chunk.ToolCalls = append(chunk.ToolCalls, plugins.ChatToolCall{
			ID:        acc.id,
			Name:      acc.name,
			Arguments: args,
		})
	}

	if usage != nil && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		cached := 0
		if usage.PromptTokensDetails != nil {
			cached = usage.PromptTokensDetails.CachedTokens
		}
		nonCached := max(usage.PromptTokens-cached, 0)

		u := &plugins.TokenUsage{
			InputTokens:       usage.PromptTokens,
			OutputTokens:      usage.CompletionTokens,
			TotalTokens:       usage.TotalTokens,
			CachedInputTokens: cached,
		}
		if s.costPerInputToken > 0 {
			u.InputCost = float64(nonCached) * s.costPerInputToken
		}
		if s.costPerOutputToken > 0 {
			u.OutputCost = float64(usage.CompletionTokens) * s.costPerOutputToken
		}
		if cached > 0 {
			rate := s.costPerCachedInputToken
			if rate == 0 {
				rate = s.costPerInputToken
			}
			u.CachedInputCost = float64(cached) * rate
		}
		u.TotalCost = u.InputCost + u.OutputCost + u.CachedInputCost
		chunk.Usage = u
	}

	return chunk
}

func (s *ChatStream) Close() error {
	return s.stream.Close()
}
