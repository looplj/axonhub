package orchestrator

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

type ChannelRequestTracker struct {
	mu        sync.RWMutex
	counters  map[int]map[time.Duration]*rateLimitWindow
	cooldowns map[int]time.Time
	clock     func() time.Time
	evictInt  time.Duration
	ttl       time.Duration
	worker    BackgroundWorker
}

type ClockOption func(*ChannelRequestTracker)

func WithClock(c func() time.Time) ClockOption {
	return func(t *ChannelRequestTracker) {
		t.clock = c
	}
}

func WithTrackerTTL(d time.Duration) ClockOption {
	return func(t *ChannelRequestTracker) {
		t.ttl = d
	}
}

type rateLimitWindow struct {
	requests         int64
	tokens           int64
	requestSeed      int64
	tokenSeed        int64
	requestDbQueried bool
	tokenDbQueried   bool
	windowStart      time.Time
	anchor           *time.Time
}

func NewChannelRequestTracker(opts ...ClockOption) *ChannelRequestTracker {
	t := &ChannelRequestTracker{
		counters:  make(map[int]map[time.Duration]*rateLimitWindow),
		cooldowns: make(map[int]time.Time),
		clock:     time.Now,
		evictInt:  60 * time.Second,
		ttl:       5 * time.Minute,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *ChannelRequestTracker) Start() {
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

func (t *ChannelRequestTracker) Stop() {
	t.worker.Stop()
}

func (t *ChannelRequestTracker) getOrResetWindow(channelID int, d time.Duration, anchor *time.Time) *rateLimitWindow {
	now := t.clock()
	windowStart := objects.ComputeWindowStart(now, d, anchor)

	durationMap, ok := t.counters[channelID]
	if !ok {
		durationMap = make(map[time.Duration]*rateLimitWindow)
		t.counters[channelID] = durationMap
	}

	w, ok := durationMap[d]
	if !ok {
		w = &rateLimitWindow{windowStart: windowStart, anchor: copyAnchor(anchor)}
		durationMap[d] = w
	} else if w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		if w.requests > 0 || w.tokens > 0 {
			log.Warn(context.Background(), "rate limit window data discarded due to anchor or window change",
				log.Int("channel_id", channelID),
				log.Duration("duration", d),
				log.Int64("discarded_requests", w.requestSeed+w.requests),
				log.Int64("discarded_tokens", w.tokenSeed+w.tokens),
			)
		}
		w = &rateLimitWindow{windowStart: windowStart, anchor: copyAnchor(anchor)}
		durationMap[d] = w
	}

	return w
}

func anchorEqual(a, b *time.Time) bool {
	aIsZero := a == nil || a.IsZero()
	bIsZero := b == nil || b.IsZero()
	if aIsZero && bIsZero {
		return true
	}
	if aIsZero || bIsZero {
		return false
	}
	return a.Equal(*b)
}

// copyAnchor creates a deep copy of a *time.Time, normalizing to UTC.
func copyAnchor(a *time.Time) *time.Time {
	if a == nil {
		return nil
	}

	cp := a.UTC()
	return &cp
}

func (t *ChannelRequestTracker) IncrementRequest(channelID int) {
	t.IncrementRequestForDuration(channelID, time.Minute, nil)
}

func (t *ChannelRequestTracker) IncrementRequestForDuration(channelID int, d time.Duration, anchor *time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d, anchor)
	if w.requests < math.MaxInt64-w.requestSeed {
		w.requests++
	}
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
	if w.tokens > math.MaxInt64-tokens {
		w.tokens = math.MaxInt64
	} else {
		w.tokens += tokens
	}
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

	windowStart := objects.ComputeWindowStart(t.clock(), d, anchor)
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

	windowStart := objects.ComputeWindowStart(t.clock(), d, anchor)
	if w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		return 0
	}

	result := w.requestSeed + w.requests
	if result < 0 {
		result = math.MaxInt64
	}

	return result
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

	windowStart := objects.ComputeWindowStart(t.clock(), d, anchor)
	if w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		return 0
	}

	result := w.tokenSeed + w.tokens
	if result < 0 {
		result = math.MaxInt64
	}

	return result
}

func (t *ChannelRequestTracker) IsRequestWindowDbQueried(channelID int, d time.Duration, anchor *time.Time) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	durationMap, ok := t.counters[channelID]
	if !ok {
		return false
	}

	w, ok := durationMap[d]
	if !ok {
		return false
	}

	windowStart := objects.ComputeWindowStart(t.clock(), d, anchor)
	if w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		return false
	}

	return w.requestDbQueried
}

// MarkRequestWindowDbQueried marks that the request count for the given channel
// and duration has been fetched from the database.
// Note: This method creates a rateLimitWindow entry as a side effect if one does
// not yet exist for the given channel/duration/anchor combination.
func (t *ChannelRequestTracker) MarkRequestWindowDbQueried(channelID int, d time.Duration, anchor *time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d, anchor)
	w.requestDbQueried = true
}

func (t *ChannelRequestTracker) IsTokenWindowDbQueried(channelID int, d time.Duration, anchor *time.Time) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	durationMap, ok := t.counters[channelID]
	if !ok {
		return false
	}

	w, ok := durationMap[d]
	if !ok {
		return false
	}

	windowStart := objects.ComputeWindowStart(t.clock(), d, anchor)
	if w.windowStart != windowStart || !anchorEqual(w.anchor, anchor) {
		return false
	}

	return w.tokenDbQueried
}

// MarkTokenWindowDbQueried marks that the token count for the given channel
// and duration has been fetched from the database.
// Note: This method creates a rateLimitWindow entry as a side effect if one does
// not yet exist for the given channel/duration/anchor combination.
func (t *ChannelRequestTracker) MarkTokenWindowDbQueried(channelID int, d time.Duration, anchor *time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d, anchor)
	w.tokenDbQueried = true
}

func (t *ChannelRequestTracker) SeedRequestCountForDuration(channelID int, count int64, d time.Duration, anchor *time.Time) {
	if count <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d, anchor)
	seed := count - w.requests
	if seed < 0 {
		seed = 0
	}
	// Clamp to prevent overflow when seed + requests exceeds MaxInt64
	maxSeed := math.MaxInt64 - w.requests
	if seed > maxSeed {
		seed = maxSeed
	}
	w.requestSeed = seed
}

func (t *ChannelRequestTracker) SeedTokenCountForDuration(channelID int, count int64, d time.Duration, anchor *time.Time) {
	if count <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	w := t.getOrResetWindow(channelID, d, anchor)
	seed := count - w.tokens
	if seed < 0 {
		seed = 0
	}
	// Clamp to prevent overflow when seed + tokens exceeds MaxInt64
	maxSeed := math.MaxInt64 - w.tokens
	if seed > maxSeed {
		seed = maxSeed
	}
	w.tokenSeed = seed
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

	now := t.clock()
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

type expiredDurationEntry struct {
	channelID int
	dur       time.Duration
}

type evictionCandidates struct {
	now              time.Time
	expiredDurations []expiredDurationEntry
	emptyChannels    []int
	expiredCooldowns []int
}

func (t *ChannelRequestTracker) collectEvictionCandidates() evictionCandidates {
	t.mu.RLock()
	defer t.mu.RUnlock()

	c := evictionCandidates{now: t.clock()}

	for channelID, durationMap := range t.counters {
		if len(durationMap) == 0 {
			c.emptyChannels = append(c.emptyChannels, channelID)
			continue
		}

		allExpired := true
		for dur, w := range durationMap {
			windowEnd := w.windowStart.Add(dur)
			if c.now.After(windowEnd) && c.now.Sub(windowEnd) > t.ttl {
				c.expiredDurations = append(c.expiredDurations, expiredDurationEntry{channelID, dur})
			} else {
				allExpired = false
			}
		}

		if allExpired {
			c.emptyChannels = append(c.emptyChannels, channelID)
		}
	}

	for channelID, until := range t.cooldowns {
		if c.now.After(until) {
			c.expiredCooldowns = append(c.expiredCooldowns, channelID)
		}
	}

	return c
}

func (t *ChannelRequestTracker) deleteEvictionCandidates(c evictionCandidates) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, ed := range c.expiredDurations {
		if dm, ok := t.counters[ed.channelID]; ok {
			if w, exists := dm[ed.dur]; exists {
				windowEnd := w.windowStart.Add(ed.dur)
				if c.now.After(windowEnd) && c.now.Sub(windowEnd) > t.ttl {
					delete(dm, ed.dur)
				}
			}
		}
	}

	for _, cid := range c.emptyChannels {
		if dm, ok := t.counters[cid]; ok && len(dm) == 0 {
			delete(t.counters, cid)
		}
	}

	for _, cid := range c.expiredCooldowns {
		if until, ok := t.cooldowns[cid]; ok && c.now.After(until) {
			delete(t.cooldowns, cid)
		}
	}
}

func (t *ChannelRequestTracker) EvictExpired() {
	candidates := t.collectEvictionCandidates()
	if len(candidates.expiredDurations) == 0 && len(candidates.expiredCooldowns) == 0 {
		return
	}

	t.deleteEvictionCandidates(candidates)
}
