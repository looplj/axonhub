package live

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/watcher"
)

func TestCache(t *testing.T) {
	var callCount int32

	refreshFunc := func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		atomic.AddInt32(&callCount, 1)

		now := time.Now()
		if lastUpdate.IsZero() {
			return "initial", now, true, nil
		}

		return "updated", now, true, nil
	}

	cache := NewCache(Options[string]{
		Name:            "test_cache",
		RefreshFunc:     refreshFunc,
		RefreshInterval: time.Hour, // Long interval for manual control
	})
	defer cache.Stop()

	// Test Initial Load
	err := cache.Load(context.Background(), false)
	assert.NoError(t, err)
	assert.Equal(t, "initial", cache.GetData())
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Test Skip Refresh (Fingerprint)
	// Mock refreshFunc to return changed=false
	cache.refreshFunc = func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		atomic.AddInt32(&callCount, 1)
		return "", lastUpdate, false, nil
	}
	err = cache.Load(context.Background(), false)
	assert.NoError(t, err)
	assert.Equal(t, "initial", cache.GetData())
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))

	// Test Force Refresh
	cache.refreshFunc = func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		atomic.AddInt32(&callCount, 1)
		return "forced", time.Now(), true, nil
	}
	err = cache.Load(context.Background(), true)
	assert.NoError(t, err)
	assert.Equal(t, "forced", cache.GetData())
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}

func TestCache_SingleFlight(t *testing.T) {
	var callCount int32

	refreshFunc := func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(100 * time.Millisecond) // Simulate slow load

		return "data", time.Now(), true, nil
	}

	cache := NewCache(Options[string]{
		Name:            "test_sf",
		RefreshFunc:     refreshFunc,
		RefreshInterval: time.Hour,
	})
	defer cache.Stop()

	// Concurrent loads
	done := make(chan bool)

	for range 5 {
		go func() {
			_ = cache.Load(context.Background(), true)

			done <- true
		}()
	}

	for range 5 {
		<-done
	}

	// Should only be called once due to SingleFlight
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestCache_AsyncReload(t *testing.T) {
	var callCount int32

	refreshFunc := func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		atomic.AddInt32(&callCount, 1)
		return "async_data", time.Now(), true, nil
	}

	cache := NewCache(Options[string]{
		Name:            "test_async",
		RefreshFunc:     refreshFunc,
		RefreshInterval: time.Hour,
		DebounceDelay:   100 * time.Millisecond, // Shorter debounce for faster test
	})
	defer cache.Stop()

	// Trigger multiple times rapidly
	for range 10 {
		cache.TriggerAsyncReload()
	}

	// Wait for debounce and execution
	time.Sleep(500 * time.Millisecond)

	// Due to debounce and serial reloadMu, callCount should be small (likely 1 or 2)
	count := atomic.LoadInt32(&callCount)
	assert.True(t, count > 0 && count <= 2, "Expected 1 or 2 calls, got %d", count)
	assert.Equal(t, "async_data", cache.GetData())
}

func TestCache_PeriodicRefresh(t *testing.T) {
	var callCount int32

	refreshFunc := func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		atomic.AddInt32(&callCount, 1)
		return "periodic_data", time.Now(), true, nil
	}

	cache := NewCache(Options[string]{
		Name:            "test_periodic",
		RefreshFunc:     refreshFunc,
		RefreshInterval: 100 * time.Millisecond,
	})
	defer cache.Stop()

	// Wait for a few periodic refreshes
	time.Sleep(350 * time.Millisecond)

	count := atomic.LoadInt32(&callCount)
	assert.True(t, count >= 2, "Expected at least 2 periodic refreshes, got %d", count)
	assert.Equal(t, "periodic_data", cache.GetData())
}

func TestCache_OnSwap(t *testing.T) {
	var (
		swapCalled         int32
		oldValue, newValue string
	)

	refreshFunc := func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		return "new_data", time.Now(), true, nil
	}

	cache := NewCache(Options[string]{
		Name:            "test_onswap",
		RefreshFunc:     refreshFunc,
		InitialValue:    "old_data",
		RefreshInterval: time.Hour,
		OnSwap: func(old, new string) {
			atomic.AddInt32(&swapCalled, 1)

			oldValue = old
			newValue = new
		},
	})
	defer cache.Stop()

	err := cache.Load(context.Background(), true)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&swapCalled))
	assert.Equal(t, "old_data", oldValue)
	assert.Equal(t, "new_data", newValue)
}

func TestCache_Stop(t *testing.T) {
	var callCount int32

	refreshFunc := func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		atomic.AddInt32(&callCount, 1)
		return "data", time.Now(), true, nil
	}

	cache := NewCache(Options[string]{
		Name:            "test_stop",
		RefreshFunc:     refreshFunc,
		RefreshInterval: 50 * time.Millisecond,
	})

	// Wait for some refreshes
	time.Sleep(200 * time.Millisecond)

	// Stop the cache
	cache.Stop()

	// Small delay to ensure any in-flight refresh completes
	time.Sleep(100 * time.Millisecond)

	countBefore := atomic.LoadInt32(&callCount)

	// Wait and verify no more refreshes
	time.Sleep(200 * time.Millisecond)

	countAfter := atomic.LoadInt32(&callCount)

	assert.Equal(t, countBefore, countAfter, "No refreshes should happen after Stop")
}

func TestCache_WatcherReload(t *testing.T) {
	var callCount int32

	refreshFunc := func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
		count := atomic.AddInt32(&callCount, 1)
		return "v" + string('0'+count), time.Now(), true, nil
	}

	w := watcher.NewMemoryWatcher[CacheEvent[struct{}]](watcher.MemoryWatcherOptions{Buffer: 1})

	cache := NewCache(Options[string]{
		Name:            "test_watcher",
		RefreshFunc:     refreshFunc,
		RefreshInterval: time.Hour,
		DebounceDelay:   10 * time.Millisecond,
		Watcher:         w,
	})
	defer cache.Stop()

	require.NoError(t, w.Notify(context.Background(), NewRefreshEvent[struct{}](time.Now())))

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&callCount) >= 1 && cache.GetData() != ""
	}, time.Second, 10*time.Millisecond)

	assert.NotEmpty(t, cache.GetData())
}

func TestCache_RefreshFuncRequired(t *testing.T) {
	// RefreshFunc is required, should panic if not provided
	assert.Panics(t, func() {
		NewCache(Options[string]{
			Name:            "test_no_refreshfunc",
			RefreshInterval: time.Hour,
		})
	})
}

func TestCache_RefreshIntervalRequired(t *testing.T) {
	// RefreshInterval is required, should panic if not provided
	assert.Panics(t, func() {
		NewCache(Options[string]{
			Name: "test_no_refreshinterval",
			RefreshFunc: func(ctx context.Context, current string, lastUpdate time.Time) (string, time.Time, bool, error) {
				return "", time.Now(), false, nil
			},
			RefreshInterval: 0,
		})
	})
}
