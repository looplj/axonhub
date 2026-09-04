package provider_quota

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestZenmuxQuotaChecker_CheckQuota(t *testing.T) {
	tests := []struct {
		name               string
		statusCode         int
		body               string
		wantStatus         string
		wantReady          bool
		wantFiveHourStatus string
		wantFiveHourReset  bool
		wantAccountStatus  string
		wantErr            bool
	}{
		{
			name:               "healthy mid-usage",
			statusCode:         http.StatusOK,
			body:               zenmuxQuotaFixture("healthy", 0.4, `"2026-09-03T15:00:00Z"`),
			wantStatus:         "available",
			wantReady:          true,
			wantFiveHourStatus: "available",
			wantFiveHourReset:  true,
			wantAccountStatus:  "healthy",
		},
		{
			name:               "five hour window exhausted",
			statusCode:         http.StatusOK,
			body:               zenmuxQuotaFixture("healthy", 1, `"2026-09-03T15:00:00Z"`),
			wantStatus:         "exhausted",
			wantReady:          false,
			wantFiveHourStatus: "exhausted",
			wantFiveHourReset:  true,
			wantAccountStatus:  "healthy",
		},
		{
			name:               "five hour reset is null",
			statusCode:         http.StatusOK,
			body:               zenmuxQuotaFixture("healthy", 0.2, "null"),
			wantStatus:         "available",
			wantReady:          true,
			wantFiveHourStatus: "available",
			wantFiveHourReset:  false,
			wantAccountStatus:  "healthy",
		},
		{
			name:               "suspended account",
			statusCode:         http.StatusOK,
			body:               zenmuxQuotaFixture("suspended", 0.2, `"2026-09-03T15:00:00Z"`),
			wantStatus:         "exhausted",
			wantReady:          false,
			wantFiveHourStatus: "available",
			wantFiveHourReset:  true,
			wantAccountStatus:  "suspended",
		},
		{
			name:       "unsuccessful response",
			statusCode: http.StatusOK,
			body:       `{"success":false,"message":"invalid management key"}`,
			wantErr:    true,
		},
		{
			name:       "non-success status code",
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"success":false,"message":"rate limited"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/management/subscription/detail", r.URL.Path)
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.body))
				assert.NoError(t, err)
			}))
			t.Cleanup(server.Close)

			target, err := url.Parse(server.URL)
			require.NoError(t, err)
			client := httpclient.NewHttpClientWithClient(&http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					require.Equal(t, "https://zenmux.ai/api/v1/management/subscription/detail", req.URL.String())
					require.Equal(t, "Bearer management-key", req.Header.Get("Authorization"))

					forwarded := req.Clone(req.Context())
					forwarded.URL.Scheme = target.Scheme
					forwarded.URL.Host = target.Host
					return http.DefaultTransport.RoundTrip(forwarded)
				}),
			})
			checker := NewZenmuxQuotaChecker(client)

			quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
				Credentials: objects.ChannelCredentials{ManagementAPIKey: " management-key "},
			})
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, quota.Status)
			require.Equal(t, tt.wantReady, quota.Ready)
			require.Equal(t, "zenmux", quota.ProviderType)
			require.Len(t, quota.Limits, 2)
			require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
			require.Equal(t, tt.wantFiveHourStatus, quota.Limits[0].Status)
			require.Equal(t, tt.wantFiveHourReset, quota.Limits[0].NextResetAt != nil)
			require.Equal(t, tt.wantFiveHourReset, quota.Limits[0].PeriodStart != nil)
			require.Equal(t, QuotaWindow7d, quota.Limits[1].Window)
			require.Contains(t, quota.RawData, "plan")
			require.Contains(t, quota.RawData, "quota_monthly")
			require.Equal(t, tt.wantAccountStatus, quota.RawData["account_status"])
		})
	}
}

func TestZenmuxQuotaChecker_SupportsOnlyZenMuxChannelTypes(t *testing.T) {
	checker := NewZenmuxQuotaChecker(httpclient.NewHttpClient())

	for _, channelType := range []channel.Type{
		channel.TypeZenmux,
		channel.TypeZenmuxResponses,
		channel.TypeZenmuxAnthropic,
		channel.TypeZenmuxGemini,
	} {
		require.True(t, checker.SupportsChannel(&ent.Channel{Type: channelType}))
	}
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai}))
}

func TestZenmuxQuotaChecker_RequiresManagementKey(t *testing.T) {
	checker := NewZenmuxQuotaChecker(httpclient.NewHttpClient())

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{})

	require.ErrorContains(t, err, "management API key")
}

func zenmuxQuotaFixture(accountStatus string, fiveHourUsage float64, fiveHourReset string) string {
	return fmt.Sprintf(`{
		"success": true,
		"data": {
			"plan": {"tier":"pro","amount_usd":200,"interval":"month","expires_at":"2026-10-01T00:00:00Z"},
			"account_status": %q,
			"quota_5_hour": {
				"usage_percentage": %v,
				"resets_at": %s,
				"max_flows": 1000,
				"used_flows": 400,
				"remaining_flows": 600,
				"used_value_usd": 40,
				"max_value_usd": 100
			},
			"quota_7_day": {
				"usage_percentage": 0.6,
				"resets_at": "2026-09-10T10:00:00Z",
				"max_flows": 10000,
				"used_flows": 6000,
				"remaining_flows": 4000,
				"used_value_usd": 120,
				"max_value_usd": 200
			},
			"quota_monthly": {"max_flows":50000,"max_value_usd":500}
		}
	}`, accountStatus, fiveHourUsage, fiveHourReset)
}
