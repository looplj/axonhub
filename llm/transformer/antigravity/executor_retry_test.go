package antigravity

import (
	"context"
	"net/http"
	"testing"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor_RetryOnNilResponseWithError(t *testing.T) {
	// Verify that retry logic works when httpclient returns nil response but *httpclient.Error
	callCount := 0
	mockClient := &mockHTTPClient{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			callCount++
			if callCount == 1 {
				// First call simulates httpclient behavior for 429: nil response, specific error
				return nil, &httpclient.Error{
					Method:     request.Method,
					URL:        request.URL,
					StatusCode: 429,
					Status:     "429 Too Many Requests",
					Body:       []byte(`{"error": "rate limit"}`),
				}
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
	// Should have tried Daily (failed) then Autopush (succeeded)
	assert.Equal(t, EndpointDaily, mockClient.endpoints[0])
	assert.Equal(t, EndpointAutopush, mockClient.endpoints[1])
}

func TestExecutor_NoRetryOnNonRetryableNilResponseWithError(t *testing.T) {
	// Verify that we DO NOT retry for 400 Bad Request
	callCount := 0
	mockClient := &mockHTTPClient{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			callCount++
			return nil, &httpclient.Error{
				Method:     request.Method,
				URL:        request.URL,
				StatusCode: 400,
				Status:     "400 Bad Request",
				Body:       []byte(`{"error": "bad request"}`),
			}
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

	require.Error(t, err) // Should return error
	assert.Nil(t, resp)
	assert.Equal(t, 1, mockClient.callCount) // Should NOT retry
}
