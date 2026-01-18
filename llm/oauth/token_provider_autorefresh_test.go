package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
