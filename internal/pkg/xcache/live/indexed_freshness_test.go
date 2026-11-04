package live

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/stretchr/testify/require"
)

func TestIndexedCacheSlowLoadCannotRenewSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := NewIndexedCache(IndexedOptions[string, string]{
			Name: "freshness", TTL: time.Second, RefreshInterval: time.Hour,
			KeyFunc: func(v string) string { return v },
			LoadOneFunc: func(context.Context, string) (string, error) {
				time.Sleep(2 * time.Second)
				return "key", nil
			},
			LoadSinceFunc: func(context.Context, time.Time) ([]string, time.Time, error) {
				time.Sleep(2 * time.Second)
				return []string{"key"}, time.Now(), nil
			},
		})
		defer cache.Stop()
		ctx := context.Background()
		_, err := cache.Get(ctx, "key")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorIs(t, cache.Load(ctx), context.DeadlineExceeded)
		require.ErrorIs(t, cache.inner.Load(ctx, true), context.DeadlineExceeded)
		require.Empty(t, cache.index)
		require.True(t, cache.inner.GetLastUpdate().IsZero())
	})
}

func TestIndexedCacheExpiredBatchPreservesIndexAndWatermark(t *testing.T) {
	for _, mode := range []string{"initial", "incremental", "forced"} {
		for _, empty := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/empty=%v", mode, empty), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					previous := time.Now().Add(-time.Minute)
					next := previous.Add(time.Second)
					calls := 0
					var cursors []time.Time
					cache := NewIndexedCache(IndexedOptions[string, string]{
						Name: "batch-freshness", TTL: time.Second, RefreshInterval: time.Hour,
						KeyFunc:     func(string) string { return "key" },
						LoadOneFunc: func(context.Context, string) (string, error) { return "new", nil },
						LoadSinceFunc: func(_ context.Context, since time.Time) ([]string, time.Time, error) {
							cursors = append(cursors, since)
							calls++
							if calls == 1 {
								time.Sleep(2 * time.Second)
								if empty {
									return nil, next, nil
								}
							}
							return []string{"new"}, next, nil
						},
					})
					defer cache.Stop()
					old := &cacheItem[string]{value: "old"}
					cache.index["key"] = old
					cache.inner.SetLastUpdate(previous)
					load := func() error {
						if mode == "initial" {
							return cache.Load(context.Background())
						}
						return cache.inner.Load(context.Background(), mode == "forced")
					}
					require.ErrorIs(t, load(), context.DeadlineExceeded)
					require.Same(t, old, cache.index["key"])
					require.Equal(t, previous, cache.inner.GetLastUpdate())
					require.NoError(t, load())
					value, found := cache.GetCached("key")
					require.True(t, found)
					require.Equal(t, "new", value)
					require.Equal(t, next, cache.inner.GetLastUpdate())
					expected := time.Time{}
					if mode == "incremental" {
						expected = previous
					}
					require.Equal(t, []time.Time{expected, expected}, cursors)
				})
			})
		}
	}
}

func TestIndexedCacheLoadOneExpiresWhileWaitingForLock(t *testing.T) {
	loaded := make(chan struct{})
	result := make(chan error, 1)
	var cache *IndexedCache[string, string]
	cache = NewIndexedCache(IndexedOptions[string, string]{
		Name: "lock-freshness", TTL: 500 * time.Millisecond, RefreshInterval: time.Hour,
		KeyFunc: func(v string) string { return v },
		LoadOneFunc: func(context.Context, string) (string, error) {
			cache.mu.Lock()
			close(loaded)
			return "key", nil
		},
		LoadSinceFunc: func(context.Context, time.Time) ([]string, time.Time, error) { return nil, time.Time{}, nil },
	})
	defer cache.Stop()
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(context.Background(), "freshness test loader panicked", log.Any("panic", recovered))
				result <- fmt.Errorf("loader panic: %v", recovered)
			}
		}()
		_, err := cache.Get(context.Background(), "key")
		result <- err
	}()
	<-loaded
	// Mutex waits are not durably blocking in synctest, so this boundary uses real time.
	time.Sleep(600 * time.Millisecond)
	cache.mu.Unlock()
	require.ErrorIs(t, <-result, context.DeadlineExceeded)
	_, found := cache.GetCached("key")
	require.False(t, found)
}
