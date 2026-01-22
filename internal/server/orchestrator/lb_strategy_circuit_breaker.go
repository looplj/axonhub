package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ModelHealthProvider provides model health information.
type ModelHealthProvider interface {
	GetEffectiveWeight(ctx context.Context, channelID int, modelID string, baseWeight float64) float64
	GetModelHealth(ctx context.Context, channelID int, modelID string) *biz.ModelHealthStats
}

// CircuitBreakerStrategy implements a load balancing strategy that considers model health status.
// It adjusts channel scores based on the health of the requested model on each channel.
type CircuitBreakerStrategy struct {
	healthProvider ModelHealthProvider
	maxScore       float64
}

// NewCircuitBreakerStrategy creates a new circuit breaker load balancing strategy.
func NewCircuitBreakerStrategy(healthProvider ModelHealthProvider) *CircuitBreakerStrategy {
	return &CircuitBreakerStrategy{
		healthProvider: healthProvider,
		maxScore:       200.0, // Higher than other strategies to prioritize health
	}
}

// Name returns the strategy name.
func (s *CircuitBreakerStrategy) Name() string {
	return "CircuitBreaker"
}

// Score calculates the score based on model health status.
// This is the production path with minimal overhead.
func (s *CircuitBreakerStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	// Get the requested model from context
	modelID := getRequestedModelFromContext(ctx)
	if modelID == "" {
		// If no specific model is requested, return neutral score
		return s.maxScore * 0.5
	}

	// Get effective weight based on model health
	effectiveWeight := s.healthProvider.GetEffectiveWeight(ctx, channel.ID, modelID, 1.0)

	// Convert weight to score (0.0 to maxScore)
	score := effectiveWeight * s.maxScore

	// Add a small random factor (0-1) to ensure even distribution when health status is equal
	// This prevents always selecting the same channel when all channels have the same health status
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
func (s *CircuitBreakerStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	startTime := time.Now()

	// Get the requested model from context
	modelID := getRequestedModelFromContext(ctx)

	details := map[string]any{
		"channel_id": channel.ID,
		"model_id":   modelID,
	}

	var score float64
	var healthStatus string

	if modelID == "" {
		// If no specific model is requested, return neutral score
		score = s.maxScore * 0.5
		healthStatus = "unknown"
		details["reason"] = "no_model_specified"
	} else {
		// Get model health information
		health := s.healthProvider.GetModelHealth(ctx, channel.ID, modelID)
		healthStatus = string(health.Status)

		// Get effective weight based on model health
		effectiveWeight := s.healthProvider.GetEffectiveWeight(ctx, channel.ID, modelID, 1.0)

		// Convert weight to score (0.0 to maxScore)
		score = effectiveWeight * s.maxScore

		// When multiple channels have the same health status, use channel weight as secondary factor
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

		details["health_status"] = healthStatus
		details["consecutive_failures"] = health.ConsecutiveFailures
		details["effective_weight"] = effectiveWeight
		details["last_success_at"] = health.LastSuccessAt
		details["last_failure_at"] = health.LastFailureAt

		if health.Status == biz.StatusDisabled && !health.NextProbeAt.IsZero() {
			details["next_probe_at"] = health.NextProbeAt
			details["can_probe"] = time.Now().After(health.NextProbeAt)
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
			log.String("health_status", healthStatus),
			log.Float64("score", score),
			log.Int("ordering_weight", channel.OrderingWeight),
		)
	}

	return score, strategyScore
}

// getRequestedModelFromContext extracts the requested model ID from the context.
func getRequestedModelFromContext(ctx context.Context) string {
	if model, ok := ctx.Value(requestedModelKey).(string); ok {
		return model
	}
	return ""
}
