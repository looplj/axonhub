package provider_quota

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexQuotaParser_ParseResponse(t *testing.T) {
	parser := &CodexQuotaParser{}

	t.Run("successful parse with complete data", func(t *testing.T) {
		body := []byte(`{
			"plan_type": "plus",
			"rate_limit": {
				"allowed": true,
				"limit_reached": false,
				"primary_window": {
					"used_percent": 35.5,
					"reset_at": 1737654000,
				"reset_after_seconds": 3600,
					"limit_window_seconds": 86400
				},
				"secondary_window": {
					"used_percent": 12.0,
					"reset_at": 1737740400,
					"reset_after_seconds": 90000,
			"limit_window_seconds": 604800
				}
			},
			"code_review_rate_limit": {
				"allowed": true,
			"limit_reached": false,
				"primary_window": {
					"used_percent": 5.0,
					"reset_at": 1737654000,
					"reset_after_seconds": 3600,
				"limit_window_seconds": 86400
				}
			}
		}`)

		quotaData, err := parser.ParseResponse(http.Header{}, body)
		require.NoError(t, err)

		assert.Equal(t, "ok", quotaData.Status)
		assert.Equal(t, "codex", quotaData.ProviderType)
		assert.Equal(t, "plus", quotaData.RawData["plan_type"])

		// Check rate_limit
		rateLimit, ok := quotaData.RawData["rate_limit"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, rateLimit["allowed"])
		assert.Equal(t, false, rateLimit["limit_reached"])

		// Check primary window
		primaryWindow, ok := rateLimit["primary_window"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 35.5, primaryWindow["used_percent"])
		assert.Equal(t, int64(1737654000), primaryWindow["reset_at"])
		assert.Equal(t, 3600, primaryWindow["reset_after_seconds"])
		assert.Equal(t, 86400, primaryWindow["limit_window_seconds"])

		// Check secondary window
		secondaryWindow, ok := rateLimit["secondary_window"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 12.0, secondaryWindow["used_percent"])

		// Check code_review_rate_limit
		codeReviewLimit, ok := quotaData.RawData["code_review_rate_limit"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, codeReviewLimit["allowed"])
	})

	t.Run("limit_reached status", func(t *testing.T) {
		body := []byte(`{
			"plan_type": "plus",
			"rate_limit": {
				"allowed": true,
				"limit_reached": true,
				"primary_window": {
					"used_percent": 100.0,
					"reset_at": 1737654000,
					"reset_after_seconds": 300,
					"limit_window_seconds": 86400
				}
			}
		}`)

		quotaData, err := parser.ParseResponse(http.Header{}, body)
		require.NoError(t, err)

		assert.Equal(t, "limit_reached", quotaData.Status)
		assert.Equal(t, "codex", quotaData.ProviderType)
	})

	t.Run("not_allowed status", func(t *testing.T) {
		body := []byte(`{
			"plan_type": "free",
			"rate_limit": {
				"allowed": false,
				"limit_reached": false
			}
		}`)

		quotaData, err := parser.ParseResponse(http.Header{}, body)
		require.NoError(t, err)

		assert.Equal(t, "not_allowed", quotaData.Status)
		assert.Equal(t, "codex", quotaData.ProviderType)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		body := []byte(`{invalid json}`)

		_, err := parser.ParseResponse(http.Header{}, body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse codex usage response")
	})

	t.Run("minimal response", func(t *testing.T) {
		body := []byte(`{
			"plan_type": "basic"
		}`)

		quotaData, err := parser.ParseResponse(http.Header{}, body)
		require.NoError(t, err)

		assert.Equal(t, "ok", quotaData.Status)
		assert.Equal(t, "codex", quotaData.ProviderType)
		assert.Equal(t, "basic", quotaData.RawData["plan_type"])
	})
}

func TestCodexQuotaParser_GetProviderType(t *testing.T) {
	parser := &CodexQuotaParser{}
	assert.Equal(t, "codex", parser.GetProviderType())
}
