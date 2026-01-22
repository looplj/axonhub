package biz

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xmap"
)

// CircuitBreakerState represents the state of the circuit breaker.
type CircuitBreakerState string

const (
	// StateClosed the circuit is "complete." Requests flow through to the upstream service.
	StateClosed CircuitBreakerState = "closed"

	// StateHalfOpen the circuit is "half-open." A limited number of requests are allowed to test the service.
	StateHalfOpen CircuitBreakerState = "half_open"

	// StateOpen the circuit is "open." No requests are allowed to flow through.
	StateOpen CircuitBreakerState = "open"
)

// ModelCircuitBreakerPolicy defines the policy for model circuit breaker management.
type ModelCircuitBreakerPolicy struct {
	// [Thresholds]
	// Number of consecutive failures to trigger Half-Open state (recommended: 3)
	HalfOpenThreshold int `json:"half_open_threshold" yaml:"half_open_threshold"`

	// Number of consecutive failures to trigger Open state (recommended: 5)
	OpenThreshold int `json:"open_threshold" yaml:"open_threshold"`

	// [Time Control]
	// Failure stats TTL (recommended: 30m). Counters reset if no new errors occur within this duration.
	FailureStatsTTL time.Duration `json:"failure_stats_ttl" yaml:"failure_stats_ttl"`

	// Probe interval after entering Open state (recommended: 5m)
	ProbeInterval time.Duration `json:"probe_interval" yaml:"probe_interval"`

	// [Weight Control]
	// Weight multiplier in Half-Open state (recommended: 0.3)
	HalfOpenWeight float64 `json:"half_open_weight" yaml:"half_open_weight"`
}

var defaultModelCircuitBreakerPolicy = ModelCircuitBreakerPolicy{
	HalfOpenThreshold: 3,
	OpenThreshold:     5,
	FailureStatsTTL:   30 * time.Minute,
	ProbeInterval:     5 * time.Minute,
	HalfOpenWeight:    0.3,
}

// DefaultModelCircuitBreakerPolicy returns the default model circuit breaker policy.
func DefaultModelCircuitBreakerPolicy() *ModelCircuitBreakerPolicy {
	return &defaultModelCircuitBreakerPolicy
}

// Validate validates the model circuit breaker policy.
func (p *ModelCircuitBreakerPolicy) Validate() error {
	if p.HalfOpenThreshold >= p.OpenThreshold {
		return fmt.Errorf("half_open_threshold (%d) must be less than open_threshold (%d)",
			p.HalfOpenThreshold, p.OpenThreshold)
	}

	if p.HalfOpenWeight < 0 || p.HalfOpenWeight > 1 {
		return fmt.Errorf("half_open_weight must be between 0 and 1, got %f", p.HalfOpenWeight)
	}

	return nil
}

// ModelCircuitBreakerStats represents the runtime health statistics for a model.
type ModelCircuitBreakerStats struct {
	sync.RWMutex // Read-write lock protection

	// Identification
	ChannelID int
	ModelID   string

	// Current State
	State CircuitBreakerState

	// Counters
	ConsecutiveFailures int       // Number of consecutive failures
	LastFailureAt       time.Time // Last failure time
	LastSuccessAt       time.Time // Last success time

	// Recovery Control
	NextProbeAt time.Time // Next allowed probe time (used for Open state)

	// Probe Control (to prevent concurrent penetration)
	probingInProgress int32 // atomic operation
	probeAttempts     int   // number of probe attempts, used for exponential backoff
}

// ModelCircuitBreaker manages the circuit breaker status of models across channels.
type ModelCircuitBreaker struct {
	// In-memory model health statistics
	modelStats *xmap.Map[ChannelModelKey, *ModelCircuitBreakerStats]
}

// NewModelCircuitBreaker creates a new model circuit breaker.
func NewModelCircuitBreaker() *ModelCircuitBreaker {
	return &ModelCircuitBreaker{
		modelStats: xmap.New[ChannelModelKey, *ModelCircuitBreakerStats](),
	}
}

// ChannelModelKey generates a unique key for channel and model combination.
func (m *ModelCircuitBreaker) ChannelModelKey(channelID int, modelID string) ChannelModelKey {
	return ChannelModelKey{
		ChannelID: channelID,
		ModelID:   modelID,
	}
}

// getStats gets or creates model circuit breaker stats for the given channel and model.
func (m *ModelCircuitBreaker) getStats(channelID int, modelID string) *ModelCircuitBreakerStats {
	key := m.ChannelModelKey(channelID, modelID)

	if stats, exists := m.modelStats.Load(key); exists {
		return stats
	}

	// Create new stats if not exists
	stats := &ModelCircuitBreakerStats{
		ChannelID:           channelID,
		ModelID:             modelID,
		State:               StateClosed,
		ConsecutiveFailures: 0,
		LastSuccessAt:       time.Now(),
	}

	actual, _ := m.modelStats.LoadOrStore(key, stats)

	return actual
}

// GetPolicy retrieves the model circuit breaker policy from system settings.
func (m *ModelCircuitBreaker) GetPolicy(ctx context.Context) *ModelCircuitBreakerPolicy {
	// For now, return default policy
	// TODO: Integrate with system settings when ready
	return DefaultModelCircuitBreakerPolicy()
}

// RecordError records an error for the specified channel and model.
func (m *ModelCircuitBreaker) RecordError(ctx context.Context, channelID int, modelID string) {
	stats := m.getStats(channelID, modelID)

	stats.Lock()
	defer stats.Unlock()

	now := time.Now()
	policy := m.GetPolicy(ctx)

	// 1. TTL Check: prevent zombie counts
	if stats.ConsecutiveFailures > 0 {
		if now.Sub(stats.LastFailureAt) > policy.FailureStatsTTL {
			log.Info(ctx, "Resetting expired failure stats for model",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Int("old_failures", stats.ConsecutiveFailures),
			)
			stats.ConsecutiveFailures = 0
		}
	}

	// 2. Update counter
	stats.ConsecutiveFailures++
	stats.LastFailureAt = now

	// 3. State transition judgment
	// Prioritize Open state, then Half-Open state
	if stats.ConsecutiveFailures >= policy.OpenThreshold {
		if stats.State != StateOpen {
			stats.State = StateOpen
			stats.NextProbeAt = now.Add(policy.ProbeInterval) // Set next probe time
			stats.probeAttempts = 0                           // Reset probe count

			log.Warn(ctx, "model turn to break due to consecutive failures",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Int("failures", stats.ConsecutiveFailures),
			)
		} else {
			// If already Open, update probe time with exponential backoff
			backoffMultiplier := math.Pow(2, float64(stats.probeAttempts))
			if backoffMultiplier > 8 { // Max 8x
				backoffMultiplier = 8
			}

			nextInterval := time.Duration(float64(policy.ProbeInterval) * backoffMultiplier)
			stats.NextProbeAt = now.Add(nextInterval)
			stats.probeAttempts++

			log.Debug(ctx, "updated probe time for open model",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Time("next_probe_at", stats.NextProbeAt),
				log.Int("probe_attempts", stats.probeAttempts),
			)
		}
	} else if stats.ConsecutiveFailures >= policy.HalfOpenThreshold {
		if stats.State != StateHalfOpen {
			stats.State = StateHalfOpen

			log.Warn(ctx, "model turn to half-open due to consecutive failures",
				log.Int("channel_id", channelID),
				log.String("model_id", modelID),
				log.Int("failures", stats.ConsecutiveFailures),
			)
		}
	}
}

// RecordSuccess records a successful request for the specified channel and model.
func (m *ModelCircuitBreaker) RecordSuccess(ctx context.Context, channelID int, modelID string) {
	stats := m.getStats(channelID, modelID)

	stats.Lock()
	defer stats.Unlock()

	stats.LastSuccessAt = time.Now()

	// Reset all negative status immediately upon a single success
	if stats.State != StateClosed {
		log.Info(ctx, "model recovered to closed state",
			log.Int("channel_id", channelID),
			log.String("model_id", modelID),
			log.String("previous_state", string(stats.State)),
			log.Int("previous_failures", stats.ConsecutiveFailures),
		)
	}

	stats.State = StateClosed
	stats.ConsecutiveFailures = 0
	stats.NextProbeAt = time.Time{} // Clear probe time
	stats.probingInProgress = 0     // Reset probing flag
	stats.probeAttempts = 0         // Reset probe count
}

// GetModelCircuitBreakerStats returns the current state and statistics of a model.
func (m *ModelCircuitBreaker) GetModelCircuitBreakerStats(ctx context.Context, channelID int, modelID string) *ModelCircuitBreakerStats {
	stats := m.getStats(channelID, modelID)

	stats.RLock()
	defer stats.RUnlock()

	// Return a copy to avoid concurrent modification
	return &ModelCircuitBreakerStats{
		ChannelID:           stats.ChannelID,
		ModelID:             stats.ModelID,
		State:               stats.State,
		ConsecutiveFailures: stats.ConsecutiveFailures,
		LastFailureAt:       stats.LastFailureAt,
		LastSuccessAt:       stats.LastSuccessAt,
		NextProbeAt:         stats.NextProbeAt,
		probeAttempts:       stats.probeAttempts,
		probingInProgress:   atomic.LoadInt32(&stats.probingInProgress),
	}
}

// GetEffectiveWeight calculates the effective weight for a model based on its circuit breaker state.
func (m *ModelCircuitBreaker) GetEffectiveWeight(ctx context.Context, channelID int, modelID string, baseWeight float64) float64 {
	stats := m.getStats(channelID, modelID)

	stats.RLock()
	defer stats.RUnlock()

	policy := m.GetPolicy(ctx)

	switch stats.State {
	case StateClosed:
		return baseWeight

	case StateHalfOpen:
		// Half-Open: reduce weight to decrease traffic, but retain probing capability
		return baseWeight * policy.HalfOpenWeight

	case StateOpen:
		// Open: default weight is 0
		// [Key] Lazy Probe logic:
		// If current time has passed NextProbeAt, allow only ONE request for probing
		if time.Now().After(stats.NextProbeAt) {
			if atomic.LoadInt32(&stats.probingInProgress) == 0 {
				return 0.01
			}
		}

		return 0.0

	default:
		return baseWeight
	}
}

func (m *ModelCircuitBreaker) TryBeginProbe(ctx context.Context, channelID int, modelID string) bool {
	stats := m.getStats(channelID, modelID)

	stats.Lock()
	defer stats.Unlock()

	if stats.State != StateOpen {
		return false
	}

	if stats.NextProbeAt.IsZero() || time.Now().Before(stats.NextProbeAt) {
		return false
	}

	return atomic.CompareAndSwapInt32(&stats.probingInProgress, 0, 1)
}

func (m *ModelCircuitBreaker) EndProbe(channelID int, modelID string) {
	stats := m.getStats(channelID, modelID)
	atomic.StoreInt32(&stats.probingInProgress, 0)
}

// GetAllNonClosedModels returns all models that are not in closed state.
func (m *ModelCircuitBreaker) GetAllNonClosedModels(ctx context.Context) []*ModelCircuitBreakerStats {
	var nonClosed []*ModelCircuitBreakerStats

	m.modelStats.Range(func(_ ChannelModelKey, stats *ModelCircuitBreakerStats) bool {
		stats.RLock()

		if stats.State != StateClosed {
			// Create a copy
			nonClosed = append(nonClosed, &ModelCircuitBreakerStats{
				ChannelID:           stats.ChannelID,
				ModelID:             stats.ModelID,
				State:               stats.State,
				ConsecutiveFailures: stats.ConsecutiveFailures,
				LastFailureAt:       stats.LastFailureAt,
				LastSuccessAt:       stats.LastSuccessAt,
				NextProbeAt:         stats.NextProbeAt,
				probeAttempts:       stats.probeAttempts,
				probingInProgress:   atomic.LoadInt32(&stats.probingInProgress),
			})
		}

		stats.RUnlock()

		return true
	})

	return nonClosed
}

// GetChannelModelCircuitBreakerStats returns circuit breaker statistics for all models in a specific channel.
func (m *ModelCircuitBreaker) GetChannelModelCircuitBreakerStats(ctx context.Context, channelID int) []*ModelCircuitBreakerStats {
	var channelModels []*ModelCircuitBreakerStats

	m.modelStats.Range(func(_ ChannelModelKey, stats *ModelCircuitBreakerStats) bool {
		stats.RLock()

		if stats.ChannelID == channelID {
			// Create a copy
			channelModels = append(channelModels, &ModelCircuitBreakerStats{
				ChannelID:           stats.ChannelID,
				ModelID:             stats.ModelID,
				State:               stats.State,
				ConsecutiveFailures: stats.ConsecutiveFailures,
				LastFailureAt:       stats.LastFailureAt,
				LastSuccessAt:       stats.LastSuccessAt,
				NextProbeAt:         stats.NextProbeAt,
				probeAttempts:       stats.probeAttempts,
				probingInProgress:   atomic.LoadInt32(&stats.probingInProgress),
			})
		}

		stats.RUnlock()

		return true
	})

	return channelModels
}

// ResetModelStatus manually resets a model's circuit breaker state to closed.
// This is useful for manual intervention by operators.
func (m *ModelCircuitBreaker) ResetModelStatus(ctx context.Context, channelID int, modelID string) error {
	stats := m.getStats(channelID, modelID)

	stats.Lock()
	defer stats.Unlock()

	oldState := stats.State
	stats.State = StateClosed
	stats.ConsecutiveFailures = 0
	stats.NextProbeAt = time.Time{}
	stats.probeAttempts = 0
	atomic.StoreInt32(&stats.probingInProgress, 0)

	log.Info(ctx, "Model circuit breaker manually reset to closed",
		log.Int("channel_id", channelID),
		log.String("model_id", modelID),
		log.String("previous_state", string(oldState)),
	)

	return nil
}
