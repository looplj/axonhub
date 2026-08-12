package provider_quota

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestCharmHyper_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			require.Equal(t, "/v1/credits", req.URL.Path)

			body := `{"balance": 80}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "https://hyper.charm.land",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "charm_hyper", quota.ProviderType)
	require.InDelta(t, 0.2, quota.Limits[0].UsageRatio, 0.001)
	require.Nil(t, quota.NextResetAt)
}

func TestCharmHyper_CheckQuota_WarningState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"balance": 15}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "https://hyper.charm.land",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, 0.85, quota.Limits[0].UsageRatio)
}

func TestCharmHyper_CheckQuota_ExhaustedState(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"balance": 0}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "https://hyper.charm.land",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "exhausted", quota.Status)
	require.False(t, quota.Ready)
	require.Equal(t, 1.0, quota.Limits[0].UsageRatio)
}

func TestCharmHyper_CheckQuota_MissingCredentials(t *testing.T) {
	checker := NewCharmHyperQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{}))

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing API key")
}

func TestCharmHyper_CheckQuota_MalformedJSON(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{invalid json}`)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing Charm Hyper quota response")
}

func TestCharmHyper_CheckQuota_MissingBalanceField(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing balance field")
}

func TestCharmHyper_CheckQuota_NullBalance(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"balance":null}`)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing balance field")
}

func TestCharmHyper_CheckQuota_HTTPError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error": "unauthorized"}`)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "status")
}

func TestCharmHyper_CheckQuota_CustomBaseURL(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "custom.charm.land", req.URL.Hostname())
			require.Equal(t, "/v1/credits", req.URL.Path)

			body := `{"balance": 50}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL:     "https://custom.charm.land",
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestCharmHyper_CheckQuota_FallbackKey(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Bearer fallback-key", req.Header.Get("Authorization"))
			body := `{"balance": 50}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{
			APIKeys: []string{"fallback-key"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}

func TestCharmHyper_CheckQuota_WithProxy(t *testing.T) {
	// WithProxy creates a new HttpClient with its own transport,
	// so we must use a real test server rather than a mock transport.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		require.Equal(t, "/v1/credits", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"balance": 50}`))
	}))
	defer server.Close()

	checker := NewCharmHyperQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{}))

	proxyConfig := &httpclient.ProxyConfig{
		Type: httpclient.ProxyTypeDisabled,
	}

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: server.URL,
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
		Settings: &objects.ChannelSettings{
			Proxy: proxyConfig,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Equal(t, "charm_hyper", quota.ProviderType)
}

func TestCharmHyper_CheckQuota_WarningBoundary(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"balance": 20}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewCharmHyperQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		BaseURL: "https://hyper.charm.land",
		Credentials: objects.ChannelCredentials{
			APIKey: "test-api-key",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
}

func TestCharmHyper_SupportsChannel(t *testing.T) {
	checker := NewCharmHyperQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{}))

	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://hyper.charm.land",
	}))

	require.True(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenaiResponses,
		BaseURL: "https://custom.hyper.charm.land",
	}))
}

func TestCharmHyper_SupportsChannel_WrongType(t *testing.T) {
	checker := NewCharmHyperQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{}))

	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeAnthropic,
		BaseURL: "https://hyper.charm.land",
	}))
}

func TestCharmHyper_SupportsChannel_WrongURL(t *testing.T) {
	checker := NewCharmHyperQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{}))

	require.False(t, checker.SupportsChannel(&ent.Channel{
		Type:    channel.TypeOpenai,
		BaseURL: "https://api.openai.com",
	}))
}
