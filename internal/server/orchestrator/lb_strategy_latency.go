package orchestrator

import (
	"context"
	"math"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

const (
	defaultLatencyMaxScore = 80.0

	defaultStreamingFirstTokenMaxLatencyMs = 3000.0
	defaultStreamingMinTokensPerSecond     = 5.0
	defaultStreamingMaxTokensPerSecond     = 100.0
	defaultNonStreamingMaxLatencyMs        = 3000.0

	streamingFirstTokenWeight = 0.7
	streamingThroughputWeight = 0.3
)

// LatencyAwareStrategy prioritizes channels using request-type-specific UX signals.
// Streaming requests are scored by first-token latency plus output throughput.
// Non-streaming requests are scored by end-to-end latency.
type LatencyAwareStrategy struct {
	metricsProvider ChannelMetricsProvider
	maxScore        float64
}

func NewLatencyAwareStrategy(metricsProvider ChannelMetricsProvider) *LatencyAwareStrategy {
	return &LatencyAwareStrategy{
		metricsProvider: metricsProvider,
		maxScore:        defaultLatencyMaxScore,
	}
}

func (s *LatencyAwareStrategy) Name() string {
	return "LatencyAware"
}

func (s *LatencyAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		return s.maxScore / 2
	}

	score, _, hasSignal := s.calculateScore(ctx, metrics)
	if !hasSignal {
		return s.maxScore / 2
	}

	return score
}

func (s *LatencyAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	metrics, err := s.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		neutralScore := s.maxScore / 2

		log.Warn(ctx, "LatencyAwareStrategy: failed to get metrics, using neutral score",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Cause(err),
		)

		return neutralScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        neutralScore,
			Details: map[string]any{
				"error": err.Error(),
			},
		}
	}

	score, details, hasSignal := s.calculateScore(ctx, metrics)
	if !hasSignal {
		score = s.maxScore / 2
		details["reason"] = "no_matching_latency_data"
	}

	log.Info(ctx, "LatencyAwareStrategy: calculated latency-based score",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Bool("stream", requestStreamFromContext(ctx)),
		log.Float64("score", score),
	)

	details["score"] = score

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

func (s *LatencyAwareStrategy) calculateScore(ctx context.Context, metrics *biz.AggregatedMetrics) (float64, map[string]any, bool) {
	if metrics == nil {
		return 0, map[string]any{}, false
	}

	if requestStreamFromContext(ctx) {
		return s.calculateStreamingScore(metrics)
	}

	return s.calculateNonStreamingScore(metrics)
}

func (s *LatencyAwareStrategy) calculateStreamingScore(metrics *biz.AggregatedMetrics) (float64, map[string]any, bool) {
	details := map[string]any{
		"request_type": "streaming",
	}

	if metrics.StreamingSampleCount == 0 {
		return 0, details, false
	}

	firstTokenScore := clampNormalizedInverse(metrics.StreamingFirstTokenLatencyEWMA, defaultStreamingFirstTokenMaxLatencyMs)

	throughputScore := 0.5
	if metrics.StreamingTokensPerSecondEWMA > 0 {
		throughputScore = clampNormalized(metrics.StreamingTokensPerSecondEWMA, defaultStreamingMinTokensPerSecond, defaultStreamingMaxTokensPerSecond)
	}

	score := s.maxScore * (streamingFirstTokenWeight*firstTokenScore + streamingThroughputWeight*throughputScore)

	details["streaming_samples"] = metrics.StreamingSampleCount
	details["first_token_latency_ewma_ms"] = metrics.StreamingFirstTokenLatencyEWMA
	details["tokens_per_second_ewma"] = metrics.StreamingTokensPerSecondEWMA
	details["first_token_component"] = firstTokenScore
	details["throughput_component"] = throughputScore

	return score, details, true
}

func (s *LatencyAwareStrategy) calculateNonStreamingScore(metrics *biz.AggregatedMetrics) (float64, map[string]any, bool) {
	details := map[string]any{
		"request_type": "non_streaming",
	}

	if metrics.NonStreamingSampleCount == 0 {
		return 0, details, false
	}

	latencyComponent := clampNormalizedInverse(metrics.NonStreamingLatencyEWMA, defaultNonStreamingMaxLatencyMs)
	score := s.maxScore * latencyComponent

	details["non_streaming_samples"] = metrics.NonStreamingSampleCount
	details["latency_ewma_ms"] = metrics.NonStreamingLatencyEWMA
	details["latency_component"] = latencyComponent

	return score, details, true
}

func clampNormalizedInverse(value float64, max float64) float64 {
	if max <= 0 {
		return 0
	}

	return math.Max(0, math.Min(1, 1-(value/max)))
}

func clampNormalized(value float64, min float64, max float64) float64 {
	if max <= min {
		return 0
	}

	return math.Max(0, math.Min(1, (value-min)/(max-min)))
}
