package orchestrator

import (
	"sync"
	"time"
)

// ChannelRequestTracker tracks per-channel request and token counts
// within configurable duration windows for rate limiting.
// It supports multiple window durations per channel (e.g., 1min, 1hr, 5hr, 1mo).
// It also manages cooldown periods for channels that received 429 errors.
type ChannelRequestTracker struct {
	mu        sync.RWMutex
	counters  map[int]map[time.Duration]*rateLimitWindow // channelID -> duration -> window
	cooldowns map[int]time.Time                          // channelID -> cooldown expiration time
}

type rateLimitWindow struct {
	requests    int64
	tokens      int64
	windowStart time.Time
}

// NewChannelRequestTracker creates a new rate limit tracker.
func NewChannelRequestTracker() *ChannelRequestTracker {
	return &ChannelRequestTracker{
		counters:  make(map[int]map[time.Duration]*rateLimitWindow),
		cooldowns: make(map[int]time.Time),
	}
}

// getOrResetWindow returns the current window for a channel and duration, resetting if expired.
func (t *ChannelRequestTracker) getOrResetWindow(channelID int, d time.Duration) *rateLimitWindow {
	now := time.Now()
	windowStart := now.Truncate(d)

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

	// Clean up empty channel entries to prevent memory leaks
	if len(durationMap) == 0 {
		delete(t.counters, channelID)
		durationMap = make(map[time.Duration]*rateLimitWindow)
		t.counters[channelID] = durationMap
	}

	w, ok := durationMap[d]
	if !ok || w.windowStart != windowStart {
		w = &rateLimitWindow{windowStart: windowStart}
		durationMap[d] = w
	}

	return w
}

// IncrementRequest increments the request count for a channel (1-minute window, for backward compatibility).
func (t *ChannelRequestTracker) IncrementRequest(channelID int) {
	t.IncrementRequestForDuration(channelID, time.Minute)
}

// IncrementRequestForDuration increments the request count for a channel with a specific duration window.
func (t *ChannelRequestTracker) IncrementRequestForDuration(channelID int, d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d)
	w.requests++
}

// AddTokens adds token count for a channel (1-minute window, for backward compatibility).
func (t *ChannelRequestTracker) AddTokens(channelID int, tokens int64) {
	t.AddTokensForDuration(channelID, tokens, time.Minute)
}

// AddTokensForDuration adds token count for a channel with a specific duration window.
func (t *ChannelRequestTracker) AddTokensForDuration(channelID int, tokens int64, d time.Duration) {
	if tokens <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d)
	w.tokens += tokens
}

// GetRequestCount returns the current request count for a channel in the current 1-minute window (backward compatibility).
func (t *ChannelRequestTracker) GetRequestCount(channelID int) int64 {
	return t.GetRequestCountForDuration(channelID, time.Minute)
}

// GetRequestCountForDuration returns the current request count for a channel in the current window for the specified duration.
func (t *ChannelRequestTracker) GetRequestCountForDuration(channelID int, d time.Duration) int64 {
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

	windowStart := time.Now().Truncate(d)
	if w.windowStart != windowStart {
		return 0
	}

	return w.requests
}

// GetTokenCount returns the current token count for a channel in the current 1-minute window (backward compatibility).
func (t *ChannelRequestTracker) GetTokenCount(channelID int) int64 {
	return t.GetTokenCountForDuration(channelID, time.Minute)
}

// GetTokenCountForDuration returns the current token count for a channel in the current window for the specified duration.
func (t *ChannelRequestTracker) GetTokenCountForDuration(channelID int, d time.Duration) int64 {
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

	windowStart := time.Now().Truncate(d)
	if w.windowStart != windowStart {
		return 0
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
