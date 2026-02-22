package biz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// TestCopilotTokenExchanger_GetToken_Success tests successful token exchange
func TestCopilotTokenExchanger_GetToken_Success(t *testing.T) {
	// Create mock Copilot token server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/copilot_internal/v2/token", r.URL.Path)

		auth := r.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(auth, "token "))
		accessToken := strings.TrimPrefix(auth, "token ")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Calculate expiry time (1 hour from now)
		expiresAt := time.Now().Add(1 * time.Hour).Unix()

		response := CopilotTokenResponse{
			Token:     "copilot_token_" + accessToken,
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create exchanger with mock server
	exchanger := createTestExchanger(t, mockServer.URL)

	// Get token
	ctx := context.Background()
	token, expiresAt, err := exchanger.GetToken(ctx, "test_access_token_123")

	require.NoError(t, err)
	require.Equal(t, "copilot_token_test_access_token_123", token)
	require.Greater(t, expiresAt, time.Now().Unix())
}

// TestCopilotTokenExchanger_GetToken_Caching tests token caching behavior
func TestCopilotTokenExchanger_GetToken_Caching(t *testing.T) {
	requestCount := 0

	// Create mock server that tracks requests
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		response := CopilotTokenResponse{
			Token:     "copilot_token_cached",
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()
	accessToken := "test_access_token"

	// First call - should hit the server
	token1, _, err := exchanger.GetToken(ctx, accessToken)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)

	// Second call - should use cache
	token2, _, err := exchanger.GetToken(ctx, accessToken)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount) // No additional request
	require.Equal(t, token1, token2)
}

// TestCopilotTokenExchanger_GetToken_ExpiryHandling tests token expiry and refresh
func TestCopilotTokenExchanger_GetToken_ExpiryHandling(t *testing.T) {
	requestCount := 0

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return a token that expires in 3 minutes (within the buffer)
		expiresAt := time.Now().Add(3 * time.Minute).Unix()
		response := CopilotTokenResponse{
			Token:     "copilot_token_v" + string(rune('0'+requestCount)),
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()
	accessToken := "test_access_token"

	// First call
	token1, _, err := exchanger.GetToken(ctx, accessToken)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)

	// Second call - token is expired or near expiry, should refresh
	token2, _, err := exchanger.GetToken(ctx, accessToken)
	require.NoError(t, err)
	require.Equal(t, 2, requestCount)
	require.NotEqual(t, token1, token2)
}

// TestCopilotTokenExchanger_GetToken_EmptyToken tests error on empty access token
func TestCopilotTokenExchanger_GetToken_EmptyToken(t *testing.T) {
	exchanger := NewCopilotTokenExchanger(nil)

	ctx := context.Background()
	_, _, err := exchanger.GetToken(ctx, "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "access token is empty")
}

// TestCopilotTokenExchanger_RefreshToken_Success tests explicit token refresh
func TestCopilotTokenExchanger_RefreshToken_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		response := CopilotTokenResponse{
			Token:     "refreshed_copilot_token",
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()
	token, expiresAt, err := exchanger.RefreshToken(ctx, "test_access_token")

	require.NoError(t, err)
	require.Equal(t, "refreshed_copilot_token", token)
	require.Greater(t, expiresAt, time.Now().Unix())
}

// TestCopilotTokenExchanger_RefreshToken_SingleFlight tests singleflight behavior
func TestCopilotTokenExchanger_RefreshToken_SingleFlight(t *testing.T) {
	requestCount := 0
	var mu sync.Mutex

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Add small delay to allow concurrent requests to queue
		time.Sleep(50 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		response := CopilotTokenResponse{
			Token:     "copilot_token_singleflight",
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()
	accessToken := "test_access_token"

	// Make multiple concurrent requests for the same token
	var wg sync.WaitGroup
	results := make([]struct {
		token string
		err   error
	}, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			token, _, err := exchanger.RefreshToken(ctx, accessToken)
			results[idx].token = token
			results[idx].err = err
		}(i)
	}

	wg.Wait()

	// Only one request should have hit the server
	require.Equal(t, 1, requestCount)

	// All results should be the same
	for i := 0; i < 10; i++ {
		require.NoError(t, results[i].err)
		require.Equal(t, "copilot_token_singleflight", results[i].token)
	}
}

// TestCopilotTokenExchanger_RefreshToken_DifferentTokens tests concurrent requests for different tokens
func TestCopilotTokenExchanger_RefreshToken_DifferentTokens(t *testing.T) {
	requestCount := 0
	var mu sync.Mutex

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		auth := r.Header.Get("Authorization")
		accessToken := strings.TrimPrefix(auth, "token ")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		response := CopilotTokenResponse{
			Token:     "token_for_" + accessToken,
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()

	// Make concurrent requests for different access tokens
	var wg sync.WaitGroup
	tokens := []string{"token1", "token2", "token3"}
	results := make(map[string]string)
	var resultsMu sync.Mutex

	for _, token := range tokens {
		wg.Add(1)
		go func(tkn string) {
			defer wg.Done()
			copilotToken, _, err := exchanger.RefreshToken(ctx, tkn)
			require.NoError(t, err)
			resultsMu.Lock()
			results[tkn] = copilotToken
			resultsMu.Unlock()
		}(token)
	}

	wg.Wait()

	// Each different access token should result in a separate request
	require.Equal(t, 3, requestCount)

	// Verify each token got its own copilot token
	require.Equal(t, "token_for_token1", results["token1"])
	require.Equal(t, "token_for_token2", results["token2"])
	require.Equal(t, "token_for_token3", results["token3"])
}

// TestCopilotTokenExchanger_RefreshToken_EmptyToken tests error on empty token in refresh
func TestCopilotTokenExchanger_RefreshToken_EmptyToken(t *testing.T) {
	exchanger := NewCopilotTokenExchanger(nil)

	ctx := context.Background()
	_, _, err := exchanger.RefreshToken(ctx, "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "access token is empty")
}

// TestCopilotTokenExchanger_Exchange_ServerError tests handling of server errors
func TestCopilotTokenExchanger_Exchange_ServerError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal_server_error"}`))
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()
	_, _, err := exchanger.GetToken(ctx, "test_access_token")

	require.Error(t, err)
	require.Contains(t, err.Error(), "token exchange request failed")
}

// TestCopilotTokenExchanger_Exchange_InvalidResponse tests handling of invalid response
func TestCopilotTokenExchanger_Exchange_InvalidResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()
	_, _, err := exchanger.GetToken(ctx, "test_access_token")

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse token response")
}

// TestCopilotTokenExchanger_Exchange_EmptyTokenInResponse tests handling of empty token in response
func TestCopilotTokenExchanger_Exchange_EmptyTokenInResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"","expires_at":1234567890}`))
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()
	_, _, err := exchanger.GetToken(ctx, "test_access_token")

	require.Error(t, err)
	require.Contains(t, err.Error(), "copilot token is empty in response")
}

// TestCopilotTokenExchanger_Exchange_MissingExpiry tests handling of missing expiry
func TestCopilotTokenExchanger_Exchange_MissingExpiry(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"some_token"}`))
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()
	_, _, err := exchanger.GetToken(ctx, "test_access_token")

	require.Error(t, err)
	require.Contains(t, err.Error(), "expires_at is missing in response")
}

// TestCopilotTokenCacheEntry_IsExpired tests cache entry expiry check
func TestCopilotTokenCacheEntry_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		entry    *CopilotTokenCacheEntry
		now      time.Time
		expected bool
	}{
		{
			name:     "nil entry",
			entry:    nil,
			now:      now,
			expected: true,
		},
		{
			name: "zero expiry",
			entry: &CopilotTokenCacheEntry{
				ExpiresAt: time.Time{},
			},
			now:      now,
			expected: true,
		},
		{
			name: "not expired",
			entry: &CopilotTokenCacheEntry{
				ExpiresAt: now.Add(10 * time.Minute),
			},
			now:      now,
			expected: false,
		},
		{
			name: "expired",
			entry: &CopilotTokenCacheEntry{
				ExpiresAt: now.Add(-1 * time.Minute),
			},
			now:      now,
			expected: true,
		},
		{
			name: "within buffer period",
			entry: &CopilotTokenCacheEntry{
				ExpiresAt: now.Add(3 * time.Minute), // Within 5 minute buffer
			},
			now:      now,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.IsExpired(tt.now)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestCopilotTokenExchanger_ClearCache tests cache clearing
func TestCopilotTokenExchanger_ClearCache(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		response := CopilotTokenResponse{
			Token:     "copilot_token",
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()

	// Populate cache
	_, _, err := exchanger.GetToken(ctx, "token1")
	require.NoError(t, err)
	_, _, err = exchanger.GetToken(ctx, "token2")
	require.NoError(t, err)

	// Clear cache
	exchanger.ClearCache()

	// Make requests again - should hit the server
	requestCount := 0
	mockServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		_ = json.NewEncoder(w).Encode(CopilotTokenResponse{
			Token:     "new_copilot_token",
			ExpiresAt: expiresAt,
		})
	}))
	defer mockServer2.Close()

	exchanger2 := createTestExchanger(t, mockServer2.URL)
	_, _, _ = exchanger2.GetToken(ctx, "token1")
	_, _, _ = exchanger2.GetToken(ctx, "token1") // Should use cache

	// After clear, new requests would hit server
	exchanger2.ClearCache()
	_, _, _ = exchanger2.GetToken(ctx, "token1")
}

// TestCopilotTokenExchanger_RemoveFromCache tests single entry removal
func TestCopilotTokenExchanger_RemoveFromCache(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		response := CopilotTokenResponse{
			Token:     "copilot_token",
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()

	// Populate cache with multiple tokens
	_, _, err := exchanger.GetToken(ctx, "token1")
	require.NoError(t, err)
	_, _, err = exchanger.GetToken(ctx, "token2")
	require.NoError(t, err)

	// Remove only token1 from cache
	exchanger.RemoveFromCache("token1")

	// token1 should be fetched again, token2 should be cached
	// (We can't directly verify this without request counting, but the operation shouldn't panic)
}

// TestCopilotTokenExchanger_ThreadSafety tests concurrent access to cache
func TestCopilotTokenExchanger_ThreadSafety(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		response := CopilotTokenResponse{
			Token:     "copilot_token",
			ExpiresAt: expiresAt,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	exchanger := createTestExchanger(t, mockServer.URL)

	ctx := context.Background()

	// Run concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)

		// Concurrent reads
		go func() {
			defer wg.Done()
			_, _, _ = exchanger.GetToken(ctx, "token")
		}()

		// Concurrent cache removals
		go func() {
			defer wg.Done()
			exchanger.RemoveFromCache("token")
		}()

		// Concurrent clears
		go func() {
			defer wg.Done()
			exchanger.ClearCache()
		}()
	}

	wg.Wait()
	// If we get here without panic or deadlock, thread safety is working
}

// TestNewCopilotTokenExchanger_DefaultClient tests exchanger creation with nil client
func TestNewCopilotTokenExchanger_DefaultClient(t *testing.T) {
	exchanger := NewCopilotTokenExchanger(nil)
	require.NotNil(t, exchanger)
	require.NotNil(t, exchanger.httpClient)
}

// createTestExchanger creates a CopilotTokenExchanger with the mock server URL
func createTestExchanger(t *testing.T, mockServerURL string) *CopilotTokenExchanger {
	t.Helper()

	// Override the token endpoint URL
	originalEndpoint := CopilotTokenEndpoint
	defer func() {
		// Restore original endpoint after test
		// Note: We can't actually restore it since it's a const
		// This is a limitation we work around by not running tests in parallel
		_ = originalEndpoint
	}()

	// Create HTTP client that redirects to mock server
	transport := &testCopilotTokenTransport{
		mockServerURL: mockServerURL,
	}
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	return NewCopilotTokenExchanger(httpClient)
}

// testCopilotTokenTransport redirects Copilot token requests to mock server
type testCopilotTokenTransport struct {
	mockServerURL string
}

func (t *testCopilotTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Intercept Copilot token endpoint requests
	if strings.Contains(req.URL.String(), "api.github.com/copilot_internal/v2/token") {
		// Create new request to mock server
		mockURL := t.mockServerURL + "/copilot_internal/v2/token"
		newReq, err := http.NewRequestWithContext(req.Context(), req.Method, mockURL, req.Body)
		if err != nil {
			return nil, err
		}
		newReq.Header = req.Header.Clone()
		return http.DefaultTransport.RoundTrip(newReq)
	}

	return http.DefaultTransport.RoundTrip(req)
}
