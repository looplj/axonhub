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
	entry, ok := t.cache[channelID]
	t.mu.RUnlock()

	if !ok {
		return decimal.Zero, false
	}

	now := time.Now()

	if now.After(entry.windowEnd) {
		t.mu.Lock()
		if e, exists := t.cache[channelID]; exists && e.fetchedAt.Equal(entry.fetchedAt) {
			delete(t.cache, channelID)
		}
		t.mu.Unlock()

		return decimal.Zero, false
	}

	if now.Sub(entry.fetchedAt) > t.ttl {
		t.mu.Lock()
		if e, exists := t.cache[channelID]; exists && e.fetchedAt.Equal(entry.fetchedAt) {
			delete(t.cache, channelID)
		}
		t.mu.Unlock()

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

func (t *ChannelCostTracker) EvictExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	for id, entry := range t.cache {
		if now.After(entry.windowEnd) || now.Sub(entry.fetchedAt) > t.ttl {
			delete(t.cache, id)
		}
	}
}
