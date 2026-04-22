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

func WithMaxEntries(n int) CostTrackerOption {
	return func(t *ChannelCostTracker) {
		t.maxEntries = n
	}
}

type ChannelCostTracker struct {
	mu         sync.RWMutex
	cache      map[int]costCacheEntry
	ttl        time.Duration
	clock      func() time.Time
	evictInt   time.Duration
	maxEntries int
	worker     BackgroundWorker
}

func NewChannelCostTracker(opts ...CostTrackerOption) *ChannelCostTracker {
	t := &ChannelCostTracker{
		cache:      make(map[int]costCacheEntry),
		ttl:        30 * time.Second,
		clock:      time.Now,
		evictInt:   60 * time.Second,
		maxEntries: 10000,
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
	if !ok {
		t.mu.RUnlock()
		return decimal.Zero, false
	}

	now := t.clock()

	// Window expired — need to delete (write operation)
	if now.After(entry.windowEnd) {
		t.mu.RUnlock()
		t.mu.Lock()
		// Recheck after upgrading lock
		if entry, ok = t.cache[channelID]; ok && now.After(entry.windowEnd) {
			delete(t.cache, channelID)
		}
		t.mu.Unlock()
		return decimal.Zero, false
	}

	// Stale entry — cache miss but don't delete (might still be useful as fallback)
	if now.Sub(entry.fetchedAt) > t.ttl {
		t.mu.RUnlock()
		return decimal.Zero, false
	}

	t.mu.RUnlock()
	return entry.cost, true
}

func (t *ChannelCostTracker) SetCachedCost(channelID int, cost decimal.Decimal, windowEnd time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.clock()

	// Reject past windowEnd values
	if !windowEnd.After(now) {
		return
	}

	// Enforce max entries bound
	if _, exists := t.cache[channelID]; !exists && len(t.cache) >= t.maxEntries {
		// Evict one expired entry if possible, otherwise skip caching
		for id, e := range t.cache {
			if now.After(e.windowEnd) {
				delete(t.cache, id)
				break
			}
		}
		if len(t.cache) >= t.maxEntries {
			return
		}
	}

	t.cache[channelID] = costCacheEntry{
		cost:      cost,
		windowEnd: windowEnd,
		fetchedAt: now,
	}
}

func (t *ChannelCostTracker) Invalidate(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.cache, channelID)
}

func (t *ChannelCostTracker) EvictExpired() {
	// Two-phase: collect under RLock, delete under Lock
	t.mu.RLock()
	now := t.clock()
	var expired []int
	for id, entry := range t.cache {
		if now.After(entry.windowEnd) {
			expired = append(expired, id)
		}
	}
	t.mu.RUnlock()

	if len(expired) == 0 {
		return
	}

	t.mu.Lock()
	for _, id := range expired {
		// Recheck in case entry was updated between phases
		if entry, ok := t.cache[id]; ok && now.After(entry.windowEnd) {
			delete(t.cache, id)
		}
	}
	t.mu.Unlock()
}