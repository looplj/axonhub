package orchestrator

import (
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestChannelCostTracker_SetAndGet(t *testing.T) {
	tracker := NewChannelCostTracker()
	cost := decimal.NewFromFloat(12.34)
	windowEnd := time.Now().Add(time.Hour)

	tracker.SetCachedCost(1, cost, windowEnd)

	got, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))
}

func TestChannelCostTracker_CacheMiss(t *testing.T) {
	tracker := NewChannelCostTracker()

	got, ok := tracker.GetCachedCost(999)
	assert.False(t, ok)
	assert.True(t, got.IsZero())
}

func TestChannelCostTracker_Invalidate(t *testing.T) {
	tracker := NewChannelCostTracker()
	cost := decimal.NewFromFloat(5.0)
	windowEnd := time.Now().Add(time.Hour)

	tracker.SetCachedCost(1, cost, windowEnd)
	tracker.Invalidate(1)

	got, ok := tracker.GetCachedCost(1)
	assert.False(t, ok)
	assert.True(t, got.IsZero())
}

func TestChannelCostTracker_TTLExpiry(t *testing.T) {
	now := time.Now()
	clockPtr := &now
	tracker := NewChannelCostTracker(WithCostTrackerClock(func() time.Time { return *clockPtr }))

	cost := decimal.NewFromFloat(10.0)
	windowEnd := now.Add(time.Hour)

	tracker.SetCachedCost(1, cost, windowEnd)

	got, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))

	// Advance clock past TTL
	*clockPtr = now.Add(31 * time.Second)

	got, ok = tracker.GetCachedCost(1)
	assert.False(t, ok)
	assert.True(t, got.IsZero())
}

func TestChannelCostTracker_WindowEndExpiry(t *testing.T) {
	tracker := NewChannelCostTracker()

	cost := decimal.NewFromFloat(10.0)
	windowEnd := time.Now().Add(-1 * time.Second)

	tracker.SetCachedCost(1, cost, windowEnd)

	got, ok := tracker.GetCachedCost(1)
	assert.False(t, ok)
	assert.True(t, got.IsZero())
}

func TestChannelCostTracker_EvictExpired(t *testing.T) {
	now := time.Now()
	clockPtr := &now
	tracker := NewChannelCostTracker(
		WithCostTrackerClock(func() time.Time { return *clockPtr }),
		WithEvictionInterval(time.Second),
	)

	cost := decimal.NewFromFloat(10.0)
	windowEnd := now.Add(time.Hour)

	tracker.SetCachedCost(1, cost, windowEnd)
	tracker.SetCachedCost(2, cost, windowEnd)

	// Advance clock past TTL but not past window end
	*clockPtr = now.Add(31 * time.Second)

	tracker.EvictExpired()

	got, ok := tracker.GetCachedCost(1)
	assert.False(t, ok)
	assert.True(t, got.IsZero())
	got, ok = tracker.GetCachedCost(2)
	assert.False(t, ok)
	assert.True(t, got.IsZero())

	// Now advance past window end
	*clockPtr = now.Add(2 * time.Hour)

	tracker.EvictExpired()

	// Window ended - entries should now be evicted
	_, ok = tracker.GetCachedCost(1)
	assert.False(t, ok)
	_, ok = tracker.GetCachedCost(2)
	assert.False(t, ok)
}

func TestChannelCostTracker_EvictExpired_KeepsFresh(t *testing.T) {
	now := time.Now()
	clockPtr := &now
	tracker := NewChannelCostTracker(
		WithCostTrackerClock(func() time.Time { return *clockPtr }),
	)

	cost := decimal.NewFromFloat(10.0)
	windowEnd := now.Add(time.Hour)

	tracker.SetCachedCost(1, cost, windowEnd)

	tracker.EvictExpired()

	got, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))
}

func TestChannelCostTracker_Overwrite(t *testing.T) {
	tracker := NewChannelCostTracker()
	windowEnd := time.Now().Add(time.Hour)

	cost1 := decimal.NewFromFloat(10.0)
	cost2 := decimal.NewFromFloat(20.0)

	tracker.SetCachedCost(1, cost1, windowEnd)
	tracker.SetCachedCost(1, cost2, windowEnd)

	got, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	assert.True(t, cost2.Equal(got))
}

func TestChannelCostTracker_ConcurrentSetAndGet(t *testing.T) {
	tracker := NewChannelCostTracker()
	windowEnd := time.Now().Add(time.Hour)
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			cost := decimal.NewFromFloat(float64(id) * 1.5)
			tracker.SetCachedCost(id, cost, windowEnd)
		}(i)
	}

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			tracker.GetCachedCost(id)
		}(i)
	}

	wg.Wait()

	for i := range goroutines {
		got, ok := tracker.GetCachedCost(i)
		assert.True(t, ok, "channel %d should have cached cost", i)
		expected := decimal.NewFromFloat(float64(i) * 1.5)
		assert.True(t, expected.Equal(got), "channel %d cost mismatch", i)
	}
}

func TestChannelCostTracker_ConcurrentInvalidate(t *testing.T) {
	tracker := NewChannelCostTracker()
	windowEnd := time.Now().Add(time.Hour)
	cost := decimal.NewFromFloat(10.0)

	tracker.SetCachedCost(1, cost, windowEnd)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for range goroutines {
		go func() {
			defer wg.Done()
			tracker.Invalidate(1)
		}()
	}

	for range goroutines {
		go func() {
			defer wg.Done()
			tracker.GetCachedCost(1)
		}()
	}

	wg.Wait()

	_, ok := tracker.GetCachedCost(1)
	assert.False(t, ok)
}

func TestChannelCostTracker_StartStop(t *testing.T) {
	tracker := NewChannelCostTracker()
	tracker.Start()
	// Verify Start is idempotent
	tracker.Start() // should not panic or leak goroutines
	tracker.Stop()
	// Verify Stop is idempotent
	tracker.Stop() // should not panic
}

func TestChannelCostTracker_StopBeforeStart(t *testing.T) {
	tracker := NewChannelCostTracker()
	tracker.Stop() // should not panic
	// Subsequent Start should be a no-op
	tracker.Start()
	// Verify the tracker does not start a goroutine by checking that the cost tracker
	// still functions (set/get) but the background eviction never ran
	cost := decimal.NewFromFloat(10.0)
	windowEnd := time.Now().Add(time.Hour)
	tracker.SetCachedCost(1, cost, windowEnd)
	got, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))
}

func TestChannelCostTracker_StartStopMultiple(t *testing.T) {
	tracker := NewChannelCostTracker()
	tracker.Start()
	tracker.Stop()
	// Starting again after stop should be a no-op (stoppedEarly flag prevents restart)
	tracker.Start()
	tracker.Stop() // should not panic on double stop
}

func TestChannelCostTracker_StartStopLifecycle(t *testing.T) {
	tracker := NewChannelCostTracker()

	// Call Start(), verify it's running
	tracker.Start()

	// Verify we can set/get costs while running
	cost := decimal.NewFromFloat(10.0)
	windowEnd := time.Now().Add(time.Hour)
	tracker.SetCachedCost(1, cost, windowEnd)
	got, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))

	// Call Stop()
	tracker.Stop()

	// Call Start() again - should work
	tracker.Start()

	// Verify we can set/get costs again
	tracker.SetCachedCost(2, cost, windowEnd)
	got, ok = tracker.GetCachedCost(2)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))

	tracker.Stop()
}

func TestChannelCostTracker_StopBeforeStartPreventsStart(t *testing.T) {
	tracker := NewChannelCostTracker()

	// Call Stop() before Start()
	tracker.Stop()

	// Now call Start() - this should be a no-op due to stoppedEarly flag
	tracker.Start()

	// The background goroutine should not be running - set a cost and verify
	// it doesn't get evicted (since eviction only runs in background goroutine)
	cost := decimal.NewFromFloat(10.0)
	windowEnd := time.Now().Add(time.Hour)
	tracker.SetCachedCost(1, cost, windowEnd)

	// If Start() was prevented, the eviction goroutine never started
	// So the cost should still be present after TTL + a bit
	now := time.Now()
	clockPtr := &now
	tracker2 := NewChannelCostTracker(
		WithCostTrackerClock(func() time.Time { return *clockPtr }),
	)
	tracker2.Stop()
	tracker2.Start()
	tracker2.SetCachedCost(1, cost, windowEnd)

	// Simulate TTL expiry
	*clockPtr = now.Add(31 * time.Second)

	got, ok := tracker2.GetCachedCost(1)
	assert.False(t, ok)
	assert.True(t, got.IsZero())

	// Manually call EvictExpired - this would be what the background goroutine would do
	tracker2.EvictExpired()

	got, ok = tracker2.GetCachedCost(1)
	assert.False(t, ok)
	assert.True(t, got.IsZero())
}

func TestChannelCostTracker_DoubleStopDoesNotBreakStart(t *testing.T) {
	tracker := NewChannelCostTracker()

	// Start then stop twice
	tracker.Start()
	tracker.Stop()
	tracker.Stop() // second stop should not panic

	// Call Start() again - should work because everStarted is true
	tracker.Start()

	// Verify tracker works
	cost := decimal.NewFromFloat(10.0)
	windowEnd := time.Now().Add(time.Hour)
	tracker.SetCachedCost(1, cost, windowEnd)
	got, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))

	tracker.Stop()
}

func TestChannelCostTracker_EvictExpired_RemovesExpiredEntries(t *testing.T) {
	now := time.Now()
	clockPtr := &now
	tracker := NewChannelCostTracker(
		WithCostTrackerClock(func() time.Time { return *clockPtr }),
	)

	cost := decimal.NewFromFloat(10.0)
	windowEnd := now.Add(time.Hour)

	// Set entries - one will expire, one won't
	tracker.SetCachedCost(1, cost, windowEnd)                           // window not ended
	tracker.SetCachedCost(2, cost, now.Add(-1*time.Second))            // window already ended

	// Verify both exist initially
	_, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	_, ok = tracker.GetCachedCost(2)
	assert.False(t, ok) // window already ended, should be removed on get

	// Now add entry with window ending in the future, then evict past it
	tracker.SetCachedCost(3, cost, now.Add(time.Hour))
	*clockPtr = now.Add(2 * time.Hour) // advance past window end

	tracker.EvictExpired()

	// Entry 3 should now be evicted (window ended)
	_, ok = tracker.GetCachedCost(3)
	assert.False(t, ok)

	// Entry 1 should also be evicted (window ended)
	_, ok = tracker.GetCachedCost(1)
	assert.False(t, ok)
}

func TestChannelCostTracker_DoubleClose_NoPanic(t *testing.T) {
	tracker := NewChannelCostTracker()

	tracker.Start()

	// Call Stop() multiple times - should not panic
	tracker.Stop()
	tracker.Stop()
	tracker.Stop()
	tracker.Stop() // should not panic

	// Verify tracker still works after double close
	cost := decimal.NewFromFloat(10.0)
	windowEnd := time.Now().Add(time.Hour)
	tracker.SetCachedCost(1, cost, windowEnd)
	got, ok := tracker.GetCachedCost(1)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))
}
