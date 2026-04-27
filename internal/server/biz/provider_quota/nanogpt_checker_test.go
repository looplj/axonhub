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

func TestNanoGPT_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))

			body := `{
				"active": true,
				"provider": "openai",
				"providerStatus": "active",
				"state": "active",
				"limits": {"weeklyInputTokens": 1000000, "dailyInputTokens": 200000, "dailyImages": 50},
				"dailyInputTokens": {"used": 1000, "remaining": 199000, "percentUsed": 0.5, "resetAt": 1717200000000},
				"weeklyInputTokens": {"used": 5000, "remaining": 995000, "percentUsed": 0.05, "resetAt": 1717804800000}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "nanogpt", quota.ProviderType)
}

func TestNanoGPT_CheckQuota_Warning(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"active": true,
				"state": "active",
				"weeklyInputTokens": {"used": 850000, "remaining": 150000, "percentUsed": 0.85, "resetAt": 1717804800000}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}

func TestNanoGPT_CheckQuota_Exhausted(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"active": false,
				"state": "inactive"
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
}

func TestNanoGPT_CheckQuota_Grace(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"active": true,
				"state": "grace",
				"graceUntil": "2025-04-30T00:00:00Z"
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}

func TestNanoGPT_CheckQuota_MissingCredentials(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no API key")
}

func TestNanoGPT_CheckQuota_APIError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "bad-key",
		},
	})
	require.Error(t, err)
}

func TestNanoGPT_CheckQuota_NextResetAt(t *testing.T) {
	expectedResetAt := time.UnixMilli(1717200000000)

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{
				"active": true,
				"state": "active",
				"dailyInputTokens": {"used": 1000, "remaining": 199000, "percentUsed": 0.5, "resetAt": 1717200000000},
				"weeklyInputTokens": {"used": 5000, "remaining": 995000, "percentUsed": 0.05, "resetAt": 1717804800000}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedResetAt, *quota.NextResetAt)
}

func TestNanoGPT_CheckQuota_NullWindow(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// dailyInputTokens absent, only weeklyInputTokens present
			body := `{
				"active": true,
				"state": "active",
				"weeklyInputTokens": {"used": 5000, "remaining": 995000, "percentUsed": 0.05, "resetAt": 1717804800000}
			}`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)

	windowsRaw, ok := quota.RawData["windows"]
	require.True(t, ok)
	windowsMap, ok := windowsRaw.(map[string]any)
	require.True(t, ok)

	_, hasDaily := windowsMap["dailyInputTokens"]
	require.False(t, hasDaily)

	_, hasWeekly := windowsMap["weeklyInputTokens"]
	require.True(t, hasWeekly)

	require.NotNil(t, quota.NextResetAt)
	expectedResetAt := time.UnixMilli(1717804800000)
	require.Equal(t, expectedResetAt, *quota.NextResetAt)
}

func TestNanoGPT_CheckQuota_GraceUntilAsNextResetAt(t *testing.T) {
	graceUntil := "2025-04-30T00:00:00Z"
	expectedTime, _ := time.Parse(time.RFC3339, graceUntil)

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"active": true, "state": "grace", "graceUntil": "2025-04-30T00:00:00Z"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.NotNil(t, quota.NextResetAt)
	require.Equal(t, expectedTime, *quota.NextResetAt)
}

func TestNanoGPT_SupportsChannel(t *testing.T) {
	checker := NewNanoGPTQuotaChecker(nil)

	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeNanogpt}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeNanogptResponses}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeClaudecode}))
}

func TestBuildNanoGPTQuotaURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "empty base URL uses default",
			baseURL:  "",
			expected: "https://nano-gpt.com/api/subscription/v1/usage",
		},
		{
			name:     "whitespace-only base URL uses default",
			baseURL:  "  ",
			expected: "https://nano-gpt.com/api/subscription/v1/usage",
		},
		{
			name:     "valid https URL extracts scheme and host",
			baseURL:  "https://api.nano-gpt.com",
			expected: "https://api.nano-gpt.com/api/subscription/v1/usage",
		},
		{
			name:     "URL with path strips path",
			baseURL:  "https://api.nano-gpt.com/v1/chat",
			expected: "https://api.nano-gpt.com/api/subscription/v1/usage",
		},
		{
			name:     "URL with trailing slash strips path",
			baseURL:  "https://api.nano-gpt.com/",
			expected: "https://api.nano-gpt.com/api/subscription/v1/usage",
		},
		{
			name:     "http URL is upgraded to https",
			baseURL:  "http://api.nano-gpt.com",
			expected: "https://api.nano-gpt.com/api/subscription/v1/usage",
		},
		{
			name:     "invalid URL uses default",
			baseURL:  "://invalid",
			expected: "https://nano-gpt.com/api/subscription/v1/usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildNanoGPTQuotaURL(tt.baseURL)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNanoGPT_CheckQuota_UnknownState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"active": true, "state": "suspended"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "unknown", quota.Status)
	require.False(t, quota.Ready)
}

func TestNanoGPT_CheckQuota_APIKeysFallback(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer fallback-key", req.Header.Get("Authorization"))
			body := `{"active": true, "state": "active"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey:  "",
			APIKeys: []string{"fallback-key"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestNanoGPT_CheckQuota_InvalidJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`not json`)),
			}, nil
		}),
	})

	checker := NewNanoGPTQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse nanogpt usage response")
}
