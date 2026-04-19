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
	stopCh   chan struct{}
	evictInt time.Duration
}

func NewChannelCostTracker(opts ...CostTrackerOption) *ChannelCostTracker {
	t := &ChannelCostTracker{
		cache:    make(map[int]costCacheEntry),
		ttl:      30 * time.Second,
		clock:    time.Now,
		evictInt: 60 * time.Second,
		stopCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Start begins the background eviction goroutine.
func (t *ChannelCostTracker) Start() {
	if t.stopCh == nil {
		t.stopCh = make(chan struct{})
	}
	go func() {
		ticker := time.NewTicker(t.evictInt)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.EvictExpired()
			case <-t.stopCh:
				return
			}
		}
	}()
}

// Stop stops the background eviction goroutine.
func (t *ChannelCostTracker) Stop() {
	if t.stopCh != nil {
		close(t.stopCh)
		t.stopCh = nil
	}
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
		// TTL expired but window not ended - return stale data (fail-closed)
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
		if now.After(entry.windowEnd) || now.Sub(entry.fetchedAt) > t.ttl {
			delete(t.cache, id)
		}
	}
}
