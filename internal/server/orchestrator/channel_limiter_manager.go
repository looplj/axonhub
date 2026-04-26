package orchestrator

import (
	"sync"

	"github.com/looplj/axonhub/internal/server/biz"
)

// ChannelLimiterManager owns one ChannelLimiter per channel, recreating an entry
// whenever the channel's rate-limit configuration changes.
//
// The manager only allocates a limiter when MaxConcurrent > 0. Channels without
// a concurrency limit return nil from GetOrCreate so callers can fast-path past
// admission control entirely.
type ChannelLimiterManager struct {
	mu      sync.RWMutex
	entries map[int]*limiterEntry // key = channel.ID
}

type limiterEntry struct {
	limiter     *ChannelLimiter
	cfg         limiterConfig
	channelName string
}

// NewChannelLimiterManager returns an empty manager.
func NewChannelLimiterManager() *ChannelLimiterManager {
	return &ChannelLimiterManager{entries: make(map[int]*limiterEntry)}
}

// limiterConfig is the runtime view of ChannelRateLimit, normalized for the
// limiter. The struct is comparable so map-and-equality checks detect config
// changes directly without a separate hash.
type limiterConfig struct {
	capacity  int   // == MaxConcurrent; > 0 means limiter enabled
	queueSize int   // == QueueSize; 0 = soft mode
	timeoutMs int64 // == QueueTimeoutMs; 0 = no per-channel timeout
}

// extractLimiterConfig pulls limiter parameters from a channel's settings.
// A returned cfg with capacity == 0 means the channel has no concurrency limit.
func extractLimiterConfig(ch *biz.Channel) limiterConfig {
	if ch == nil || ch.Settings == nil || ch.Settings.RateLimit == nil {
		return limiterConfig{}
	}

	rl := ch.Settings.RateLimit
	if rl.MaxConcurrent == nil || *rl.MaxConcurrent <= 0 {
		return limiterConfig{}
	}

	cfg := limiterConfig{capacity: int(*rl.MaxConcurrent)}

	if rl.QueueSize != nil && *rl.QueueSize > 0 {
		cfg.queueSize = int(*rl.QueueSize)
	}

	if rl.QueueTimeoutMs != nil && *rl.QueueTimeoutMs > 0 {
		cfg.timeoutMs = *rl.QueueTimeoutMs
	}

	return cfg
}

// GetOrCreate returns the limiter for the given channel, rebuilding the entry
// transparently when the channel's rate-limit configuration has changed.
//
// Returns nil when the channel has no concurrency limit configured. Callers
// should treat nil as "no admission control" and proceed without Acquire/Release.
//
// When concurrency limiting is disabled but a stale entry exists (i.e. the user
// just cleared MaxConcurrent), the entry is dropped so Stats/Snapshot stop
// reporting it and downstream scoring code can't dereference now-nil rate-limit
// pointers. In-flight requests that still hold slots Release into the orphaned
// limiter; the limiter is GC'd once the last reference drops.
func (m *ChannelLimiterManager) GetOrCreate(ch *biz.Channel) *ChannelLimiter {
	cfg := extractLimiterConfig(ch)
	if cfg.capacity == 0 {
		if ch != nil {
			m.mu.Lock()
			delete(m.entries, ch.ID)
			m.mu.Unlock()
		}

		return nil
	}

	m.mu.RLock()
	if e, ok := m.entries[ch.ID]; ok && e.cfg == cfg {
		m.mu.RUnlock()

		return e.limiter
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.entries[ch.ID]; ok && e.cfg == cfg {
		return e.limiter
	}

	lim := NewChannelLimiter(cfg.capacity, cfg.queueSize, cfg.timeoutMs)
	m.entries[ch.ID] = &limiterEntry{
		limiter:     lim,
		cfg:         cfg,
		channelName: ch.Name,
	}

	return lim
}

// ChannelLimiterSnapshot is a point-in-time view of one channel's limiter for
// observability callbacks.
type ChannelLimiterSnapshot struct {
	ChannelID   int
	ChannelName string
	InFlight    int
	Waiting     int
}

// Snapshot returns a copy of the current limiter state across all known channels.
// Safe to call from observable-gauge callbacks; the returned slice does not share
// any pointers with the manager's internal state.
func (m *ChannelLimiterManager) Snapshot() []ChannelLimiterSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.entries) == 0 {
		return nil
	}

	out := make([]ChannelLimiterSnapshot, 0, len(m.entries))
	for id, e := range m.entries {
		inFlight, waiting := e.limiter.Stats()
		out = append(out, ChannelLimiterSnapshot{
			ChannelID:   id,
			ChannelName: e.channelName,
			InFlight:    inFlight,
			Waiting:     waiting,
		})
	}

	return out
}

// Forget drops the limiter for a channel. Called from Channel CRUD paths after
// Update or Delete so the next GetOrCreate sees fresh configuration.
//
// Forgetting a limiter while requests are still holding slots is safe — those
// requests Release into a now-orphaned limiter, GC'd once the last reference drops.
func (m *ChannelLimiterManager) Forget(channelID int) {
	m.mu.Lock()
	delete(m.entries, channelID)
	m.mu.Unlock()
}

// Stats returns the current load for a channel. ok=false means the channel has
// no limiter (i.e. no concurrency limit configured), which the load balancer
// should treat as "unlimited capacity" rather than a hard zero.
func (m *ChannelLimiterManager) Stats(channelID int) (inFlight, waiting int, ok bool) {
	m.mu.RLock()
	e, found := m.entries[channelID]
	m.mu.RUnlock()

	if !found {
		return 0, 0, false
	}

	inFlight, waiting = e.limiter.Stats()

	return inFlight, waiting, true
}
