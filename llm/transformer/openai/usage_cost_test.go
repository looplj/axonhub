package openai

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestUsage_CostJSON(t *testing.T) {
	t.Run("omitted when nil", func(t *testing.T) {
		payload, err := json.Marshal(&Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		})
		require.NoError(t, err)
		require.NotContains(t, string(payload), `"cost"`)
	})

	t.Run("serialized when set", func(t *testing.T) {
		payload, err := json.Marshal(&Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
			Cost:             lo.ToPtr(0.000005),
		})
		require.NoError(t, err)
		require.Contains(t, string(payload), `"cost":0.000005`)

		var decoded Usage
		require.NoError(t, json.Unmarshal(payload, &decoded))
		require.NotNil(t, decoded.Cost)
		require.InDelta(t, 0.000005, *decoded.Cost, 1e-12)
	})

	t.Run("UsageFromLLM preserves cost for client responses", func(t *testing.T) {
		usage := &llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             lo.ToPtr(0.000005),
		}
		converted := UsageFromLLM(usage)
		require.NotNil(t, converted.Cost)
		require.InDelta(t, 0.000005, *converted.Cost, 1e-12)
	})

	t.Run("ToLLMUsage drops upstream cost", func(t *testing.T) {
		var decoded Usage
		require.NoError(t, json.Unmarshal([]byte(`{
			"prompt_tokens": 10,
			"completion_tokens": 20,
			"total_tokens": 30,
			"cost": 0
		}`), &decoded))
		require.NotNil(t, decoded.Cost)

		usage := decoded.ToLLMUsage()
		require.NotNil(t, usage)
		require.Nil(t, usage.Cost)
	})

	t.Run("object-shaped upstream cost is ignored", func(t *testing.T) {
		var decoded Usage
		require.NoError(t, json.Unmarshal([]byte(`{
			"prompt_tokens": 15102,
			"completion_tokens": 282,
			"total_tokens": 15384,
			"cost": {
				"usd": 0.00701712,
				"hypercredits": 0.1403424
			}
		}`), &decoded))
		require.Nil(t, decoded.Cost)
		require.Equal(t, int64(15102), decoded.PromptTokens)
		require.Equal(t, int64(282), decoded.CompletionTokens)
		require.Equal(t, int64(15384), decoded.TotalTokens)
	})
}

func TestAggregateStreamChunks_ObjectShapedUsageCost(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion.chunk",
			"model": "test-model",
			"choices": [{
				"index": 0,
				"delta": {"content": "ok"},
				"finish_reason": "stop"
			}]
		}`)},
		{Data: []byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion.chunk",
			"model": "test-model",
			"choices": [],
			"usage": {
				"prompt_tokens": 15102,
				"completion_tokens": 282,
				"total_tokens": 15384,
				"cost": {
					"usd": 0.00701712,
					"hypercredits": 0.1403424
				}
			}
		}`)},
	}

	_, meta, err := AggregateStreamChunks(t.Context(), chunks, DefaultTransformChunk)
	require.NoError(t, err)
	require.NotNil(t, meta.Usage)
	require.Equal(t, int64(15102), meta.Usage.PromptTokens)
	require.Equal(t, int64(282), meta.Usage.CompletionTokens)
	require.Equal(t, int64(15384), meta.Usage.TotalTokens)
	require.Nil(t, meta.Usage.Cost)
}

func TestCompletionUsage_CostJSON(t *testing.T) {
	payload, err := json.Marshal(completionUsageFromLLM(&llm.Usage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		Cost:             lo.ToPtr(0.000005),
	}))
	require.NoError(t, err)
	require.Contains(t, string(payload), `"cost":0.000005`)

	empty, err := json.Marshal(completionUsageFromLLM(&llm.Usage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
	}))
	require.NoError(t, err)
	require.NotContains(t, string(empty), `"cost"`)
}
