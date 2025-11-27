package chat

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ChannelMetricsProvider provides channel performance metrics.
type ChannelMetricsProvider interface {
	GetChannelMetrics(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error)
}

// ChannelTraceProvider provides trace-related channel information.
type ChannelTraceProvider interface {
	GetLastSuccessfulChannelID(ctx context.Context, traceID int) (int, error)
}

// TraceAwareStrategy prioritizes the last successful channel from the trace context.
// If a trace ID exists and has a last successful channel, that channel gets maximum score.
type TraceAwareStrategy struct {
	traceProvider ChannelTraceProvider
	// Score boost for the last successful channel (default: 1000)
	boostScore float64
}

// NewTraceAwareStrategy creates a new trace-aware strategy.
func NewTraceAwareStrategy(traceProvider ChannelTraceProvider) *TraceAwareStrategy {
	return &TraceAwareStrategy{
		traceProvider: traceProvider,
		boostScore:    1000.0,
	}
}

// Score returns maximum score if this channel was the last successful one in the trace.
// Production path without debug logging.
func (s *TraceAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	trace, hasTrace := contexts.GetTrace(ctx)
	if !hasTrace {
		return 0
	}

	lastChannelID, err := s.traceProvider.GetLastSuccessfulChannelID(ctx, trace.ID)
	if err != nil {
		return 0
	}

	if lastChannelID == 0 {
		return 0
	}

	if channel.ID == lastChannelID {
		return s.boostScore
	}

	return 0
}

// ScoreWithDebug returns maximum score with detailed debug information.
// Debug path with comprehensive logging.
func (s *TraceAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	trace, hasTrace := contexts.GetTrace(ctx)
	if !hasTrace {
		log.Info(ctx, "TraceAwareStrategy: no trace in context, returning 0 score")

		return 0, StrategyScore{
			StrategyName: s.Name(),
			Score:        0,
			Details: map[string]interface{}{
				"reason": "no_trace_in_context",
			},
		}
	}

	lastChannelID, err := s.traceProvider.GetLastSuccessfulChannelID(ctx, trace.ID)
	if err != nil {
		log.Info(ctx, "TraceAwareStrategy: failed to get last successful channel ID",
			log.Int("trace_id", trace.ID),
			log.Cause(err),
		)

		return 0, StrategyScore{
			StrategyName: s.Name(),
			Score:        0,
			Details: map[string]interface{}{
				"reason":   "error_getting_last_channel",
				"trace_id": trace.ID,
				"error":    err.Error(),
			},
		}
	}

	if lastChannelID == 0 {
		log.Info(ctx, "TraceAwareStrategy: no last successful channel for trace",
			log.Int("trace_id", trace.ID),
		)

		return 0, StrategyScore{
			StrategyName: s.Name(),
			Score:        0,
			Details: map[string]interface{}{
				"reason":   "no_last_successful_channel",
				"trace_id": trace.ID,
			},
		}
	}

	isLastChannel := channel.ID == lastChannelID
	score := 0.0
	details := map[string]interface{}{
		"trace_id":        trace.ID,
		"last_channel_id": lastChannelID,
		"is_last_channel": isLastChannel,
	}

	if isLastChannel {
		score = s.boostScore
		details["reason"] = "last_successful_channel_in_trace"

		log.Info(ctx, "TraceAwareStrategy: boosting channel",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int("trace_id", trace.ID),
			log.Float64("score", score),
			log.String("reason", "last_successful_channel_in_trace"),
		)
	} else {
		details["reason"] = "not_last_successful_channel"

		log.Info(ctx, "TraceAwareStrategy: channel not in trace",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Int("trace_id", trace.ID),
		)
	}

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

// Name returns the strategy name.
func (s *TraceAwareStrategy) Name() string {
	return "TraceAware"
}

// WeightStrategy prioritizes channels based on their ordering weight.
// Higher weight = higher priority.
type WeightStrategy struct {
	// maxScore is the maximum score this strategy can assign (default: 100)
	maxScore float64
}

// NewWeightStrategy creates a new weight-based strategy.
func NewWeightStrategy() *WeightStrategy {
	return &WeightStrategy{
		maxScore: 100.0,
	}
}

// Score returns a score based on the channel's ordering weight.
// Score is normalized to 0-maxScore range.
// Production path without debug logging.
func (s *WeightStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	// Weight is typically 0-100, normalize to 0-maxScore
	weight := float64(channel.OrderingWeight)
	if weight < 0 {
		weight = 0
	}

	// Assume max weight is 100, scale accordingly
	score := (weight / 100.0) * s.maxScore

	return score
}

// ScoreWithDebug returns a score with detailed debug information.
// Debug path with comprehensive logging.
func (s *WeightStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	// Weight is typically 0-100, normalize to 0-maxScore
	weight := float64(channel.OrderingWeight)
	details := map[string]interface{}{
		"ordering_weight": weight,
		"max_score":       s.maxScore,
	}

	if weight < 0 {
		log.Info(ctx, "WeightStrategy: channel has negative weight, clamping to 0",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("weight", weight),
		)

		details["clamped"] = true
		details["original_weight"] = weight
		weight = 0
	}

	// Assume max weight is 100, scale accordingly
	score := (weight / 100.0) * s.maxScore
	details["calculated_score"] = score

	log.Info(ctx, "WeightStrategy: calculated score",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Float64("ordering_weight", weight),
		log.Float64("max_score", s.maxScore),
		log.Float64("score", score),
	)

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

// Name returns the strategy name.
func (s *WeightStrategy) Name() string {
	return "Weight"
}

// ErrorAwareStrategy deprioritizes channels with recent errors.
// Uses channel performance metrics to calculate a health score.
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

	// Boost for channels with recent success
	if metrics.LastSuccessAt != nil {
		timeSinceSuccess := time.Since(*metrics.LastSuccessAt)
		if timeSinceSuccess < 1*time.Minute {
			// Recent success within 1 minute gets a small boost
			boost := 20.0
			score += boost
		}
	}

	// Consider success rate (only if we have enough data)
	if metrics.RequestCount >= 10 {
		successRate := metrics.CalculateSuccessRate()

		// If success rate is very low, apply additional penalty
		if successRate < 50 {
			penalty := 50.0
			score -= penalty
		} else if successRate > 90 {
			// High success rate gets a small boost
			boost := 30.0
			score += boost
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

	// Boost for channels with recent success
	if metrics.LastSuccessAt != nil {
		timeSinceSuccess := time.Since(*metrics.LastSuccessAt)
		if timeSinceSuccess < 1*time.Minute {
			// Recent success within 1 minute gets a small boost
			boost := 20.0
			score += boost
			details["recent_success_boost"] = boost
			log.Info(ctx, "ErrorAwareStrategy: applying recent success boost",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Duration("time_since_success", timeSinceSuccess),
				log.Float64("boost", boost),
			)
		} else {
			log.Info(ctx, "ErrorAwareStrategy: success outside boost window",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Duration("time_since_success", timeSinceSuccess),
			)
		}
	}

	// Consider success rate (only if we have enough data)
	if metrics.RequestCount >= 10 {
		successRate := metrics.CalculateSuccessRate()
		details["success_rate"] = successRate

		// If success rate is very low, apply additional penalty
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
		} else if successRate > 90 {
			// High success rate gets a small boost
			boost := 30.0
			score += boost
			details["high_success_rate_boost"] = boost
			log.Info(ctx, "ErrorAwareStrategy: applying high success rate boost",
				log.Int("channel_id", channel.ID),
				log.String("channel_name", channel.Name),
				log.Int64("success_rate", successRate),
				log.Float64("boost", boost),
			)
		} else {
			log.Info(ctx, "ErrorAwareStrategy: success rate in normal range",
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

// CompositeStrategy combines multiple strategies with configurable weights.
type CompositeStrategy struct {
	strategies []weightedStrategy
}

type weightedStrategy struct {
	strategy LoadBalanceStrategy
	weight   float64
}

// NewCompositeStrategy creates a new composite strategy.
func NewCompositeStrategy(strategies ...LoadBalanceStrategy) *CompositeStrategy {
	weighted := make([]weightedStrategy, len(strategies))
	for i, s := range strategies {
		weighted[i] = weightedStrategy{
			strategy: s,
			weight:   1.0, // Default weight
		}
	}

	return &CompositeStrategy{
		strategies: weighted,
	}
}

// WithWeights sets custom weights for the strategies.
// weights slice should match the order of strategies.
func (c *CompositeStrategy) WithWeights(weights ...float64) *CompositeStrategy {
	for i, w := range weights {
		if i < len(c.strategies) {
			c.strategies[i].weight = w
		}
	}

	return c
}

// Score combines all strategy scores with their weights.
// Production path without debug logging.
func (c *CompositeStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	totalScore := 0.0

	for _, ws := range c.strategies {
		score := ws.strategy.Score(ctx, channel)
		totalScore += score * ws.weight
	}

	return totalScore
}

// ScoreWithDebug combines all strategy scores with detailed debug information.
// Debug path with comprehensive logging.
func (c *CompositeStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	totalScore := 0.0
	details := map[string]any{}

	strategies := make([]map[string]any, 0, len(c.strategies))

	for _, ws := range c.strategies {
		score, strategyScore := ws.strategy.ScoreWithDebug(ctx, channel)
		weightedScore := score * ws.weight
		totalScore += weightedScore

		strategy := map[string]any{
			"name":           strategyScore.StrategyName,
			"score":          score,
			"weight":         ws.weight,
			"weighted_score": weightedScore,
			"details":        strategyScore.Details,
		}
		strategies = append(strategies, strategy)
	}

	details["strategies"] = strategies
	details["total_score"] = totalScore

	return totalScore, StrategyScore{
		StrategyName: c.Name(),
		Score:        totalScore,
		Details:      details,
	}
}

// Name returns the strategy name.
func (c *CompositeStrategy) Name() string {
	return "Composite"
}
