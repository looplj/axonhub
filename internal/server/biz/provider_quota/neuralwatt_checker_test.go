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
	expectedResetAt, _ := time.Parse(time.RFC3339, "2099-06-02T05:58:36Z")

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
					"kwh_included": 20.0,
					"kwh_used": 17.0,
					"kwh_remaining": 3.0,
					"in_overage": false,
					"kwh_reset_date": "2099-06-02T05:58:36Z"
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
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedResetAt, *quota.NextResetAt)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "kwh", quota.Limits[0].Window)
	require.NotNil(t, quota.Limits[0].NextResetAt)
	expectedPeriodStart := PeriodStartFromMonthlyReset(&expectedResetAt)
	require.NotNil(t, expectedPeriodStart)
	require.NotNil(t, quota.Limits[0].PeriodStart)
	require.Equal(t, *expectedPeriodStart, *quota.Limits[0].PeriodStart)
}

func TestNeuralWatt_CheckQuota_MissingResetDate(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"subscription": {
					"plan": "standard",
					"status": "active",
					"kwh_included": 20.0,
					"kwh_used": 5.0,
					"kwh_remaining": 15.0,
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
	require.Nil(t, quota.NextResetAt)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "kwh", quota.Limits[0].Window)
	require.Nil(t, quota.Limits[0].NextResetAt)
	require.Nil(t, quota.Limits[0].PeriodStart)
}

func TestNeuralWatt_CheckQuota_ExhaustedState(t *testing.T) {
	expectedResetAt, _ := time.Parse(time.RFC3339, "2099-06-02T05:58:36Z")

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
					"kwh_included": 20.0,
					"kwh_used": 22.5,
					"kwh_remaining": 0.0,
					"in_overage": true,
					"kwh_reset_date": "2099-06-02T05:58:36Z"
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
	expectedResetAt, _ := time.Parse(time.RFC3339, "2099-06-02T05:58:36Z")

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
					"kwh_included": 20.0,
					"kwh_used": 20.0,
					"kwh_remaining": 0.0,
					"in_overage": false,
					"kwh_reset_date": "2099-06-02T05:58:36Z"
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

func TestNeuralWatt_CheckQuota_KwhResetDate(t *testing.T) {
	expectedResetAt, _ := time.Parse(time.RFC3339, "2099-06-26T19:25:50Z")

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"balance": {
					"credits_remaining_usd": 120.01,
					"total_credits_usd": 120.01,
					"accounting_method": "energy"
				},
				"subscription": {
					"plan": "pro",
					"status": "active",
					"kwh_included": 33.0,
					"kwh_used": 8.7808,
					"kwh_remaining": 24.2192,
					"in_overage": false,
					"kwh_reset_date": "2099-06-26T19:25:50Z"
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
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedResetAt, *quota.NextResetAt)
}

func neuralWattDailyFreeEnergyClient(t *testing.T, body string) *NeuralWattQuotaChecker {
	t.Helper()

	return NewNeuralWattQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}))
}

const neuralWattSubscriptionBody = `"subscription":{"plan":"pro","status":"active","kwh_included":20.0,"kwh_used":10.0,"kwh_remaining":10.0,"in_overage":false,"kwh_reset_date":"2099-02-01T00:00:00Z"}`

func TestNeuralWatt_CheckQuota_DailyFreeEnergy(t *testing.T) {
	expectedResetAt, _ := time.Parse(time.RFC3339, "2099-01-14T00:00:00Z")
	expectedMonthlyReset, _ := time.Parse(time.RFC3339, "2099-02-01T00:00:00Z")

	body := `{` + neuralWattSubscriptionBody + `,"daily_free_energy":{"kwh_included":0.25,"kwh_used":0.127673,"kwh_remaining":0.122327,"reset_at":"2099-01-14T00:00:00Z","model_labels":["Model A"]},"usage":{"current_month":{"cost_usd":91.8,"requests":29728,"energy_kwh":26.8}}}`

	quota, err := neuralWattDailyFreeEnergyClient(t, body).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)

	dfe := quota.Limits[1]
	require.Equal(t, QuotaLimitTypeFreeEnergy, dfe.Type)
	require.Equal(t, "daily_free_energy", dfe.Window)
	require.InDelta(t, 0.127673/0.25, dfe.UsageRatio, 1e-4)
	require.Equal(t, "available", dfe.Status)
	require.True(t, dfe.Ready)
	require.Equal(t, expectedResetAt, *dfe.NextResetAt)
	require.Nil(t, dfe.PeriodStart)

	require.Equal(t, "available", quota.Status)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedMonthlyReset, *quota.NextResetAt)

	dfeRaw, ok := quota.RawData["daily_free_energy"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []string{"Model A"}, dfeRaw["model_labels"])

	cmRaw, ok := quota.RawData["usage_current_month"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 91.8, cmRaw["cost_usd"])
}

func TestNeuralWatt_CheckQuota_DailyFreeEnergyNull(t *testing.T) {
	expectedMonthlyReset, _ := time.Parse(time.RFC3339, "2099-02-01T00:00:00Z")

	body := `{` + neuralWattSubscriptionBody + `,"usage":{"current_month":{"cost_usd":12.5,"requests":100,"energy_kwh":1.5}}}`

	quota, err := neuralWattDailyFreeEnergyClient(t, body).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "kwh", quota.Limits[0].Window)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedMonthlyReset, *quota.NextResetAt)
	require.NotContains(t, quota.RawData, "daily_free_energy")
	require.Contains(t, quota.RawData, "usage_current_month")
}

func TestNeuralWatt_CheckQuota_DailyFreeEnergyZeroIncluded(t *testing.T) {
	body := `{` + neuralWattSubscriptionBody + `,"daily_free_energy":{"kwh_included":0,"kwh_used":0,"kwh_remaining":0,"reset_at":"2099-01-14T00:00:00Z","model_labels":[]}}`

	quota, err := neuralWattDailyFreeEnergyClient(t, body).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)

	dfeRaw, ok := quota.RawData["daily_free_energy"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), dfeRaw["kwh_included"])
}

func TestNeuralWatt_CheckQuota_DailyFreeEnergyExhausted(t *testing.T) {
	body := `{` + neuralWattSubscriptionBody + `,"daily_free_energy":{"kwh_included":0.25,"kwh_used":0.25,"kwh_remaining":0,"reset_at":"2099-01-14T00:00:00Z"}}`

	quota, err := neuralWattDailyFreeEnergyClient(t, body).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Limits[1].Status)
	require.Equal(t, 1.0, quota.Limits[1].UsageRatio)

	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)

	state, _ := EvaluateQuotaRouting(quota.Limits, quota.Status, QuotaLimitTypeToken, time.Now())
	require.Equal(t, RoutingOpen, state)
}

func TestNeuralWatt_CheckQuota_DailyFreeEnergyWarning(t *testing.T) {
	body := `{` + neuralWattSubscriptionBody + `,"daily_free_energy":{"kwh_included":0.25,"kwh_used":0.225,"kwh_remaining":0.025,"reset_at":"2099-01-14T00:00:00Z"}}`

	quota, err := neuralWattDailyFreeEnergyClient(t, body).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Limits[1].Status)
	require.Equal(t, "available", quota.Status)
}

func TestNeuralWatt_CheckQuota_DailyFreeEnergyMissingReset(t *testing.T) {
	body := `{` + neuralWattSubscriptionBody + `,"daily_free_energy":{"kwh_included":0.25,"kwh_used":0.1}}`

	quota, err := neuralWattDailyFreeEnergyClient(t, body).CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)
	require.Nil(t, quota.Limits[1].NextResetAt)
	require.Nil(t, quota.Limits[1].PeriodStart)
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
