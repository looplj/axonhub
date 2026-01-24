package provider_quota

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeCodeQuotaParser_ParseResponse(t *testing.T) {
	parser := &ClaudeCodeQuotaParser{}

	t.Run("successful parse with all headers", func(t *testing.T) {
		headers := http.Header{
			"Anthropic-Ratelimit-Unified-Status":               {"ok"},
			"Anthropic-Ratelimit-Unified-5h-Status":            {"ok"},
			"Anthropic-Ratelimit-Unified-5h-Reset":             {"1737654000"},
			"Anthropic-Ratelimit-Unified-5h-Utilization":       {"0.45"},
			"Anthropic-Ratelimit-Unified-7d-Status":            {"ok"},
			"Anthropic-Ratelimit-Unified-7d-Reset":             {"1738258800"},
			"Anthropic-Ratelimit-Unified-7d-Utilization":       {"0.23"},
			"Anthropic-Ratelimit-Unified-Overage-Status":       {"ok"},
			"Anthropic-Ratelimit-Unified-Overage-Reset":        {"0"},
			"Anthropic-Ratelimit-Unified-Overage-Utilization":  {"0.0"},
			"Anthropic-Ratelimit-Unified-Representative-Claim": {"5h"},
			"Anthropic-Ratelimit-Unified-Fallback":             {"overage"},
			"Anthropic-Ratelimit-Unified-Fallback-Percentage":  {"10.0"},
			"Anthropic-Ratelimit-Unified-Reset":                {"1737654000"},
		}

		quotaData, err := parser.ParseResponse(headers, nil)
		require.NoError(t, err)

		assert.Equal(t, "ok", quotaData.Status)
		assert.Equal(t, "claudecode", quotaData.ProviderType)

		// Check raw data
		assert.Equal(t, "ok", quotaData.RawData["unified_status"])
		assert.Equal(t, "5h", quotaData.RawData["representative_claim"])
		assert.Equal(t, "overage", quotaData.RawData["fallback"])
		assert.Equal(t, 10.0, quotaData.RawData["fallback_percentage"])
		assert.Equal(t, int64(1737654000), quotaData.RawData["reset"])

		// Check windows
		windows, ok := quotaData.RawData["windows"].(map[string]interface{})
		require.True(t, ok)

		// Check 5h window
		window5h, ok := windows["5h"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ok", window5h["status"])
		assert.Equal(t, int64(1737654000), window5h["reset"])
		assert.Equal(t, 0.45, window5h["utilization"])

		// Check 7d window
		window7d, ok := windows["7d"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ok", window7d["status"])
		assert.Equal(t, int64(1738258800), window7d["reset"])
		assert.Equal(t, 0.23, window7d["utilization"])

		// Check overage window
		windowOverage, ok := windows["overage"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ok", windowOverage["status"])
		assert.Equal(t, int64(0), windowOverage["reset"])
		assert.Equal(t, 0.0, windowOverage["utilization"])
	})

	t.Run("missing headers returns empty data", func(t *testing.T) {
		headers := http.Header{}

		quotaData, err := parser.ParseResponse(headers, nil)
		require.NoError(t, err)

		assert.Empty(t, quotaData.Status)
		assert.Empty(t, quotaData.ProviderType)
		assert.Empty(t, quotaData.RawData)
	})

	t.Run("invalid numeric values default to zero", func(t *testing.T) {
		headers := http.Header{
			"Anthropic-Ratelimit-Unified-Status":              {"throttled"},
			"Anthropic-Ratelimit-Unified-5h-Reset":            {"invalid"},
			"Anthropic-Ratelimit-Unified-5h-Utilization":      {"not-a-number"},
			"Anthropic-Ratelimit-Unified-Fallback-Percentage": {"bad"},
		}

		quotaData, err := parser.ParseResponse(headers, nil)
		require.NoError(t, err)

		assert.Equal(t, "throttled", quotaData.Status)
		assert.Equal(t, "claudecode", quotaData.ProviderType)

		// Invalid values should default to zero
		windows, ok := quotaData.RawData["windows"].(map[string]interface{})
		require.True(t, ok)

		window5h, ok := windows["5h"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, int64(0), window5h["reset"])
		assert.Equal(t, 0.0, window5h["utilization"])

		assert.Equal(t, 0.0, quotaData.RawData["fallback_percentage"])
	})

	t.Run("throttled status", func(t *testing.T) {
		headers := http.Header{
			"Anthropic-Ratelimit-Unified-Status":         {"throttled"},
			"Anthropic-Ratelimit-Unified-5h-Status":      {"throttled"},
			"Anthropic-Ratelimit-Unified-5h-Utilization": {"0.95"},
		}

		quotaData, err := parser.ParseResponse(headers, nil)
		require.NoError(t, err)

		assert.Equal(t, "throttled", quotaData.Status)
	})
}

func TestClaudeCodeQuotaParser_GetProviderType(t *testing.T) {
	parser := &ClaudeCodeQuotaParser{}
	assert.Equal(t, "claudecode", parser.GetProviderType())
}
