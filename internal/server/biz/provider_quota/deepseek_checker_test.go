package provider_quota

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func deepSeekTestChannel(baseURL, apiKey string) *ent.Channel {
	return &ent.Channel{
		Type:    channel.TypeDeepseek,
		BaseURL: baseURL,
		Credentials: objects.ChannelCredentials{
			APIKey: apiKey,
		},
	}
}

func TestDeepSeek_CheckQuota_Success(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "https://api.deepseek.com/user/balance", req.URL.String())
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Accept"))

			body := `{
				"is_available": true,
				"balance_infos": [
					{"currency":"CNY","total_balance":"110.00","granted_balance":"10.00","topped_up_balance":"100.00"},
					{"currency":"USD","total_balance":"5.50","granted_balance":"0.00","topped_up_balance":"5.50"}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewDeepSeekQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), deepSeekTestChannel("https://api.deepseek.com/v1", "test-api-key"))
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, deepSeekProviderType, quota.ProviderType)
	require.Nil(t, quota.NextResetAt)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaWindowCredits, quota.Limits[0].Window)
	require.Equal(t, 0.0, quota.Limits[0].UsageRatio)

	require.Equal(t, true, quota.RawData["is_available"])
	balances, ok := quota.RawData["balances"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, balances, 2)
	require.Equal(t, "CNY", balances[0]["currency"])
	require.InDelta(t, 110.0, balances[0]["total_balance"], 0.001)
	require.InDelta(t, 10.0, balances[0]["granted_balance"], 0.001)
	require.InDelta(t, 100.0, balances[0]["topped_up_balance"], 0.001)
	require.Equal(t, "USD", balances[1]["currency"])
	require.InDelta(t, 5.5, balances[1]["total_balance"], 0.001)
}

func TestDeepSeek_CheckQuota_Exhausted(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "account unavailable",
			body: `{"is_available":false,"balance_infos":[{"currency":"USD","total_balance":"5.00","granted_balance":"0.00","topped_up_balance":"5.00"}]}`,
		},
		{
			name: "zero balance",
			body: `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"0.00","granted_balance":"0.00","topped_up_balance":"0.00"}]}`,
		},
		{
			name: "empty balance list while unavailable",
			body: `{"is_available":false,"balance_infos":[]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewDeepSeekQuotaChecker(nil)
			quota, err := checker.parseResponse([]byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, "exhausted", quota.Status)
			require.False(t, quota.Ready)
			require.Len(t, quota.Limits, 1)
			require.Equal(t, 1.0, quota.Limits[0].UsageRatio)
		})
	}
}

func TestDeepSeek_CheckQuota_InvalidCredentials(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"invalid api key"}`)),
			}, nil
		}),
	})

	checker := NewDeepSeekQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), deepSeekTestChannel("", "bad-key"))
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidCredentials))
}

func TestDeepSeek_CheckQuota_ServerError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"unavailable"}`)),
			}, nil
		}),
	})

	checker := NewDeepSeekQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), deepSeekTestChannel("", "test-key"))
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrInvalidCredentials))
	require.Contains(t, err.Error(), "500")
}

func TestDeepSeek_CheckQuota_MissingAPIKey(t *testing.T) {
	checker := NewDeepSeekQuotaChecker(nil)
	_, err := checker.CheckQuota(context.Background(), deepSeekTestChannel("", ""))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no API key")
}

func TestDeepSeek_CheckQuota_UsesFirstConfiguredKey(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer first-key", req.Header.Get("Authorization"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"1.00"}]}`)),
			}, nil
		}),
	})

	checker := NewDeepSeekQuotaChecker(httpClient)
	ch := &ent.Channel{
		Type: channel.TypeDeepseekAnthropic,
		Credentials: objects.ChannelCredentials{
			APIKeys: []string{"", "first-key", "second-key"},
		},
	}

	quota, err := checker.CheckQuota(context.Background(), ch)
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestDeepSeek_ParseResponse_Errors(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed json", body: `{invalid json}`, wantErr: "failed to parse"},
		{name: "missing is_available", body: `{"balance_infos":[{"currency":"USD","total_balance":"1.00"}]}`, wantErr: "missing is_available"},
		{name: "missing currency", body: `{"is_available":true,"balance_infos":[{"total_balance":"1.00"}]}`, wantErr: "without a currency"},
		{name: "missing total balance", body: `{"is_available":true,"balance_infos":[{"currency":"USD"}]}`, wantErr: "missing total_balance"},
		{name: "invalid total balance", body: `{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"abc"}]}`, wantErr: "invalid total_balance"},
		{name: "negative total balance", body: `{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"-1.00"}]}`, wantErr: "invalid total_balance"},
		{name: "empty balances while available", body: `{"is_available":true,"balance_infos":[]}`, wantErr: "no balance info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewDeepSeekQuotaChecker(nil)
			_, err := checker.parseResponse([]byte(tt.body))
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestDeepSeek_CheckQuota_GrantedBalanceCountsTowardAvailability(t *testing.T) {
	checker := NewDeepSeekQuotaChecker(nil)
	quota, err := checker.parseResponse([]byte(`{
		"is_available": true,
		"balance_infos": [
			{"currency":"CNY","total_balance":"10.00","granted_balance":"10.00","topped_up_balance":"0.00"}
		]
	}`))
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)

	balances, ok := quota.RawData["balances"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, balances, 1)
	// total_balance is DeepSeek's authoritative amount and already includes the
	// granted balance; the breakdown must not be added to it a second time.
	require.InDelta(t, 10.0, balances[0]["total_balance"], 0.001)
	require.InDelta(t, 10.0, balances[0]["granted_balance"], 0.001)
	require.InDelta(t, 0.0, balances[0]["topped_up_balance"], 0.001)
}

func TestDeepSeek_SupportsChannel(t *testing.T) {
	checker := NewDeepSeekQuotaChecker(nil)
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeDeepseek}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeDeepseekAnthropic}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeDeepinfra}))
}

func TestBuildDeepSeekBalanceURL(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		{"", "https://api.deepseek.com/user/balance"},
		{"https://api.deepseek.com", "https://api.deepseek.com/user/balance"},
		{"https://api.deepseek.com/", "https://api.deepseek.com/user/balance"},
		{"https://api.deepseek.com/v1", "https://api.deepseek.com/user/balance"},
		{"https://api.deepseek.com/v1/", "https://api.deepseek.com/user/balance"},
		{"https://api.deepseek.com/anthropic", "https://api.deepseek.com/user/balance"},
		{"https://api.deepseek.com/anthropic/v1", "https://api.deepseek.com/user/balance"},
		{"https://api.deepseek.com/beta", "https://api.deepseek.com/user/balance"},
		{"https://api.deepseek.com/user/balance", "https://api.deepseek.com/user/balance"},
		{"https://proxy.example.com/v1?foo=bar#frag", "https://proxy.example.com/user/balance"},
		{"not-a-url", "https://api.deepseek.com/user/balance"},
	}

	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			require.Equal(t, tt.want, buildDeepSeekBalanceURL(tt.baseURL))
		})
	}
}
