package provider_quota

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFilterLimitsForModel(t *testing.T) {
	limits := []QuotaLimitStatus{
		{
			Type:              QuotaLimitTypeToken,
			Status:            "available",
			UsageRatio:        0.1,
			Window:            "gemini_7d",
			AvailabilityGroup: antigravityPoolAvailabilityGroup("gemini"),
		},
		{
			Type:              QuotaLimitTypeToken,
			Status:            "exhausted",
			UsageRatio:        1.0,
			Window:            "claude_gpt_7d",
			AvailabilityGroup: antigravityPoolAvailabilityGroup("claude_gpt"),
		},
		{
			Type:       QuotaLimitTypeToken,
			Status:     "available",
			UsageRatio: 0.2,
			Window:     "global_rate",
		},
	}

	geminiFiltered := FilterLimitsForModel(limits, "gemini-3.8-flash")
	require.Len(t, geminiFiltered, 2)
	require.Equal(t, "gemini_7d", geminiFiltered[0].Window)
	require.Equal(t, "global_rate", geminiFiltered[1].Window)

	claudeFiltered := FilterLimitsForModel(limits, "claude-sonnet-5-5")
	require.Len(t, claudeFiltered, 2)
	require.Equal(t, "claude_gpt_7d", claudeFiltered[0].Window)
	require.Equal(t, "global_rate", claudeFiltered[1].Window)

	now := time.Now()
	// Gemini requests should see Gemini pool + global limits (both available -> RoutingOpen)
	geminiState, _ := EvaluateQuotaRouting(geminiFiltered, "available", QuotaLimitTypeToken, now)
	require.Equal(t, RoutingOpen, geminiState)

	// Claude requests should see Claude pool (exhausted -> RoutingExhausted)
	claudeState, _ := EvaluateQuotaRouting(claudeFiltered, "available", QuotaLimitTypeToken, now)
	require.Equal(t, RoutingExhausted, claudeState)
}
