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
	defaultRoundRobinInactivityDecay = 15 * time.Second
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

// WeightRoundRobinStrategy combines round-robin (request count based) and weight-based strategies.
// It prioritizes channels with fewer historical requests while respecting user-configured ordering weights.
// The final score is a combination of:
//   - Round-robin score: Based on request count (10-150 range)
//   - Weight score: Based on ordering weight (0-50 range)
//   - Total: 10-200 range
//
// This allows new channels (low request count) to get high priority, while also
// letting administrators boost priority of specific channels via ordering weight.
type WeightRoundRobinStrategy struct {
	metricsProvider ChannelMetricsProvider
	// maxRoundRobinScore is the maximum score from round-robin component (default: 150)
	maxRoundRobinScore float64
	// minScore is the minimum total score (default: 10)
	minScore float64
	// requestCountCap caps the maximum request count considered (default: 1000)
	requestCountCap int64
	// maxWeightScore is the maximum score from weight component (default: 50)
	maxWeightScore float64
	// inactivityDecay mirrors RoundRobinStrategy to decay historical load when channel is idle
	inactivityDecay time.Duration
}

// NewWeightRoundRobinStrategy creates a new combined weight + round-robin strategy.
func NewWeightRoundRobinStrategy(metricsProvider ChannelMetricsProvider) *WeightRoundRobinStrategy {
	return &WeightRoundRobinStrategy{
		metricsProvider:    metricsProvider,
		maxRoundRobinScore: 150.0,
		minScore:           10.0,
		requestCountCap:    1000,
		maxWeightScore:     50.0,
		inactivityDecay:    defaultRoundRobinInactivityDecay,
	}
}

// calculateRoundRobinScore calculates the round-robin component based on request count.
func (s *WeightRoundRobinStrategy) calculateRoundRobinScore(metrics *biz.AggregatedMetrics) (float64, float64, float64, *time.Time, float64) {
	if metrics == nil {
		metrics = &biz.AggregatedMetrics{}
	}

	lastActivity := latestActivityAt(metrics)
	cappedCount, effectiveCount, inactivitySeconds := computeRequestLoad(metrics.RequestCount, s.requestCountCap, lastActivity, s.inactivityDecay)

	score := s.maxRoundRobinScore * math.Exp(-effectiveCount/roundRobinScalingFactor)
	if score < 0 {
		score = 0
	}

	return score, cappedCount, effectiveCount, lastActivity, inactivitySeconds
}

// calculateWeightScore calculates the weight component based on ordering weight.
func (s *WeightRoundRobinStrategy) calculateWeightScore(orderingWeight int) float64 {
	// Weight is typically 0-100, normalize to 0-maxWeightScore
	weight := float64(orderingWeight)
	if weight < 0 {
		weight = 0
	}

	// Assume max weight is 100, scale accordingly
	score := (weight / 100.0) * s.maxWeightScore

	return score
}

// Score returns a combined score based on both request count and ordering weight.
// Production path without debug logging.
func (s *WeightRoundRobinStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		// If we can't get metrics, return a moderate score (midpoint of round-robin + max weight)
		moderateRoundRobin := (s.maxRoundRobinScore + s.minScore) / 2
		moderateWeight := s.maxWeightScore / 2

		return moderateRoundRobin + moderateWeight
	}

	// Calculate round-robin component from request count with recency awareness
	roundRobinScore, _, _, _, _ := s.calculateRoundRobinScore(metrics)

	// Calculate weight component from ordering weight
	weightScore := s.calculateWeightScore(channel.OrderingWeight)

	// Total score is the sum of both components
	totalScore := roundRobinScore + weightScore

	// Ensure total doesn't go below minimum
	if totalScore < s.minScore {
		totalScore = s.minScore
	}

	return totalScore
}

// ScoreWithDebug returns a combined score with detailed debug information.
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
		moderateRoundRobin := (s.maxRoundRobinScore + s.minScore) / 2
		moderateWeight := s.maxWeightScore / 2
		moderateTotal := moderateRoundRobin + moderateWeight

		log.Warn(ctx, "WeightRoundRobinStrategy: failed to get metrics, using moderate score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Cause(err),
			log.Float64("moderate_round_robin", moderateRoundRobin),
			log.Float64("moderate_weight", moderateWeight),
			log.Float64("moderate_total", moderateTotal),
		)

		return moderateTotal, StrategyScore{
			StrategyName: s.Name(),
			Score:        moderateTotal,
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	requestCount := metrics.RequestCount
	roundRobinScore, cappedCount, effectiveCount, lastActivity, inactivitySeconds := s.calculateRoundRobinScore(metrics)

	details := map[string]interface{}{
		"request_count":           requestCount,
		"original_cap":            s.requestCountCap,
		"capped_request_count":    cappedCount,
		"effective_request_count": effectiveCount,
		"max_roundrobin_score":    s.maxRoundRobinScore,
		"min_score":               s.minScore,
		"max_weight_score":        s.maxWeightScore,
		"ordering_weight":         channel.OrderingWeight,
		"last_activity_at":        lastActivity,
		"inactivity_seconds":      inactivitySeconds,
		"scaling_factor":          roundRobinScalingFactor,
		"round_robin_score":       roundRobinScore,
	}

	if requestCount == 0 {
		details["round_robin_reason"] = "zero_requests"

		log.Info(ctx, "WeightRoundRobinStrategy: channel has zero requests, giving max round-robin score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("round_robin_score", s.maxRoundRobinScore),
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

	log.Info(ctx, "WeightRoundRobinStrategy: calculated round-robin component",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Float64("request_count", float64(requestCount)),
		log.Float64("round_robin_score", roundRobinScore),
	)

	// Calculate weight component from ordering weight
	weight := float64(channel.OrderingWeight)
	if weight < 0 {
		log.Info(ctx, "WeightRoundRobinStrategy: channel has negative weight, clamping to 0",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("weight", weight),
		)

		details["weight_clamped"] = true
		details["original_weight"] = weight
		weight = 0
	}

	weightScore := (weight / 100.0) * s.maxWeightScore
	details["weight_factor"] = weight / 100.0
	details["weight_score"] = weightScore

	log.Info(ctx, "WeightRoundRobinStrategy: calculated weight component",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Float64("ordering_weight", weight),
		log.Float64("weight_score", weightScore),
	)

	// Total score is the sum of both components
	totalScore := roundRobinScore + weightScore
	details["total_score_before_clamp"] = totalScore

	// Ensure total doesn't go below minimum
	if totalScore < s.minScore {
		log.Info(ctx, "WeightRoundRobinStrategy: total score clamped to minimum",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("original_total_score", totalScore),
			log.Float64("min_score", s.minScore),
		)
		totalScore = s.minScore
		details["total_score_clamped"] = true
		details["final_total_score"] = totalScore
	}

	log.Info(ctx, "WeightRoundRobinStrategy: calculated final total score",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Float64("round_robin_score", roundRobinScore),
		log.Float64("weight_score", weightScore),
		log.Float64("final_total_score", totalScore),
		log.Any("calculation_details", details),
	)

	return totalScore, StrategyScore{
		StrategyName: s.Name(),
		Score:        totalScore,
		Details:      details,
	}
}

// Name returns the strategy name.
func (s *WeightRoundRobinStrategy) Name() string {
	return "WeightRoundRobin"
}
