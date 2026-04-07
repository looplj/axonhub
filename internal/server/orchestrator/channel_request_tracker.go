package orchestrator

import (
	"sync"
	"time"
)

// ChannelRequestTracker tracks per-channel request and token counts
// within a fixed 1-minute sliding window for rate limiting.
type ChannelRequestTracker struct {
	mu       sync.RWMutex
	counters map[int]*rateLimitWindow // channelID -> window
}

type rateLimitWindow struct {
	requests    int64
	tokens      int64
	windowStart time.Time
}

// NewChannelRequestTracker creates a new rate limit tracker.
func NewChannelRequestTracker() *ChannelRequestTracker {
	return &ChannelRequestTracker{
		counters: make(map[int]*rateLimitWindow),
	}
}

// getOrResetWindow returns the current window for a channel, resetting if expired.
func (t *ChannelRequestTracker) getOrResetWindow(channelID int) *rateLimitWindow {
	now := time.Now()
	windowStart := now.Truncate(time.Minute)

	w, ok := t.counters[channelID]
	if !ok || w.windowStart != windowStart {
		w = &rateLimitWindow{windowStart: windowStart}
		t.counters[channelID] = w
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

// GetRequestCount returns the current request count for a channel in the current window.
func (t *ChannelRequestTracker) GetRequestCount(channelID int) int64 {
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

	return w.requests
}

// GetTokenCount returns the current token count for a channel in the current window.
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

	return w.tokens
}
