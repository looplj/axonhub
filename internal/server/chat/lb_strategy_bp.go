package chat

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ErrorAwareStrategy deprioritizes channels with recent errors.
// Uses channel performance metrics to calculate a health score.
//
// This strategy only applies PENALTIES for errors, never boosts for success.
// This ensures that the weighted round-robin distribution is not disrupted
// by success-based boosts that would cause the "Matthew effect" (rich get richer).
//
// Penalties applied:
//   - Consecutive failures: -50 per failure
//   - Recent failure (within 5 min): up to -100, decreasing over time
//   - Low success rate (<50%): -50
type ErrorAwareStrategy struct {
	metricsProvider ChannelMetricsProvider
	// maxScore is the maximum score for a perfectly healthy channel (default: 200)
	maxScore float64
	// penaltyPerConsecutiveFailure is the score penalty per consecutive failure
	penaltyPerConsecutiveFailure float64
	// errorCooldownMinutes is how long to remember errors (default: 5 minutes)
	errorCooldownMinutes int
}

// NewErrorAwareStrategy creates a new error-aware strategy.
func NewErrorAwareStrategy(metricsProvider ChannelMetricsProvider) *ErrorAwareStrategy {
	return &ErrorAwareStrategy{
		metricsProvider:              metricsProvider,
		maxScore:                     200.0,
		penaltyPerConsecutiveFailure: 50.0,
		errorCooldownMinutes:         5,
	}
}

// Score returns a health score based on recent errors and success rate.
// Production path without debug logging.
func (s *ErrorAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, give neutral score
		return s.maxScore / 2
	}

	score := s.maxScore

	// Penalize for consecutive failures
	if metrics.ConsecutiveFailures > 0 {
		penalty := float64(metrics.ConsecutiveFailures) * s.penaltyPerConsecutiveFailure
		score -= penalty
	}

	// Check if there was a recent failure (within cooldown period)
	if metrics.LastFailureAt != nil {
		timeSinceFailure := time.Since(*metrics.LastFailureAt)
		if timeSinceFailure < time.Duration(s.errorCooldownMinutes)*time.Minute {
			// Apply time-based penalty that decreases over time
			cooldownRatio := 1.0 - (timeSinceFailure.Minutes() / float64(s.errorCooldownMinutes))
			penalty := 100.0 * cooldownRatio
			score -= penalty
		}
	}

	// Only apply penalty for very low success rate (indicates a problematic channel)
	if metrics.RequestCount >= 5 {
		successRate := metrics.CalculateSuccessRate()
		if successRate < 50 {
			penalty := 50.0
			score -= penalty
		}
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

// ScoreWithDebug returns a health score with detailed debug information.
// Debug path with comprehensive logging.
func (s *ErrorAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	log.Info(ctx, "ErrorAwareStrategy: starting score calculation",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
	)

	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, give neutral score
		neutralScore := s.maxScore / 2
		log.Warn(ctx, "ErrorAwareStrategy: failed to get metrics, using neutral score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Cause(err),
			log.Float64("neutral_score", neutralScore),
		)

		return neutralScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        neutralScore,
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	score := s.maxScore
	details := map[string]interface{}{
		"consecutive_failures": metrics.ConsecutiveFailures,
		"request_count":        metrics.RequestCount,
	}

	// Penalize for consecutive failures
	if metrics.ConsecutiveFailures > 0 {
		penalty := float64(metrics.ConsecutiveFailures) * s.penaltyPerConsecutiveFailure
		score -= penalty
		details["consecutive_failures_penalty"] = penalty
		log.Info(ctx, "ErrorAwareStrategy: applying consecutive failures penalty",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int64("consecutive_failures", metrics.ConsecutiveFailures),
			log.Float64("penalty", penalty),
		)
	}

	// Check if there was a recent failure (within cooldown period)
	if metrics.LastFailureAt != nil {
		timeSinceFailure := time.Since(*metrics.LastFailureAt)
		if timeSinceFailure < time.Duration(s.errorCooldownMinutes)*time.Minute {
			// Apply time-based penalty that decreases over time
			cooldownRatio := 1.0 - (timeSinceFailure.Minutes() / float64(s.errorCooldownMinutes))
			penalty := 100.0 * cooldownRatio
			score -= penalty
			details["recent_failure_penalty"] = penalty
			details["time_since_failure_minutes"] = timeSinceFailure.Minutes()
			log.Info(ctx, "ErrorAwareStrategy: applying recent failure penalty",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Duration("time_since_failure", timeSinceFailure),
				log.Float64("penalty", penalty),
			)
		} else {
			log.Info(ctx, "ErrorAwareStrategy: failure outside cooldown period",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Duration("time_since_failure", timeSinceFailure),
			)
		}
	}

	// Only apply penalty for very low success rate (indicates a problematic channel)
	if metrics.RequestCount >= 5 {
		successRate := metrics.CalculateSuccessRate()
		details["success_rate"] = successRate

		if successRate < 50 {
			penalty := 50.0
			score -= penalty
			details["low_success_rate_penalty"] = penalty
			log.Info(ctx, "ErrorAwareStrategy: applying low success rate penalty",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Int64("success_rate", successRate),
				log.Float64("penalty", penalty),
			)
		} else {
			log.Info(ctx, "ErrorAwareStrategy: success rate acceptable, no penalty",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Int64("success_rate", successRate),
			)
		}
	} else {
		log.Info(ctx, "ErrorAwareStrategy: insufficient request count for success rate check",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int64("request_count", metrics.RequestCount),
		)
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		log.Info(ctx, "ErrorAwareStrategy: score clamped to 0",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("original_score", score),
		)
		score = 0
	}

	log.Info(ctx, "ErrorAwareStrategy: calculated final score",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Float64("final_score", score),
		log.Any("calculation_details", details),
	)

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

// Name returns the strategy name.
func (s *ErrorAwareStrategy) Name() string {
	return "ErrorAware"
}

// ConnectionAwareStrategy considers the current number of active connections.
// Channels with fewer active connections get higher priority.
// This is a placeholder implementation - you'll need to track active connections.
type ConnectionAwareStrategy struct {
	channelService *biz.ChannelService
	// maxScore is the maximum score (default: 50)
	maxScore float64
	// This would need integration with actual connection tracking
	connectionTracker ConnectionTracker
}

// ConnectionTracker is an interface for tracking active connections per channel.
// This needs to be implemented based on your connection pooling mechanism.
type ConnectionTracker interface {
	GetActiveConnections(channelID int) int
	GetMaxConnections(channelID int) int
}

// NewConnectionAwareStrategy creates a new connection-aware strategy.
func NewConnectionAwareStrategy(channelService *biz.ChannelService, tracker ConnectionTracker) *ConnectionAwareStrategy {
	return &ConnectionAwareStrategy{
		channelService:    channelService,
		maxScore:          50.0,
		connectionTracker: tracker,
	}
}

// Score returns a score based on available connection capacity.
// Production path without debug logging.
func (s *ConnectionAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	if s.connectionTracker == nil {
		// If no tracker, give neutral score
		return s.maxScore / 2
	}

	activeConns := s.connectionTracker.GetActiveConnections(channel.ID)
	maxConns := s.connectionTracker.GetMaxConnections(channel.ID)

	if maxConns == 0 {
		// No limit, give full score
		return s.maxScore
	}

	// Calculate utilization ratio (0-1)
	utilization := float64(activeConns) / float64(maxConns)

	// Score decreases as utilization increases
	// 0% utilization = maxScore, 100% utilization = 0
	score := s.maxScore * (1.0 - utilization)

	return score
}

// ScoreWithDebug returns a score with detailed debug information.
// Debug path with comprehensive logging.
func (s *ConnectionAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	if s.connectionTracker == nil {
		// If no tracker, give neutral score
		neutralScore := s.maxScore / 2
		log.Info(ctx, "ConnectionAwareStrategy: no connection tracker available, using neutral score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("neutral_score", neutralScore),
		)

		return neutralScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        neutralScore,
			Details: map[string]any{
				"reason": "no_connection_tracker",
			},
		}
	}

	activeConns := s.connectionTracker.GetActiveConnections(channel.ID)
	maxConns := s.connectionTracker.GetMaxConnections(channel.ID)

	log.Info(ctx, "ConnectionAwareStrategy: retrieved connection info",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Int("active_connections", activeConns),
		log.Int("max_connections", maxConns),
	)

	details := map[string]any{
		"active_connections": activeConns,
		"max_connections":    maxConns,
	}

	if maxConns == 0 {
		// No limit, give full score
		details["reason"] = "no_connection_limit"

		log.Info(ctx, "ConnectionAwareStrategy: no connection limit set, giving full score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("score", s.maxScore),
		)

		return s.maxScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        s.maxScore,
			Details:      details,
		}
	}

	// Calculate utilization ratio (0-1)
	utilization := float64(activeConns) / float64(maxConns)
	details["utilization"] = utilization

	// Score decreases as utilization increases
	// 0% utilization = maxScore, 100% utilization = 0
	score := s.maxScore * (1.0 - utilization)
	details["calculated_score"] = score

	log.Info(ctx, "ConnectionAwareStrategy: calculated utilization-based score",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Float64("utilization", utilization),
		log.Float64("max_score", s.maxScore),
		log.Float64("final_score", score),
	)

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

// Name returns the strategy name.
func (s *ConnectionAwareStrategy) Name() string {
	return "ConnectionAware"
}
