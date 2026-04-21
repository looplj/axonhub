package orchestrator

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type costCacheEntry struct {
	cost      decimal.Decimal
	windowEnd time.Time
	fetchedAt time.Time
}

type CostTrackerOption func(*ChannelCostTracker)

func WithCostTrackerClock(clock func() time.Time) CostTrackerOption {
	return func(t *ChannelCostTracker) {
		t.clock = clock
	}
}

func WithEvictionInterval(d time.Duration) CostTrackerOption {
	return func(t *ChannelCostTracker) {
		t.evictInt = d
	}
}

type ChannelCostTracker struct {
	mu       sync.RWMutex
	cache    map[int]costCacheEntry
	ttl      time.Duration
	clock    func() time.Time
	evictInt time.Duration
	worker   BackgroundWorker
}

func NewChannelCostTracker(opts ...CostTrackerOption) *ChannelCostTracker {
	t := &ChannelCostTracker{
		cache:    make(map[int]costCacheEntry),
		ttl:      30 * time.Second,
		clock:    time.Now,
		evictInt: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *ChannelCostTracker) Start() {
	t.worker.Start(func(stopCh <-chan struct{}) {
		ticker := time.NewTicker(t.evictInt)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t.EvictExpired()
			case <-stopCh:
				return
			}
		}
	})
}

func (t *ChannelCostTracker) Stop() {
	t.worker.Stop()
}

func (t *ChannelCostTracker) GetCachedCost(channelID int) (decimal.Decimal, bool) {
	t.mu.RLock()
	entry, ok := t.cache[channelID]
	now := t.clock()
	t.mu.RUnlock()

	if !ok {
		return decimal.Zero, false
	}

	// Check if entry needs eviction
	windowExpired := now.After(entry.windowEnd)
	ttlExpired := now.Sub(entry.fetchedAt) > t.ttl

	if windowExpired {
		// Window ended - entry is invalid, remove it
		t.mu.Lock()
		if e, exists := t.cache[channelID]; exists && e.fetchedAt.Equal(entry.fetchedAt) {
			delete(t.cache, channelID)
		}
		t.mu.Unlock()
		return decimal.Zero, false
	}

	if ttlExpired {
		// TTL expired but window not ended - return stale data (fail-open for rate limiting:
// stale/low cost data makes the limiter think more budget is available).
		return entry.cost, true
	}

	return entry.cost, true
}

func (t *ChannelCostTracker) SetCachedCost(channelID int, cost decimal.Decimal, windowEnd time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cache[channelID] = costCacheEntry{
		cost:      cost,
		windowEnd: windowEnd,
		fetchedAt: t.clock(),
	}
}

func (t *ChannelCostTracker) Invalidate(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.cache, channelID)
}

func (t *ChannelCostTracker) EvictExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.clock()

	for id, entry := range t.cache {
		// Only evict entries whose window has ended.
		// TTL-expired entries within the active window are kept for stale reads (fail-open for rate limiting).
		if now.After(entry.windowEnd) {
			delete(t.cache, id)
		}
	}
}
