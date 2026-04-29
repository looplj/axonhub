package orchestrator

import (
	"sync"
	"time"
)

// ChannelRequestTracker tracks per-channel request and token counts
// within a 1-minute sliding window for rate limiting.
// It also manages cooldown periods for channels that received 429 errors.
type ChannelRequestTracker struct {
	mu        sync.RWMutex
	counters  map[int]*rateLimitWindow // channelID -> window
	cooldowns map[int]time.Time        // channelID -> cooldown expiration time
}

// rateLimitWindow holds counters for the current and previous minute window.
type rateLimitWindow struct {
	requests    int64
	tokens      int64
	windowStart time.Time

	// Previous window counters, used for sliding window calculation.
	prevRequests    int64
	prevTokens      int64
	prevWindowStart time.Time
}

// NewChannelRequestTracker creates a new rate limit tracker.
func NewChannelRequestTracker() *ChannelRequestTracker {
	return &ChannelRequestTracker{
		counters:  make(map[int]*rateLimitWindow),
		cooldowns: make(map[int]time.Time),
	}
}

// getOrResetWindow returns the current window for a channel, resetting if expired.
// When the minute boundary crosses, current counters are rotated into prev.
func (t *ChannelRequestTracker) getOrResetWindow(channelID int) *rateLimitWindow {
	now := time.Now()
	windowStart := now.Truncate(time.Minute)

	w, ok := t.counters[channelID]
	if !ok {
		w = &rateLimitWindow{windowStart: windowStart}
		t.counters[channelID] = w
	} else if w.windowStart != windowStart {
		// Window flipped: rotate current → previous
		w.prevRequests = w.requests
		w.prevTokens = w.tokens
		w.prevWindowStart = w.windowStart
		w.requests = 0
		w.tokens = 0
		w.windowStart = windowStart
	}

	return w
}

// IncrementRequest increments the request count for a channel.
func (t *ChannelRequestTracker) IncrementRequest(channelID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID)
	w.requests++
}

// AddTokens adds token count for a channel.
func (t *ChannelRequestTracker) AddTokens(channelID int, tokens int64) {
	if tokens <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID)
	w.tokens += tokens
}

// GetRequestCount returns the effective request count for a channel
// using a sliding window: effective = prev * (1 - elapsed/60s) + curr.
func (t *ChannelRequestTracker) GetRequestCount(channelID int) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	w, ok := t.counters[channelID]
	if !ok {
		return 0
	}

	windowStart := time.Now().Truncate(time.Minute)
	if w.windowStart != windowStart {
		// Expired and no rotate happened; treat as empty.
		return 0
	}

	// Sliding window: weighted decay of previous window.
	if w.prevWindowStart == windowStart.Add(-time.Minute) {
		elapsed := time.Since(windowStart).Seconds()
		weight := 1.0 - elapsed/60.0
		if weight < 0 {
			weight = 0
		}
		return int64(float64(w.prevRequests)*weight) + w.requests
	}

	return w.requests
}

// GetTokenCount returns the effective token count for a channel
// using a sliding window: effective = prev * (1 - elapsed/60s) + curr.
func (t *ChannelRequestTracker) GetTokenCount(channelID int) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	w, ok := t.counters[channelID]
	if !ok {
		return 0
	}

	windowStart := time.Now().Truncate(time.Minute)
	if w.windowStart != windowStart {
		return 0
	}

	// Sliding window: weighted decay of previous window.
	if w.prevWindowStart == windowStart.Add(-time.Minute) {
		elapsed := time.Since(windowStart).Seconds()
		weight := 1.0 - elapsed/60.0
		if weight < 0 {
			weight = 0
		}
		return int64(float64(w.prevTokens)*weight) + w.tokens
	}

	return w.tokens
}

// SetCooldown sets a cooldown period for a channel until the specified time.
// It only extends the cooldown; a shorter value will not overwrite an existing longer one.
func (t *ChannelRequestTracker) SetCooldown(channelID int, until time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if existing, ok := t.cooldowns[channelID]; ok && existing.After(until) {
		return
	}

	t.cooldowns[channelID] = until
}

// IsCoolingDown checks if a channel is currently in a cooldown period.
// It also performs lazy cleanup by removing expired cooldown entries.
func (t *ChannelRequestTracker) IsCoolingDown(channelID int) bool {
	_, ok := t.GetCooldownUntil(channelID)
	return ok
}

// GetCooldownUntil returns the cooldown expiration time for a channel.
// Returns false if the channel is not in cooldown or the cooldown has expired.
func (t *ChannelRequestTracker) GetCooldownUntil(channelID int) (time.Time, bool) {
	t.mu.RLock()
	until, ok := t.cooldowns[channelID]
	t.mu.RUnlock()

	if !ok {
		return time.Time{}, false
	}

	// Check if cooldown has expired
	now := time.Now()
	if now.After(until) {
		t.clearExpiredCooldown(channelID, until, now)
		return time.Time{}, false
	}

	return until, true
}

// clearExpiredCooldown removes an expired cooldown entry only if it still matches
// the value observed by the caller, preventing races with newer SetCooldown writes.
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
