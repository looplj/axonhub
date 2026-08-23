package openai

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
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

	t.Run("round trip through UsageFromLLM", func(t *testing.T) {
		usage := &llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             lo.ToPtr(0.000005),
		}
		require.Equal(t, usage, UsageFromLLM(usage).ToLLMUsage())
	})
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
