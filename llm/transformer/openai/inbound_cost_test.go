package openai

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestInboundTransformer_TransformResponse_IncludesUsageCost(t *testing.T) {
	transformer := NewInboundTransformer()

	resp, err := transformer.TransformResponse(t.Context(), &llm.Response{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1677652288,
		Model:   "gpt-4",
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello"),
					},
				},
				FinishReason: lo.ToPtr("stop"),
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             lo.ToPtr(0.000005),
		},
	})
	require.NoError(t, err)

	var chatResp Response
	require.NoError(t, json.Unmarshal(resp.Body, &chatResp))
	require.NotNil(t, chatResp.Usage)
	require.NotNil(t, chatResp.Usage.Cost)
	require.InDelta(t, 0.000005, *chatResp.Usage.Cost, 1e-12)
}

func TestInboundTransformer_TransformStreamChunk_IncludesUsageCost(t *testing.T) {
	transformer := NewInboundTransformer()

	event, err := transformer.TransformStreamChunk(t.Context(), &llm.Response{
		ID:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Created: 1677652288,
		Model:   "gpt-4",
		Choices: []llm.Choice{},
		Usage: &llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             lo.ToPtr(0.000005),
		},
	})
	require.NoError(t, err)

	var chatResp Response
	require.NoError(t, json.Unmarshal(event.Data, &chatResp))
	require.NotNil(t, chatResp.Usage)
	require.NotNil(t, chatResp.Usage.Cost)
	require.InDelta(t, 0.000005, *chatResp.Usage.Cost, 1e-12)
}
