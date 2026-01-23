package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ErrorAwareStrategy deprioritizes channels based on their recent error history.
// It calculates a health score by applying time-decayed penalties for failures.
//
// This strategy only applies PENALTIES for errors and does not provide boosts for
// successful requests. This design prevents the "Matthew effect" where high-performing
// channels dominate the distribution at the expense of others.
//
// Historical success rate is intentionally excluded to allow channels to recover
// quickly after a period of instability, as long as they are currently working.
//
// Penalties applied:
//   - Consecutive failures: -30 per failure, decaying linearly over the cooldown period.
//   - Recent failure: A base penalty of -40 that decays linearly over the cooldown period.
type ErrorAwareStrategy struct {
	metricsProvider ChannelMetricsProvider
	// maxScore is the maximum score for a perfectly healthy channel (default: 200)
	maxScore float64
	// basePenalty is the base penalty for any recent failure (default: 40)
	basePenalty float64
	// penaltyPerConsecutiveFailure is the score penalty per consecutive failure (default: 30)
	penaltyPerConsecutiveFailure float64
	// errorCooldownMinutes is how long to remember errors (default: 5 minutes)
	errorCooldownMinutes int
}

// NewErrorAwareStrategy creates a new error-aware strategy.
func NewErrorAwareStrategy(metricsProvider ChannelMetricsProvider) *ErrorAwareStrategy {
	return &ErrorAwareStrategy{
		metricsProvider:              metricsProvider,
		maxScore:                     200.0,
		basePenalty:                  40.0,
		penaltyPerConsecutiveFailure: 30.0,
		errorCooldownMinutes:         5,
	}
}

// Score returns a health score based on recent errors.
// Production path without debug logging.
func (s *ErrorAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, give neutral score
		return s.maxScore / 2
	}

	score := s.maxScore

	// Calculate time-based decay factor if there was a recent failure
	// This ensures penalties (including consecutive failures) eventually decay to zero,
	// allowing the channel to recover and avoiding permanent deadlock.
	cooldownRatio := 0.0
	if metrics.LastFailureAt != nil {
		timeSinceFailure := time.Since(*metrics.LastFailureAt)
		if timeSinceFailure < time.Duration(s.errorCooldownMinutes)*time.Minute {
			cooldownRatio = 1.0 - (timeSinceFailure.Minutes() / float64(s.errorCooldownMinutes))
		}
	} else if metrics.ConsecutiveFailures > 0 {
		// Fallback: if we have consecutive failures but no timestamp, assume it just happened
		cooldownRatio = 1.0
	}

	// Penalize for consecutive failures with time-based decay
	if metrics.ConsecutiveFailures > 0 && cooldownRatio > 0 {
		penalty := float64(metrics.ConsecutiveFailures) * s.penaltyPerConsecutiveFailure * cooldownRatio
		score -= penalty
	}

	// Apply recent failure penalty (also time-based)
	if cooldownRatio > 0 {
		penalty := s.basePenalty * cooldownRatio
		score -= penalty
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
			Details: map[string]any{
				"error": err.Error(),
			},
		}
	}

	score := s.maxScore
	details := map[string]any{
		"consecutive_failures": metrics.ConsecutiveFailures,
		"request_count":        metrics.RequestCount,
	}

	// Calculate time-based decay factor if there was a recent failure
	// This ensures penalties (including consecutive failures) eventually decay to zero,
	// allowing the channel to recover and avoiding permanent deadlock.
	cooldownRatio := 0.0

	var timeSinceFailure time.Duration
	if metrics.LastFailureAt != nil {
		timeSinceFailure = time.Since(*metrics.LastFailureAt)
		if timeSinceFailure < time.Duration(s.errorCooldownMinutes)*time.Minute {
			cooldownRatio = 1.0 - (timeSinceFailure.Minutes() / float64(s.errorCooldownMinutes))
		}
	} else if metrics.ConsecutiveFailures > 0 {
		// Fallback: if we have consecutive failures but no timestamp, assume it just happened
		cooldownRatio = 1.0
	}

	// Penalize for consecutive failures with time-based decay
	if metrics.ConsecutiveFailures > 0 && cooldownRatio > 0 {
		penalty := float64(metrics.ConsecutiveFailures) * s.penaltyPerConsecutiveFailure * cooldownRatio
		score -= penalty
		details["consecutive_failures_penalty"] = penalty
		log.Info(ctx, "ErrorAwareStrategy: applying consecutive failures penalty (with decay)",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int64("consecutive_failures", metrics.ConsecutiveFailures),
			log.Float64("penalty", penalty),
			log.Float64("decay_ratio", cooldownRatio),
		)
	}

	// Apply recent failure penalty (also time-based)
	if cooldownRatio > 0 {
		penalty := s.basePenalty * cooldownRatio
		score -= penalty
		details["recent_failure_penalty"] = penalty

		details["decay_ratio"] = cooldownRatio
		if metrics.LastFailureAt != nil {
			details["time_since_failure_minutes"] = timeSinceFailure.Minutes()
			log.Info(ctx, "ErrorAwareStrategy: applying recent failure penalty",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Duration("time_since_failure", timeSinceFailure),
				log.Float64("penalty", penalty),
			)
		}
	} else if metrics.LastFailureAt != nil {
		log.Info(ctx, "ErrorAwareStrategy: failure outside cooldown period",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Duration("time_since_failure", timeSinceFailure),
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
	IncrementConnection(channelID int)
	DecrementConnection(channelID int)
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
