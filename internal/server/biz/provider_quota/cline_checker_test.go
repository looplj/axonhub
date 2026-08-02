package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestParseClineUsageLimits_MapsFieldsIndependentlyAndKeepsFirstValidValue(t *testing.T) {
	firstPercent := 25.0
	duplicatePercent := 90.0
	negativePercent := -5.0
	overPercent := 150.0

	limits, meta := parseClineUsageLimits([]clineUsageLimit{
		{Type: "unknown", PercentUsed: &firstPercent, ResetsAt: "2026-07-14T13:00:00Z"},
		{Type: "five_hour", PercentUsed: &firstPercent},
		{Type: "five_hour", PercentUsed: &duplicatePercent, ResetsAt: "2026-07-14T15:00:00Z"},
		{Type: "weekly", PercentUsed: &negativePercent, ResetsAt: "not-a-time"},
		{Type: "monthly", PercentUsed: &overPercent, ResetsAt: "2026-08-01T11:13:17Z"},
	})

	require.Equal(t, clineUsageLimitsFetchStatusPartial, meta.Status)
	require.Equal(t, 5, meta.EntriesSeen)
	require.Equal(t, 4, meta.RecognizedEntries)
	require.Equal(t, 3, meta.UsableWindows)
	require.Equal(t, 5, meta.UsableFields)

	fiveHour := limits["last5h"]
	require.NotNil(t, fiveHour.UsageRatio)
	require.InDelta(t, 0.25, *fiveHour.UsageRatio, 0.000001)
	require.NotNil(t, fiveHour.NextResetAt)
	require.Equal(t, "2026-07-14T15:00:00Z", fiveHour.NextResetAt.Format(time.RFC3339))

	weekly := limits["last7d"]
	require.NotNil(t, weekly.UsageRatio)
	require.Zero(t, *weekly.UsageRatio)
	require.Nil(t, weekly.NextResetAt)

	monthly := limits["last30d"]
	require.NotNil(t, monthly.UsageRatio)
	require.Equal(t, 1.0, *monthly.UsageRatio)
	require.NotNil(t, monthly.NextResetAt)
}

func TestClineUsageLimitUnmarshal_TracksResetFieldState(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected clineResetState
	}{
		{name: "missing", raw: `{"type":"five_hour","percentUsed":0}`, expected: clineOfficialResetStateUnavailable},
		{name: "null", raw: `{"type":"five_hour","percentUsed":0,"resetsAt":null}`, expected: clineOfficialResetStateInactive},
		{name: "empty", raw: `{"type":"five_hour","percentUsed":0,"resetsAt":""}`, expected: clineOfficialResetStateInactive},
		{name: "active", raw: `{"type":"five_hour","percentUsed":0,"resetsAt":"2026-08-02T02:45:10Z"}`, expected: clineOfficialResetStateActive},
		{name: "invalid", raw: `{"type":"five_hour","percentUsed":0,"resetsAt":123}`, expected: clineOfficialResetStateInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var limit clineUsageLimit
			require.NoError(t, json.Unmarshal([]byte(tt.raw), &limit))
			require.Equal(t, tt.expected, limit.ResetFieldState)
		})
	}
}

func TestParseClineUsageLimits_EmptyResetOnlyMeansInactiveAtZeroUsage(t *testing.T) {
	zero := 0.0
	used := 25.0

	limits, meta := parseClineUsageLimits([]clineUsageLimit{
		{Type: "five_hour", PercentUsed: &zero, ResetFieldState: clineOfficialResetStateInactive},
		{Type: "weekly", PercentUsed: &used, ResetFieldState: clineOfficialResetStateInactive},
		{Type: "monthly", PercentUsed: &zero, ResetFieldState: clineOfficialResetStateUnavailable},
	})

	require.Equal(t, clineUsageLimitsFetchStatusPartial, meta.Status)
	require.Equal(t, clineOfficialResetStateInactive, limits["last5h"].ResetState)
	require.Equal(t, clineOfficialResetStateUnavailable, limits["last7d"].ResetState)
	require.Equal(t, clineOfficialResetStateUnavailable, limits["last30d"].ResetState)
}

func TestBuildClineQuotaData_OfficialValuesDriveStatusAndResetWhileCostRemainsExact(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	fiveHourRatio := 0.20
	weeklyRatio := 0.90
	monthlyRatio := 0.40
	fiveHourReset := time.Date(2026, 7, 14, 14, 0, 0, 0, time.UTC)
	weeklyReset := time.Date(2026, 7, 17, 2, 56, 47, 0, time.UTC)
	monthlyReset := time.Date(2026, 8, 1, 11, 13, 17, 0, time.UTC)

	quota := buildClineQuotaData(
		now,
		clineModelScopePassOnly,
		clineInferenceCapThreshold{
			Last5HoursUsageCostUSDPerUser: 100,
			Last7DaysUsageCostUSDPerUser:  100,
			Last30DaysUsageCostUSDPerUser: 100,
		},
		nil,
		nil,
		[]clineUsageItem{{CreatedAt: "2026-07-14T11:00:00Z", CostUSD: 50, CreditsUsed: 7, AIModelTypeName: "cline-pass"}},
		clineUsageFetchMeta{Pages: 1, ItemsSeen: 1},
		map[string]clineOfficialWindowLimit{
			"last5h":  {UsageRatio: &fiveHourRatio, NextResetAt: &fiveHourReset},
			"last7d":  {UsageRatio: &weeklyRatio, NextResetAt: &weeklyReset},
			"last30d": {UsageRatio: &monthlyRatio, NextResetAt: &monthlyReset},
		},
		clineUsageLimitsFetchMeta{
			Status:            clineUsageLimitsFetchStatusComplete,
			EntriesSeen:       3,
			RecognizedEntries: 3,
			UsableWindows:     3,
			UsableFields:      6,
		},
	)

	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, fiveHourReset, *quota.NextResetAt)
	require.Len(t, quota.Limits, 3)
	require.InDelta(t, 0.90, quota.Limits[1].UsageRatio, 0.000001)
	require.Equal(t, "warning", quota.Limits[1].Status)

	windows := quota.RawData["windows"].(map[string]any)
	weekly := windows["last7d"].(map[string]any)
	require.Equal(t, int64(50), weekly["used_cost_units"])
	require.Equal(t, int64(100), weekly["limit_cost_units"])
	require.Equal(t, int64(50), weekly["remaining_cost_units"])
	require.Equal(t, int64(7), weekly["credits_used"])
	require.InDelta(t, 0.90, weekly["usage_ratio"].(float64), 0.000001)
	require.InDelta(t, 90.0, weekly["usage_percent"].(float64), 0.000001)
	require.InDelta(t, 0.50, weekly["cost_usage_ratio"].(float64), 0.000001)
	require.InDelta(t, 50.0, weekly["cost_usage_percent"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceOfficialUsageLimits, weekly["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, weekly["reset_source"])
	require.Equal(t, weeklyReset.Format(time.RFC3339), weekly["next_reset_at"])
}

func TestBuildClineWindow_UsesOfficialBucketBoundaryAndFiltersDirectUsage(t *testing.T) {
	now := time.Date(2026, 8, 1, 16, 0, 0, 0, time.UTC)
	resetAt := time.Date(2026, 8, 31, 15, 12, 9, 0, time.UTC)
	usageRatio := 0.01

	window := buildClineWindow(
		now,
		"last30d",
		30*24*time.Hour,
		100,
		[]clineUsageItem{
			{CreatedAt: "2026-08-01T15:12:08.917369Z", CostUSD: 10, AIModelTypeName: "cline-pass"},
			{CreatedAt: "2026-08-01T15:30:00Z", CostUSD: 20, AIModelTypeName: "cline-pass"},
			{CreatedAt: "2026-08-01T15:40:00Z", CostUSD: 30, AIModelTypeName: "openai"},
			{CreatedAt: "2026-07-31T15:30:00Z", CostUSD: 40, AIModelTypeName: "cline-pass"},
		},
		false,
		clineOfficialWindowLimit{UsageRatio: &usageRatio, NextResetAt: &resetAt},
	)

	require.Equal(t, clineWindowStateActive, window.state)
	require.True(t, window.costAvailable)
	require.Equal(t, int64(30), window.usedUnits)
	require.Equal(t, 2, window.itemsCount)
	require.NotNil(t, window.windowStartAt)
	require.Equal(t, "2026-08-01T15:12:09Z", window.windowStartAt.Format(time.RFC3339))
	require.NotNil(t, window.costStartAt)
	require.Equal(t, "2026-08-01T15:12:08Z", window.costStartAt.Format(time.RFC3339))
	require.Equal(t, clineWindowSourceOfficialWindowLedger, window.costSource)
	require.Equal(t, resetAt, *window.nextResetAt)
}

func TestBuildClineWindow_ZeroPercentWithResetRemainsActive(t *testing.T) {
	now := time.Date(2026, 8, 1, 21, 46, 0, 0, time.UTC)
	resetAt := time.Date(2026, 8, 2, 2, 45, 10, 0, time.UTC)
	usageRatio := 0.0

	window := buildClineWindow(
		now,
		"last5h",
		5*time.Hour,
		1_000_000_000,
		[]clineUsageItem{{
			CreatedAt:       "2026-08-01T21:45:10.443078Z",
			CostUSD:         1232,
			AIModelTypeName: "cline-pass",
		}},
		false,
		clineOfficialWindowLimit{UsageRatio: &usageRatio, NextResetAt: &resetAt},
	)

	require.Equal(t, clineWindowStateActive, window.state)
	require.True(t, window.active)
	require.True(t, window.costAvailable)
	require.Equal(t, int64(1232), window.usedUnits)
	require.Equal(t, resetAt, *window.nextResetAt)
}

func TestBuildClineWindow_UsesOfficialBoundaryForEveryDuration(t *testing.T) {
	now := time.Date(2026, 8, 2, 1, 0, 0, 0, time.UTC)
	usageRatio := 0.1

	tests := []struct {
		name     string
		duration time.Duration
	}{
		{name: "five hour", duration: 5 * time.Hour},
		{name: "seven day", duration: 7 * 24 * time.Hour},
		{name: "thirty day", duration: 30 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAt := now.Add(tt.duration / 2)
			expectedStart := resetAt.Add(-tt.duration)
			window := buildClineWindow(
				now,
				tt.name,
				tt.duration,
				100,
				[]clineUsageItem{
					{CreatedAt: expectedStart.Add(-time.Minute).Format(time.RFC3339Nano), CostUSD: 90, AIModelTypeName: "cline-pass"},
					{CreatedAt: expectedStart.Add(time.Minute).Format(time.RFC3339Nano), CostUSD: 10, AIModelTypeName: "cline-pass"},
				},
				false,
				clineOfficialWindowLimit{UsageRatio: &usageRatio, NextResetAt: &resetAt},
			)

			require.True(t, window.costAvailable)
			require.Equal(t, int64(10), window.usedUnits)
			require.Equal(t, 1, window.itemsCount)
		})
	}
}

func TestBuildClineWindow_UnclassifiedCurrentUsageHidesCost(t *testing.T) {
	now := time.Date(2026, 8, 1, 16, 0, 0, 0, time.UTC)
	resetAt := now.Add(4 * time.Hour)
	usageRatio := 0.1

	window := buildClineWindow(
		now,
		"last5h",
		5*time.Hour,
		100,
		[]clineUsageItem{{CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), CostUSD: 10}},
		false,
		clineOfficialWindowLimit{UsageRatio: &usageRatio, NextResetAt: &resetAt},
	)

	require.Equal(t, clineWindowStateActive, window.state)
	require.False(t, window.costAvailable)
	require.Equal(t, clineWindowSourceUnavailable, window.costSource)
	require.Zero(t, window.usedUnits)
	require.InDelta(t, 0.1, *window.usageRatio, 0.000001)
}

func TestBuildClineWindow_OfficialInactiveWindowIgnoresRecentHistory(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 41, 30, 0, time.UTC)
	usageRatio := 0.0

	window := buildClineWindow(
		now,
		"last5h",
		5*time.Hour,
		100,
		[]clineUsageItem{{
			CreatedAt:       "2026-08-01T16:31:16Z",
			CostUSD:         80,
			AIModelTypeName: "cline-pass",
		}},
		false,
		clineOfficialWindowLimit{
			UsageRatio: &usageRatio,
			ResetState: clineOfficialResetStateInactive,
		},
	)

	require.Equal(t, clineWindowStateInactive, window.state)
	require.False(t, window.active)
	require.True(t, window.costAvailable)
	require.Zero(t, window.itemsCount)
	require.Zero(t, window.usedUnits)
	require.Nil(t, window.nextResetAt)
	require.Equal(t, clineWindowSourceOfficialNoActiveWindow, window.costSource)
	require.NotNil(t, window.costUsageRatio)
	require.Zero(t, *window.costUsageRatio)
}

func TestBuildClineWindow_StaleOfficialResetDoesNotFallBackToRollingEstimate(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	staleReset := now.Add(-2 * time.Hour)
	usageRatio := 0.5

	window := buildClineWindow(
		now,
		"last5h",
		5*time.Hour,
		100,
		[]clineUsageItem{{
			CreatedAt:       now.Add(-time.Hour).Format(time.RFC3339),
			CostUSD:         50,
			AIModelTypeName: "cline-pass",
		}},
		false,
		clineOfficialWindowLimit{UsageRatio: &usageRatio, NextResetAt: &staleReset},
	)

	require.Equal(t, clineWindowStateInvalid, window.state)
	require.False(t, window.costAvailable)
	require.Zero(t, window.usedUnits)
	require.Nil(t, window.nextResetAt)
	require.Equal(t, clineWindowSourceUnavailable, window.costSource)
	require.Equal(t, clineWindowSourceUnavailable, window.resetSource)
	require.InDelta(t, 0.5, *window.usageRatio, 0.000001)
}

func TestBuildClineWindow_TruncatedUsageHistoryKeepsOfficialStatusButHidesCost(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	resetAt := now.Add(4 * time.Hour)
	usageRatio := 0.25

	window := buildClineWindow(
		now,
		"last5h",
		5*time.Hour,
		100,
		[]clineUsageItem{{
			CreatedAt:       now.Add(-time.Hour).Format(time.RFC3339),
			CostUSD:         25,
			AIModelTypeName: "cline-pass",
		}},
		true,
		clineOfficialWindowLimit{UsageRatio: &usageRatio, NextResetAt: &resetAt},
	)

	require.Equal(t, clineWindowStateActive, window.state)
	require.False(t, window.costAvailable)
	require.Equal(t, clineWindowSourceUnavailable, window.costSource)
	require.InDelta(t, 0.25, *window.usageRatio, 0.000001)
	require.Equal(t, resetAt, *window.nextResetAt)
}

func TestCline_CheckQuota_HappyPathPassOnly(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 30, 0, 0, time.UTC)
	requestCount := 0

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Accept"))

			switch requestCount {
			case 1:
				require.Equal(t, "GET", req.Method)
				require.Equal(t, "/api/v1/users/me", req.URL.Path)
				return jsonResponse(http.StatusOK, `{"data":{"id":"user_test","organizations":[]}}`), nil
			case 2:
				require.Equal(t, "GET", req.Method)
				require.Equal(t, "/api/v1/plans", req.URL.Path)
				return jsonResponse(http.StatusOK, `{
					"data": [{
						"type": "individual",
						"interval": "Monthly",
						"isActive": true,
						"entitlements": {
							"cline_pass": {
								"enabled": true,
								"inferenceCapThreshold": {
									"last5HoursUsageCostUSDPerUser": 1000000000,
									"last7daysUsageCostUSDPerUser": 2500000000,
									"last30daysUsageCostUSDPerUser": 5000000000
								}
							}
						}
					}]
				}`), nil
			case 3:
				require.Equal(t, "GET", req.Method)
				require.Equal(t, "/api/v1/users/user_test/balance", req.URL.Path)
				return jsonResponse(http.StatusOK, `{"data":{"balance":497582}}`), nil
			case 4:
				require.Equal(t, "GET", req.Method)
				require.Equal(t, clineUsageLimitsPath, req.URL.Path)
				return jsonResponse(http.StatusOK, `{
					"data": {
						"limits": [
							{"type":"five_hour","percentUsed":10,"resetsAt":"2026-07-07T14:00:00Z"},
							{"type":"weekly","percentUsed":79,"resetsAt":"2026-07-14T10:18:10Z"},
							{"type":"monthly","percentUsed":49,"resetsAt":"2026-08-01T11:13:17Z"}
						]
					}
				}`), nil
			case 5:
				require.Equal(t, "GET", req.Method)
				require.Equal(t, "/api/v1/users/user_test/usages", req.URL.Path)
				require.Equal(t, "100", req.URL.Query().Get("limit"))
				return jsonResponse(http.StatusOK, `{
					"data": {
						"items": [
							{"createdAt":"2026-07-07T10:18:10Z","costUsd":462,"creditsUsed":0,"aiModelTypeName":"cline-pass"},
							{"createdAt":"2026-07-02T10:31:31Z","costUsd":497184013,"creditsUsed":0,"aiModelTypeName":"cline-pass"}
						]
					}
				}`), nil
			default:
				t.Fatalf("unexpected Cline quota request %d to %s", requestCount, req.URL.String())
				return nil, nil
			}
		}),
	})

	checker := NewClineQuotaChecker(httpClient)
	checker.now = func() time.Time { return now }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:            channel.TypeCline,
		BaseURL:         "https://api.cline.bot/v1",
		SupportedModels: []string{"cline-pass/deepseek-v4-flash", "cline-pass/qwen3.7-plus"},
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "cline", quota.ProviderType)
	require.Len(t, quota.Limits, 3)
	require.Equal(t, 5, requestCount)

	raw := quota.RawData
	require.Equal(t, "cline_pass_only", raw["model_scope"])
	require.Equal(t, "cline_pass_windows", raw["status_basis"])
	require.NotContains(t, raw, "user_id")
	require.NotContains(t, raw, "email")

	windows := raw["windows"].(map[string]any)
	last7d := windows["last7d"].(map[string]any)
	require.InDelta(t, 0.79, last7d["usage_ratio"].(float64), 0.000001)
	require.InDelta(t, 79.0, last7d["usage_percent"].(float64), 0.0001)
	require.Equal(t, int64(462), last7d["used_cost_units"])
	require.InDelta(t, 0.0000001848, last7d["cost_usage_ratio"].(float64), 0.0000000001)
	require.InDelta(t, 0.00001848, last7d["cost_usage_percent"].(float64), 0.00000001)
	require.Equal(t, "2026-07-14T10:18:10Z", last7d["next_reset_at"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, last7d["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, last7d["reset_source"])

	usageLimitsFetch := raw["usage_limits_fetch"].(map[string]any)
	require.Equal(t, clineUsageLimitsFetchStatusComplete, usageLimitsFetch["status"])
	require.Equal(t, 3, usageLimitsFetch["usable_windows"])
	require.Equal(t, 6, usageLimitsFetch["usable_fields"])
}

func TestCline_CheckQuota_UsageLimitsNotFoundMarksPassUnavailable(t *testing.T) {
	for _, tt := range []struct {
		name                string
		models              []string
		expectedStatus      string
		expectedReady       bool
		expectedStatusBasis string
	}{
		{
			name:                "pass only",
			models:              []string{"cline-pass/deepseek-v4-flash"},
			expectedStatus:      "exhausted",
			expectedReady:       false,
			expectedStatusBasis: "cline_pass_unavailable",
		},
		{
			name:                "mixed pass and direct",
			models:              []string{"cline-pass/deepseek-v4-flash", "zai/glm-5.2"},
			expectedStatus:      "warning",
			expectedReady:       true,
			expectedStatusBasis: "cline_pass_unavailable_mixed_pool",
		},
		{
			name:                "unknown model scope",
			expectedStatus:      "warning",
			expectedReady:       true,
			expectedStatusBasis: "cline_pass_unavailable",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			httpClient := httpclient.NewHttpClientWithClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requestCount++
					switch requestCount {
					case 1:
						require.Equal(t, "/api/v1/users/me", req.URL.Path)
						return jsonResponse(http.StatusOK, `{"data":{"id":"user_test"}}`), nil
					case 2:
						require.Equal(t, "/api/v1/plans", req.URL.Path)
						return jsonResponse(http.StatusOK, `{"data":[{"type":"individual","interval":"Monthly","isActive":true,"entitlements":{"cline_pass":{"enabled":true,"inferenceCapThreshold":{"last5HoursUsageCostUSDPerUser":100,"last7daysUsageCostUSDPerUser":200,"last30daysUsageCostUSDPerUser":400}}}}]}`), nil
					case 3:
						require.Equal(t, "/api/v1/users/user_test/balance", req.URL.Path)
						return jsonResponse(http.StatusOK, `{"data":{"balance":497582}}`), nil
					case 4:
						require.Equal(t, clineUsageLimitsPath, req.URL.Path)
						return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
					default:
						t.Fatalf("Cline Pass unavailable result made unexpected request %d to %s", requestCount, req.URL.Path)
						return nil, nil
					}
				}),
			})

			checker := NewClineQuotaChecker(httpClient)
			quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
				Type:            channel.TypeCline,
				BaseURL:         "https://api.cline.bot/v1",
				SupportedModels: tt.models,
				Credentials:     objects.ChannelCredentials{APIKey: "test-api-key"},
			})

			require.NoError(t, err)
			require.Equal(t, 4, requestCount)
			require.Equal(t, tt.expectedStatus, quota.Status)
			require.Equal(t, tt.expectedReady, quota.Ready)
			require.Nil(t, quota.NextResetAt)
			require.Empty(t, quota.Limits)
			require.Equal(t, tt.expectedStatusBasis, quota.RawData["status_basis"])
			require.Equal(t, "unavailable", quota.RawData["pass_state"])
			require.NotContains(t, quota.RawData, "windows")
			require.NotContains(t, quota.RawData, "usage_fetch")

			balance := quota.RawData["balance"].(map[string]any)
			require.Equal(t, int64(497582), balance["raw_balance"])
			usageLimitsFetch := quota.RawData["usage_limits_fetch"].(map[string]any)
			require.Equal(t, "cline_pass_unavailable", usageLimitsFetch["status"])
		})
	}
}

func TestCline_CheckQuota_UserIdentityNotFoundRemainsFailure(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v1/users/me", req.URL.Path)
		return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
	})})
	checker := NewClineQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:            channel.TypeCline,
		BaseURL:         "https://api.cline.bot/v1",
		SupportedModels: []string{"cline-pass/deepseek-v4-flash"},
		Credentials:     objects.ChannelCredentials{APIKey: "test-api-key"},
	})

	require.Error(t, err)
	require.Empty(t, quota.RawData)
	require.Contains(t, err.Error(), "failed to read Cline user identity")
	require.Contains(t, err.Error(), "HTTP 404 Not Found")
}

func TestCline_CheckQuota_UsageLimitsNonNotFoundErrorsRemainFailures(t *testing.T) {
	for _, statusCode := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(statusCode, `{"error":"upstream failure"}`), nil
			})})
			checker := NewClineQuotaChecker(httpClient)

			limits, meta, err := checker.fetchUsageLimits(context.Background(), httpClient, "https://api.cline.bot/v1", "test-api-key")

			require.Error(t, err)
			require.Nil(t, limits)
			require.Equal(t, clineUsageLimitsFetchStatusUnavailable, meta.Status)
			require.Contains(t, err.Error(), fmt.Sprintf("HTTP %d", statusCode))
		})
	}
}

func TestCline_CheckQuota_UsageLimitsFailureReturnsSafeError(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	requestCount := 0
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return jsonResponse(http.StatusOK, `{"data":{"id":"user_sensitive_123"}}`), nil
			case 2:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"individual","interval":"Monthly","isActive":true,"entitlements":{"cline_pass":{"enabled":true,"inferenceCapThreshold":{"last5HoursUsageCostUSDPerUser":100,"last7daysUsageCostUSDPerUser":200,"last30daysUsageCostUSDPerUser":400}}}}]}`), nil
			case 3:
				return jsonResponse(http.StatusOK, `{"data":{"balance":497582}}`), nil
			case 4:
				require.Equal(t, clineUsageLimitsPath, req.URL.Path)
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Status:     "503 Service Unavailable",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"api_key":"sk-sensitive-test-key","email":"person@example.test","user_id":"user_sensitive_123"}`)),
				}, nil
			case 5:
				return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-07T11:00:00Z","costUsd":50,"creditsUsed":7}]}}`), nil
			default:
				t.Fatalf("unexpected Cline quota request %d", requestCount)
				return nil, nil
			}
		}),
	})

	checker := NewClineQuotaChecker(httpClient)
	checker.now = func() time.Time { return now }
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:            channel.TypeCline,
		BaseURL:         "https://api.cline.bot/v1",
		SupportedModels: []string{"cline-pass/test"},
		Credentials: objects.ChannelCredentials{
			APIKey: "sk-sensitive-test-key",
		},
	})

	require.Error(t, err)
	require.Empty(t, quota.RawData)
	require.Equal(t, 4, requestCount)
	require.Contains(t, err.Error(), "failed to read Cline usage limits")
	assertClineErrorOmitsSensitiveValues(t, err.Error())
}

func TestCline_CheckQuota_PartialUsageLimitsFallbackIsPerFieldAndPerWindow(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	requestCount := 0
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return jsonResponse(http.StatusOK, `{"data":{"id":"user_test"}}`), nil
			case 2:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"individual","interval":"Monthly","isActive":true,"entitlements":{"cline_pass":{"enabled":true,"inferenceCapThreshold":{"last5HoursUsageCostUSDPerUser":100,"last7daysUsageCostUSDPerUser":200,"last30daysUsageCostUSDPerUser":400}}}}]}`), nil
			case 3:
				return jsonResponse(http.StatusOK, `{"data":{"balance":1000}}`), nil
			case 4:
				return jsonResponse(http.StatusOK, `{"data":{"limits":[{"type":"five_hour","percentUsed":80},{"type":"weekly","resetsAt":"2026-07-17T02:56:47Z"},{"type":"unrecognized","percentUsed":99}]}}`), nil
			case 5:
				return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-14T11:00:00Z","costUsd":50,"creditsUsed":3,"aiModelTypeName":"cline-pass"}]}}`), nil
			default:
				t.Fatalf("unexpected request %d", requestCount)
				return nil, nil
			}
		}),
	})

	checker := NewClineQuotaChecker(httpClient)
	checker.now = func() time.Time { return now }
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:            channel.TypeCline,
		BaseURL:         "https://api.cline.bot/v1",
		SupportedModels: []string{"cline-pass/test"},
		Credentials:     objects.ChannelCredentials{APIKey: "test-api-key"},
	})

	require.NoError(t, err)
	windows := quota.RawData["windows"].(map[string]any)
	fiveHour := windows["last5h"].(map[string]any)
	weekly := windows["last7d"].(map[string]any)
	monthly := windows["last30d"].(map[string]any)

	require.InDelta(t, 0.80, fiveHour["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceOfficialUsageLimits, fiveHour["usage_source"])
	require.Equal(t, clineWindowSourceUnavailable, fiveHour["reset_source"])
	require.Equal(t, clineWindowSourceUnavailable, fiveHour["cost_source"])
	require.NotContains(t, fiveHour, "next_reset_at")
	require.NotContains(t, fiveHour, "used_cost_units")

	require.InDelta(t, 0.25, weekly["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceOfficialWindowLedger, weekly["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, weekly["reset_source"])
	require.Equal(t, clineWindowSourceOfficialWindowLedger, weekly["cost_source"])
	require.Equal(t, "2026-07-17T02:56:47Z", weekly["next_reset_at"])

	require.NotContains(t, monthly, "usage_ratio")
	require.Equal(t, clineWindowSourceUnavailable, monthly["usage_source"])
	require.Equal(t, clineWindowSourceUnavailable, monthly["reset_source"])
	require.Equal(t, clineWindowSourceUnavailable, monthly["cost_source"])

	fetch := quota.RawData["usage_limits_fetch"].(map[string]any)
	require.Equal(t, clineUsageLimitsFetchStatusPartial, fetch["status"])
	require.Equal(t, 2, fetch["recognized_entries"])
	require.Equal(t, 2, fetch["usable_windows"])
	require.Equal(t, 2, fetch["usable_fields"])
}

func TestCline_CheckQuota_MalformedUsageLimitFieldsPreserveOtherOfficialValues(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	requestCount := 0
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return jsonResponse(http.StatusOK, `{"data":{"id":"user_test"}}`), nil
			case 2:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"individual","interval":"Monthly","isActive":true,"entitlements":{"cline_pass":{"enabled":true,"inferenceCapThreshold":{"last5HoursUsageCostUSDPerUser":100,"last7daysUsageCostUSDPerUser":200,"last30daysUsageCostUSDPerUser":400}}}}]}`), nil
			case 3:
				return jsonResponse(http.StatusOK, `{"data":{"balance":1000}}`), nil
			case 4:
				return jsonResponse(http.StatusOK, `{"data":{"limits":[{"type":"five_hour","percentUsed":"unknown","resetsAt":"2026-07-14T15:00:00Z"},{"type":"weekly","percentUsed":90,"resetsAt":123},{"type":"monthly","percentUsed":40,"resetsAt":"2026-08-01T11:13:17Z"},false]}}`), nil
			case 5:
				return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-14T11:00:00Z","costUsd":50,"creditsUsed":3,"aiModelTypeName":"cline-pass"}]}}`), nil
			default:
				t.Fatalf("unexpected request %d", requestCount)
				return nil, nil
			}
		}),
	})

	checker := NewClineQuotaChecker(httpClient)
	checker.now = func() time.Time { return now }
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:            channel.TypeCline,
		BaseURL:         "https://api.cline.bot/v1",
		SupportedModels: []string{"cline-pass/test"},
		Credentials:     objects.ChannelCredentials{APIKey: "test-api-key"},
	})

	require.NoError(t, err)
	windows := quota.RawData["windows"].(map[string]any)
	fiveHour := windows["last5h"].(map[string]any)
	weekly := windows["last7d"].(map[string]any)
	monthly := windows["last30d"].(map[string]any)

	require.InDelta(t, 0.5, fiveHour["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceOfficialWindowLedger, fiveHour["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, fiveHour["reset_source"])
	require.Equal(t, clineWindowSourceOfficialWindowLedger, fiveHour["cost_source"])
	require.Equal(t, "2026-07-14T15:00:00Z", fiveHour["next_reset_at"])

	require.InDelta(t, 0.9, weekly["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceOfficialUsageLimits, weekly["usage_source"])
	require.Equal(t, clineWindowSourceUnavailable, weekly["reset_source"])
	require.Equal(t, clineWindowSourceUnavailable, weekly["cost_source"])
	require.NotContains(t, weekly, "next_reset_at")

	require.InDelta(t, 0.4, monthly["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceOfficialUsageLimits, monthly["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, monthly["reset_source"])
	require.Equal(t, clineWindowSourceOfficialWindowLedger, monthly["cost_source"])
	require.Equal(t, "2026-08-01T11:13:17Z", monthly["next_reset_at"])

	fetch := quota.RawData["usage_limits_fetch"].(map[string]any)
	require.Equal(t, clineUsageLimitsFetchStatusPartial, fetch["status"])
	require.Equal(t, 4, fetch["entries_seen"])
	require.Equal(t, 3, fetch["recognized_entries"])
	require.Equal(t, 3, fetch["usable_windows"])
	require.Equal(t, 4, fetch["usable_fields"])
}

func TestCline_CheckQuota_DirectOnlySkipsPassUsageLimitsAndHistory(t *testing.T) {
	requestCount := 0
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return jsonResponse(http.StatusOK, `{"data":{"id":"user_test"}}`), nil
			case 2:
				return jsonResponse(http.StatusOK, `{"data":[{"type":"individual","interval":"Monthly","isActive":true,"entitlements":{}}]}`), nil
			case 3:
				return jsonResponse(http.StatusOK, `{"data":{"balance":497582}}`), nil
			default:
				t.Fatalf("direct-only quota made unexpected request %d to %s", requestCount, req.URL.Path)
				return nil, nil
			}
		}),
	})

	checker := NewClineQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:            channel.TypeCline,
		BaseURL:         "https://api.cline.bot/v1",
		SupportedModels: []string{"anthropic/claude-sonnet-5"},
		Credentials:     objects.ChannelCredentials{APIKey: "test-api-key"},
	})

	require.NoError(t, err)
	require.Equal(t, 3, requestCount)
	require.Equal(t, "direct_only", quota.RawData["model_scope"])
	require.NotContains(t, quota.RawData, "usage_limits_fetch")
	require.Empty(t, quota.Limits)
}

func TestCline_CheckQuota_WarningAtEightyPercent(t *testing.T) {
	usageRatio := 0.8
	quota := buildClineQuotaData(
		time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
		clineModelScopePassOnly,
		clineInferenceCapThreshold{Last5HoursUsageCostUSDPerUser: 100, Last7DaysUsageCostUSDPerUser: 1000, Last30DaysUsageCostUSDPerUser: 2000},
		nil,
		nil,
		nil,
		clineUsageFetchMeta{},
		map[string]clineOfficialWindowLimit{
			"last5h": {UsageRatio: &usageRatio, ResetState: clineOfficialResetStateInactive},
		},
		clineUsageLimitsFetchMeta{Status: clineUsageLimitsFetchStatusPartial},
	)

	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "cline_pass_windows", quota.RawData["status_basis"])
}

func TestCline_CheckQuota_ExhaustedWhenPassOnly(t *testing.T) {
	usageRatio := 1.0
	quota := buildClineQuotaData(
		time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
		clineModelScopePassOnly,
		clineInferenceCapThreshold{Last5HoursUsageCostUSDPerUser: 100, Last7DaysUsageCostUSDPerUser: 1000, Last30DaysUsageCostUSDPerUser: 2000},
		nil,
		nil,
		nil,
		clineUsageFetchMeta{},
		map[string]clineOfficialWindowLimit{
			"last5h": {UsageRatio: &usageRatio, ResetState: clineOfficialResetStateInactive},
		},
		clineUsageLimitsFetchMeta{Status: clineUsageLimitsFetchStatusPartial},
	)

	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestCline_CheckQuota_MixedScopeDoesNotExhaustWholeChannelFromPassPool(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	officialRatio := 1.0
	zeroRatio := 0.0
	fiveHourReset := now.Add(4 * time.Hour)
	quota := buildClineQuotaData(
		now,
		clineModelScopeMixed,
		clineInferenceCapThreshold{
			Last5HoursUsageCostUSDPerUser: 100,
			Last7DaysUsageCostUSDPerUser:  1000,
			Last30DaysUsageCostUSDPerUser: 2000,
		},
		nil,
		nil,
		[]clineUsageItem{{CreatedAt: "2026-07-07T11:00:00Z", CostUSD: 1, AIModelTypeName: "cline-pass"}},
		clineUsageFetchMeta{Pages: 1, ItemsSeen: 1},
		map[string]clineOfficialWindowLimit{
			"last5h":  {UsageRatio: &officialRatio, NextResetAt: &fiveHourReset},
			"last7d":  {UsageRatio: &zeroRatio, ResetState: clineOfficialResetStateInactive},
			"last30d": {UsageRatio: &zeroRatio, ResetState: clineOfficialResetStateInactive},
		},
		clineUsageLimitsFetchMeta{
			Status:            clineUsageLimitsFetchStatusComplete,
			EntriesSeen:       3,
			RecognizedEntries: 3,
			UsableWindows:     3,
			UsableFields:      6,
		},
	)

	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "mixed_pool_pass_exhausted", quota.RawData["status_basis"])
	require.NotEmpty(t, quota.Limits)
	for _, limit := range quota.Limits {
		if limit.Type != QuotaLimitTypeToken {
			continue
		}
		require.NotEqual(t, "exhausted", limit.Status)
		require.True(t, limit.Ready)
	}

	windows := quota.RawData["windows"].(map[string]any)
	fiveHour := windows["last5h"].(map[string]any)
	require.InDelta(t, 1.0, fiveHour["usage_ratio"].(float64), 0.000001)
	require.InDelta(t, 0.01, fiveHour["cost_usage_ratio"].(float64), 0.000001)
}

func TestCline_CheckQuota_DirectOnlyUsesBalanceInformationally(t *testing.T) {
	balance := int64(497582)
	quota := buildClineDirectOnlyQuota(&balance, []map[string]any{{"type": "individual", "interval": "Monthly"}})

	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "direct_only", quota.RawData["model_scope"])
	require.Equal(t, "direct_credit_balance_informational", quota.RawData["status_basis"])
	require.Empty(t, quota.Limits)
}

func TestCline_CheckQuota_MissingCredentials(t *testing.T) {
	checker := NewClineQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request without credentials")
		return nil, nil
	})}))

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{Type: channel.TypeCline})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no API key")
}

func TestCline_GetJSON_UpstreamHTTPErrorOmitsSensitiveBody(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"email":"person@example.test",
				"api_key":"sk-sensitive-test-key",
				"user_id":"user_sensitive_123",
				"account_id":"acct_sensitive_456",
				"generation_id":"gen_sensitive_789"
			}`)),
		}, nil
	})})

	checker := NewClineQuotaChecker(httpClient)
	err := checker.getJSON(context.Background(), httpClient, "https://api.cline.bot", "/api/v1/users/me", nil, "test-api-key", &clineEnvelope[clineMeData]{})
	require.Error(t, err)

	message := err.Error()
	if strings.Contains(message, "person@example.test") ||
		strings.Contains(message, "sk-sensitive-test-key") ||
		strings.Contains(message, "user_sensitive_123") ||
		strings.Contains(message, "acct_sensitive_456") ||
		strings.Contains(message, "gen_sensitive_789") {
		t.Fatalf("error leaked raw upstream body")
	}
	require.Contains(t, message, "HTTP 403")
	require.Contains(t, message, "Forbidden")
}

func TestCline_GetJSON_NonHTTPFailuresOmitSensitiveValues(t *testing.T) {
	tests := []struct {
		name      string
		transport http.RoundTripper
		out       any
	}{
		{
			name: "transport error",
			transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("dial to %s failed with api key sk-sensitive-test-key for person@example.test account acct_sensitive_456 generation gen_sensitive_789 payment_id pay_sensitive_000", req.URL.String())
			}),
			out: &clineEnvelope[clineMeData]{},
		},
		{
			name: "read error",
			transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: erringReadCloser{err: fmt.Errorf("read failed for person@example.test sk-sensitive-test-key user_sensitive_123 acct_sensitive_456 gen_sensitive_789 payment_id pay_sensitive_000")}}, nil
			}),
			out: &clineEnvelope[clineMeData]{},
		},
		{
			name: "decode error",
			transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"data": {"id": "user_sensitive_123", "email": "person@example.test", "api_key": "sk-sensitive-test-key", "account_id": "acct_sensitive_456", "generation_id": "gen_sensitive_789", "payment_id": "pay_sensitive_000"}`), nil
			}),
			out: &clineEnvelope[clineMeData]{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: tt.transport})
			checker := NewClineQuotaChecker(httpClient)
			err := checker.getJSON(context.Background(), httpClient, "https://api.cline.bot", "/api/v1/users/user_sensitive_123/usages", map[string][]string{"cursor": {"cursor_sensitive_456"}}, "sk-sensitive-test-key", tt.out)
			require.Error(t, err)
			assertClineErrorOmitsSensitiveValues(t, err.Error())
		})
	}
}

func TestCline_CheckQuota_UsageTransportErrorOmitsSensitiveValues(t *testing.T) {
	requestCount := 0
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonResponse(http.StatusOK, `{"data":{"id":"user_sensitive_123","organizations":[]}}`), nil
		case 2:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"individual","interval":"Monthly","isActive":true,"entitlements":{"cline_pass":{"enabled":true,"inferenceCapThreshold":{"last5HoursUsageCostUSDPerUser":100,"last7daysUsageCostUSDPerUser":100,"last30daysUsageCostUSDPerUser":100}}}}]}`), nil
		case 3:
			return jsonResponse(http.StatusOK, `{"data":{"balance":497582}}`), nil
		case 4:
			return jsonResponse(http.StatusOK, `{"data":{"limits":[{"type":"five_hour","percentUsed":1,"resetsAt":"2026-07-07T16:00:00Z"},{"type":"weekly","percentUsed":1,"resetsAt":"2026-07-14T11:00:00Z"},{"type":"monthly","percentUsed":1,"resetsAt":"2026-08-06T11:00:00Z"}]}}`), nil
		case 5:
			return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-07T11:00:00Z","costUsd":1,"aiModelTypeName":"cline-pass"}],"nextToken":"cursor_sensitive_456"}}`), nil
		case 6:
			return nil, fmt.Errorf("dial to %s failed with api key sk-sensitive-test-key for person@example.test account acct_sensitive_456 generation gen_sensitive_789 payment_id pay_sensitive_000", req.URL.String())
		default:
			t.Fatalf("unexpected Cline quota request %d", requestCount)
			return nil, nil
		}
	})})

	checker := NewClineQuotaChecker(httpClient)
	checker.now = func() time.Time { return time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC) }

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:            channel.TypeCline,
		BaseURL:         "https://api.cline.bot/v1",
		SupportedModels: []string{"cline-pass/deepseek-v4-flash"},
		Credentials: objects.ChannelCredentials{
			APIKey: "sk-sensitive-test-key",
		},
	})
	require.Error(t, err)
	assertClineErrorOmitsSensitiveValues(t, err.Error())
}

func TestCline_CheckQuota_APIKeysFallbackSkipsBlankEntries(t *testing.T) {
	ch := &ent.Channel{Credentials: objects.ChannelCredentials{APIKeys: []string{"", " fallback-key "}}}
	require.Equal(t, "fallback-key", clineAPIKey(ch))
}

func TestCline_SupportsChannel(t *testing.T) {
	checker := NewClineQuotaChecker(nil)
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeCline}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai}))
}

func TestBuildClineQuotaURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{"empty base URL", "", "/api/v1/users/me", "https://api.cline.bot/api/v1/users/me"},
		{"chat completion path stripped", "https://api.cline.bot/v1", "/api/v1/plans", "https://api.cline.bot/api/v1/plans"},
		{"http upgraded", "http://api.cline.bot/v1", "/api/v1/plans", "https://api.cline.bot/api/v1/plans"},
		{"invalid URL falls back", "://invalid", "/api/v1/plans", "https://api.cline.bot/api/v1/plans"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, buildClineQuotaURL(tt.baseURL, tt.path, nil))
		})
	}
}

func TestCline_FetchUsageItems_MultiplePages(t *testing.T) {
	requestCount := 0
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			require.Empty(t, req.URL.Query().Get("cursor"))
			return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-07T11:00:00Z","costUsd":1}],"nextToken":"cursor_2"}}`), nil
		case 2:
			require.Equal(t, "cursor_2", req.URL.Query().Get("cursor"))
			return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-06-01T11:00:00Z","costUsd":2}]}}`), nil
		default:
			t.Fatalf("unexpected page request %d", requestCount)
			return nil, nil
		}
	})})

	checker := NewClineQuotaChecker(httpClient)
	checker.now = func() time.Time { return time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC) }

	items, meta, err := checker.fetchUsageItems(context.Background(), httpClient, "https://api.cline.bot/v1", "user_test", "key")
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, 2, meta.Pages)
	require.Equal(t, 2, meta.ItemsSeen)
	require.False(t, meta.Truncated)
}

func TestCline_FetchUsageItems_ReturnsTruncatedItemsWhenPaginationDoesNotReachBoundary(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-07T11:00:00Z","costUsd":1}],"nextToken":"still_more"}}`), nil
	})})

	checker := NewClineQuotaChecker(httpClient)
	checker.now = func() time.Time { return time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC) }

	items, meta, err := checker.fetchUsageItems(context.Background(), httpClient, "https://api.cline.bot/v1", "user_test", "key")
	require.NoError(t, err)
	require.Len(t, items, clineMaxUsagePages)
	require.True(t, meta.Truncated)
	require.Equal(t, clineMaxUsagePages, meta.Pages)
	require.Equal(t, clineMaxUsagePages, meta.ItemsSeen)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type erringReadCloser struct {
	err error
}

func (r erringReadCloser) Read(_ []byte) (int, error) {
	return 0, r.err
}

func (r erringReadCloser) Close() error {
	return nil
}

func assertClineErrorOmitsSensitiveValues(t *testing.T, message string) {
	t.Helper()
	sensitiveMarkers := []string{
		"person@example.test",
		"sk-sensitive-test-key",
		"user_sensitive_123",
		"cursor_sensitive_456",
		"acct_sensitive_456",
		"gen_sensitive_789",
		"payment_id",
		"pay_sensitive_000",
		"/api/v1/users/",
	}
	for _, marker := range sensitiveMarkers {
		require.NotContains(t, message, marker)
	}
}
