package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestLatencyAwareStrategy_Name(t *testing.T) {
	strategy := NewLatencyAwareStrategy()
	assert.Equal(t, "LatencyAware", strategy.Name())
}

func TestLatencyAwareStrategy_Score_NoMatchingData_ReturnsNeutral(t *testing.T) {
	ctx := contextWithRequestStream(context.Background(), true)

	metrics := &biz.AggregatedMetrics{NonStreamingLatencyEWMA: 500, NonStreamingSampleCount: 10}
	strategy := NewLatencyAwareStrategy()

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}
	score := strategy.Score(ctx, channel, metrics)
	assert.Equal(t, defaultLatencyMaxScore/2, score)
}

func TestLatencyAwareStrategy_Score_StreamingUsesFirstTokenAndTPS(t *testing.T) {
	ctx := contextWithRequestStream(context.Background(), true)

	metrics := &biz.AggregatedMetrics{
		StreamingFirstTokenLatencyEWMA: 300,
		StreamingTokensPerSecondEWMA:   60,
		StreamingSampleCount:           10,
	}
	strategy := NewLatencyAwareStrategy()

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "stream-fast"}}
	score := strategy.Score(ctx, channel, metrics)

	// 0.7 * (1 - 300/3000) + 0.3 * ((60 - 5) / (100 - 5)) = 0.803684...
	assert.InDelta(t, 64.29, score, 0.01)
}

func TestLatencyAwareStrategy_Score_StreamingPrefersBetterFTTL(t *testing.T) {
	ctx := contextWithRequestStream(context.Background(), true)

	fastStartMetrics := &biz.AggregatedMetrics{
		StreamingFirstTokenLatencyEWMA: 200,
		StreamingTokensPerSecondEWMA:   40,
		StreamingSampleCount:           10,
	}
	slowStartMetrics := &biz.AggregatedMetrics{
		StreamingFirstTokenLatencyEWMA: 1200,
		StreamingTokensPerSecondEWMA:   70,
		StreamingSampleCount:           10,
	}
	strategy := NewLatencyAwareStrategy()

	fastStart := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "fast-start"}}
	slowStart := &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "slow-start"}}

	assert.Greater(t, strategy.Score(ctx, fastStart, fastStartMetrics), strategy.Score(ctx, slowStart, slowStartMetrics))
}

func TestLatencyAwareStrategy_Score_NonStreamingUsesTotalLatency(t *testing.T) {
	ctx := contextWithRequestStream(context.Background(), false)

	metrics := &biz.AggregatedMetrics{
		NonStreamingLatencyEWMA: 1200,
		NonStreamingSampleCount: 8,
	}
	strategy := NewLatencyAwareStrategy()

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "batch"}}
	score := strategy.Score(ctx, channel, metrics)

	assert.InDelta(t, 48.0, score, 0.01)
}

func TestLatencyAwareStrategy_ScoreWithDebug_Streaming(t *testing.T) {
	ctx := contextWithRequestStream(context.Background(), true)

	metrics := &biz.AggregatedMetrics{
		StreamingFirstTokenLatencyEWMA: 500,
		StreamingTokensPerSecondEWMA:   50,
		StreamingSampleCount:           20,
	}
	strategy := NewLatencyAwareStrategy()

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}
	score, details := strategy.ScoreWithDebug(ctx, channel, metrics)

	assert.InDelta(t, 58.04, score, 0.01)
	assert.Equal(t, "LatencyAware", details.StrategyName)
	assert.Equal(t, "streaming", details.Details["request_type"])
	assert.Equal(t, 500.0, details.Details["first_token_latency_ewma_ms"])
	assert.Equal(t, 50.0, details.Details["tokens_per_second_ewma"])
	assert.Equal(t, int64(20), details.Details["streaming_samples"])
}

func TestLatencyAwareStrategy_ScoreWithDebug_NilMetrics(t *testing.T) {
	ctx := context.Background()

	strategy := NewLatencyAwareStrategy()

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}
	score, details := strategy.ScoreWithDebug(ctx, channel, nil)

	assert.Equal(t, defaultLatencyMaxScore/2, score)
	assert.Equal(t, metricsUnavailableReason, details.Details["reason"])
}
