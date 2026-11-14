package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestTokenProvider_AutoRefresh_StartStop_Idempotent(t *testing.T) {
	p := NewTokenProvider(TokenProviderParams{
		Credentials: &OAuthCredentials{
			AccessToken: "access",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		},
	})

	p.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: 1 * time.Minute,
	})
	p.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: 1 * time.Minute,
	})

	time.Sleep(30 * time.Millisecond)

	p.StopAutoRefresh()
	p.StopAutoRefresh()

	p.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: 1 * time.Minute,
	})
	p.StopAutoRefresh()
}

func TestTokenProvider_AutoRefresh_StopCancelsSchedule(t *testing.T) {
	var refreshCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalls.Add(1)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "refresh_token", form.Get("grant_type"))
		require.Equal(t, "client-1", form.Get("client_id"))
		require.Equal(t, "refresh-1", form.Get("refresh_token"))

		resp := TokenResponse{
			AccessToken: "access-" + time.Now().Format(time.RFC3339Nano),
			TokenType:   "Bearer",
			ExpiresIn:   1,
		}
		b, err := json.Marshal(resp)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	var onRefreshedCalls atomic.Int32

	p := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL},
		Credentials: &OAuthCredentials{
			ClientID:     "client-1",
			AccessToken:  "access-0",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(-1 * time.Hour),
		},
		OnRefreshed: func(ctx context.Context, refreshed *OAuthCredentials) error {
			onRefreshedCalls.Add(1)
			return nil
		},
	})

	p.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: 950 * time.Millisecond,
	})

	deadline := time.Now().Add(2 * time.Second)

	for {
		if onRefreshedCalls.Load() >= 2 {
			break
		}

		if time.Now().After(deadline) {
			require.FailNow(t, "auto refresh did not trigger enough times", "onRefreshed=%d, refreshCalls=%d", onRefreshedCalls.Load(), refreshCalls.Load())
		}

		time.Sleep(10 * time.Millisecond)
	}

	p.StopAutoRefresh()

	stoppedRefreshed := onRefreshedCalls.Load()
	stoppedRefreshCalls := refreshCalls.Load()

	time.Sleep(200 * time.Millisecond)

	require.Equal(t, stoppedRefreshed, onRefreshedCalls.Load())
	require.Equal(t, stoppedRefreshCalls, refreshCalls.Load())
}

// TestTokenProvider_AutoRefresh_PermanentFailureStopsRetrying captures the old
// behavior: an invalid_grant response was retried forever on the fixed fallback
// interval, even though every retry was guaranteed to fail.
func TestTokenProvider_AutoRefresh_PermanentFailureStopsRetrying(t *testing.T) {
	var refreshCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token revoked"}`))
	}))
	t.Cleanup(server.Close)

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL},
		Credentials: &OAuthCredentials{
			ClientID:     "client-1",
			AccessToken:  "expired-access",
			RefreshToken: "revoked-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	})

	provider.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: 1 * time.Minute,
	})

	require.Eventually(t, func() bool {
		provider.autoMu.Lock()
		defer provider.autoMu.Unlock()
		return provider.autoCancel == nil
	}, 2*time.Second, 10*time.Millisecond, "auto refresh must stop after a permanent credential failure")

	provider.StopAutoRefresh()

	require.Equal(t, int32(1), refreshCalls.Load(), "permanent failures must not be retried in the background")
}

func TestAutoRefreshBackoff_BoundedExponentialAndReset(t *testing.T) {
	t.Parallel()

	backoff := newAutoRefreshBackoff(100*time.Millisecond, 5*time.Second)

	first := backoff.next()
	require.GreaterOrEqual(t, first, 100*time.Millisecond)
	require.LessOrEqual(t, first, 125*time.Millisecond)

	second := backoff.next()
	require.GreaterOrEqual(t, second, 200*time.Millisecond)
	require.LessOrEqual(t, second, 250*time.Millisecond)

	var last time.Duration
	for range 10 {
		last = backoff.next()
	}
	require.Equal(t, 5*time.Second, last)

	backoff.reset()

	afterReset := backoff.next()
	require.GreaterOrEqual(t, afterReset, 100*time.Millisecond)
	require.LessOrEqual(t, afterReset, 125*time.Millisecond)
}

func TestIsPermanentOAuthRefreshError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "invalid_grant_400",
			err:  &httpclient.Error{StatusCode: http.StatusBadRequest, Body: []byte(`{"error":"invalid_grant"}`)},
			want: true,
		},
		{
			name: "invalid_client_401",
			err:  &httpclient.Error{StatusCode: http.StatusUnauthorized, Body: []byte(`{"error":"invalid_client"}`)},
			want: true,
		},
		{
			name: "unauthorized_client_403",
			err:  &httpclient.Error{StatusCode: http.StatusForbidden, Body: []byte(`{"error":"unauthorized_client"}`)},
			want: true,
		},
		{
			name: "nested_error_code",
			err:  &httpclient.Error{StatusCode: http.StatusBadRequest, Body: []byte(`{"error":{"code":"invalid_grant"}}`)},
			want: true,
		},
		{
			name: "typed_token_response_error",
			err:  &tokenResponseError{Code: "invalid_grant"},
			want: true,
		},
		{
			name: "bare_401_is_transient",
			err:  &httpclient.Error{StatusCode: http.StatusUnauthorized, Body: []byte(`Unauthorized`)},
			want: false,
		},
		{
			name: "proxy_html_401_is_transient",
			err:  &httpclient.Error{StatusCode: http.StatusUnauthorized, Body: []byte(`<html>login required</html>`)},
			want: false,
		},
		{
			name: "invalid_grant_with_500_is_transient",
			err:  &httpclient.Error{StatusCode: http.StatusInternalServerError, Body: []byte(`{"error":"invalid_grant"}`)},
			want: false,
		},
		{
			name: "rate_limit_is_transient",
			err:  &httpclient.Error{StatusCode: http.StatusTooManyRequests, Body: []byte(`{"error":"invalid_grant"}`)},
			want: false,
		},
		{
			name: "unsupported_grant_type_is_transient",
			err:  &httpclient.Error{StatusCode: http.StatusBadRequest, Body: []byte(`{"error":"unsupported_grant_type"}`)},
			want: false,
		},
		{
			name: "network_error_is_transient",
			err:  errors.New("dial tcp 127.0.0.1:1: connect: connection refused"),
			want: false,
		},
		{
			name: "nil_error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, isPermanentOAuthRefreshError(tt.err))
		})
	}
}

func TestTokenProvider_AutoRefresh_TransientFailuresBackOffThenRecover(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"temporarily_unavailable"}`))

			return
		}

		_ = writeTokenResponse(w, "recovered-access", "recovered-refresh", 3600)
	}))
	t.Cleanup(server.Close)

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL},
		Credentials: &OAuthCredentials{
			AccessToken:  "expired-access",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	})

	startAutoRefreshForTest(t, provider, autoRefreshConfig{
		refreshBefore:    time.Minute,
		fallbackInterval: 100 * time.Millisecond,
		failureBase:      5 * time.Millisecond,
		failureMax:       20 * time.Millisecond,
	})

	require.Eventually(t, func() bool {
		return accessTokenForTest(provider) == "recovered-access"
	}, 2*time.Second, 5*time.Millisecond, "transient failures should eventually recover")
	require.GreaterOrEqual(t, calls.Load(), int32(3))
}

func TestTokenProvider_AutoRefresh_StopCancelsPendingBackoff(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"temporarily_unavailable"}`))
	}))
	t.Cleanup(server.Close)

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL},
		Credentials: &OAuthCredentials{
			AccessToken:  "expired-access",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	})

	startAutoRefreshForTest(t, provider, autoRefreshConfig{
		refreshBefore:    time.Minute,
		fallbackInterval: time.Second,
		failureBase:      100 * time.Millisecond,
		failureMax:       200 * time.Millisecond,
	})

	require.Eventually(t, func() bool {
		return calls.Load() >= 1
	}, time.Second, 5*time.Millisecond)

	provider.StopAutoRefresh()

	stoppedCalls := calls.Load()
	time.Sleep(300 * time.Millisecond)

	require.Equal(t, stoppedCalls, calls.Load(), "StopAutoRefresh must interrupt the backoff wait")
}

func TestTokenProvider_AutoRefresh_PermanentFailureDoesNotBlockExplicitCallsOrRestart(t *testing.T) {
	var mode atomic.Int32
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if mode.Load() == 0 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))

			return
		}

		_ = writeTokenResponse(w, "fresh-access", "fresh-refresh", 3600)
	}))
	t.Cleanup(server.Close)

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL},
		Credentials: &OAuthCredentials{
			AccessToken:  "expired-access",
			RefreshToken: "stale-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	})

	provider.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: time.Minute,
	})

	require.Eventually(t, func() bool {
		provider.autoMu.Lock()
		defer provider.autoMu.Unlock()
		return provider.autoCancel == nil
	}, 2*time.Second, 10*time.Millisecond)

	require.Equal(t, int32(1), calls.Load())

	mode.Store(1)

	creds, err := provider.Get(context.Background())
	require.NoError(t, err, "an explicit request must still be able to refresh after background retirement")
	require.Equal(t, "fresh-access", creds.AccessToken)

	provider.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: time.Minute,
	})

	provider.autoMu.Lock()
	restarted := provider.autoCancel != nil
	provider.autoMu.Unlock()
	require.True(t, restarted, "auto refresh must be startable again after new credentials are available")

	provider.StopAutoRefresh()
}

type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestTokenProvider_AutoRefresh_DoesNotLogCredentialMaterial(t *testing.T) {
	const (
		accessToken  = "secret-access-token-7f3a"
		refreshToken = "secret-refresh-token-9c1b"
	)

	body, err := json.Marshal(TokenError{
		Error:            "invalid_grant",
		ErrorDescription: "rejected " + accessToken + " / " + refreshToken,
	})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	var logs lockedLogBuffer

	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL},
		Credentials: &OAuthCredentials{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresAt:    time.Now().Add(-time.Hour),
		},
	})

	provider.StartAutoRefresh(context.Background(), AutoRefreshOptions{
		Interval:      10 * time.Millisecond,
		RefreshBefore: time.Minute,
	})

	require.Eventually(t, func() bool {
		provider.autoMu.Lock()
		defer provider.autoMu.Unlock()
		return provider.autoCancel == nil
	}, 2*time.Second, 10*time.Millisecond)

	provider.StopAutoRefresh()

	output := logs.String()
	require.Contains(t, output, "invalid_grant")
	require.NotContains(t, output, accessToken)
	require.NotContains(t, output, refreshToken)
	require.NotContains(t, strings.ToLower(output), "response body")
}

func startAutoRefreshForTest(t *testing.T, provider *TokenProvider, cfg autoRefreshConfig) {
	t.Helper()

	autoCtx, cancel := context.WithCancel(context.Background())

	provider.autoMu.Lock()
	provider.autoSeq++
	id := provider.autoSeq
	provider.autoCancel = cancel
	provider.autoMu.Unlock()

	go provider.runAutoRefresh(autoCtx, cancel, id, cfg)

	t.Cleanup(provider.StopAutoRefresh)
}

func accessTokenForTest(provider *TokenProvider) string {
	provider.mu.RLock()
	defer provider.mu.RUnlock()

	if provider.creds == nil {
		return ""
	}

	return provider.creds.AccessToken
}

func TestSafeOAuthErrorCode(t *testing.T) {
	t.Parallel()

	require.Equal(t, "invalid_grant", safeOAuthErrorCode("invalid_grant"))
	require.Equal(t, "invalid_client", safeOAuthErrorCode("INVALID_CLIENT"))
	require.Equal(t, "unauthorized_client", safeOAuthErrorCode(" unauthorized_client "))
	require.Equal(t, unknownOAuthErrorCode, safeOAuthErrorCode("synthetic-token-in-code"))
}

// TestTokenProvider_AutoRefresh_LogsSafeOAuthErrorCode covers both upstream
// shapes: HTTP 400 (httpclient.Error body) and HTTP 200 with an OAuth error
// object. A provider-controlled code must never reach the log verbatim.
func TestTokenProvider_AutoRefresh_LogsSafeOAuthErrorCode(t *testing.T) {
	const marker = "synthetic-refresh-secret-in-error-code"

	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "http_400", statusCode: http.StatusBadRequest},
		{name: "http_200", statusCode: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(TokenError{
				Error:            marker,
				ErrorDescription: "synthetic description",
			})
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write(body)
			}))
			t.Cleanup(server.Close)

			var logs lockedLogBuffer

			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
			t.Cleanup(func() {
				slog.SetDefault(previousLogger)
			})

			provider := NewTokenProvider(TokenProviderParams{
				HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
				OAuthUrls:  OAuthUrls{TokenUrl: server.URL},
				Credentials: &OAuthCredentials{
					AccessToken:  "synthetic-access",
					RefreshToken: marker,
					ExpiresAt:    time.Now().Add(-time.Hour),
				},
			})

			require.Equal(t, autoRefreshTransientFailure, provider.runAutoRefreshOnce(context.Background(), time.Minute))

			output := logs.String()
			require.Contains(t, output, unknownOAuthErrorCode)
			require.NotContains(t, output, marker)
			require.NotContains(t, output, "response body")
		})
	}
}
