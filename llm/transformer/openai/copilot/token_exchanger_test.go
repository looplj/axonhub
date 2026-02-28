package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func newTestExchanger(t *testing.T, srv *httptest.Server) *TokenExchanger {
	t.Helper()

	hc := httpclient.NewHttpClientWithClient(srv.Client())
	return NewTokenExchanger(TokenExchangerParams{
		HTTPClient: hc,
		Endpoint:   srv.URL + "/copilot_internal/v2/token",
	})
}

func TestTokenExchanger_Exchange_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/copilot_internal/v2/token", r.URL.Path)

		auth := r.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(auth, "token "))
		accessToken := strings.TrimPrefix(auth, "token ")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(copilotTokenResponse{
			Token:     "copilot_token_" + accessToken,
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		})
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)

	token, expiresAt, err := exchanger.Exchange(context.Background(), "test_access_token_123")
	require.NoError(t, err)
	require.Equal(t, "copilot_token_test_access_token_123", token)
	require.Greater(t, expiresAt, time.Now().Unix())
}

func TestTokenExchanger_Exchange_CacheHit(t *testing.T) {
	var requestCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(copilotTokenResponse{
			Token:     "copilot_token_cached",
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		})
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)
	ctx := context.Background()

	token1, _, err := exchanger.Exchange(ctx, "test_access_token")
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)

	token2, _, err := exchanger.Exchange(ctx, "test_access_token")
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)
	require.Equal(t, token1, token2)
}

func TestTokenExchanger_Exchange_ExpiryBuffer(t *testing.T) {
	var requestCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(copilotTokenResponse{
			Token:     "copilot_token_v" + string(rune('0'+requestCount)),
			ExpiresAt: time.Now().Add(3 * time.Minute).Unix(),
		})
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)
	ctx := context.Background()

	token1, _, err := exchanger.Exchange(ctx, "test_access_token")
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)

	token2, _, err := exchanger.Exchange(ctx, "test_access_token")
	require.NoError(t, err)
	require.Equal(t, 2, requestCount)
	require.NotEqual(t, token1, token2)
}

func TestTokenExchanger_Exchange_EmptyAccessToken(t *testing.T) {
	exchanger := NewTokenExchanger(TokenExchangerParams{
		HTTPClient: httpclient.NewHttpClient(),
	})

	_, _, err := exchanger.Exchange(context.Background(), "")
	require.Error(t, err)
}

func TestTokenExchanger_Exchange_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)
	_, _, err := exchanger.Exchange(context.Background(), "test_access_token")
	require.Error(t, err)
}

func TestTokenExchanger_Exchange_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid-json"))
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)
	_, _, err := exchanger.Exchange(context.Background(), "test_access_token")
	require.Error(t, err)
}

func TestTokenExchanger_Exchange_EmptyTokenInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(copilotTokenResponse{
			Token:     "",
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		})
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)
	_, _, err := exchanger.Exchange(context.Background(), "test_access_token")
	require.Error(t, err)
}

func TestTokenExchanger_Exchange_MissingExpiresAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(copilotTokenResponse{
			Token:     "copilot_token",
			ExpiresAt: 0,
		})
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)
	_, _, err := exchanger.Exchange(context.Background(), "test_access_token")
	require.Error(t, err)
}

func TestTokenExchanger_Exchange_Singleflight(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(copilotTokenResponse{
			Token:     "copilot_token",
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		})
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := exchanger.Exchange(context.Background(), "test_access_token")
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, requestCount)
}

func TestTokenExchanger_Exchange_Singleflight_DifferentTokens(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		accessToken := strings.TrimPrefix(r.Header.Get("Authorization"), "token ")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(copilotTokenResponse{
			Token:     "copilot_token_" + accessToken,
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		})
	}))
	defer srv.Close()

	exchanger := newTestExchanger(t, srv)

	var wg sync.WaitGroup
	results := make(chan string, 2)
	errs := make(chan error, 2)

	for _, tok := range []string{"token_a", "token_b"} {
		wg.Add(1)
		go func(accessToken string) {
			defer wg.Done()
			token, _, err := exchanger.Exchange(context.Background(), accessToken)
			if err != nil {
				errs <- err
				return
			}
			results <- token
		}(tok)
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	var got []string
	for v := range results {
		got = append(got, v)
	}
	require.Len(t, got, 2)
	require.NotEqual(t, got[0], got[1])

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, requestCount, fmt.Sprintf("expected 2 requests, got %d", requestCount))
}
