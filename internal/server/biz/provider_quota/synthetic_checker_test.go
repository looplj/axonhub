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

func TestSynthetic_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))

			body := `{
				"subscription": {"limit": 750, "requests": 0, "renewsAt": "2026-04-25T08:47:39.947Z"},
				"search": {"hourly": {"limit": 250, "requests": 0}},
				"weeklyTokenLimit": {
					"nextRegenAt": "2026-04-25T04:48:26.000Z",
					"percentRemaining": 38.6,
					"maxCredits": "$36.00",
					"remainingCredits": "$13.90"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2026-04-25T04:00:01.000Z",
					"tickPercent": 0.05,
					"remaining": 750,
					"max": 750,
					"limited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "synthetic", quota.ProviderType)
}

func TestSynthetic_CheckQuota_WarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"subscription": {"limit": 750, "requests": 0, "renewsAt": "2026-04-25T08:47:39.947Z"},
				"weeklyTokenLimit": {
					"nextRegenAt": "2026-04-25T04:48:26.000Z",
					"percentRemaining": 15.0,
					"maxCredits": "$36.00",
					"remainingCredits": "$5.40"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2026-04-25T04:00:01.000Z",
					"tickPercent": 0.85,
					"remaining": 112,
					"max": 750,
					"limited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}

func TestSynthetic_CheckQuota_ExhaustedState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"subscription": {"limit": 750, "requests": 750, "renewsAt": "2026-04-25T08:47:39.947Z"},
				"weeklyTokenLimit": {
					"nextRegenAt": "2026-04-25T04:48:26.000Z",
					"percentRemaining": 0.0,
					"maxCredits": "$36.00",
					"remainingCredits": "$0.00"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2026-04-25T04:00:01.000Z",
					"tickPercent": 1.0,
					"remaining": 0,
					"max": 750,
					"limited": true
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestSynthetic_CheckQuota_MissingCredentials(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no API key")
}

func TestSynthetic_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`not json`)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse synthetic usage response")
}

func TestSynthetic_CheckQuota_HTTPError(t *testing.T) {
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

	checker := NewSyntheticQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestSynthetic_CheckQuota_CustomBaseURL(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://us-east.api.synthetic.new/v2/quotas", req.URL.String())

			body := `{
				"subscription": {"limit": 750, "requests": 0, "renewsAt": "2026-04-25T08:47:39.947Z"},
				"weeklyTokenLimit": {
					"nextRegenAt": "2026-04-25T04:48:26.000Z",
					"percentRemaining": 50.0,
					"maxCredits": "$36.00",
					"remainingCredits": "$18.00"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2026-04-25T04:00:01.000Z",
					"tickPercent": 0.5,
					"remaining": 375,
					"max": 750,
					"limited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "https://us-east.api.synthetic.new",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
}

func TestSynthetic_SupportsChannel(t *testing.T) {
	checker := NewSyntheticQuotaChecker(nil)

	// TypeOpenai + Synthetic URL → true
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.synthetic.new",
	}))
	// TypeOpenaiResponses + Synthetic URL → true
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenaiResponses,
		BaseURL: "https://api.synthetic.new",
	}))
	// TypeOpenai + non-Synthetic URL → false
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.openai.com",
	}))
	// Non-OpenAI type + Synthetic URL → false
	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeNanogpt,
		BaseURL: "https://api.synthetic.new",
	}))
	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.synthetic.new:443",
	}))
}

func TestSynthetic_CheckQuota_NextResetAt(t *testing.T) {
	expectedTime, _ := time.Parse(time.RFC3339, "2026-04-25T04:00:01.000Z")

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// rollingFiveHourLimit.nextTickAt is earliest
			body := `{
				"subscription": {"limit": 750, "requests": 0, "renewsAt": "2026-04-25T08:47:39.947Z"},
				"weeklyTokenLimit": {
					"nextRegenAt": "2026-04-25T04:48:26.000Z",
					"percentRemaining": 38.6,
					"maxCredits": "$36.00",
					"remainingCredits": "$13.90"
				},
				"rollingFiveHourLimit": {
					"nextTickAt": "2026-04-25T04:00:01.000Z",
					"tickPercent": 0.05,
					"remaining": 750,
					"max": 750,
					"limited": false
				}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewSyntheticQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedTime, *quota.NextResetAt)
}

func TestBuildSyntheticQuotaURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "empty base URL uses default",
			baseURL:  "",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "whitespace-only base URL uses default",
			baseURL:  "  ",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "valid https URL extracts scheme and host",
			baseURL:  "https://api.synthetic.new",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "URL with path strips path",
			baseURL:  "https://api.synthetic.new/v1/chat",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "URL with trailing slash strips path",
			baseURL:  "https://api.synthetic.new/",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "http URL is upgraded to https",
			baseURL:  "http://api.synthetic.new",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "invalid URL uses default",
			baseURL:  "://invalid",
			expected: "https://api.synthetic.new/v2/quotas",
		},
		{
			name:     "subdomain URL",
			baseURL:  "https://us-east.api.synthetic.new",
			expected: "https://us-east.api.synthetic.new/v2/quotas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSyntheticQuotaURL(tt.baseURL)
			require.Equal(t, tt.expected, result)
		})
	}
}
