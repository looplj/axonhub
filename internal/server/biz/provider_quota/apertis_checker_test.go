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

func TestApertis_CheckQuota_HappyPath_PaygOnly(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "https://api.apertis.ai/v1/dashboard/billing/credits", req.URL.String())

			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 5.0,
					"token_used": 2.0,
					"token_total": 7.0,
					"token_remaining": 5.0,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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
	require.Equal(t, QuotaLimitTypeToken, quota.Limits[0].Type)
	require.Equal(t, "available", quota.Limits[0].Status)
}

func TestApertis_CheckQuota_WarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 50.00,
					"token_used": 9.0,
					"token_total": 10.0,
					"token_remaining": 1.0,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 0,
					"token_used": 10.0,
					"token_total": 10.0,
					"token_remaining": 0,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Matches the official API docs subscription response
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 9.98,
					"token_used": 0.05,
					"token_total": 1.0,
					"token_remaining": 0.95,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "lite",
					"status": "active",
					"cycle_quota_limit": 600,
					"cycle_quota_used": 10,
					"cycle_quota_remaining": 590,
					"cycle_start": "2026-03-16T10:02:35Z",
					"cycle_end": "2026-04-16T10:02:35Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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

	// Should have one limit: subscription cycle only (PAYG skipped for active subscription)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaLimitTypeSubscriptionCycle, quota.Limits[0].Type)
}

func TestApertis_CheckQuota_SubscriptionWarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 100.00,
					"token_used": 0.5,
					"token_total": 3.5,
					"token_remaining": 3.0,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "pro",
					"status": "active",
					"cycle_quota_limit": 1000,
					"cycle_quota_used": 850,
					"cycle_quota_remaining": 150,
					"cycle_start": "2026-01-01T00:00:00Z",
					"cycle_end": "2026-02-01T00:00:00Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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

func TestApertis_CheckQuota_SubscriptionSuspended_WithPAYGCredits(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 100.00,
					"token_used": 0.5,
					"token_total": 3.5,
					"token_remaining": 3.0,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "pro",
					"status": "suspended",
					"cycle_quota_limit": 1000,
					"cycle_quota_used": 500,
					"cycle_quota_remaining": 500,
					"cycle_start": "2026-01-01T00:00:00Z",
					"cycle_end": "2026-02-01T00:00:00Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	// Subscription is suspended but PAYG credits are available
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	// Subscription cycle limit should be exhausted (suspended), PAYG token limit is available
	require.Len(t, quota.Limits, 2)
	var subLimit, paygLimit QuotaLimitStatus
	for _, l := range quota.Limits {
		if l.Type == QuotaLimitTypeSubscriptionCycle {
			subLimit = l
		} else if l.Type == QuotaLimitTypeToken {
			paygLimit = l
		}
	}
	require.Equal(t, "exhausted", subLimit.Status, "suspended subscription cycle should be exhausted")
	require.Equal(t, "available", paygLimit.Status)
}

func TestApertis_CheckQuota_SubscriptionSuspended_NoPAYGCredits(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 0,
					"token_used": 10.0,
					"token_total": 10.0,
					"token_remaining": 0,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "pro",
					"status": "suspended",
					"cycle_quota_limit": 1000,
					"cycle_quota_used": 500,
					"cycle_quota_remaining": 500,
					"cycle_start": "2026-01-01T00:00:00Z",
					"cycle_end": "2026-02-01T00:00:00Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	// Subscription suspended and no PAYG credits
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	// Subscription cycle limit should be exhausted (suspended)
	require.Len(t, quota.Limits, 2)
	var subLimit QuotaLimitStatus
	for _, l := range quota.Limits {
		if l.Type == QuotaLimitTypeSubscriptionCycle {
			subLimit = l
		}
	}
	require.Equal(t, "exhausted", subLimit.Status, "suspended subscription cycle should be exhausted")
}

func TestApertis_CheckQuota_SubscriptionExhausted_WithPAYGFallback(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Matches the official API docs "Subscription with PAYG Fallback" response
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 3.0,
					"token_used": 0.5,
					"token_total": 3.5,
					"token_remaining": 3.0,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "max",
					"status": "active",
					"cycle_quota_limit": 5000,
					"cycle_quota_used": 5000,
					"cycle_quota_remaining": 0,
					"cycle_start": "2026-03-01T00:00:00Z",
					"cycle_end": "2026-04-01T00:00:00Z",
					"payg_fallback_enabled": true,
					"payg_spent_usd": 2.5,
					"payg_limit_usd": 10.0
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	// Cycle quota is exhausted but PAYG fallback is enabled with credits
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	// Both subscription cycle (exhausted) and PAYG token (available) limits present
	require.Len(t, quota.Limits, 2)
	// Verify subscription cycle and PAYG token are separate limit types,
	// so EffectiveStatus(QuotaLimitTypeToken) does not incorrectly merge them.
	var subscriptionLimit, paygLimit QuotaLimitStatus
	for _, l := range quota.Limits {
		if l.Type == QuotaLimitTypeSubscriptionCycle {
			subscriptionLimit = l
		} else if l.Type == QuotaLimitTypeToken {
			paygLimit = l
		}
	}
	require.Equal(t, "exhausted", subscriptionLimit.Status)
	require.Equal(t, "available", paygLimit.Status)
}

func TestApertis_CheckQuota_SubscriptionExhausted_NoPAYGFallback(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 0,
					"token_used": 10.0,
					"token_total": 10.0,
					"token_remaining": 0,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "lite",
					"status": "active",
					"cycle_quota_limit": 600,
					"cycle_quota_used": 600,
					"cycle_quota_remaining": 0,
					"cycle_start": "2026-01-01T00:00:00Z",
					"cycle_end": "2026-02-01T00:00:00Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	// Cycle quota exhausted, no PAYG fallback, no PAYG credits
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestApertis_CheckQuota_SubscriberWithUnlimitedPayg(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Real production response: active subscription, unlimited PAYG with 0 credits, no fallback
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 0,
					"token_used": 0,
					"token_total": "unlimited",
					"token_remaining": "unlimited",
					"token_is_unlimited": true
				},
				"subscription": {
					"plan_type": "lite",
					"status": "active",
					"cycle_quota_limit": 600,
					"cycle_quota_used": 183,
					"cycle_quota_remaining": 417,
					"cycle_start": "2026-05-20T23:28:04Z",
					"cycle_end": "2026-06-20T23:28:04Z",
					"payg_fallback_enabled": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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
	// Only subscription limit — PAYG skipped for active subscription
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaLimitTypeSubscriptionCycle, quota.Limits[0].Type)
	require.Equal(t, "available", quota.Limits[0].Status)
	// usageRatio = 183/600 ≈ 0.305
	require.InDelta(t, 0.305, quota.Limits[0].UsageRatio, 0.001)
	require.NotNil(t, quota.NextResetAt)

	// RawData should still contain both payg and subscription
	rawData := quota.RawData
	require.Equal(t, true, rawData["is_subscriber"])
	require.NotNil(t, rawData["subscription"])
}

func TestApertis_CheckQuota_UnlimitedTokens(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Matches the official API docs "PAYG with Unlimited Token" response
			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 500.00,
					"token_used": 87.61,
					"token_total": "unlimited",
					"token_remaining": "unlimited",
					"token_is_unlimited": true
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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

func TestApertis_CheckQuota_APIKeysFallback(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer fallback-key", req.Header.Get("Authorization"))
			require.Equal(t, "https://api.apertis.ai/v1/dashboard/billing/credits", req.URL.String())
			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 5.0,
					"token_used": 2.0,
					"token_total": 7.0,
					"token_remaining": 5.0,
					"token_is_unlimited": false
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	})

	checker := NewApertisQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKeys: []string{"fallback-key"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "apertis", quota.ProviderType)
}

func TestApertis_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{invalid json`))}, nil
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
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error": "invalid key"}`))}, nil
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
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://custom.apertis.ai/v1/dashboard/billing/credits", req.URL.String())

			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 5.0,
					"token_used": 0,
					"token_total": 7.0,
					"token_remaining": 7.0
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://api.apertis.ai/v1/dashboard/billing/credits", req.URL.String())

			body := `{
				"object": "billing_credits",
				"is_subscriber": false,
				"payg": {
					"account_credits": 5.0,
					"token_used": 0,
					"token_total": 7.0,
					"token_remaining": 7.0
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 9.98,
					"token_used": 0.05,
					"token_total": 1.0,
					"token_remaining": 0.95,
					"token_is_unlimited": false
				},
				"subscription": {
					"plan_type": "lite",
					"status": "active",
					"cycle_quota_limit": 600,
					"cycle_quota_used": 10,
					"cycle_quota_remaining": 590,
					"cycle_start": "2026-01-15T00:00:00Z",
					"cycle_end": "2026-02-15T12:00:00Z",
					"payg_fallback_enabled": false
				}
			}`
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"object": "billing_credits",
				"is_subscriber": true,
				"payg": {
					"account_credits": 9.98,
					"token_used": 0.05,
					"token_total": 1.0,
					"token_remaining": 0.95,
					"token_is_unlimited": false,
					"token_monthly_limit_usd": 100.0,
					"token_monthly_used_usd": 12.50,
					"monthly_reset_day": 1
				},
				"subscription": {
					"plan_type": "max",
					"status": "active",
					"cycle_quota_limit": 5000,
					"cycle_quota_used": 5000,
					"cycle_quota_remaining": 0,
					"cycle_start": "2026-03-01T00:00:00Z",
					"cycle_end": "2026-04-01T00:00:00Z",
					"payg_fallback_enabled": true,
					"payg_spent_usd": 2.5,
					"payg_limit_usd": 10.0
				}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
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
	require.Equal(t, 9.98, payg["account_credits"])
	require.Equal(t, 0.05, payg["token_used"])
	require.Equal(t, 1.0, payg["token_total"])
	require.Equal(t, 0.95, payg["token_remaining"])
	require.Equal(t, false, payg["token_is_unlimited"])
	require.Equal(t, 100.0, payg["token_monthly_limit_usd"])
	require.Equal(t, 12.50, payg["token_monthly_used_usd"])
	require.Equal(t, 1, payg["monthly_reset_day"])

	subscription := rawData["subscription"].(map[string]any)
	require.Equal(t, "max", subscription["plan_type"])
	require.Equal(t, "active", subscription["status"])
	require.Equal(t, 5000, subscription["cycle_quota_limit"])
	require.Equal(t, 5000, subscription["cycle_quota_used"])
	require.Equal(t, 0, subscription["cycle_quota_remaining"])
	require.Equal(t, "2026-03-01T00:00:00Z", subscription["cycle_start"])
	require.Equal(t, "2026-04-01T00:00:00Z", subscription["cycle_end"])
	require.Equal(t, true, subscription["payg_fallback_enabled"])
	require.Equal(t, 2.5, subscription["payg_spent_usd"])
	require.Equal(t, 10.0, subscription["payg_limit_usd"])
}
