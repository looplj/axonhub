package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ModelCircuitBreakerProvider provides model circuit breaker information.
type ModelCircuitBreakerProvider interface {
	GetEffectiveWeight(ctx context.Context, channelID int, modelID string, baseWeight float64) float64
	GetModelCircuitBreakerStats(ctx context.Context, channelID int, modelID string) *biz.ModelCircuitBreakerStats
}

// ModelAwareCircuitBreakerStrategy implements a load balancing strategy that considers model circuit breaker status.
// It adjusts channel scores based on the state of the requested model on each channel.
type ModelAwareCircuitBreakerStrategy struct {
	cbProvider ModelCircuitBreakerProvider
	maxScore   float64
}

// NewModelAwareCircuitBreakerStrategy creates a new circuit breaker load balancing strategy.
func NewModelAwareCircuitBreakerStrategy(cbProvider ModelCircuitBreakerProvider) *ModelAwareCircuitBreakerStrategy {
	return &ModelAwareCircuitBreakerStrategy{
		cbProvider: cbProvider,
		maxScore:   200.0, // Higher than other strategies to prioritize health
	}
}

// Name returns the strategy name.
func (s *ModelAwareCircuitBreakerStrategy) Name() string {
	return "ModelAwareCircuitBreaker"
}

// Score calculates the score based on model circuit breaker status.
// This is the production path with minimal overhead.
func (s *ModelAwareCircuitBreakerStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	// Get the requested model from context
	modelID := requestedModelFromContext(ctx)
	if modelID == "" {
		// If no specific model is requested, return neutral score
		return s.maxScore * 0.5
	}

	// Get effective weight based on model circuit breaker state
	effectiveWeight := s.cbProvider.GetEffectiveWeight(ctx, channel.ID, modelID, 1.0)

	// Convert weight to score (0.0 to maxScore)
	score := effectiveWeight * s.maxScore

	// Add a small random factor (0-1) to ensure even distribution when circuit breaker state is equal
	// This prevents always selecting the same channel when all channels have the same status
	// Use time-based randomization to ensure different scores on each request
	if effectiveWeight > 0 {
		now := time.Now()
		// Use channel ID and current time to create a distributed but changing random factor
		// This ensures that the same channel gets different scores on different requests
		randomSeed := float64(channel.ID)*0.1 + float64(now.UnixNano()%1000000000)/1000000000.0
		randomFactor := randomSeed - float64(int(randomSeed)) // Get fractional part (0-1)
		score += randomFactor
	}

	return score
}

// ScoreWithDebug calculates the score with detailed debug information.
func (s *ModelAwareCircuitBreakerStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	startTime := time.Now()

	// Get the requested model from context
	modelID := requestedModelFromContext(ctx)

	details := map[string]any{
		"channel_id": channel.ID,
		"model_id":   modelID,
	}

	var score float64
	var cbState string

	if modelID == "" {
		// If no specific model is requested, return neutral score
		score = s.maxScore * 0.5
		cbState = "unknown"
		details["reason"] = "no_model_specified"
	} else {
		// Get model circuit breaker information
		stats := s.cbProvider.GetModelCircuitBreakerStats(ctx, channel.ID, modelID)
		cbState = string(stats.State)

		// Get effective weight based on model health
		effectiveWeight := s.cbProvider.GetEffectiveWeight(ctx, channel.ID, modelID, 1.0)

		// Convert weight to score (0.0 to maxScore)
		score = effectiveWeight * s.maxScore

		// When multiple channels have the same status, use channel weight as secondary factor
		if effectiveWeight > 0 {
			// Add channel weight as a factor (scaled to 0-10 range)
			weightFactor := float64(channel.OrderingWeight) / 100.0
			score += weightFactor
			details["weight_factor"] = weightFactor

			// Add a small random factor (0-1) to ensure even distribution
			now := time.Now()
			randomSeed := float64(channel.ID)*0.1 + float64(now.UnixNano()%1000000000)/1000000000.0
			randomFactor := (randomSeed - float64(int(randomSeed)))
			score += randomFactor
			details["random_factor"] = randomFactor
		}

		details["cb_state"] = cbState
		details["consecutive_failures"] = stats.ConsecutiveFailures
		details["effective_weight"] = effectiveWeight
		details["last_success_at"] = stats.LastSuccessAt
		details["last_failure_at"] = stats.LastFailureAt

		if stats.State == biz.StateOpen && !stats.NextProbeAt.IsZero() {
			details["next_probe_at"] = stats.NextProbeAt
			details["can_probe"] = time.Now().After(stats.NextProbeAt)
		}
	}

	strategyScore := StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
		Duration:     time.Since(startTime),
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "CircuitBreaker strategy scoring",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.String("model_id", modelID),
			log.String("cb_state", cbState),
			log.Float64("score", score),
			log.Int("ordering_weight", channel.OrderingWeight),
		)
	}

	return score, strategyScore
}
