package provider_quota

import (
	"context"
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
		[]clineUsageItem{{CreatedAt: "2026-07-14T11:00:00Z", CostUSD: 50, CreditsUsed: 7}},
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

func TestBuildClineWindow_StaleOfficialResetPreservesEstimate(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	staleReset := now.Add(-2 * time.Hour)

	window := buildClineWindow(
		now,
		"last5h",
		5*time.Hour,
		100,
		[]clineUsageItem{{CreatedAt: now.Add(-time.Hour).Format(time.RFC3339)}},
		clineOfficialWindowLimit{NextResetAt: &staleReset},
	)

	require.Equal(t, now.Add(4*time.Hour), *window.nextResetAt)
	require.Equal(t, clineWindowSourceEstimatedUsage, window.resetSource)
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
							{"type":"weekly","percentUsed":79,"resetsAt":"2026-07-17T02:56:47Z"},
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
							{"createdAt":"2026-07-07T10:18:10Z","costUsd":462,"creditsUsed":0},
							{"createdAt":"2026-07-02T10:31:31Z","costUsd":497184013,"creditsUsed":0}
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
	require.InDelta(t, 0.19887379, last7d["cost_usage_ratio"].(float64), 0.000001)
	require.InDelta(t, 19.887379, last7d["cost_usage_percent"].(float64), 0.0001)
	require.Equal(t, "2026-07-17T02:56:47Z", last7d["next_reset_at"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, last7d["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, last7d["reset_source"])

	usageLimitsFetch := raw["usage_limits_fetch"].(map[string]any)
	require.Equal(t, clineUsageLimitsFetchStatusComplete, usageLimitsFetch["status"])
	require.Equal(t, 3, usageLimitsFetch["usable_windows"])
	require.Equal(t, 6, usageLimitsFetch["usable_fields"])
}

func TestCline_CheckQuota_UsageLimitsFailureFallsBackWithoutLosingCostData(t *testing.T) {
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

	require.NoError(t, err)
	require.Equal(t, 5, requestCount)
	windows := quota.RawData["windows"].(map[string]any)
	last5h := windows["last5h"].(map[string]any)
	require.Equal(t, int64(50), last5h["used_cost_units"])
	require.Equal(t, int64(100), last5h["limit_cost_units"])
	require.InDelta(t, 0.5, last5h["usage_ratio"].(float64), 0.000001)
	require.InDelta(t, 0.5, last5h["cost_usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceEstimatedCost, last5h["usage_source"])
	require.Equal(t, clineWindowSourceEstimatedUsage, last5h["reset_source"])
	require.Equal(t, "2026-07-07T16:00:00Z", last5h["next_reset_at"])

	fetch := quota.RawData["usage_limits_fetch"].(map[string]any)
	require.Equal(t, clineUsageLimitsFetchStatusUnavailable, fetch["status"])
	assertClineErrorOmitsSensitiveValues(t, fmt.Sprint(quota.RawData))
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
				return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-14T11:00:00Z","costUsd":50,"creditsUsed":3}]}}`), nil
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
	require.Equal(t, clineWindowSourceEstimatedUsage, fiveHour["reset_source"])
	require.Equal(t, "2026-07-14T16:00:00Z", fiveHour["next_reset_at"])

	require.InDelta(t, 0.25, weekly["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceEstimatedCost, weekly["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, weekly["reset_source"])
	require.Equal(t, "2026-07-17T02:56:47Z", weekly["next_reset_at"])

	require.InDelta(t, 0.125, monthly["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceEstimatedCost, monthly["usage_source"])
	require.Equal(t, clineWindowSourceEstimatedUsage, monthly["reset_source"])

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
				return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-14T11:00:00Z","costUsd":50,"creditsUsed":3}]}}`), nil
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
	require.Equal(t, clineWindowSourceEstimatedCost, fiveHour["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, fiveHour["reset_source"])
	require.Equal(t, "2026-07-14T15:00:00Z", fiveHour["next_reset_at"])

	require.InDelta(t, 0.9, weekly["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceOfficialUsageLimits, weekly["usage_source"])
	require.Equal(t, clineWindowSourceEstimatedUsage, weekly["reset_source"])
	require.Equal(t, "2026-07-21T11:00:00Z", weekly["next_reset_at"])

	require.InDelta(t, 0.4, monthly["usage_ratio"].(float64), 0.000001)
	require.Equal(t, clineWindowSourceOfficialUsageLimits, monthly["usage_source"])
	require.Equal(t, clineWindowSourceOfficialUsageLimits, monthly["reset_source"])
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
	quota := buildClineQuotaData(
		time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
		clineModelScopePassOnly,
		clineInferenceCapThreshold{Last5HoursUsageCostUSDPerUser: 100, Last7DaysUsageCostUSDPerUser: 1000, Last30DaysUsageCostUSDPerUser: 2000},
		nil,
		nil,
		[]clineUsageItem{{CreatedAt: "2026-07-07T11:00:00Z", CostUSD: 80}},
		clineUsageFetchMeta{Pages: 1, ItemsSeen: 1},
		nil,
		clineUsageLimitsFetchMeta{Status: clineUsageLimitsFetchStatusUnavailable},
	)

	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "cline_pass_windows", quota.RawData["status_basis"])
}

func TestCline_CheckQuota_ExhaustedWhenPassOnly(t *testing.T) {
	quota := buildClineQuotaData(
		time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
		clineModelScopePassOnly,
		clineInferenceCapThreshold{Last5HoursUsageCostUSDPerUser: 100, Last7DaysUsageCostUSDPerUser: 1000, Last30DaysUsageCostUSDPerUser: 2000},
		nil,
		nil,
		[]clineUsageItem{{CreatedAt: "2026-07-07T11:00:00Z", CostUSD: 100}},
		clineUsageFetchMeta{Pages: 1, ItemsSeen: 1},
		nil,
		clineUsageLimitsFetchMeta{Status: clineUsageLimitsFetchStatusUnavailable},
	)

	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestCline_CheckQuota_MixedScopeDoesNotExhaustWholeChannelFromPassPool(t *testing.T) {
	officialRatio := 1.0
	quota := buildClineQuotaData(
		time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
		clineModelScopeMixed,
		clineInferenceCapThreshold{
			Last5HoursUsageCostUSDPerUser: 100,
			Last7DaysUsageCostUSDPerUser:  1000,
			Last30DaysUsageCostUSDPerUser: 2000,
		},
		nil,
		nil,
		[]clineUsageItem{{CreatedAt: "2026-07-07T11:00:00Z", CostUSD: 1}},
		clineUsageFetchMeta{Pages: 1, ItemsSeen: 1},
		map[string]clineOfficialWindowLimit{
			"last5h": {UsageRatio: &officialRatio},
		},
		clineUsageLimitsFetchMeta{
			Status:            clineUsageLimitsFetchStatusPartial,
			EntriesSeen:       1,
			RecognizedEntries: 1,
			UsableWindows:     1,
			UsableFields:      1,
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
			return jsonResponse(http.StatusOK, `{"data":{"limits":[]}}`), nil
		case 5:
			return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-07T11:00:00Z","costUsd":1}],"nextToken":"cursor_sensitive_456"}}`), nil
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
