package biz

import (
	"context"
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

func newCatalogTestService(t *testing.T) (context.Context, *SystemService, *CatalogService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })

	ctx := authz.WithTestBypass(context.Background())
	ctx = ent.NewContext(ctx, client)

	system := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})

	return ctx, system, NewCatalogService(system, nil)
}

func useCatalogUpstream(t *testing.T, ctx context.Context, system *SystemService, svc *CatalogService, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	require.NoError(t, system.SetCatalogSettings(ctx, CatalogSettings{UpstreamURL: server.URL}))
	svc.httpClient = httpclient.NewHttpClientWithClient(server.Client())
}

func TestCatalogService_FallsBackThenFetchesAfterInvalidate(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)

	var online atomic.Bool
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		if !online.Load() {
			http.Error(w, "offline", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"providers":{"openai":{"id":"openai","models":[{"id":"gpt-hot"}]}}}`))
	})
	payload, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceFallback, payload.Source)
	require.True(t, payload.Filtered)
	online.Store(true)
	held, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceFallback, held.Source, "the held fallback is served until it is released")

	svc.Invalidate()
	fetched, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceUpstream, fetched.Source)
	require.True(t, fetched.Filtered)
	require.Contains(t, string(fetched.Data), "gpt-hot")
	require.WithinDuration(t, time.Now().UTC(), *fetched.FetchedAt, 5*time.Second)
}

func TestCatalogService_CachesFallbackAndDeduplicatesFetch(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)

	var hits atomic.Int32
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(150 * time.Millisecond)
		http.Error(w, "unavailable", http.StatusBadGateway)
	})

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

func TestCatalogService_NothingIsResidentAfterTTL(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)
	svc.holdTTL = 200 * time.Millisecond

	body := []byte(`{"providers":{` +
		`"openai":{"id":"openai","tags":["x"],"models":[{"id":"gpt-hot","cost":{"input":1}}]},` +
		`"openrouter":{"id":"openrouter","models":[{"id":"openai/gpt-hot","cost":{"input":2}}]}` +
		`},"updated_at":"2026-11-08"}`)

	var hits atomic.Int32
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})

	require.Nil(t, svc.held(true))
	require.Nil(t, svc.held(false))

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			snapshot, err := svc.Snapshot(ctx, false)
			require.NoError(t, err)
			require.False(t, snapshot.Filtered)
			require.Equal(t, catalogSourceUpstream, snapshot.Source)
			require.JSONEq(t, string(body), string(snapshot.Data))
		})
	}
	wg.Wait()
	require.Equal(t, int32(1), hits.Load(), "concurrent unfiltered requests share one fetch")

	_, err := svc.Snapshot(ctx, false)
	require.NoError(t, err)
	require.Equal(t, int32(1), hits.Load(), "unfiltered catalog is reused within its TTL")

	filtered, err := svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, int32(2), hits.Load())
	require.Contains(t, string(filtered.Data), "gpt-hot")
	require.NotContains(t, string(filtered.Data), "openrouter")

	_, err = svc.Snapshot(ctx, true)
	require.NoError(t, err)
	require.Equal(t, int32(2), hits.Load(), "filtered catalog is reused within its TTL")

	require.Eventually(t, func() bool { return svc.held(true) == nil && svc.held(false) == nil },
		2*time.Second, 20*time.Millisecond, "both catalogs are released after their TTL")

	_, err = svc.Snapshot(ctx, true)
	require.NoError(t, err)
	_, err = svc.Snapshot(ctx, false)
	require.NoError(t, err)
	require.Equal(t, int32(4), hits.Load(), "released catalogs are fetched again on demand")
}

func TestCatalogService_InvalidUpstreamServesHeldFallback(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)

	responses := []string{`{"providers":[]}`, `{"providers":{`}
	var hits atomic.Int32
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		i := int(hits.Add(1)) - 1
		if i >= len(responses) {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(responses[i]))
	})

	for range 2 {
		svc.Invalidate()
		snapshot, err := svc.Snapshot(ctx, false)
		require.NoError(t, err)
		require.Equal(t, catalogSourceFallback, snapshot.Source)
		require.Equal(t, string(catalogFallbackJSON), string(snapshot.Data))
	}

	_, err := svc.Snapshot(ctx, false)
	require.NoError(t, err)
	require.Equal(t, int32(2), hits.Load(), "fallback is held for the TTL instead of retrying upstream")
}

func TestCatalogService_CancelledCallerDoesNotCauseFallback(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)

	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"providers":{"openai":{"id":"openai","models":[{"id":"gpt-hot"}]}}}`))
	})

	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	snapshot, err := svc.Snapshot(cancelled, true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceUpstream, snapshot.Source)
	require.Contains(t, string(snapshot.Data), "gpt-hot")
}

func TestCatalogImportCancellationLeavesSharedFetchRunning(t *testing.T) {
	f := catalogFixture(t)
	scope := CatalogImportScope{Providers: []string{"moonshot-ai"}}
	release := make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	defer finish()
	loading, cancel := context.WithCancel(f.ctx)
	defer cancel()
	var hits atomic.Int32
	useCatalogUpstream(t, f.ctx, f.svc.systemService, f.svc.catalog, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		cancel()
		select {
		case <-r.Context().Done():
			t.Error("canceling the preview stopped the shared request")
		case <-release:
			_, _ = w.Write([]byte(testCatalog))
		}
	})

	cancelled, stop := context.WithCancel(f.ctx)
	stop()
	_, err := f.svc.CatalogImportPreview(cancelled, scope)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, hits.Load())

	_, err = f.svc.CatalogImportPreview(loading, scope)
	require.ErrorIs(t, err, context.Canceled)
	finish()
	require.Equal(t, catalogSourceUpstream, f.preview(t, scope).Source)
	require.EqualValues(t, 1, hits.Load())
}

func TestCatalogService_MirrorChangeDiscardsInflightCacheFill(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return
		}
		_, _ = w.Write([]byte(`{"providers":{"old":{}}}`))
	})
	// Use the already established transport for both servers.
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"providers":{"new":{}}}`)) }))
	t.Cleanup(mirror.Close)
	var wg sync.WaitGroup
	wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("catalog snapshot panicked: %v", r)
			}
		}()
		if _, err := svc.Snapshot(ctx, false); err != nil {
			t.Errorf("catalog snapshot: %v", err)
		}
	})
	finish := sync.OnceFunc(func() { close(release); wg.Wait() })
	defer finish()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("old mirror did not receive the request")
	}
	other := NewSystemService(SystemServiceParams{Ent: ent.FromContext(ctx), CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
	require.NoError(t, other.SetCatalogSettings(ctx, CatalogSettings{UpstreamURL: mirror.URL}))
	fresh, err := svc.Snapshot(ctx, false)
	require.NoError(t, err)
	require.Equal(t, mirror.URL, fresh.upstreamURL)
	finish()
	held, err := svc.Snapshot(ctx, false)
	require.NoError(t, err)
	require.Equal(t, mirror.URL, held.upstreamURL)
	require.JSONEq(t, `{"providers":{"new":{}}}`, string(held.Data))
}

func (s *CatalogService) held(filtered bool) *catalogPayload {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.slot(filtered).payload
}

func TestCatalogImportRejectsMirrorSwitch(t *testing.T) {
	for _, when := range []string{"before submit", "during download"} {
		t.Run(when, func(t *testing.T) {
			f := catalogFixture(t)
			other := NewSystemService(SystemServiceParams{Ent: f.db, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
			mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(testCatalog))
			}))
			t.Cleanup(mirror.Close)
			var hits atomic.Int32
			useCatalogUpstream(t, f.ctx, f.svc.systemService, f.svc.catalog, func(w http.ResponseWriter, _ *http.Request) {
				if hits.Add(1) == 2 && when == "during download" {
					ctx, cancel := context.WithTimeout(f.ctx, 3*time.Second)
					defer cancel()
					if err := other.SetCatalogSettings(ctx, CatalogSettings{UpstreamURL: mirror.URL}); err != nil {
						t.Errorf("switch mirror: %v", err)
					}
				}
				_, _ = w.Write([]byte(testCatalog))
			})
			scope := CatalogImportScope{Providers: []string{"moonshot-ai"}}
			p := f.preview(t, scope)
			if when == "before submit" {
				require.NoError(t, other.SetCatalogSettings(f.ctx, CatalogSettings{UpstreamURL: mirror.URL}))
			}
			require.Equal(t, []string{"catalogChanged"}, f.apply(t, scope, p, selection(t, p, "kimi-k3")))
			require.Zero(t, f.db.Model.Query().CountX(f.ctx))
			require.Equal(t, mirror.URL, f.preview(t, scope).UpstreamURL)
		})
	}
}

func TestCatalogService_SettingsReadErrorSkipsDownload(t *testing.T) {
	f := catalogFixture(t)
	scope := CatalogImportScope{Channels: true}
	f.preview(t, scope)
	hits := f.requests.Load()
	require.NoError(t, f.svc.systemService.setSystemValue(f.ctx, SystemKeyCatalogSettings, "{"))
	_, err := f.svc.CatalogImportPreview(f.ctx, scope)
	require.ErrorContains(t, err, "unmarshal catalog settings")
	require.Equal(t, hits, f.requests.Load())
}

func TestModelCatalogOffersPagination(t *testing.T) {
	ctx, system, svc := newCatalogTestService(t)
	var requests atomic.Int32
	var changed atomic.Bool
	useCatalogUpstream(t, ctx, system, svc, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if changed.Load() {
			_, _ = w.Write([]byte(`{"providers":{"other":{"models":[{"id":"other"}]},"test":{"models":[{"id":"z"},{"id":"a","cost":{"input":6}}]}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"providers":{"other":{"models":[{"id":"other"}]},"test":{"models":[{"id":"z"},{"id":"a","cost":{"input":5}}]}}}`))
	})
	page, err := svc.ModelCatalogOffers(ctx, []string{"test"}, 1, "", true)
	require.NoError(t, err)
	require.Equal(t, catalogSourceUpstream, page.Source)
	require.Len(t, page.Offers, 1)
	require.Equal(t, "a", page.Offers[0].ModelID)
	require.Equal(t, 5.0, *page.Offers[0].Rates[0].Base)
	require.NotNil(t, page.NextCursor)

	next, err := svc.ModelCatalogOffers(ctx, []string{"test"}, 1, *page.NextCursor, false)
	require.NoError(t, err)
	require.Equal(t, page.FetchedAt, next.FetchedAt)
	require.Len(t, next.Offers, 1)
	require.Equal(t, "z", next.Offers[0].ModelID)
	require.Nil(t, next.NextCursor)
	require.EqualValues(t, 1, requests.Load())

	replica := NewCatalogService(system, svc.httpClient)
	next, err = replica.ModelCatalogOffers(ctx, []string{"test"}, 1, *page.NextCursor, false)
	require.NoError(t, err)
	require.Len(t, next.Offers, 1)
	require.Equal(t, "z", next.Offers[0].ModelID)
	require.EqualValues(t, 2, requests.Load())

	refreshed, err := svc.ModelCatalogOffers(ctx, []string{"test"}, 1, "", true)
	require.NoError(t, err)
	require.Equal(t, page.NextCursor, refreshed.NextCursor)
	next, err = svc.ModelCatalogOffers(ctx, []string{"test"}, 1, *page.NextCursor, false)
	require.NoError(t, err)
	require.Len(t, next.Offers, 1)
	require.Equal(t, "z", next.Offers[0].ModelID)

	changed.Store(true)
	_, err = svc.ModelCatalogOffers(ctx, []string{"test"}, 1, "", true)
	require.NoError(t, err)
	_, err = svc.ModelCatalogOffers(ctx, []string{"test"}, 1, *page.NextCursor, false)
	require.ErrorContains(t, err, "snapshot changed")
	require.EqualValues(t, 4, requests.Load())
}
