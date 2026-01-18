package codex

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestTokenProviderGet_SingleflightDedupesRefresh(t *testing.T) {
	var calls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(server.Close)

	old := DefaultTokenURLs
	DefaultTokenURLs = TokenURLs{Authorize: old.Authorize, Token: server.URL}

	t.Cleanup(func() { DefaultTokenURLs = old })

	hc := httpclient.NewHttpClientWithClient(server.Client())
	p := NewTokenProvider(&OAuth2Credentials{
		AccessToken:  "old",
		RefreshToken: "r",
		ExpiresAt:    time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	}, hc)

	ctx := contexts.WithProjectID(context.Background(), 123)

	start := make(chan struct{})
	errs := make(chan error, 10)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			<-start

			tok, _, err := p.Get(ctx)
			if err != nil {
				errs <- err
				return
			}

			if tok != "new" {
				errs <- fmt.Errorf("unexpected token: %q", tok)
				return
			}
		})
	}

	close(start)

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, int64(1), calls.Load())
}

func TestTokenProviderGet_UsesInMemoryCredentialsAfterRefresh(t *testing.T) {
	var calls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(server.Close)

	old := DefaultTokenURLs
	DefaultTokenURLs = TokenURLs{Authorize: old.Authorize, Token: server.URL}

	t.Cleanup(func() { DefaultTokenURLs = old })

	hc := httpclient.NewHttpClientWithClient(server.Client())
	p := NewTokenProvider(&OAuth2Credentials{
		AccessToken:  "old",
		RefreshToken: "r",
		ExpiresAt:    time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	}, hc)

	ctx := contexts.WithProjectID(context.Background(), 123)

	tok, _, err := p.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "new", tok)
	require.Equal(t, int64(1), calls.Load())

	tok2, _, err := p.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "new", tok2)
	require.Equal(t, int64(1), calls.Load())
}
