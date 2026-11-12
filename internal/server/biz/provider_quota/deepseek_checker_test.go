package provider_quota

import (
	"context"
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

func TestDeepSeek_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://api.deepseek.com/user/balance", req.URL.String())
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))

			body := `{
				"is_available": true,
				"balance_infos": [
					{
						"currency": "CNY",
						"total_balance": "110.00",
						"granted_balance": "10.00",
						"topped_up_balance": "100.00"
					}
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

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:    channel.TypeDeepseek,
		BaseURL: "https://api.deepseek.com/v1",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "deepseek", quota.ProviderType)
	require.Empty(t, quota.Limits)
	require.Len(t, quota.Balances, 1)
	require.Equal(t, "CNY", quota.Balances[0].Currency)
	require.Equal(t, 110.0, quota.Balances[0].Total)
	require.NotNil(t, quota.Balances[0].Granted)
	require.Equal(t, 10.0, *quota.Balances[0].Granted)
	require.NotNil(t, quota.Balances[0].ToppedUp)
	require.Equal(t, 100.0, *quota.Balances[0].ToppedUp)

	infos, ok := quota.RawData["balance_infos"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, infos, 1)
	require.Equal(t, "CNY", infos[0]["currency"])
	require.Equal(t, "110.00", infos[0]["total_balance"])
	require.Equal(t, "10.00", infos[0]["granted_balance"])
	require.Equal(t, "100.00", infos[0]["topped_up_balance"])
}

func TestDeepSeek_CheckQuota_UnavailableBalance(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"is_available": false,
				"balance_infos": [
					{"currency": "USD", "total_balance": "0.00", "granted_balance": "0.00", "topped_up_balance": "0.00"}
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

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeDeepseekAnthropic,
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.Empty(t, quota.Limits)
	require.Len(t, quota.Balances, 1)
	require.Equal(t, "USD", quota.Balances[0].Currency)
	require.Equal(t, 0.0, quota.Balances[0].Total)
}

func TestDeepSeek_CheckQuota_MissingAPIKey(t *testing.T) {
	checker := NewDeepSeekQuotaChecker(httpclient.NewHttpClient())

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{Type: channel.TypeDeepseek})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no API key")
}

func TestDeepSeek_CheckQuota_AnyFundedKeyKeepsChannelAvailable(t *testing.T) {
	var requested []string

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			key := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
			requested = append(requested, key)

			var body string
			switch key {
			case "empty-0001":
				body = `{"is_available": false, "balance_infos": [{"currency": "CNY", "total_balance": "0.00", "granted_balance": "0.00", "topped_up_balance": "0.00"}]}`
			case "funded-0002":
				body = `{"is_available": true, "balance_infos": [{"currency": "CNY", "total_balance": "88.00", "granted_balance": "8.00", "topped_up_balance": "80.00"}]}`
			default:
				t.Fatalf("unexpected API key %q", key)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewDeepSeekQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeDeepseek,
		Credentials: objects.ChannelCredentials{
			APIKeys: []string{"empty-0001", "funded-0002"},
		},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"empty-0001", "funded-0002"}, requested)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Len(t, quota.Balances, 2)

	byAccount := map[string]QuotaBalance{}
	for _, balance := range quota.Balances {
		byAccount[balance.Account] = balance
	}
	require.Equal(t, 0.0, byAccount["#1 ****0001"].Total)
	require.Equal(t, 88.0, byAccount["#2 ****0002"].Total)
}

func TestDeepSeek_CheckQuota_DisabledKeyIsDisplayOnly(t *testing.T) {
	var requested []string

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requested = append(requested, strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"is_available": true, "balance_infos": [{"currency": "CNY", "total_balance": "5.00"}]}`)),
			}, nil
		}),
	})

	checker := NewDeepSeekQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeDeepseek,
		Credentials: objects.ChannelCredentials{
			APIKeys: []string{"disabled-0001", "serving-0002"},
		},
		DisabledAPIKeys: []objects.DisabledAPIKey{{Key: "disabled-0001"}},
	})
	require.NoError(t, err)
	// Every configured key is read, so the per-key view stays complete; the
	// parked key simply stops counting as serving capacity.
	require.ElementsMatch(t, []string{"disabled-0001", "serving-0002"}, requested)
	require.Equal(t, "available", quota.Status)
	require.Len(t, quota.APIKeyQuotas, 2)
	require.True(t, quota.APIKeyQuotas[0].Disabled)
	require.Len(t, quota.Balances, 2, "both balances stay visible per key")
	for _, balance := range quota.Balances {
		require.NotEmpty(t, balance.Account)
	}
}

func TestBuildDeepSeekBalanceURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"empty", "", "https://api.deepseek.com/user/balance"},
		{"official", "https://api.deepseek.com", "https://api.deepseek.com/user/balance"},
		{"official v1", "https://api.deepseek.com/v1", "https://api.deepseek.com/user/balance"},
		{"official v1 trailing slash", "https://api.deepseek.com/v1/", "https://api.deepseek.com/user/balance"},
		{"official anthropic", "https://api.deepseek.com/anthropic", "https://api.deepseek.com/user/balance"},
		{"http relay v1", "http://relay:8080/deepseek/v1", "http://relay:8080/deepseek/user/balance"},
		{"http relay anthropic", "http://relay:8080/deepseek/anthropic", "http://relay:8080/deepseek/user/balance"},
		{"http relay prefix", "http://relay:8080/deepseek", "http://relay:8080/deepseek/user/balance"},
		{"drops query", "http://relay:8080/deepseek/?token=x", "http://relay:8080/deepseek/user/balance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, buildDeepSeekBalanceURL(tt.baseURL))
		})
	}
}

func TestDeepSeek_SupportsChannel(t *testing.T) {
	checker := NewDeepSeekQuotaChecker(httpclient.NewHttpClient())

	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeDeepseek}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeDeepseekAnthropic}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai, BaseURL: "https://api.deepseek.com/v1"}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenaiResponses, BaseURL: "https://api.deepseek.com"}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai, BaseURL: "https://api.openai.com"}))
}
