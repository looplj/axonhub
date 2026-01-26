package antigravity

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// mockHTTPClient is a mock HTTP client for testing.
type mockHTTPClient struct {
	doFunc       func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error)
	doStreamFunc func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error)
	callCount    int
	endpoints    []string
}

func (m *mockHTTPClient) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	m.callCount++
	// Extract endpoint from URL
	m.endpoints = append(m.endpoints, extractEndpoint(request.URL))
	if m.doFunc != nil {
		return m.doFunc(ctx, request)
	}

	return &httpclient.Response{
		StatusCode: 200,
		Body:       []byte(`{"response": {"candidates": []}}`),
	}, nil
}

func (m *mockHTTPClient) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	m.callCount++

	m.endpoints = append(m.endpoints, extractEndpoint(request.URL))
	if m.doStreamFunc != nil {
		return m.doStreamFunc(ctx, request)
	}

	return nil, nil
}

func extractEndpoint(url string) string {
	// Extract base endpoint from URL
	// URL format: https://endpoint.../v1internal:action
	pathStart := -1

	for i := 0; i < len(url); i++ {
		if i+11 < len(url) && url[i:i+11] == "/v1internal" {
			pathStart = i
			break
		}
	}

	if pathStart == -1 {
		return url
	}

	return url[:pathStart]
}

func TestExecutor_DoWithEndpointFallback(t *testing.T) {
	t.Run("succeeds on first endpoint for gemini-cli model", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
				return &httpclient.Response{
					StatusCode: 200,
					Body:       []byte(`{"response": {"candidates": []}}`),
				}, nil
			},
		}

		executor := NewExecutor(mockClient)

		request := &httpclient.Request{
			Method: http.MethodPost,
			URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
			Metadata: map[string]string{
				"antigravity_model": "gemini-2.5-pro", // Gemini CLI quota
			},
		}

		resp, err := executor.Do(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 1, mockClient.callCount)
		// All models now start with Daily for better quota distribution
		assert.Equal(t, EndpointDaily, mockClient.endpoints[0])
	})

	t.Run("succeeds on first endpoint for antigravity model", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
				return &httpclient.Response{
					StatusCode: 200,
					Body:       []byte(`{"response": {"candidates": []}}`),
				}, nil
			},
		}

		executor := NewExecutor(mockClient)

		request := &httpclient.Request{
			Method: http.MethodPost,
			URL:    "https://cloudcode-pa.googleapis.com/v1internal:generateContent",
			Metadata: map[string]string{
				"antigravity_model": "claude-sonnet-4-5", // Antigravity quota
			},
		}

		resp, err := executor.Do(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 1, mockClient.callCount)
		// Should try daily first for antigravity models
		assert.Equal(t, EndpointDaily, mockClient.endpoints[0])
	})

	t.Run("retries on 403 error", func(t *testing.T) {
		callCount := 0
		mockClient := &mockHTTPClient{
			doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
				callCount++
				if callCount == 1 {
					// First call fails with 403
					return &httpclient.Response{
						StatusCode: 403,
						Body:       []byte(`{"error": "forbidden"}`),
					}, nil
				}
				// Second call succeeds
				return &httpclient.Response{
					StatusCode: 200,
					Body:       []byte(`{"response": {"candidates": []}}`),
				}, nil
			},
		}

		executor := NewExecutor(mockClient)

		request := &httpclient.Request{
			Method: http.MethodPost,
			URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
			Metadata: map[string]string{
				"antigravity_model": "claude-sonnet-4-5",
			},
		}

		resp, err := executor.Do(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 2, mockClient.callCount)
		// Should try daily, then autopush
		assert.Equal(t, EndpointDaily, mockClient.endpoints[0])
		assert.Equal(t, EndpointAutopush, mockClient.endpoints[1])
	})

	t.Run("retries on 404 error", func(t *testing.T) {
		callCount := 0
		mockClient := &mockHTTPClient{
			doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
				callCount++
				if callCount < 3 {
					// First two calls fail with 404
					return &httpclient.Response{
						StatusCode: 404,
						Body:       []byte(`{"error": "not found"}`),
					}, nil
				}
				// Third call succeeds
				return &httpclient.Response{
					StatusCode: 200,
					Body:       []byte(`{"response": {"candidates": []}}`),
				}, nil
			},
		}

		executor := NewExecutor(mockClient)

		request := &httpclient.Request{
			Method: http.MethodPost,
			URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
			Metadata: map[string]string{
				"antigravity_model": "claude-sonnet-4-5",
			},
		}

		resp, err := executor.Do(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 3, mockClient.callCount)
		// Should try all three endpoints
		assert.Equal(t, EndpointDaily, mockClient.endpoints[0])
		assert.Equal(t, EndpointAutopush, mockClient.endpoints[1])
		assert.Equal(t, EndpointProd, mockClient.endpoints[2])
	})

	t.Run("retries on 5xx error", func(t *testing.T) {
		callCount := 0
		mockClient := &mockHTTPClient{
			doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
				callCount++
				if callCount == 1 {
					// First call fails with 500
					return &httpclient.Response{
						StatusCode: 500,
						Body:       []byte(`{"error": "internal server error"}`),
					}, nil
				}
				// Second call succeeds
				return &httpclient.Response{
					StatusCode: 200,
					Body:       []byte(`{"response": {"candidates": []}}`),
				}, nil
			},
		}

		executor := NewExecutor(mockClient)

		request := &httpclient.Request{
			Method: http.MethodPost,
			URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
			Metadata: map[string]string{
				"antigravity_model": "claude-sonnet-4-5",
			},
		}

		resp, err := executor.Do(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 2, mockClient.callCount)
	})

	t.Run("does not retry on 400 error", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
				return &httpclient.Response{
					StatusCode: 400,
					Body:       []byte(`{"error": "bad request"}`),
				}, nil
			},
		}

		executor := NewExecutor(mockClient)

		request := &httpclient.Request{
			Method: http.MethodPost,
			URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
			Metadata: map[string]string{
				"antigravity_model": "claude-sonnet-4-5",
			},
		}

		resp, err := executor.Do(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
		assert.Equal(t, 1, mockClient.callCount) // Should not retry
	})

	t.Run("exhausts all endpoints", func(t *testing.T) {
		mockClient := &mockHTTPClient{
			doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
				return &httpclient.Response{
					StatusCode: 403,
					Body:       []byte(`{"error": "forbidden"}`),
				}, nil
			},
		}

		executor := NewExecutor(mockClient)

		request := &httpclient.Request{
			Method: http.MethodPost,
			URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
			Metadata: map[string]string{
				"antigravity_model": "claude-sonnet-4-5",
			},
		}

		resp, err := executor.Do(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, 403, resp.StatusCode)    // Returns last error
		assert.Equal(t, 3, mockClient.callCount) // Tried all three endpoints
	})

	t.Run("handles network errors", func(t *testing.T) {
		callCount := 0
		mockClient := &mockHTTPClient{
			doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
				callCount++
				if callCount == 1 {
					return nil, fmt.Errorf("network error")
				}

				return &httpclient.Response{
					StatusCode: 200,
					Body:       []byte(`{"response": {"candidates": []}}`),
				}, nil
			},
		}

		executor := NewExecutor(mockClient)

		request := &httpclient.Request{
			Method: http.MethodPost,
			URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
			Metadata: map[string]string{
				"antigravity_model": "claude-sonnet-4-5",
			},
		}

		resp, err := executor.Do(context.Background(), request)

		// Network errors should return immediately without retrying
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, 1, mockClient.callCount)
	})
}

func TestExecutor_ReplaceBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		originalURL string
		newBase     string
		expected    string
	}{
		{
			name:        "replace daily with prod",
			originalURL: "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
			newBase:     "https://cloudcode-pa.googleapis.com",
			expected:    "https://cloudcode-pa.googleapis.com/v1internal:generateContent",
		},
		{
			name:        "replace prod with autopush",
			originalURL: "https://cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse",
			newBase:     "https://autopush-cloudcode-pa.sandbox.googleapis.com",
			expected:    "https://autopush-cloudcode-pa.sandbox.googleapis.com/v1internal:streamGenerateContent?alt=sse",
		},
		{
			name:        "no path found returns original",
			originalURL: "https://example.com/other/path",
			newBase:     "https://newbase.com",
			expected:    "https://example.com/other/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceBaseURL(tt.originalURL, tt.newBase)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutor_GetEndpointsInOrder(t *testing.T) {
	executor := NewExecutor(nil)

	t.Run("gemini-cli model starts with daily", func(t *testing.T) {
		// All models now start with Daily for better quota distribution (matches reference)
		endpoints := executor.getEndpointsInOrder("gemini-2.5-pro")
		require.Len(t, endpoints, 3)
		assert.Equal(t, EndpointDaily, endpoints[0])
		assert.Contains(t, endpoints, EndpointProd)
		assert.Contains(t, endpoints, EndpointAutopush)
	})

	t.Run("antigravity model starts with daily", func(t *testing.T) {
		endpoints := executor.getEndpointsInOrder("claude-sonnet-4-5")
		require.Len(t, endpoints, 3)
		assert.Equal(t, EndpointDaily, endpoints[0])
		assert.Contains(t, endpoints, EndpointProd)
		assert.Contains(t, endpoints, EndpointAutopush)
	})

	t.Run("explicit suffix works with model", func(t *testing.T) {
		// All models start with Daily regardless of suffix (quota preference)
		endpoints := executor.getEndpointsInOrder("gemini-2.5-pro:antigravity")
		require.Len(t, endpoints, 3)
		assert.Equal(t, EndpointDaily, endpoints[0])
	})

	t.Run("no model uses default fallback order", func(t *testing.T) {
		endpoints := executor.getEndpointsInOrder("")
		require.Len(t, endpoints, 3)
		assert.Equal(t, EndpointDaily, endpoints[0])
		assert.Equal(t, EndpointAutopush, endpoints[1])
		assert.Equal(t, EndpointProd, endpoints[2])
	})
}

func TestExecutor_DoWithCooldown_SkipsFailedEndpoint(t *testing.T) {
	callCount := 0

	var lastEndpoint string

	mockClient := &mockHTTPClient{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			callCount++
			endpoint := extractEndpoint(request.URL)
			lastEndpoint = endpoint

			// Daily always fails with 429
			if endpoint == EndpointDaily {
				return &httpclient.Response{
					StatusCode: 429,
					Body:       []byte(`{"error": "rate limit exceeded"}`),
				}, nil
			}

			// Other endpoints succeed
			return &httpclient.Response{
				StatusCode: 200,
				Body:       []byte(`{"response": {"candidates": []}}`),
			}, nil
		},
	}

	executor := NewExecutor(mockClient)

	request := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
		Metadata: map[string]string{
			"antigravity_model": "claude-sonnet-4-5",
		},
	}

	// First request - tries Daily (fails 429), then Autopush (succeeds)
	resp1, err1 := executor.Do(context.Background(), request)
	require.NoError(t, err1)
	assert.Equal(t, 200, resp1.StatusCode) // Succeeded on fallback
	assert.Equal(t, 2, callCount)          // Tried Daily + Autopush
	assert.Equal(t, EndpointAutopush, lastEndpoint)

	// Second request - should skip Daily (in cooldown) and go straight to Autopush
	callCount = 0
	resp2, err2 := executor.Do(context.Background(), request)
	require.NoError(t, err2)
	assert.Equal(t, 200, resp2.StatusCode)
	assert.Equal(t, 1, callCount) // Only tried Autopush (skipped Daily)
	assert.Equal(t, EndpointAutopush, lastEndpoint)
}

func TestExecutor_DoWithCooldown_FailsFastWhenAllInCooldown(t *testing.T) {
	mockClient := &mockHTTPClient{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			// All endpoints return 503
			return &httpclient.Response{
				StatusCode: 503,
				Body:       []byte(`{"error": "service unavailable"}`),
			}, nil
		},
	}

	executor := NewExecutor(mockClient)

	request := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
		Metadata: map[string]string{
			"antigravity_model": "claude-sonnet-4-5",
		},
	}

	// First request - all endpoints fail and go into cooldown
	resp1, err1 := executor.Do(context.Background(), request)
	require.NoError(t, err1)
	assert.Equal(t, 503, resp1.StatusCode)

	// Second request - should fail fast without trying any endpoint
	resp2, err2 := executor.Do(context.Background(), request)
	require.Error(t, err2)
	assert.Nil(t, resp2)
	assert.Contains(t, err2.Error(), "all antigravity endpoints in cooldown")
}

func TestExecutor_DoWithCooldown_PerModelIsolation(t *testing.T) {
	callsByModel := make(map[string]int)
	mockClient := &mockHTTPClient{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			model := request.Metadata["antigravity_model"]
			callsByModel[model]++
			endpoint := extractEndpoint(request.URL)

			if model == "claude-sonnet-4-5" && endpoint == EndpointDaily {
				// Claude fails on Daily
				return &httpclient.Response{
					StatusCode: 429,
					Body:       []byte(`{"error": "rate limit"}`),
				}, nil
			}

			// Everything else succeeds
			return &httpclient.Response{
				StatusCode: 200,
				Body:       []byte(`{"response": {"candidates": []}}`),
			}, nil
		},
	}

	executor := NewExecutor(mockClient)

	claudeRequest := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
		Metadata: map[string]string{
			"antigravity_model": "claude-sonnet-4-5",
		},
	}

	geminiRequest := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "https://cloudcode-pa.googleapis.com/v1internal:generateContent",
		Metadata: map[string]string{
			"antigravity_model": "gemini-2.5-pro",
		},
	}

	// Claude request fails on Daily (429), succeeds on Autopush (fallback)
	resp1, err1 := executor.Do(context.Background(), claudeRequest)
	require.NoError(t, err1)
	assert.Equal(t, 200, resp1.StatusCode) // Succeeded on fallback

	// Claude should have called Daily (failed) + Autopush (succeeded)
	assert.Equal(t, 2, callsByModel["claude-sonnet-4-5"])

	// Gemini request should try Daily (all models start with Daily)
	// and Daily is a different model so isolation works
	resp2, err2 := executor.Do(context.Background(), geminiRequest)
	require.NoError(t, err2)
	assert.Equal(t, 200, resp2.StatusCode)

	// Verify that gemini called Daily endpoint (all models start with Daily)
	assert.Equal(t, 1, callsByModel["gemini-2.5-pro"])
}

func TestExecutor_DoWithCooldown_RecoveryAfterSuccess(t *testing.T) {
	callCount := 0
	mockClient := &mockHTTPClient{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			callCount++
			endpoint := extractEndpoint(request.URL)

			if callCount == 1 && endpoint == EndpointDaily {
				// First call to Daily fails
				return &httpclient.Response{
					StatusCode: 503,
					Body:       []byte(`{"error": "service unavailable"}`),
				}, nil
			}

			// All other calls succeed
			return &httpclient.Response{
				StatusCode: 200,
				Body:       []byte(`{"response": {"candidates": []}}`),
			}, nil
		},
	}

	executor := NewExecutor(mockClient)

	request := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
		Metadata: map[string]string{
			"antigravity_model": "claude-sonnet-4-5",
		},
	}

	// First request - Daily fails, falls back to Autopush
	resp1, err1 := executor.Do(context.Background(), request)
	require.NoError(t, err1)
	assert.Equal(t, 200, resp1.StatusCode) // Succeeded on fallback

	// Second request - Daily is in cooldown, goes straight to Autopush
	resp2, err2 := executor.Do(context.Background(), request)
	require.NoError(t, err2)
	assert.Equal(t, 200, resp2.StatusCode)

	// Autopush succeeded, so it should NOT be in cooldown
	// Third request should still skip Daily (in cooldown) but use Autopush
	resp3, err3 := executor.Do(context.Background(), request)
	require.NoError(t, err3)
	assert.Equal(t, 200, resp3.StatusCode)
}

func TestExecutor_DoStreamWithCooldown_SkipsFailedEndpoint(t *testing.T) {
	callCount := 0

	var lastEndpoint string

	mockClient := &mockHTTPClient{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			callCount++
			endpoint := extractEndpoint(request.URL)
			lastEndpoint = endpoint

			// Daily always fails
			if endpoint == EndpointDaily {
				return nil, fmt.Errorf("connection error")
			}

			// Other endpoints succeed
			return nil, nil
		},
	}

	executor := NewExecutor(mockClient)

	request := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:streamGenerateContent?alt=sse",
		Metadata: map[string]string{
			"antigravity_model": "claude-sonnet-4-5",
		},
	}

	// First request - tries Daily (fails), then Autopush (succeeds)
	_, err1 := executor.DoStream(context.Background(), request)
	require.NoError(t, err1) // Succeeded on fallback
	assert.Equal(t, 2, callCount)
	assert.Equal(t, EndpointAutopush, lastEndpoint)

	// Second request - should skip Daily and go straight to Autopush
	callCount = 0
	_, err2 := executor.DoStream(context.Background(), request)
	require.NoError(t, err2)
	assert.Equal(t, 1, callCount) // Only tried Autopush (skipped Daily)
	assert.Equal(t, EndpointAutopush, lastEndpoint)
}
