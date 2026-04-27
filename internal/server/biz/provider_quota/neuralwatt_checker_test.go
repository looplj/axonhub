package provider_quota

import (
	"context"
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

func TestNeuralWatt_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))

			body := `{
				"balance": {
					"credits_remaining_usd": 5.0,
					"total_credits_usd": 5.0,
					"accounting_method": "energy"
				},
				"subscription": {
					"plan": "standard",
					"status": "active",
					"current_period_start": "2026-04-02T05:58:36Z",
					"current_period_end": "2026-05-02T05:58:36Z",
					"kwh_included": 20.0,
					"kwh_used": 14.3126,
					"kwh_remaining": 5.6874,
					"in_overage": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "neuralwatt", quota.ProviderType)
}

func TestNeuralWatt_CheckQuota_WarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// kwh_remaining = 3.0, kwh_included = 20.0 → 15% < 20% → warning
			body := `{
				"balance": {
					"credits_remaining_usd": 1.5,
					"total_credits_usd": 5.0,
					"accounting_method": "energy"
				},
				"subscription": {
					"plan": "standard",
					"status": "active",
					"current_period_start": "2026-04-02T05:58:36Z",
					"current_period_end": "2026-05-02T05:58:36Z",
					"kwh_included": 20.0,
					"kwh_used": 17.0,
					"kwh_remaining": 3.0,
					"in_overage": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}

func TestNeuralWatt_CheckQuota_ExhaustedState(t *testing.T) {
	expectedResetAt, _ := time.Parse(time.RFC3339, "2026-05-02T05:58:36Z")

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"balance": {
					"credits_remaining_usd": 0.0,
					"total_credits_usd": 5.0,
					"accounting_method": "energy"
				},
				"subscription": {
					"plan": "standard",
					"status": "active",
					"current_period_start": "2026-04-02T05:58:36Z",
					"current_period_end": "2026-05-02T05:58:36Z",
					"kwh_included": 20.0,
					"kwh_used": 22.5,
					"kwh_remaining": 0.0,
					"in_overage": true
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedResetAt, *quota.NextResetAt)
}

func TestNeuralWatt_CheckQuota_MissingCredentials(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no API key")
}

func TestNeuralWatt_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`not json`)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse neuralwatt usage response")
}

func TestNeuralWatt_CheckQuota_HTTPError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestNeuralWatt_CheckQuota_ZeroRemainingWithoutOverage(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"balance": {
					"credits_remaining_usd": 0.0,
					"total_credits_usd": 5.0,
					"accounting_method": "energy"
				},
				"subscription": {
					"plan": "standard",
					"status": "active",
					"current_period_start": "2026-04-02T05:58:36Z",
					"current_period_end": "2026-05-02T05:58:36Z",
					"kwh_included": 20.0,
					"kwh_used": 20.0,
					"kwh_remaining": 0.0,
					"in_overage": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestNeuralWatt_CheckQuota_SubscriptionWithoutKeyFields(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"balance": {
					"credits_remaining_usd": 5.0,
					"total_credits_usd": 5.0
				},
				"subscription": {
					"plan": "standard",
					"status": "active"
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "unknown", quota.Status)
	require.False(t, quota.Ready)
}

func TestNeuralWatt_CheckQuota_CustomBaseURL(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://custom.neuralwatt.com/v1/quota", req.URL.String())

			body := `{
				"balance": {
					"credits_remaining_usd": 5.0,
					"total_credits_usd": 5.0,
					"accounting_method": "energy"
				},
				"subscription": {
					"plan": "standard",
					"status": "active",
					"current_period_start": "2026-04-02T05:58:36Z",
					"current_period_end": "2026-05-02T05:58:36Z",
					"kwh_included": 20.0,
					"kwh_used": 10.0,
					"kwh_remaining": 10.0,
					"in_overage": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNeuralWattQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
		BaseURL: "https://custom.neuralwatt.com",
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
}

func TestNeuralWatt_SupportsChannel(t *testing.T) {
	checker := NewNeuralWattQuotaChecker(nil)

	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.neuralwatt.com",
	}))
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenaiResponses,
		BaseURL: "https://us.api.neuralwatt.com",
	}))
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.openai.com",
	}))
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeClaudecode,
		BaseURL: "https://api.neuralwatt.com",
	}))
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.neuralwatt.com:443",
	}))
}
