package provider_quota

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
)

func TestCodexQuotaChecker_ReportedExhaustionKeepsWeeklyUsage(t *testing.T) {
	t.Parallel()

	// The reported rate_limit in #2608 exhausts the five-hour window only.
	body := []byte(`{
		"plan_type": "plus",
		"rate_limit": {
			"allowed": false,
			"limit_reached": true,
			"primary_window": {
				"limit_window_seconds": 18000,
				"reset_after_seconds": 65,
				"reset_at": 1791129646,
				"used_percent": 100
			},
			"secondary_window": {
				"limit_window_seconds": 604800,
				"reset_after_seconds": 529934,
				"reset_at": 1791659515,
				"used_percent": 36
			}
		}
	}`)

	quota, err := (&CodexQuotaChecker{}).parseResponse(body)
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)
	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.Equal(t, "exhausted", quota.Limits[0].Status)
	require.False(t, quota.Limits[0].Ready)
	require.Equal(t, float64(1), quota.Limits[0].UsageRatio)
	require.Equal(t, QuotaWindow7d, quota.Limits[1].Window)
	require.Equal(t, "available", quota.Limits[1].Status)
	require.True(t, quota.Limits[1].Ready)
	require.InDelta(t, 0.36, quota.Limits[1].UsageRatio, 1e-9)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestCodexQuotaChecker_DeniedRequestsPreserveEachWindow(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name             string
		allowed          bool
		limitReached     bool
		primaryPercent   float64
		primaryStatus    string
		secondaryPercent float64
		secondaryStatus  string
	}{
		{"weekly exhausted", false, true, 10, "available", 100, "exhausted"},
		{"warning window", false, true, 90, "warning", 100, "exhausted"},
		{"denied below limits", false, false, 0, "available", 36, "available"},
		{"limit reached flag", true, true, 25, "available", 36, "available"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body, err := json.Marshal(map[string]any{
				"rate_limit": map[string]any{
					"allowed":       tc.allowed,
					"limit_reached": tc.limitReached,
					"primary_window": map[string]any{
						"used_percent":         tc.primaryPercent,
						"limit_window_seconds": 18000,
					},
					"secondary_window": map[string]any{
						"used_percent":         tc.secondaryPercent,
						"limit_window_seconds": 604800,
					},
				},
			})
			require.NoError(t, err)

			quota, err := (&CodexQuotaChecker{}).parseResponse(body)
			require.NoError(t, err)
			require.Len(t, quota.Limits, 2)
			require.InDelta(t, tc.primaryPercent/100, quota.Limits[0].UsageRatio, 1e-9)
			require.Equal(t, tc.primaryStatus, quota.Limits[0].Status)
			require.Equal(t, IsReadyStatus(tc.primaryStatus), quota.Limits[0].Ready)
			require.InDelta(t, tc.secondaryPercent/100, quota.Limits[1].UsageRatio, 1e-9)
			require.Equal(t, tc.secondaryStatus, quota.Limits[1].Status)
			require.Equal(t, IsReadyStatus(tc.secondaryStatus), quota.Limits[1].Ready)
			require.Equal(t, "exhausted", quota.Status)
			require.False(t, quota.Ready)

			status, ready := EffectiveStatus(quota.Limits, providerquotastatus.Status(quota.Status), quota.Ready, QuotaLimitTypeToken)
			require.Equal(t, providerquotastatus.StatusExhausted, status)
			require.False(t, ready)
			state, reason := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
			require.Equal(t, RoutingExhausted, state)
			require.Empty(t, reason)
		})
	}
}

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
