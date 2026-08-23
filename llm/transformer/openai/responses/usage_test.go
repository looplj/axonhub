package responses

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestUsageCacheWriteTokensRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("responses usage to unified usage", func(t *testing.T) {
		t.Parallel()

		responsesUsage := &Usage{
			InputTokens:  100,
			OutputTokens: 20,
			TotalTokens:  120,
		}
		responsesUsage.InputTokenDetails.CacheWriteTokens = 80
		responsesUsage.InputTokenDetails.CachedTokens = 10

		usage := responsesUsage.ToUsage()

		require.NotNil(t, usage.PromptTokensDetails)
		require.Equal(t, int64(80), usage.PromptTokensDetails.WriteCachedTokens)
		require.Equal(t, int64(10), usage.PromptTokensDetails.CachedTokens)
	})

	t.Run("unified usage to responses usage", func(t *testing.T) {
		t.Parallel()

		responsesUsage := ConvertLLMUsageToResponsesUsage(&llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 20,
			TotalTokens:      120,
			PromptTokensDetails: &llm.PromptTokensDetails{
				WriteCachedTokens: 80,
				CachedTokens:      10,
			},
		})

		require.Equal(t, int64(80), responsesUsage.InputTokenDetails.CacheWriteTokens)
		require.Equal(t, int64(10), responsesUsage.InputTokenDetails.CachedTokens)

		body, err := json.Marshal(responsesUsage)
		require.NoError(t, err)
		require.JSONEq(t, `{
			"input_tokens": 100,
			"input_tokens_details": {
				"cache_write_tokens": 80,
				"cached_tokens": 10
			},
			"output_tokens": 20,
			"output_tokens_details": {
				"reasoning_tokens": 0
			},
			"total_tokens": 120
		}`, string(body))
	})
}

func TestConvertLLMUsageToResponsesUsage_IncludesCost(t *testing.T) {
	t.Parallel()

	responsesUsage := ConvertLLMUsageToResponsesUsage(&llm.Usage{
		PromptTokens:     100,
		CompletionTokens: 20,
		TotalTokens:      120,
		Cost:             lo.ToPtr(0.000005),
	})
	require.NotNil(t, responsesUsage.Cost)
	require.InDelta(t, 0.000005, *responsesUsage.Cost, 1e-12)

	body, err := json.Marshal(responsesUsage)
	require.NoError(t, err)
	require.Contains(t, string(body), `"cost":0.000005`)
}
