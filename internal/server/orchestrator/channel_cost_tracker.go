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

type ChannelCostTracker struct {
	mu    sync.RWMutex
	cache map[int]costCacheEntry
	ttl   time.Duration
}

func NewChannelCostTracker() *ChannelCostTracker {
	return &ChannelCostTracker{
		cache: make(map[int]costCacheEntry),
		ttl:   30 * time.Second,
	}
}

func (t *ChannelCostTracker) GetCachedCost(channelID int) (decimal.Decimal, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, ok := t.cache[channelID]
	if !ok {
		return decimal.Zero, false
	}

	if time.Since(entry.fetchedAt) > t.ttl {
		return decimal.Zero, false
	}

	return entry.cost, true
}

func (t *ChannelCostTracker) SetCachedCost(channelID int, cost decimal.Decimal, windowEnd time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cache[channelID] = costCacheEntry{
		cost:      cost,
		windowEnd: windowEnd,
		fetchedAt: time.Now(),
	}
}

func (t *ChannelCostTracker) Invalidate(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.cache, channelID)
}
