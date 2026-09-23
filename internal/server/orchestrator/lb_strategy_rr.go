package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

const (
	roundRobinScalingFactor    = 150.0
	roundRobinFailureThreshold = int64(3)
	roundRobinHealthCooldown   = 5 * time.Minute
)

func latestActivityAt(metrics *biz.AggregatedMetrics) *time.Time {
	if metrics == nil {
		return nil
	}

	var latest *time.Time
	if metrics.LastSelectedAt != nil {
		latest = metrics.LastSelectedAt
	}

	if metrics.LastFailureAt != nil {
		if latest == nil || metrics.LastFailureAt.After(*latest) {
			latest = metrics.LastFailureAt
		}
	}

	return latest
}

// RoundRobinStrategy prioritizes channels based on their request count history.
// Channels with fewer historical requests get higher priority to ensure even load distribution.
// This strategy is particularly effective when combined with other strategies in a composite approach.
type RoundRobinStrategy struct {
	metricsProvider ChannelMetricsProvider
	// maxScore is the maximum score for a channel with zero requests (default: 150)
	maxScore float64
}

// NewRoundRobinStrategy creates a new round-robin load balancing strategy.
// This strategy implements true round-robin by prioritizing channels with fewer historical requests.
func NewRoundRobinStrategy(metricsProvider ChannelMetricsProvider) *RoundRobinStrategy {
	return &RoundRobinStrategy{
		metricsProvider: metricsProvider,
		maxScore:        150.0,
	}
}

// Score returns a priority score based on the channel's historical request count.
// Production path without debug logging.
// Channels with fewer requests receive higher scores to promote even distribution.
func (s *RoundRobinStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, return a moderate score to be safe
		return s.maxScore / 2
	}

	score, _ := s.calculateScoreComponents(metrics)

	return score
}

// ScoreWithDebug returns a priority score with detailed debug information.
// Debug path with comprehensive logging.
func (s *RoundRobinStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	log.Info(ctx, "RoundRobinStrategy: starting score calculation",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
	)

	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, return a moderate score to be safe
		moderateScore := s.maxScore / 2
		log.Warn(ctx, "RoundRobinStrategy: failed to get metrics, using moderate score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Cause(err),
			log.Float64("moderate_score", moderateScore),
		)

		return moderateScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        moderateScore,
			Details: map[string]any{
				"error": err.Error(),
			},
		}
	}

	score, effectiveCount := s.calculateScoreComponents(metrics)
	requestCount := metrics.RequestCount
	lastActivity := latestActivityAt(metrics)

	details := map[string]any{
		"request_count":           requestCount,
		"effective_request_count": effectiveCount,
		"max_score":               s.maxScore,
		"last_activity_at":        lastActivity,
		"scaling_factor":          roundRobinScalingFactor,
		"calculated_score":        score,
	}

	if requestCount == 0 {
		details["reason"] = "zero_requests"

		log.Info(ctx, "RoundRobinStrategy: channel has zero requests, giving max score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("score", s.maxScore),
		)
	}

	log.Info(ctx, "RoundRobinStrategy: calculated final score",
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
func (s *RoundRobinStrategy) Name() string {
	return "RoundRobin"
}

func (s *RoundRobinStrategy) calculateScoreComponents(metrics *biz.AggregatedMetrics) (float64, float64) {
	if metrics == nil {
		metrics = &biz.AggregatedMetrics{}
	}

	effectiveCount := float64(metrics.RequestCount)
	if effectiveCount < 0 {
		effectiveCount = 0
	}

	// Keep the score strictly decreasing for every request count. The previous
	// exponential score was clamped to a shared minimum, causing busy channels
	// to tie permanently and fall back to deterministic input order.
	// Match WRR's default scale so adding rate-limit scores does not overwhelm
	// request-count differences after only a few requests.
	return s.maxScore / (1 + effectiveCount/roundRobinScalingFactor), effectiveCount
}

// RoundRobinHealthStrategy pushes repeatedly failing channels behind healthy
// round-robin candidates. It is intentionally only used by the round-robin
// top-level strategy so adaptive balancing can keep its softer ErrorAware scoring.
type RoundRobinHealthStrategy struct {
	metricsProvider     ChannelMetricsProvider
	failureThreshold    int64
	failureCooldown     time.Duration
	unhealthyPenalty    float64
	metricsErrorPenalty float64
}

func NewRoundRobinHealthStrategy(metricsProvider ChannelMetricsProvider) *RoundRobinHealthStrategy {
	return &RoundRobinHealthStrategy{
		metricsProvider:     metricsProvider,
		failureThreshold:    roundRobinFailureThreshold,
		failureCooldown:     roundRobinHealthCooldown,
		unhealthyPenalty:    rateLimitExhaustedScore,
		metricsErrorPenalty: 0,
	}
}

func (s *RoundRobinHealthStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		return s.metricsErrorPenalty
	}

	if s.isUnhealthy(metrics) {
		return s.unhealthyPenalty
	}

	return 0
}

func (s *RoundRobinHealthStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		return s.metricsErrorPenalty, StrategyScore{
			StrategyName: s.Name(),
			Score:        s.metricsErrorPenalty,
			Details: map[string]any{
				"error": err.Error(),
			},
		}
	}

	score := 0.0
	unhealthy := s.isUnhealthy(metrics)
	if unhealthy {
		score = s.unhealthyPenalty
	}

	details := map[string]any{
		"consecutive_failures": metrics.ConsecutiveFailures,
		"failure_threshold":    s.failureThreshold,
		"failure_cooldown":     s.failureCooldown.String(),
		"unhealthy":            unhealthy,
	}

	if metrics.LastFailureAt != nil {
		details["last_failure_at"] = metrics.LastFailureAt
		details["time_since_failure"] = time.Since(*metrics.LastFailureAt).String()
	}

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

func (s *RoundRobinHealthStrategy) Name() string {
	return "RoundRobinHealth"
}

func (s *RoundRobinHealthStrategy) IsUnhealthy(ctx context.Context, channel *biz.Channel) bool {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		return false
	}

	return s.isUnhealthy(metrics)
}

func (s *RoundRobinHealthStrategy) isUnhealthy(metrics *biz.AggregatedMetrics) bool {
	if metrics == nil || metrics.ConsecutiveFailures < s.failureThreshold {
		return false
	}

	if metrics.LastFailureAt == nil {
		return true
	}

	return time.Since(*metrics.LastFailureAt) < s.failureCooldown
}

// WeightRoundRobinStrategy implements weighted round-robin load balancing.
// It distributes requests proportionally based on channel weights.
//
// The algorithm normalizes request counts by weight, so higher weight channels
// need proportionally more requests to get the same penalty.
//
// Formula:
//
//	normalizedCount = effectiveCount / (weight / 100.0)
//	score = maxScore / (1 + normalizedCount / scalingFactor)
//
// This means:
//   - weight=80, 80 requests → normalized=100 → score=90
//   - weight=20, 20 requests → normalized=100 → score=90
//   - weight=80, 0 requests → normalized=0 → score=150
//   - weight=20, 0 requests → normalized=0 → score=150
//
// All channels start equal, but higher weight channels can handle more requests
// before their score drops. This achieves proportional distribution:
// - weight=80 gets ~80/(80+50+20+10) = 50% of requests
// - weight=50 gets ~50/(80+50+20+10) = 31% of requests
// - weight=20 gets ~20/(80+50+20+10) = 12.5% of requests
// - weight=10 gets ~10/(80+50+20+10) = 6.25% of requests
//
// The score stays strictly decreasing at every request count, including high
// volume, while the sliding window owns all load expiration.
type WeightRoundRobinStrategy struct {
	metricsProvider ChannelMetricsProvider
	// maxScore is the maximum score for a channel (default: 150)
	maxScore float64
}

// NewWeightRoundRobinStrategy creates a new weighted round-robin strategy.
func NewWeightRoundRobinStrategy(metricsProvider ChannelMetricsProvider) *WeightRoundRobinStrategy {
	return &WeightRoundRobinStrategy{
		metricsProvider: metricsProvider,
		maxScore:        150.0,
	}
}

// calculateScore calculates the weighted round-robin score.
// Normalizes request count by weight to achieve proportional distribution.
func (s *WeightRoundRobinStrategy) calculateScore(metrics *biz.AggregatedMetrics, weight int) (float64, float64, float64) {
	if metrics == nil {
		metrics = &biz.AggregatedMetrics{}
	}

	requestCount := float64(metrics.RequestCount)
	if requestCount < 0 {
		requestCount = 0
	}

	// Normalize request count by weight to achieve proportional distribution.
	// Higher weight channels need more requests to get the same penalty.
	// Formula: normalizedCount = effectiveCount / (weight / 100.0)
	// This means:
	//   - weight=80, 80 requests → normalized=100
	//   - weight=20, 20 requests → normalized=100
	// Both get the same score after receiving their proportional share.
	//
	// Preserve the default weight factor of 1.0 for non-positive weights.
	// Adaptive balancing adds this score to health and latency scores, so the
	// default must retain its scale even when all channels have equal weights.
	effectiveWeight := weight
	if effectiveWeight <= 0 {
		effectiveWeight = 100
	}
	weightFactor := float64(effectiveWeight) / 100.0

	normalizedCount := requestCount / weightFactor

	// Keep the score strictly decreasing without caps or floors. Request counts
	// already expire from the shared sliding window, so no second inactivity
	// decay is applied here.
	score := s.maxScore / (1 + normalizedCount/roundRobinScalingFactor)

	return score, normalizedCount, weightFactor
}

// Score returns a weighted round-robin score.
// Production path without debug logging.
func (s *WeightRoundRobinStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, return a moderate score
		return s.maxScore / 2
	}

	score, _, _ := s.calculateScore(metrics, channel.OrderingWeight)

	return score
}

// ScoreWithDebug returns a weighted round-robin score with detailed debug information.
// Debug path with comprehensive logging.
func (s *WeightRoundRobinStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	log.Info(ctx, "WeightRoundRobinStrategy: starting score calculation",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Int("ordering_weight", channel.OrderingWeight),
	)

	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, return a moderate score
		moderateScore := s.maxScore / 2

		log.Warn(ctx, "WeightRoundRobinStrategy: failed to get metrics, using moderate score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Cause(err),
			log.Float64("moderate_score", moderateScore),
		)

		return moderateScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        moderateScore,
			Details: map[string]any{
				"error": err.Error(),
			},
		}
	}

	requestCount := metrics.RequestCount
	score, normalizedCount, weightFactor := s.calculateScore(metrics, channel.OrderingWeight)

	details := map[string]any{
		"request_count":            requestCount,
		"ordering_weight":          channel.OrderingWeight,
		"weight_factor":            weightFactor,
		"normalized_request_count": normalizedCount,
		"max_score":                s.maxScore,
		"scaling_factor":           roundRobinScalingFactor,
		"calculated_score":         score,
	}

	if requestCount == 0 {
		details["reason"] = "zero_requests"

		log.Info(ctx, "WeightRoundRobinStrategy: channel has zero requests",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("score", score),
		)
	}

	log.Info(ctx, "WeightRoundRobinStrategy: calculated weighted round-robin score",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Int("ordering_weight", channel.OrderingWeight),
		log.Float64("request_count", float64(requestCount)),
		log.Float64("normalized_count", normalizedCount),
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
func (s *WeightRoundRobinStrategy) Name() string {
	return "WeightRoundRobin"
}
