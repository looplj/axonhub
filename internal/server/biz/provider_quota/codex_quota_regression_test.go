package provider_quota

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCodexQuotaChecker_ExhaustedWindowWithoutUsageReportsZeroRemaining(t *testing.T) {
	t.Parallel()

	resetAt := time.Date(2099, 9, 13, 12, 0, 0, 0, time.UTC)
	body := []byte(`{
		"rate_limit": {
			"allowed": false,
			"primary_window": {
				"reset_at": ` + strconv.FormatInt(resetAt.Unix(), 10) + `,
				"limit_window_seconds": 604800
			}
		}
	}`)

	checker := &CodexQuotaChecker{}

	quota, err := checker.parseResponse(body)

	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaWindow7d, quota.Limits[0].Window)
	require.Equal(t, "exhausted", quota.Limits[0].Status)
	require.False(t, quota.Limits[0].Ready)
	require.Equal(t, float64(1), quota.Limits[0].UsageRatio)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.True(t, quota.NextResetAt.Equal(resetAt))
	require.True(t, quota.Limits[0].PeriodStart.Equal(resetAt.Add(-7*24*time.Hour)))
}
