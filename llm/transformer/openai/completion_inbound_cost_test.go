package openai

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestCompletionInboundTransformer_TransformResponse_IncludesUsageCost(t *testing.T) {
	transformer := NewCompletionInboundTransformer()

	resp, err := transformer.TransformResponse(t.Context(), &llm.Response{
		ID:      "cmpl-123",
		Object:  "text_completion",
		Created: 1677652288,
		Model:   "gpt-3.5-turbo-instruct",
		Completion: &llm.CompletionResponse{
			Choices: []llm.CompletionChoice{
				{
					Text:         "Hello",
					Index:        0,
					FinishReason: lo.ToPtr("stop"),
				},
			},
		},
		Usage: &llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			Cost:             lo.ToPtr(0.000005),
		},
	})
	require.NoError(t, err)

	var completionResp CompletionResponse
	require.NoError(t, json.Unmarshal(resp.Body, &completionResp))
	require.NotNil(t, completionResp.Usage.Cost)
	require.InDelta(t, 0.000005, *completionResp.Usage.Cost, 1e-12)
}

func TestCompletionInboundTransformer_TransformStreamChunk_UsageOnlyIncludesCost(t *testing.T) {
	transformer := NewCompletionInboundTransformer()

	event, err := transformer.transformStreamChunk(&llm.Response{
		ID:      "cmpl-123",
		Object:  "text_completion",
		Created: 1677652288,
		Model:   "gpt-3.5-turbo-instruct",
		Usage: &llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			Cost:             lo.ToPtr(0.000005),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, event)

	var completionResp CompletionResponse
	require.NoError(t, json.Unmarshal(event.Data, &completionResp))
	require.NotNil(t, completionResp.Usage.Cost)
	require.InDelta(t, 0.000005, *completionResp.Usage.Cost, 1e-12)
	require.NotNil(t, completionResp.Choices)
	require.Empty(t, completionResp.Choices)
	require.Contains(t, string(event.Data), `"choices":[]`)
}
