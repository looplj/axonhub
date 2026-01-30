package live

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/watcher"
)

type testItem struct {
	Key       string
	Value     string
	UpdatedAt time.Time
	Deleted   bool
}

func TestIndexedCache_GetWithAutoLoad(t *testing.T) {
	var loadCount int32

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_autoload",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			atomic.AddInt32(&loadCount, 1)

			if key == "notfound" {
				return nil, ErrKeyNotFound
			}

			return &testItem{Key: key, Value: "loaded-" + key}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// Get triggers auto-load
	v, err := cache.Get(context.Background(), "key1")
	assert.NoError(t, err)
	assert.Equal(t, "loaded-key1", v.Value)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loadCount))

	// Second get should hit cache
	v, err = cache.Get(context.Background(), "key1")
	assert.NoError(t, err)
	assert.Equal(t, "loaded-key1", v.Value)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loadCount)) // no additional load

	// Not found case - should cache negative result
	_, err = cache.Get(context.Background(), "notfound")
	assert.ErrorIs(t, err, ErrKeyNotFound)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loadCount))

	// Second not found should hit negative cache, no additional load
	_, err = cache.Get(context.Background(), "notfound")
	assert.ErrorIs(t, err, ErrKeyNotFound)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loadCount)) // no additional load due to negative caching
}

func TestIndexedCache_GetCached(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_getcached",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// GetCached should not trigger load
	_, ok := cache.GetCached("key1")
	assert.False(t, ok)

	// After Get, GetCached should work
	cache.Get(context.Background(), "key1")
	v, ok := cache.GetCached("key1")
	assert.True(t, ok)
	assert.Equal(t, "loaded", v.Value)
}

func TestIndexedCache_Invalidate(t *testing.T) {
	var loadCount int32

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_invalidate",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			count := atomic.AddInt32(&loadCount, 1)
			return &testItem{Key: key, Value: "v" + string('0'+count)}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// Initial load
	v, _ := cache.Get(context.Background(), "key1")
	assert.Equal(t, "v1", v.Value)

	// Invalidate
	cache.Invalidate("key1")

	// Should not be in cache
	_, ok := cache.GetCached("key1")
	assert.False(t, ok)

	// Next Get triggers reload
	v, _ = cache.Get(context.Background(), "key1")
	assert.Equal(t, "v2", v.Value)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loadCount))
}

func TestIndexedCache_Reload(t *testing.T) {
	var loadCount int32

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_reload",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			count := atomic.AddInt32(&loadCount, 1)
			return &testItem{Key: key, Value: "v" + string('0'+count)}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// Initial load
	v, _ := cache.Get(context.Background(), "key1")
	assert.Equal(t, "v1", v.Value)

	// Reload forces fresh load
	v, err := cache.Reload(context.Background(), "key1")
	assert.NoError(t, err)
	assert.Equal(t, "v2", v.Value)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loadCount))
}

func TestIndexedCache_Set(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_set",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// Manual set
	cache.Set("key1", &testItem{Key: "key1", Value: "manual"})

	v, ok := cache.GetCached("key1")
	assert.True(t, ok)
	assert.Equal(t, "manual", v.Value)
}

func TestIndexedCache_SingleFlight(t *testing.T) {
	var loadCount int32

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_singleflight",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			atomic.AddInt32(&loadCount, 1)
			time.Sleep(100 * time.Millisecond) // Simulate slow load

			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// Concurrent loads for same key
	done := make(chan bool)

	for range 5 {
		go func() {
			cache.Get(context.Background(), "key1")

			done <- true
		}()
	}

	for range 5 {
		<-done
	}

	// Should only load once due to singleflight
	assert.Equal(t, int32(1), atomic.LoadInt32(&loadCount))
}

func TestIndexedCache_Load(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_load",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return []*testItem{
				{Key: "key1", Value: "v1"},
				{Key: "key2", Value: "v2"},
			}, time.Now(), nil
		},
	})
	defer cache.Stop()

	err := cache.Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, cache.Len())

	v, ok := cache.GetCached("key1")
	assert.True(t, ok)
	assert.Equal(t, "v1", v.Value)
}

func TestIndexedCache_IncrementalRefresh(t *testing.T) {
	var loadCount int32

	baseTime := time.Now()

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_incremental",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: 50 * time.Millisecond,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			count := atomic.AddInt32(&loadCount, 1)
			if count == 1 {
				return []*testItem{
					{Key: "key1", Value: "v1"},
				}, baseTime, nil
			}
			// Incremental update
			return []*testItem{
				{Key: "key1", Value: "v1-updated"},
				{Key: "key2", Value: "v2"},
			}, baseTime.Add(time.Second), nil
		},
	})
	defer cache.Stop()

	// Initial load
	err := cache.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Len())

	// Wait for periodic refresh
	time.Sleep(150 * time.Millisecond)

	// Should have incremental updates
	v, ok := cache.GetCached("key1")
	assert.True(t, ok)
	assert.Equal(t, "v1-updated", v.Value)

	_, ok = cache.GetCached("key2")
	assert.True(t, ok)
}

func TestIndexedCache_SoftDelete(t *testing.T) {
	var loadCount int32

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_softdelete",
		KeyFunc:         func(v *testItem) string { return v.Key },
		DeletedFunc:     func(v *testItem) bool { return v.Deleted },
		RefreshInterval: 50 * time.Millisecond,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			count := atomic.AddInt32(&loadCount, 1)
			if count == 1 {
				return []*testItem{
					{Key: "key1", Value: "v1"},
					{Key: "key2", Value: "v2"},
				}, time.Now(), nil
			}
			// Mark key1 as deleted
			return []*testItem{
				{Key: "key1", Deleted: true},
			}, time.Now(), nil
		},
	})
	defer cache.Stop()

	err := cache.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, cache.Len())

	// Wait for incremental refresh with deletion
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, 1, cache.Len())
	_, ok := cache.GetCached("key1")
	assert.False(t, ok)
	_, ok = cache.GetCached("key2")
	assert.True(t, ok)
}

func TestIndexedCache_DeletedFuncOnLoadOne(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_deleted_loadone",
		KeyFunc:         func(v *testItem) string { return v.Key },
		DeletedFunc:     func(v *testItem) bool { return v.Deleted },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "v", Deleted: true}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// LoadOne returns deleted item, should not be cached
	_, err := cache.Get(context.Background(), "key1")
	assert.Error(t, err)
	assert.Equal(t, 0, cache.Len())
}

func TestIndexedCache_GetAll(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_getall",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	cache.Set("key1", &testItem{Key: "key1", Value: "v1"})
	cache.Set("key2", &testItem{Key: "key2", Value: "v2"})

	all := cache.GetAll()
	assert.Len(t, all, 2)
	assert.Equal(t, "v1", all["key1"].Value)
	assert.Equal(t, "v2", all["key2"].Value)
}

func TestIndexedCache_TTL(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_ttl",
		TTL:             100 * time.Millisecond,
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// Set and verify
	cache.Set("key1", &testItem{Key: "key1", Value: "v1"})
	v, ok := cache.GetCached("key1")
	assert.True(t, ok)
	assert.Equal(t, "v1", v.Value)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, ok = cache.GetCached("key1")
	assert.False(t, ok)

	// Get should trigger reload
	v, err := cache.Get(context.Background(), "key1")
	assert.NoError(t, err)
	assert.Equal(t, "loaded", v.Value)
}

func TestIndexedCache_TTLWithLoadOne(t *testing.T) {
	var loadCount int32

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_ttl_loadone",
		TTL:             100 * time.Millisecond,
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			atomic.AddInt32(&loadCount, 1)
			return &testItem{Key: key, Value: "loaded-" + key}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// First Get triggers load
	v, err := cache.Get(context.Background(), "key1")
	assert.NoError(t, err)
	assert.Equal(t, "loaded-key1", v.Value)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loadCount))

	// Second Get hits cache
	v, err = cache.Get(context.Background(), "key1")
	assert.NoError(t, err)
	assert.Equal(t, "loaded-key1", v.Value)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loadCount))

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Get should trigger reload after expiration
	v, err = cache.Get(context.Background(), "key1")
	assert.NoError(t, err)
	assert.Equal(t, "loaded-key1", v.Value)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loadCount))
}

func TestIndexedCache_WatcherTriggersIncrementalRefresh(t *testing.T) {
	var loadSinceCount int32

	w := watcher.NewMemoryWatcher[CacheEvent[string]](watcher.MemoryWatcherOptions{Buffer: 1})

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_watcher_indexed",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		DebounceDelay:   10 * time.Millisecond,
		Watcher:         w,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			count := atomic.AddInt32(&loadSinceCount, 1)
			if count == 1 {
				return []*testItem{{Key: "key1", Value: "v1"}}, time.Now(), nil
			}

			return []*testItem{{Key: "key1", Value: "v2"}}, time.Now(), nil
		},
	})
	defer cache.Stop()

	require.NoError(t, w.Notify(context.Background(), NewRefreshEvent[string](time.Now().Add(time.Second))))
	require.Eventually(t, func() bool {
		v, ok := cache.GetCached("key1")
		return ok && v.Value == "v1"
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, w.Notify(context.Background(), NewRefreshEvent[string](time.Now().Add(2*time.Second))))
	require.Eventually(t, func() bool {
		v, ok := cache.GetCached("key1")
		return ok && v.Value == "v2"
	}, time.Second, 10*time.Millisecond)
}

func TestIndexedCache_WatcherInvalidateKeys(t *testing.T) {
	w := watcher.NewMemoryWatcher[CacheEvent[string]](watcher.MemoryWatcherOptions{Buffer: 1})

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_watcher_invalidate",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		DebounceDelay:   10 * time.Millisecond,
		Watcher:         w,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded-" + key}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	_, _ = cache.Get(context.Background(), "key1")
	_, ok := cache.GetCached("key1")
	require.True(t, ok)

	require.NoError(t, w.Notify(context.Background(), NewInvalidateKeysEvent("key1")))
	require.Eventually(t, func() bool {
		_, ok := cache.GetCached("key1")
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestIndexedCache_WatcherReloadKeys(t *testing.T) {
	var loadCount int32

	w := watcher.NewMemoryWatcher[CacheEvent[string]](watcher.MemoryWatcherOptions{Buffer: 1})

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_watcher_reload",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		DebounceDelay:   10 * time.Millisecond,
		Watcher:         w,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			count := atomic.AddInt32(&loadCount, 1)
			return &testItem{Key: key, Value: fmt.Sprintf("v%d", count)}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	v, _ := cache.Get(context.Background(), "key1")
	require.Equal(t, "v1", v.Value)

	require.NoError(t, w.Notify(context.Background(), NewReloadKeysEvent("key1")))
	require.Eventually(t, func() bool {
		v, ok := cache.GetCached("key1")
		return ok && v.Value == "v2"
	}, time.Second, 10*time.Millisecond)
}

func TestIndexedCache_CleanupExpired(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_cleanup",
		TTL:             100 * time.Millisecond,
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// Set items
	cache.Set("key1", &testItem{Key: "key1", Value: "v1"})
	cache.Set("key2", &testItem{Key: "key2", Value: "v2"})
	assert.Equal(t, 2, cache.Len())

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Items should still be in map but expired
	assert.Equal(t, 0, cache.Len()) // Len excludes expired items

	// Cleanup expired
	cache.CleanupExpired()

	// Verify cleanup
	assert.Equal(t, 0, cache.Len())
}

func TestIndexedCache_NoTTL(t *testing.T) {
	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_no_ttl",
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			return &testItem{Key: key, Value: "loaded"}, nil
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// Set without TTL
	cache.Set("key1", &testItem{Key: "key1", Value: "v1"})

	// Should not expire
	time.Sleep(50 * time.Millisecond)

	v, ok := cache.GetCached("key1")
	assert.True(t, ok)
	assert.Equal(t, "v1", v.Value)
}

func TestIndexedCache_LoadOneFuncRequired(t *testing.T) {
	// LoadOneFunc is required, should panic if not provided
	assert.Panics(t, func() {
		NewIndexedCache(IndexedOptions[string, *testItem]{
			Name:            "test_no_loadone",
			KeyFunc:         func(v *testItem) string { return v.Key },
			RefreshInterval: time.Hour,
			LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
				return nil, time.Now(), nil
			},
		})
	})
}

func TestIndexedCache_KeyFuncRequired(t *testing.T) {
	// KeyFunc is required, should panic if not provided
	assert.Panics(t, func() {
		NewIndexedCache(IndexedOptions[string, *testItem]{
			Name:            "test_no_keyfunc",
			RefreshInterval: time.Hour,
			LoadOneFunc:     func(ctx context.Context, key string) (*testItem, error) { return nil, nil },
			LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
				return nil, time.Now(), nil
			},
		})
	})
}

func TestIndexedCache_RefreshIntervalRequired(t *testing.T) {
	// RefreshInterval is required, should panic if not provided
	assert.Panics(t, func() {
		NewIndexedCache(IndexedOptions[string, *testItem]{
			Name:        "test_no_refreshinterval",
			KeyFunc:     func(v *testItem) string { return v.Key },
			LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) { return nil, nil },
			LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
				return nil, time.Now(), nil
			},
			RefreshInterval: 0,
		})
	})
}

func TestIndexedCache_LoadSinceFuncRequired(t *testing.T) {
	// LoadSinceFunc is required, should panic if not provided
	assert.Panics(t, func() {
		NewIndexedCache(IndexedOptions[string, *testItem]{
			Name:            "test_no_loadsince",
			KeyFunc:         func(v *testItem) string { return v.Key },
			LoadOneFunc:     func(ctx context.Context, key string) (*testItem, error) { return nil, nil },
			RefreshInterval: time.Hour,
		})
	})
}

func TestIndexedCache_NegativeCacheWithTTL(t *testing.T) {
	var loadCount int32

	cache := NewIndexedCache(IndexedOptions[string, *testItem]{
		Name:            "test_negative_ttl",
		TTL:             100 * time.Millisecond,
		KeyFunc:         func(v *testItem) string { return v.Key },
		RefreshInterval: time.Hour,
		LoadOneFunc: func(ctx context.Context, key string) (*testItem, error) {
			atomic.AddInt32(&loadCount, 1)
			return nil, ErrKeyNotFound
		},
		LoadSinceFunc: func(ctx context.Context, since time.Time) ([]*testItem, time.Time, error) {
			return nil, time.Now(), nil
		},
	})
	defer cache.Stop()

	// First request - should load and cache negative result
	_, err := cache.Get(context.Background(), "notfound")
	assert.ErrorIs(t, err, ErrKeyNotFound)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loadCount))

	// Second request - should hit negative cache
	_, err = cache.Get(context.Background(), "notfound")
	assert.ErrorIs(t, err, ErrKeyNotFound)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loadCount))

	// Wait for negative TTL expiration
	time.Sleep(6 * time.Second)

	// Third request - should reload after expiration
	_, err = cache.Get(context.Background(), "notfound")
	assert.ErrorIs(t, err, ErrKeyNotFound)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loadCount))
}
