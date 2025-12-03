package chat

import (
	"context"
	"math"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

const (
	roundRobinScalingFactor          = 150.0
	defaultRoundRobinInactivityDecay = 5 * time.Minute // Increased from 15s to 5min for better round-robin distribution
)

func latestActivityAt(metrics *biz.AggregatedMetrics) *time.Time {
	if metrics == nil {
		return nil
	}

	var latest *time.Time
	if metrics.LastSuccessAt != nil {
		latest = metrics.LastSuccessAt
	}

	if metrics.LastFailureAt != nil {
		if latest == nil || metrics.LastFailureAt.After(*latest) {
			latest = metrics.LastFailureAt
		}
	}

	return latest
}

//nolint:predeclared // Checked.
func computeRequestLoad(requestCount int64, cap int64, lastActivity *time.Time, decay time.Duration) (float64, float64, float64) {
	capped := float64(requestCount)
	if cap > 0 && capped > float64(cap) {
		capped = float64(cap)
	}

	if capped <= 0 {
		return capped, 0, 0
	}

	decaySeconds := decay.Seconds()
	decayMultiplier := 1.0

	inactivitySeconds := 0.0
	if lastActivity != nil {
		inactivitySeconds = time.Since(*lastActivity).Seconds()
		if decaySeconds > 0 && inactivitySeconds > 0 {
			decayMultiplier = math.Exp(-inactivitySeconds / decaySeconds)
		}
	}

	effective := capped * decayMultiplier

	return capped, effective, inactivitySeconds
}

// RoundRobinStrategy prioritizes channels based on their request count history.
// Channels with fewer historical requests get higher priority to ensure even load distribution.
// This strategy is particularly effective when combined with other strategies in a composite approach.
type RoundRobinStrategy struct {
	metricsProvider ChannelMetricsProvider
	// maxScore is the maximum score for a channel with zero requests (default: 150)
	maxScore float64
	// minScore is the minimum score for heavily used channels (default: 10)
	minScore float64
	// requestCountCap caps the maximum request count considered (default: 1000)
	// This prevents channels with extremely high request counts from dominating the calculation
	requestCountCap int64
	// inactivityDecay defines how quickly historical requests lose influence when the channel stays idle
	inactivityDecay time.Duration
}

// NewRoundRobinStrategy creates a new round-robin load balancing strategy.
// This strategy implements true round-robin by prioritizing channels with fewer historical requests.
func NewRoundRobinStrategy(metricsProvider ChannelMetricsProvider) *RoundRobinStrategy {
	return &RoundRobinStrategy{
		metricsProvider: metricsProvider,
		maxScore:        150.0,
		minScore:        10.0,
		requestCountCap: 1000,
		inactivityDecay: defaultRoundRobinInactivityDecay,
	}
}

// Score returns a priority score based on the channel's historical request count.
// Production path without debug logging.
// Channels with fewer requests receive higher scores to promote even distribution.
func (s *RoundRobinStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, return a moderate score to be safe
		return (s.maxScore + s.minScore) / 2
	}

	score, _, _, _, _ := s.calculateScoreComponents(metrics)

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
		moderateScore := (s.maxScore + s.minScore) / 2
		log.Warn(ctx, "RoundRobinStrategy: failed to get metrics, using moderate score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Cause(err),
			log.Float64("moderate_score", moderateScore),
		)

		return moderateScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        moderateScore,
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	score, cappedCount, effectiveCount, lastActivity, inactivitySeconds := s.calculateScoreComponents(metrics)
	requestCount := metrics.RequestCount

	details := map[string]interface{}{
		"request_count":                 requestCount,
		"capped_request_count":          cappedCount,
		"effective_request_count":       effectiveCount,
		"original_cap":                  s.requestCountCap,
		"max_score":                     s.maxScore,
		"min_score":                     s.minScore,
		"last_activity_at":              lastActivity,
		"inactivity_seconds":            inactivitySeconds,
		"scaling_factor":                roundRobinScalingFactor,
		"calculated_score_before_clamp": s.maxScore * math.Exp(-effectiveCount/roundRobinScalingFactor),
		"calculated_score":              score,
	}

	if requestCount == 0 {
		details["reason"] = "zero_requests"

		log.Info(ctx, "RoundRobinStrategy: channel has zero requests, giving max score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("score", s.maxScore),
		)
	}

	if inactivitySeconds > 0 {
		log.Info(ctx, "RoundRobinStrategy: applying inactivity decay",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("inactivity_seconds", inactivitySeconds),
			log.Float64("effective_request_count", effectiveCount),
		)
	}

	//nolint:forcetypeassert // Checked.
	if details["calculated_score_before_clamp"].(float64) != score {
		log.Info(ctx, "RoundRobinStrategy: score clamped to minimum",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("final_score", score),
			log.Float64("min_score", s.minScore),
		)

		details["clamped"] = true
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

func (s *RoundRobinStrategy) calculateScoreComponents(metrics *biz.AggregatedMetrics) (float64, float64, float64, *time.Time, float64) {
	if metrics == nil {
		metrics = &biz.AggregatedMetrics{}
	}

	lastActivity := latestActivityAt(metrics)
	cappedCount, effectiveCount, inactivitySeconds := computeRequestLoad(metrics.RequestCount, s.requestCountCap, lastActivity, s.inactivityDecay)

	rawScore := s.maxScore
	if effectiveCount > 0 {
		rawScore = s.maxScore * math.Exp(-effectiveCount/roundRobinScalingFactor)
	}

	finalScore := rawScore
	if finalScore < s.minScore {
		finalScore = s.minScore
	}

	return finalScore, cappedCount, effectiveCount, lastActivity, inactivitySeconds
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
//	score = maxScore * exp(-normalizedCount / scalingFactor)
//
// This means:
//   - weight=80, 80 requests → normalized=100 → score ~77
//   - weight=20, 20 requests → normalized=100 → score ~77
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
// Score range: 10-150.
type WeightRoundRobinStrategy struct {
	metricsProvider ChannelMetricsProvider
	// maxScore is the maximum score for a channel (default: 150)
	maxScore float64
	// minScore is the minimum score (default: 10)
	minScore float64
	// requestCountCap caps the maximum request count considered (default: 1000)
	requestCountCap int64
	// inactivityDecay mirrors RoundRobinStrategy to decay historical load when channel is idle
	inactivityDecay time.Duration
}

// NewWeightRoundRobinStrategy creates a new weighted round-robin strategy.
func NewWeightRoundRobinStrategy(metricsProvider ChannelMetricsProvider) *WeightRoundRobinStrategy {
	return &WeightRoundRobinStrategy{
		metricsProvider: metricsProvider,
		maxScore:        150.0,
		minScore:        10.0,
		requestCountCap: 1000,
		inactivityDecay: defaultRoundRobinInactivityDecay,
	}
}

// calculateScore calculates the weighted round-robin score.
// Normalizes request count by weight to achieve proportional distribution.
func (s *WeightRoundRobinStrategy) calculateScore(metrics *biz.AggregatedMetrics, weight int) (float64, float64, float64, *time.Time, float64) {
	if metrics == nil {
		metrics = &biz.AggregatedMetrics{}
	}

	lastActivity := latestActivityAt(metrics)
	cappedCount, effectiveCount, inactivitySeconds := computeRequestLoad(metrics.RequestCount, s.requestCountCap, lastActivity, s.inactivityDecay)

	// Normalize request count by weight to achieve proportional distribution.
	// Higher weight channels need more requests to get the same penalty.
	// Formula: normalizedCount = effectiveCount / (weight / 100.0)
	// This means:
	//   - weight=80, 80 requests → normalized=100
	//   - weight=20, 20 requests → normalized=100
	// Both get the same score after receiving their proportional share.
	weightFactor := float64(weight) / 100.0
	if weightFactor <= 0 {
		weightFactor = 0.01 // Avoid division by zero, treat as very low weight
	}

	normalizedCount := effectiveCount / weightFactor

	// Calculate score using the weight-normalized request count
	score := s.maxScore
	if normalizedCount > 0 {
		score = s.maxScore * math.Exp(-normalizedCount/roundRobinScalingFactor)
	}

	if score < s.minScore {
		score = s.minScore
	}

	return score, cappedCount, effectiveCount, lastActivity, inactivitySeconds
}

// Score returns a weighted round-robin score.
// Production path without debug logging.
func (s *WeightRoundRobinStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, return a moderate score
		return (s.maxScore + s.minScore) / 2
	}

	score, _, _, _, _ := s.calculateScore(metrics, channel.OrderingWeight)

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
		moderateScore := (s.maxScore + s.minScore) / 2

		log.Warn(ctx, "WeightRoundRobinStrategy: failed to get metrics, using moderate score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Cause(err),
			log.Float64("moderate_score", moderateScore),
		)

		return moderateScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        moderateScore,
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	requestCount := metrics.RequestCount
	score, cappedCount, effectiveCount, lastActivity, inactivitySeconds := s.calculateScore(metrics, channel.OrderingWeight)

	// Calculate normalized count for debug info
	weightFactor := float64(channel.OrderingWeight) / 100.0
	if weightFactor <= 0 {
		weightFactor = 0.01
	}

	normalizedCount := effectiveCount / weightFactor

	details := map[string]interface{}{
		"request_count":            requestCount,
		"original_cap":             s.requestCountCap,
		"capped_request_count":     cappedCount,
		"effective_request_count":  effectiveCount,
		"ordering_weight":          channel.OrderingWeight,
		"weight_factor":            weightFactor,
		"normalized_request_count": normalizedCount,
		"max_score":                s.maxScore,
		"min_score":                s.minScore,
		"last_activity_at":         lastActivity,
		"inactivity_seconds":       inactivitySeconds,
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

	if inactivitySeconds > 0 {
		log.Info(ctx, "WeightRoundRobinStrategy: applying inactivity decay",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("inactivity_seconds", inactivitySeconds),
			log.Float64("effective_request_count", effectiveCount),
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
