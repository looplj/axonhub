package antigravity

import (
	"sync"
	"time"
)

const (
	// DefaultCooldownDuration is the fixed cooldown period after an endpoint failure.
	DefaultCooldownDuration = 60 * time.Second

	// DefaultTTL is how long we keep failure records before considering them stale.
	DefaultTTL = 10 * time.Minute
)

// endpointKey identifies a unique model+endpoint combination.
type endpointKey struct {
	model    string
	endpoint string
}

// AntigravityHealthTracker tracks endpoint health on a per-model basis.
// It implements a simple cooldown mechanism to avoid repeatedly hitting
// failed endpoints, improving overall request success rate and latency.
type AntigravityHealthTracker struct {
	mu               sync.RWMutex
	failures         map[endpointKey]*AntigravityEndpointFailure
	cooldownDuration time.Duration
	ttl              time.Duration
}

// AntigravityEndpointFailure records information about an endpoint failure for a specific model.
type AntigravityEndpointFailure struct {
	Model         string
	Endpoint      string
	LastFailedAt  time.Time
	StatusCode    int
	CooldownUntil time.Time
}

// NewAntigravityHealthTracker creates a new health tracker with default settings.
func NewAntigravityHealthTracker() *AntigravityHealthTracker {
	return &AntigravityHealthTracker{
		failures:         make(map[endpointKey]*AntigravityEndpointFailure),
		cooldownDuration: DefaultCooldownDuration,
		ttl:              DefaultTTL,
	}
}

// NewAntigravityHealthTrackerWithConfig creates a health tracker with custom settings.
func NewAntigravityHealthTrackerWithConfig(cooldown, ttl time.Duration) *AntigravityHealthTracker {
	return &AntigravityHealthTracker{
		failures:         make(map[endpointKey]*AntigravityEndpointFailure),
		cooldownDuration: cooldown,
		ttl:              ttl,
	}
}

// ShouldSkip returns true if the endpoint should be skipped for this model
// due to being in cooldown period.
func (t *AntigravityHealthTracker) ShouldSkip(model, endpoint string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := endpointKey{model: model, endpoint: endpoint}
	failure, exists := t.failures[key]

	if !exists {
		return false
	}

	now := time.Now()

	// Lazy cleanup: Remove entry if it's past TTL
	if now.Sub(failure.LastFailedAt) > t.ttl {
		delete(t.failures, key)
		return false
	}

	// Check if still in cooldown period
	return now.Before(failure.CooldownUntil)
}

// RecordFailure records a failure for the given model+endpoint combination.
// This puts the endpoint into cooldown for the configured duration.
func (t *AntigravityHealthTracker) RecordFailure(model, endpoint string, statusCode int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	key := endpointKey{model: model, endpoint: endpoint}

	failure := &AntigravityEndpointFailure{
		Model:         model,
		Endpoint:      endpoint,
		LastFailedAt:  now,
		StatusCode:    statusCode,
		CooldownUntil: now.Add(t.cooldownDuration),
	}

	t.failures[key] = failure
}

// RecordSuccess records a successful request for the given model+endpoint.
// This clears any existing failure state for that combination.
func (t *AntigravityHealthTracker) RecordSuccess(model, endpoint string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := endpointKey{model: model, endpoint: endpoint}
	delete(t.failures, key)
}

// GetFailure retrieves the failure information for a model+endpoint, if any.
// Returns nil if no failure is recorded or if the entry has expired.
func (t *AntigravityHealthTracker) GetFailure(model, endpoint string) *AntigravityEndpointFailure {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := endpointKey{model: model, endpoint: endpoint}
	failure, exists := t.failures[key]

	if !exists {
		return nil
	}

	// Check if expired
	if time.Since(failure.LastFailedAt) > t.ttl {
		return nil
	}

	// Return a copy to avoid race conditions
	failureCopy := *failure

	return &failureCopy
}

// Stats returns statistics about the current state of the health tracker.
func (t *AntigravityHealthTracker) Stats() AntigravityHealthTrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	stats := AntigravityHealthTrackerStats{
		TotalEntries:    len(t.failures),
		InCooldown:      0,
		Expired:         0,
		CooldownEntries: make(map[string]time.Time),
	}

	for _, failure := range t.failures {
		// Check if expired
		if now.Sub(failure.LastFailedAt) > t.ttl {
			stats.Expired++
			continue
		}

		// Check if in cooldown
		if now.Before(failure.CooldownUntil) {
			stats.InCooldown++
			// Use string representation for stats key
			keyStr := failure.Model + "|" + failure.Endpoint
			stats.CooldownEntries[keyStr] = failure.CooldownUntil
		}
	}

	return stats
}

// AntigravityHealthTrackerStats provides insights into the health tracker's current state.
type AntigravityHealthTrackerStats struct {
	TotalEntries    int
	InCooldown      int
	Expired         int
	CooldownEntries map[string]time.Time // key -> cooldown expiry time
}

// Clear removes all entries from the health tracker.
// This is primarily useful for testing.
func (t *AntigravityHealthTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.failures = make(map[endpointKey]*AntigravityEndpointFailure)
}
