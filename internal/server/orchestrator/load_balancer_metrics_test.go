package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

// countingMetricsProvider records how many times each channel's metrics are read.
type countingMetricsProvider struct {
	mu      sync.Mutex
	metrics map[int]*biz.AggregatedMetrics
	calls   map[int]int
	err     error
}

func newCountingMetricsProvider() *countingMetricsProvider {
	return &countingMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{},
		calls:   map[int]int{},
	}
}

func (p *countingMetricsProvider) GetChannelMetrics(_ context.Context, channelID int) (*biz.AggregatedMetrics, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls[channelID]++
	if p.err != nil {
		return nil, p.err
	}

	if metrics, ok := p.metrics[channelID]; ok {
		return metrics.Clone(), nil
	}

	return &biz.AggregatedMetrics{}, nil
}

// recordingMetricsStrategy records the metrics snapshot each Score call receives.
type recordingMetricsStrategy struct {
	seen map[int][]*biz.AggregatedMetrics
}

func newRecordingMetricsStrategy() *recordingMetricsStrategy {
	return &recordingMetricsStrategy{seen: map[int][]*biz.AggregatedMetrics{}}
}

func (s *recordingMetricsStrategy) Score(_ context.Context, channel *biz.Channel, metrics *biz.AggregatedMetrics) float64 {
	s.seen[channel.ID] = append(s.seen[channel.ID], metrics)
	return 0
}

func (s *recordingMetricsStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel, metrics *biz.AggregatedMetrics) (float64, StrategyScore) {
	score := s.Score(ctx, channel, metrics)
	return score, StrategyScore{StrategyName: s.Name(), Score: score}
}

func (s *recordingMetricsStrategy) Name() string {
	return "RecordingMetrics"
}

func metricsTestCandidates(ids ...int) []*ChannelModelsCandidate {
	candidates := make([]*ChannelModelsCandidate, 0, len(ids))
	for _, id := range ids {
		candidates = append(candidates, &ChannelModelsCandidate{
			Channel: &biz.Channel{Channel: &ent.Channel{ID: id, Name: "ch"}},
		})
	}

	return candidates
}

func TestLoadBalancer_FetchesMetricsOncePerCandidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{name: "production", ctx: context.Background()},
		{name: "debug", ctx: EnableDebugMode(context.Background(), nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := newCountingMetricsProvider()
			recorder := newRecordingMetricsStrategy()
			lb := NewLoadBalancer(provider, &noopSelectionTracker{},
				NewErrorAwareStrategy(),
				NewWeightRoundRobinStrategy(),
				NewLatencyAwareStrategy(),
				recorder,
				recorder,
			)

			result := lb.Sort(tc.ctx, metricsTestCandidates(1, 2, 3), "model", false, 0)
			require.Len(t, result, 3)

			assert.Equal(t, map[int]int{1: 1, 2: 1, 3: 1}, provider.calls)
			for id, seen := range recorder.seen {
				require.Len(t, seen, 2, "channel %d", id)
				require.NotNil(t, seen[0])
				assert.Same(t, seen[0], seen[1], "strategies must share one snapshot for channel %d", id)
			}
		})
	}
}

func TestLoadBalancer_NilMetricsProviderPassesNilMetrics(t *testing.T) {
	recorder := newRecordingMetricsStrategy()
	lb := NewLoadBalancer(nil, &noopSelectionTracker{}, NewWeightStrategy(), recorder)

	lb.Sort(context.Background(), metricsTestCandidates(1, 2), "model", false, 0)

	require.Len(t, recorder.seen, 2)
	for id, seen := range recorder.seen {
		assert.Equal(t, []*biz.AggregatedMetrics{nil}, seen, "channel %d", id)
	}
}

func TestLoadBalancer_MetricsErrorPassesNilMetrics(t *testing.T) {
	provider := newCountingMetricsProvider()
	provider.err = errors.New("metrics unavailable")
	recorder := newRecordingMetricsStrategy()
	lb := NewLoadBalancer(provider, &noopSelectionTracker{}, NewErrorAwareStrategy(), recorder)

	for _, ctx := range []context.Context{context.Background(), EnableDebugMode(context.Background(), nil)} {
		result := lb.Sort(ctx, metricsTestCandidates(1, 2), "model", false, 0)
		require.Len(t, result, 2)
	}

	assert.Equal(t, map[int]int{1: 2, 2: 2}, provider.calls)
	require.Len(t, recorder.seen, 2)
	for id, seen := range recorder.seen {
		assert.Equal(t, []*biz.AggregatedMetrics{nil, nil}, seen, "channel %d", id)
	}

	errorAware := NewErrorAwareStrategy()
	channel := &biz.Channel{Channel: &ent.Channel{ID: 1}}
	assert.InDelta(t, errorAware.maxScore/2, errorAware.Score(context.Background(), channel, nil), 0)
}

func TestLoadBalancer_RoundRobinHealthFilterReusesScoringSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{name: "production", ctx: context.Background()},
		{name: "debug", ctx: EnableDebugMode(context.Background(), nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := newCountingMetricsProvider()
			now := time.Now()
			unhealthy := &biz.AggregatedMetrics{LastFailureAt: &now}
			unhealthy.ConsecutiveFailures = roundRobinFailureThreshold
			provider.metrics[1] = unhealthy

			lb := NewLoadBalancer(provider, &noopSelectionTracker{},
				NewRoundRobinStrategy(),
			).WithoutWeightTieBreaker().WithRoundRobinHealthFilter(NewRoundRobinHealthStrategy())

			result := lb.Sort(tc.ctx, metricsTestCandidates(1, 2, 3), "model", false, 0)
			require.Len(t, result, 3)

			assert.Equal(t, 1, result[2].Channel.ID, "unhealthy channel must rank last")
			assert.Equal(t, map[int]int{1: 1, 2: 1, 3: 1}, provider.calls)
		})
	}
}
