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

func TestWafer_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))

			body := `{
				"endpoint": "pass.wafer.ai",
				"billing_model": "pass_quota",
				"plan_tier": "pro",
				"window_start": "2026-04-25T00:00:00+00:00",
				"window_end": "2026-04-25T05:00:00+00:00",
				"request_count": 58,
				"included_request_limit": 5000,
				"included_request_count": 58,
				"remaining_included_requests": 4942,
				"overage_request_count": 0,
				"current_period_used_percent": 1.2,
				"input_tokens": 2468184,
				"output_tokens": 12148,
				"total_tokens": 2480332
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewWaferQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "wafer", quota.ProviderType)
}

func TestWafer_CheckQuota_WarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"endpoint": "pass.wafer.ai",
				"billing_model": "pass_quota",
				"plan_tier": "pro",
				"window_start": "2026-04-25T00:00:00+00:00",
				"window_end": "2026-04-25T05:00:00+00:00",
				"request_count": 4250,
				"included_request_limit": 5000,
				"included_request_count": 4250,
				"remaining_included_requests": 750,
				"overage_request_count": 0,
				"current_period_used_percent": 85.0,
				"input_tokens": 2468184,
				"output_tokens": 12148,
				"total_tokens": 2480332
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewWaferQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}

func TestWafer_CheckQuota_ExhaustedState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"endpoint": "pass.wafer.ai",
				"billing_model": "pass_quota",
				"plan_tier": "pro",
				"window_start": "2026-04-25T00:00:00+00:00",
				"window_end": "2026-04-25T05:00:00+00:00",
				"request_count": 5050,
				"included_request_limit": 5000,
				"included_request_count": 5000,
				"remaining_included_requests": 0,
				"overage_request_count": 50,
				"current_period_used_percent": 101.0,
				"input_tokens": 2468184,
				"output_tokens": 12148,
				"total_tokens": 2480332
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewWaferQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestWafer_CheckQuota_MissingCredentials(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	})

	checker := NewWaferQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no API key")
}

func TestWafer_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`not json`)),
			}, nil
		}),
	})

	checker := NewWaferQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse wafer usage response")
}

func TestWafer_CheckQuota_HTTPError(t *testing.T) {
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

	checker := NewWaferQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestWafer_CheckQuota_CustomBaseURL(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Verify the URL was built from the custom base URL
			require.Equal(t, "https://custom.wafer.ai/v1/inference/quota", req.URL.String())

			body := `{
				"endpoint": "custom.wafer.ai",
				"billing_model": "pass_quota",
				"plan_tier": "pro",
				"window_start": "2026-04-25T00:00:00+00:00",
				"window_end": "2026-04-25T05:00:00+00:00",
				"request_count": 10,
				"included_request_limit": 5000,
				"included_request_count": 10,
				"remaining_included_requests": 4990,
				"overage_request_count": 0,
				"current_period_used_percent": 0.2,
				"input_tokens": 1000,
				"output_tokens": 500,
				"total_tokens": 1500
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewWaferQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "https://custom.wafer.ai",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
}

func TestWafer_SupportsChannel(t *testing.T) {
	checker := NewWaferQuotaChecker(nil)

	// TypeOpenai + Wafer URL → true
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://pass.wafer.ai",
	}))
	// TypeOpenaiResponses + Wafer URL → true
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenaiResponses,
		BaseURL: "https://pass.wafer.ai",
	}))
	// TypeOpenai + non-Wafer URL → false
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.openai.com",
	}))
	// Non-OpenAI type + Wafer URL → false
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeNanogpt,
		BaseURL: "https://pass.wafer.ai",
	}))
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://pass.wafer.ai:443",
	}))
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://evilwafer.ai",
	}))
}

func TestWafer_CheckQuota_APIKeysFallback(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer fallback-key", req.Header.Get("Authorization"))

			body := `{
				"endpoint": "pass.wafer.ai",
				"billing_model": "pass_quota",
				"plan_tier": "pro",
				"window_start": "2026-04-25T00:00:00+00:00",
				"window_end": "2026-04-25T05:00:00+00:00",
				"request_count": 10,
				"included_request_limit": 5000,
				"included_request_count": 10,
				"remaining_included_requests": 4990,
				"overage_request_count": 0,
				"current_period_used_percent": 0.2,
				"input_tokens": 1000,
				"output_tokens": 500,
				"total_tokens": 1500
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewWaferQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey:  "",
			APIKeys: []string{"fallback-key"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
}

func TestWafer_CheckQuota_ZeroRemainingWithoutOverage(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"endpoint": "pass.wafer.ai",
				"billing_model": "pass_quota",
				"plan_tier": "pro",
				"window_start": "2026-04-25T00:00:00+00:00",
				"window_end": "2026-04-25T05:00:00+00:00",
				"request_count": 5000,
				"included_request_limit": 5000,
				"included_request_count": 5000,
				"remaining_included_requests": 0,
				"overage_request_count": 0,
				"current_period_used_percent": 100.0,
				"input_tokens": 2468184,
				"output_tokens": 12148,
				"total_tokens": 2480332
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewWaferQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}
