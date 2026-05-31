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

type apertisRoundTripFunc func(*http.Request) (*http.Response, error)

func (f apertisRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestApertis_CheckQuota_HappyPath_PaygOnly(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "https://api.apertis.ai/v1/dashboard/billing/credits", req.URL.String())

			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 10.50,
					"token_used": 50000,
					"token_total": 100000,
					"token_remaining": 50000,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "apertis", quota.ProviderType)
	require.NotNil(t, quota.Limits)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "available", quota.Limits[0].Status)
}

func TestApertis_CheckQuota_WarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 50.00,
					"token_used": 90000,
					"token_total": 100000,
					"token_remaining": 10000,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, 0.9, quota.Limits[0].UsageRatio)
}

func TestApertis_CheckQuota_ExhaustedState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 0,
					"token_used": 100000,
					"token_total": 100000,
					"token_remaining": 0,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestApertis_CheckQuota_WithSubscription(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 5.00,
					"token_used": 10000,
					"token_total": 100000,
					"token_remaining": 90000,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "pro",
					"status": "active",
					"cycle_quota_limit": 1000000,
					"cycle_quota_used": 100000,
					"cycle_quota_remaining": 900000,
					"cycle_start": "2026-01-01T00:00:00Z",
					"cycle_end": "2026-02-01T00:00:00Z",
					"payg_fallback_enabled": true,
					"payg_spent_usd": 10.00,
					"payg_limit_usd": 100.00
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.NotNil(t, quota.NextResetAt)

	// Should have two limits: PAYG tokens and subscription cycle
	require.Len(t, quota.Limits, 2)
}

func TestApertis_CheckQuota_SubscriptionWarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 100.00,
					"token_used": 0,
					"token_total": 100000,
					"token_remaining": 100000,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "pro",
					"status": "active",
					"cycle_quota_limit": 1000000,
					"cycle_quota_used": 850000,
					"cycle_quota_remaining": 150000,
					"cycle_start": "2026-01-01T00:00:00Z",
					"cycle_end": "2026-02-01T00:00:00Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	// Subscription usage is at 85%, should be warning
	require.Equal(t, "warning", quota.Status)
}

func TestApertis_CheckQuota_SubscriptionSuspended(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 100.00,
					"token_used": 0,
					"token_total": 100000,
					"token_remaining": 100000,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "pro",
					"status": "suspended",
					"cycle_quota_limit": 1000000,
					"cycle_quota_used": 500000,
					"cycle_quota_remaining": 500000,
					"cycle_start": "2026-01-01T00:00:00Z",
					"cycle_end": "2026-02-01T00:00:00Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestApertis_CheckQuota_UnlimitedTokens(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 50.00,
					"token_used": 50000,
					"token_total": "unlimited",
					"token_remaining": "unlimited",
					"token_is_unlimited": true
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	// Token limit should be unlimited
	require.Equal(t, "available", quota.Limits[0].Status)
	require.Equal(t, 0.0, quota.Limits[0].UsageRatio)
}

func TestApertis_CheckQuota_MissingCredentials(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "unknown", quota.Status)
	require.Equal(t, "apertis", quota.ProviderType)
	require.False(t, quota.Ready)
	require.Equal(t, "missing API key", quota.RawData["error"])
}

func TestApertis_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{invalid json`)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Equal(t, "unknown", quota.Status)
	require.False(t, quota.Ready)
}

func TestApertis_CheckQuota_HTTPError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error": "invalid key"}`)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Equal(t, "unknown", quota.Status)
	require.False(t, quota.Ready)
}

func TestApertis_CheckQuota_CustomBaseURL(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://custom.apertis.ai/v1/dashboard/billing/credits", req.URL.String())

			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 10.00,
					"token_used": 0,
					"token_total": 100000,
					"token_remaining": 100000,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "https://custom.apertis.ai",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestApertis_CheckQuota_EmptyBaseURL(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://api.apertis.ai/v1/dashboard/billing/credits", req.URL.String())

			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 10.00,
					"token_used": 0,
					"token_total": 100000,
					"token_remaining": 100000,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestApertis_SupportsChannel(t *testing.T) {
	checker := NewApertisQuotaChecker(nil)

	// Should support OpenAI type
	ch1 := &ent.Channel{
		Type: channel.TypeOpenai,
	}
	require.True(t, checker.SupportsChannel(ch1))

	// Should support OpenAIResponses type
	ch2 := &ent.Channel{
		Type: channel.TypeOpenaiResponses,
	}
	require.True(t, checker.SupportsChannel(ch2))

	// Should NOT support other types
	ch3 := &ent.Channel{
		Type: channel.TypeClaudecode,
	}
	require.False(t, checker.SupportsChannel(ch3))
}

func TestApertis_NextResetTimeParsing(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 10.00,
					"token_used": 0,
					"token_total": 100000,
					"token_remaining": 100000,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "pro",
					"status": "active",
					"cycle_quota_limit": 1000000,
					"cycle_quota_used": 100000,
					"cycle_quota_remaining": 900000,
					"cycle_start": "2026-01-15T00:00:00Z",
					"cycle_end": "2026-02-15T12:00:00Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)

	require.NotNil(t, quota.NextResetAt)
	expected := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
	require.Equal(t, expected, *quota.NextResetAt)
}

func TestApertis_RawDataContainsAllFields(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: apertisRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 10.00,
					"token_used": 50000,
					"token_total": 100000,
					"token_remaining": 50000,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "pro",
					"status": "active",
					"cycle_quota_limit": 1000000,
					"cycle_quota_used": 100000,
					"cycle_quota_remaining": 900000,
					"cycle_start": "2026-01-01T00:00:00Z",
					"cycle_end": "2026-02-01T00:00:00Z",
					"payg_fallback_enabled": true,
					"payg_spent_usd": 5.00,
					"payg_limit_usd": 50.00
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)

	// Check that raw data contains all the expected fields
	rawData := quota.RawData
	require.Equal(t, true, rawData["is_subscriber"])

	payg := rawData["payg"].(map[string]any)
	require.Equal(t, 10.00, payg["account_credits"])
	require.Equal(t, 50000.0, payg["token_used"])
	require.Equal(t, 100000.0, payg["token_total"])
	require.Equal(t, 50000.0, payg["token_remaining"])
	require.Equal(t, false, payg["token_is_unlimited"])

	subscription := rawData["subscription"].(map[string]any)
	require.Equal(t, "pro", subscription["plan_type"])
	require.Equal(t, "active", subscription["status"])
	require.Equal(t, 1000000, subscription["cycle_quota_limit"])
	require.Equal(t, 100000, subscription["cycle_quota_used"])
	require.Equal(t, 900000, subscription["cycle_quota_remaining"])
	require.Equal(t, "2026-01-01T00:00:00Z", subscription["cycle_start"])
	require.Equal(t, "2026-02-01T00:00:00Z", subscription["cycle_end"])
	require.Equal(t, true, subscription["payg_fallback_enabled"])
	require.Equal(t, 5.00, subscription["payg_spent_usd"])
	require.Equal(t, 50.00, subscription["payg_limit_usd"])
}