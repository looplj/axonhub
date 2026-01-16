package codex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
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
	p := NewTokenProvider(xcache.Config{Mode: xcache.ModeMemory}, hc)

	ch := &ent.Channel{
		ID:          1,
		Credentials: &objects.ChannelCredentials{APIKey: `{"access_token":"old","refresh_token":"r","expires_at":"2000-01-01T00:00:00Z"}`},
	}

	ctx := contexts.WithProjectID(context.Background(), 123)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, _, err := p.Get(ctx, ch)
			require.NoError(t, err)
			require.Equal(t, "new", tok)
		}()
	}
	wg.Wait()

	require.Equal(t, int64(1), calls.Load())
}

func TestTokenProviderGet_UsesCacheAfterRefresh(t *testing.T) {
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
	p := NewTokenProvider(xcache.Config{Mode: xcache.ModeMemory}, hc)

	ch := &ent.Channel{
		ID:          1,
		Credentials: &objects.ChannelCredentials{APIKey: `{"access_token":"old","refresh_token":"r","expires_at":"2000-01-01T00:00:00Z"}`},
	}

	ctx := contexts.WithProjectID(context.Background(), 123)

	tok, _, err := p.Get(ctx, ch)
	require.NoError(t, err)
	require.Equal(t, "new", tok)
	require.Equal(t, int64(1), calls.Load())

	tok2, _, err := p.Get(ctx, ch)
	require.NoError(t, err)
	require.Equal(t, "new", tok2)
	require.Equal(t, int64(1), calls.Load())
}
