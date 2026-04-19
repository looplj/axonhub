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
	// TTL expired but window not ended - returns stale data (fail-closed)
	assert.True(t, ok)
	assert.True(t, cost.Equal(got))
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

	// Advance clock past TTL
	*clockPtr = now.Add(31 * time.Second)

	tracker.EvictExpired()

	_, ok := tracker.GetCachedCost(1)
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
