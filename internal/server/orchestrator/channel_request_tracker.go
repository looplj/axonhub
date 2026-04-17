package orchestrator

import (
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/objects"
)

type ChannelRequestTracker struct {
	mu        sync.RWMutex
	counters  map[int]map[time.Duration]*rateLimitWindow
	cooldowns map[int]time.Time
}

type rateLimitWindow struct {
	requests    int64
	tokens      int64
	windowStart time.Time
	anchor      *time.Time
}

func NewChannelRequestTracker() *ChannelRequestTracker {
	return &ChannelRequestTracker{
		counters:  make(map[int]map[time.Duration]*rateLimitWindow),
		cooldowns: make(map[int]time.Time),
	}
}

func (t *ChannelRequestTracker) getOrResetWindow(channelID int, d time.Duration, anchor *time.Time) *rateLimitWindow {
	now := time.Now()
	windowStart := objects.ComputeWindowStart(now, d, anchor)

	durationMap, ok := t.counters[channelID]
	if !ok {
		durationMap = make(map[time.Duration]*rateLimitWindow)
		t.counters[channelID] = durationMap
	}

	for dur, w := range durationMap {
		if d != dur && now.Sub(w.windowStart) > dur*2 {
			delete(durationMap, dur)
		}
	}

	if len(durationMap) == 0 {
		delete(t.counters, channelID)
		durationMap = make(map[time.Duration]*rateLimitWindow)
		t.counters[channelID] = durationMap
	}

	w, ok := durationMap[d]
	if !ok || w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		w = &rateLimitWindow{windowStart: windowStart, anchor: copyAnchor(anchor)}
		durationMap[d] = w
	}

	return w
}

func anchorEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func copyAnchor(a *time.Time) *time.Time {
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

func (t *ChannelRequestTracker) IncrementRequest(channelID int) {
	t.IncrementRequestForDuration(channelID, time.Minute, nil)
}

func (t *ChannelRequestTracker) IncrementRequestForDuration(channelID int, d time.Duration, anchor *time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d, anchor)
	w.requests++
}

func (t *ChannelRequestTracker) AddTokens(channelID int, tokens int64) {
	t.AddTokensForDuration(channelID, tokens, time.Minute, nil)
}

func (t *ChannelRequestTracker) AddTokensForDuration(channelID int, tokens int64, d time.Duration, anchor *time.Time) {
	if tokens <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d, anchor)
	w.tokens += tokens
}

func (t *ChannelRequestTracker) GetRequestCount(channelID int) int64 {
	return t.GetRequestCountForDuration(channelID, time.Minute, nil)
}

func (t *ChannelRequestTracker) GetWindowResetTimeForDuration(channelID int, d time.Duration, anchor *time.Time) time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()

	durationMap, ok := t.counters[channelID]
	if !ok {
		return time.Time{}
	}

	w, ok := durationMap[d]
	if !ok {
		return time.Time{}
	}

	windowStart := objects.ComputeWindowStart(time.Now(), d, anchor)
	if w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		return time.Time{}
	}

	return w.windowStart.Add(d)
}

func (t *ChannelRequestTracker) GetRequestCountForDuration(channelID int, d time.Duration, anchor *time.Time) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	durationMap, ok := t.counters[channelID]
	if !ok {
		return 0
	}

	w, ok := durationMap[d]
	if !ok {
		return 0
	}

	windowStart := objects.ComputeWindowStart(time.Now(), d, anchor)
	if w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		return 0
	}

	return w.requests
}

func (t *ChannelRequestTracker) GetTokenCount(channelID int) int64 {
	return t.GetTokenCountForDuration(channelID, time.Minute, nil)
}

func (t *ChannelRequestTracker) GetTokenCountForDuration(channelID int, d time.Duration, anchor *time.Time) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	durationMap, ok := t.counters[channelID]
	if !ok {
		return 0
	}

	w, ok := durationMap[d]
	if !ok {
		return 0
	}

	windowStart := objects.ComputeWindowStart(time.Now(), d, anchor)
	if w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		return 0
	}

	return w.tokens
}

func (t *ChannelRequestTracker) SetCooldown(channelID int, until time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if existing, ok := t.cooldowns[channelID]; ok && existing.After(until) {
		return
	}

	t.cooldowns[channelID] = until
}

func (t *ChannelRequestTracker) IsCoolingDown(channelID int) bool {
	_, ok := t.GetCooldownUntil(channelID)
	return ok
}

func (t *ChannelRequestTracker) GetCooldownUntil(channelID int) (time.Time, bool) {
	t.mu.RLock()
	until, ok := t.cooldowns[channelID]
	t.mu.RUnlock()

	if !ok {
		return time.Time{}, false
	}

	now := time.Now()
	if now.After(until) {
		t.clearExpiredCooldown(channelID, until, now)
		return time.Time{}, false
	}

	return until, true
}

func (t *ChannelRequestTracker) clearExpiredCooldown(channelID int, observedUntil, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	currentUntil, ok := t.cooldowns[channelID]
	if !ok {
		return
	}

	if currentUntil.Equal(observedUntil) && now.After(currentUntil) {
		delete(t.cooldowns, channelID)
	}
}
