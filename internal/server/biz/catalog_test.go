package biz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestCatalogService_FallsBackAndRefreshes(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	ctx := authz.WithTestBypass(context.Background())
	ctx = ent.NewContext(ctx, client)

	system := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	svc := NewCatalogService(system, nil)

	snapshot := svc.fallbackCache()
	require.Equal(t, catalogSourceFallback, snapshot.source)
	require.NotEmpty(t, snapshot.raw.Providers)

	payload, err := svc.snapshotFromCache(snapshot, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceFallback, payload.Source)
	require.True(t, payload.Filtered)
	require.Contains(t, string(payload.Data), `"providers"`)

	upstream := catalogFile{Providers: map[string]catalogProvider{
		"openai": {ID: "openai", Models: []map[string]any{{"id": "gpt-hot"}}},
	}}
	body, err := json.Marshal(upstream)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	require.NoError(t, system.SetCatalogSettings(ctx, CatalogSettings{
		UpstreamURL:    server.URL,
		RefreshSeconds: 60,
	}))

	svc.httpClient = httpclient.NewHttpClientWithClient(server.Client())
	refreshed, err := svc.Refresh(ctx)
	require.NoError(t, err)
	require.Equal(t, catalogSourceUpstream, refreshed.Source)
	require.Contains(t, string(refreshed.Data), "gpt-hot")
	require.WithinDuration(t, time.Now().UTC(), *refreshed.FetchedAt, 5*time.Second)

	svc.Invalidate()
	_, err = svc.snapshotFromCache(nil, true)
	require.NoError(t, err)
}

func TestCatalogService_CachesFallbackAndDeduplicatesFetch(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	ctx := authz.WithTestBypass(context.Background())
	ctx = ent.NewContext(ctx, client)

	system := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	svc := NewCatalogService(system, nil)

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(150 * time.Millisecond)
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	require.NoError(t, system.SetCatalogSettings(ctx, CatalogSettings{
		UpstreamURL:    server.URL,
		RefreshSeconds: 3600,
	}))
	svc.httpClient = httpclient.NewHttpClientWithClient(server.Client())

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		sources []string
	)
	for range 8 {
		wg.Go(func() {
			snapshot, err := svc.Snapshot(ctx, true)
			require.NoError(t, err)
			mu.Lock()
			sources = append(sources, snapshot.Source)
			mu.Unlock()
		})
	}
	wg.Wait()

	require.Len(t, sources, 8)
	for _, source := range sources {
		require.Equal(t, catalogSourceFallback, source)
	}
	require.Equal(t, int32(1), hits.Load())

	second, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceFallback, second.Source)
	require.Equal(t, int32(1), hits.Load())
}
