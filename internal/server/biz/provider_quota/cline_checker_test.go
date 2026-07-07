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
	require.Equal(t, requestCount, 4)

	raw := quota.RawData
	require.Equal(t, "cline_pass_only", raw["model_scope"])
	require.Equal(t, "cline_pass_windows", raw["status_basis"])
	require.NotContains(t, raw, "user_id")
	require.NotContains(t, raw, "email")

	windows := raw["windows"].(map[string]any)
	last7d := windows["last7d"].(map[string]any)
	require.InDelta(t, 0.19887379, last7d["usage_ratio"].(float64), 0.000001)
	require.InDelta(t, 19.887379, last7d["usage_percent"].(float64), 0.0001)
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
	)

	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestCline_CheckQuota_MixedScopeDoesNotExhaustWholeChannelFromPassPool(t *testing.T) {
	quota := buildClineQuotaData(
		time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
		clineModelScopeMixed,
		clineInferenceCapThreshold{Last5HoursUsageCostUSDPerUser: 100, Last7DaysUsageCostUSDPerUser: 1000, Last30DaysUsageCostUSDPerUser: 2000},
		nil,
		nil,
		[]clineUsageItem{{CreatedAt: "2026-07-07T11:00:00Z", CostUSD: 100}},
		clineUsageFetchMeta{Pages: 1, ItemsSeen: 1},
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
			return jsonResponse(http.StatusOK, `{"data":{"items":[{"createdAt":"2026-07-07T11:00:00Z","costUsd":1}],"nextToken":"cursor_sensitive_456"}}`), nil
		case 5:
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
