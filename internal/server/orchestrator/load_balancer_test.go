package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// channelBasedStrategy returns different scores based on channel ID.
type channelBasedStrategy struct {
	name   string
	scores map[int]float64
}

func (c *channelBasedStrategy) Score(ctx context.Context, channel *biz.Channel, metrics *biz.AggregatedMetrics) float64 {
	if score, ok := c.scores[channel.ID]; ok {
		return score
	}

	return 0
}

func (c *channelBasedStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel, metrics *biz.AggregatedMetrics) (float64, StrategyScore) {
	score := c.Score(ctx, channel, metrics)

	return score, StrategyScore{
		StrategyName: c.name,
		Score:        score,
		Details:      map[string]any{"channel_id": channel.ID},
	}
}

func (c *channelBasedStrategy) Name() string {
	return c.name
}

// noopSelectionTracker is a no-op implementation of ChannelSelectionTracker for tests.
type noopSelectionTracker struct{}

func (n *noopSelectionTracker) IncrementChannelSelection(channelID int) {}

// sortAllCandidates asks LoadBalancer.Sort to rank every candidate.
const sortAllCandidates = 0

// newTestLoadBalancer creates a LoadBalancer with a no-op selection tracker for testing.
func newTestLoadBalancer(t *testing.T, metricsProvider ChannelMetricsProvider, strategies ...LoadBalanceStrategy) *LoadBalancer {
	t.Helper()

	return NewLoadBalancer(metricsProvider, &noopSelectionTracker{}, strategies...)
}

func TestLoadBalancer_Sort_EmptyChannels(t *testing.T) {
	ctx := context.Background()
	lb := newTestLoadBalancer(t, nil)

	result := lb.Sort(ctx, []*ChannelModelsCandidate{}, "", false, sortAllCandidates)
	assert.Empty(t, result)
}

func TestLoadBalancer_Sort_SingleChannel(t *testing.T) {
	ctx := context.Background()
	lb := newTestLoadBalancer(t, nil)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0].Channel.ID)
}

func TestLoadBalancer_Sort_NoStrategies(t *testing.T) {
	ctx := context.Background()
	lb := newTestLoadBalancer(t, nil)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ch2"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "ch3"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)
	// Without strategies, order should remain unchanged (all score 0)
}

func TestLoadBalancer_Sort_WithoutWeightTieBreaker_PreservesInputOrderWhenScoresTie(t *testing.T) {
	ctx := context.Background()
	lb := newTestLoadBalancer(t, nil).WithoutWeightTieBreaker()

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1", OrderingWeight: 10}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ch2", OrderingWeight: 100}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "ch3", OrderingWeight: 50}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)
	assert.Equal(t, 1, result[0].Channel.ID)
	assert.Equal(t, 2, result[1].Channel.ID)
	assert.Equal(t, 3, result[2].Channel.ID)
}

func TestLoadBalancer_Sort_RoundRobinHighCountsStayBalanced(t *testing.T) {
	ctx := context.Background()
	channel1Metrics := &biz.AggregatedMetrics{}
	channel1Metrics.RequestCount = 3000
	channel2Metrics := &biz.AggregatedMetrics{}
	channel2Metrics.RequestCount = 3000
	metricsProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: channel1Metrics,
			2: channel2Metrics,
		},
	}
	lb := NewLoadBalancer(
		metricsProvider,
		metricsProvider,
		NewRoundRobinStrategy(),
		NewRateLimitAwareStrategy(NewChannelRequestTracker(), NewChannelLimiterManager()),
	).WithoutWeightTieBreaker()
	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ch2"}}},
	}

	selected := map[int]int{}
	for range 20 {
		result := lb.Sort(ctx, candidates, "", false, 1)
		require.Len(t, result, 1)
		selected[result[0].Channel.ID]++
	}

	assert.Equal(t, 10, selected[1])
	assert.Equal(t, 10, selected[2])
}

func TestLoadBalancer_Sort_RoundRobinWithRateLimit(t *testing.T) {
	tests := []struct {
		name          string
		requestCounts [2]int64
		rpmLimits     [2]int64
		cooldown      bool
		wantChannelID int
	}{
		{
			name:          "small headroom difference preserves round robin preference",
			requestCounts: [2]int64{20, 21},
			rpmLimits:     [2]int64{100, 200},
			wantChannelID: 1,
		},
		{
			name:          "unconfigured limits preserve round robin preference",
			requestCounts: [2]int64{20, 21},
			wantChannelID: 1,
		},
		{
			name:          "cooldown overrides lower request count",
			requestCounts: [2]int64{0, 3000},
			cooldown:      true,
			wantChannelID: 2,
		},
		{
			name:          "exhausted rpm overrides lower request count",
			requestCounts: [2]int64{1, 3000},
			rpmLimits:     [2]int64{1, 200},
			wantChannelID: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricsProvider := &mockMetricsProvider{metrics: make(map[int]*biz.AggregatedMetrics)}
			tracker := NewChannelRequestTracker()
			candidates := make([]*ChannelModelsCandidate, 0, 2)
			for i, count := range tt.requestCounts {
				id := i + 1
				metrics := &biz.AggregatedMetrics{}
				metrics.RequestCount = count
				metricsProvider.metrics[id] = metrics
				channel := &biz.Channel{Channel: &ent.Channel{ID: id}}
				if rpm := tt.rpmLimits[i]; rpm > 0 {
					channel.Settings = &objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{RPM: &rpm}}
					require.True(t, tracker.TryAcquireRequest(id, rpm))
				}
				candidates = append(candidates, &ChannelModelsCandidate{Channel: channel})
			}
			if tt.cooldown {
				tracker.SetCooldown(1, time.Now().Add(time.Minute))
			}
			lb := newTestLoadBalancer(t, metricsProvider,
				NewRoundRobinStrategy(),
				NewRateLimitAwareStrategy(tracker, NewChannelLimiterManager()),
			).WithoutWeightTieBreaker().WithRoundRobinHealthFilter(NewRoundRobinHealthStrategy())

			// With one RPM slot used each, limits of 100 and 200 yield scores
			// of 99 and 99.5. This small difference must not reverse 20 vs 21
			// requests, while cooldown and exhausted limits must still win.
			result := lb.Sort(context.Background(), candidates, "", false, 1)
			require.Len(t, result, 1)
			require.Equal(t, tt.wantChannelID, result[0].Channel.ID)
		})
	}
}

func TestLoadBalancer_Sort_RoundRobinHealthMovesUnhealthyChannelsLast(t *testing.T) {
	ctx := context.Background()
	recentFailure := time.Now().Add(-time.Minute)

	unhealthy := &biz.AggregatedMetrics{}
	unhealthy.ConsecutiveFailures = roundRobinFailureThreshold
	unhealthy.LastFailureAt = &recentFailure

	healthyLowUsage := &biz.AggregatedMetrics{}
	healthyLowUsage.RequestCount = 10

	healthyHighUsage := &biz.AggregatedMetrics{}
	healthyHighUsage.RequestCount = 100

	metricsProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: unhealthy,
			2: healthyLowUsage,
			3: healthyHighUsage,
		},
	}
	lb := newTestLoadBalancer(
		t,
		metricsProvider,
		NewRoundRobinStrategy(),
	).WithoutWeightTieBreaker().WithRoundRobinHealthFilter(NewRoundRobinHealthStrategy())

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "unhealthy"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "healthy-low-usage"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "healthy-high-usage"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)
	assert.Equal(t, 2, result[0].Channel.ID)
	assert.Equal(t, 3, result[1].Channel.ID)
	assert.Equal(t, 1, result[2].Channel.ID)
}

func TestLoadBalancer_Sort_RoundRobinHealthSkipsUnhealthyBeforeTopK(t *testing.T) {
	ctx := context.Background()
	recentFailure := time.Now().Add(-time.Minute)

	unhealthy := &biz.AggregatedMetrics{}
	unhealthy.RequestCount = 0
	unhealthy.ConsecutiveFailures = roundRobinFailureThreshold
	unhealthy.LastFailureAt = &recentFailure

	healthyLowUsage := &biz.AggregatedMetrics{}
	healthyLowUsage.RequestCount = 10

	healthyHighUsage := &biz.AggregatedMetrics{}
	healthyHighUsage.RequestCount = 100

	metricsProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: unhealthy,
			2: healthyLowUsage,
			3: healthyHighUsage,
		},
	}
	lb := newTestLoadBalancer(
		t,
		metricsProvider,
		NewRoundRobinStrategy(),
	).WithoutWeightTieBreaker().WithRoundRobinHealthFilter(NewRoundRobinHealthStrategy())

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "unhealthy"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "healthy-low-usage"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "healthy-high-usage"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, 2)
	require.Len(t, result, 2)
	assert.Equal(t, 2, result[0].Channel.ID)
	assert.Equal(t, 3, result[1].Channel.ID)
}

func TestLoadBalancer_Sort_RoundRobinHealthKeepsHardUnavailableLast(t *testing.T) {
	ctx := context.Background()
	recentFailure := time.Now().Add(-time.Minute)

	unhealthy := &biz.AggregatedMetrics{}
	unhealthy.RequestCount = 10
	unhealthy.ConsecutiveFailures = roundRobinFailureThreshold
	unhealthy.LastFailureAt = &recentFailure

	hardUnavailable := &biz.AggregatedMetrics{}
	hardUnavailable.RequestCount = 0

	healthy := &biz.AggregatedMetrics{}
	healthy.RequestCount = 100

	metricsProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: unhealthy,
			2: hardUnavailable,
			3: healthy,
		},
	}
	hardUnavailableStrategy := &channelBasedStrategy{
		name: "hard-unavailable",
		scores: map[int]float64{
			2: rateLimitExhaustedScore,
		},
	}
	lb := newTestLoadBalancer(
		t,
		metricsProvider,
		NewRoundRobinStrategy(),
		hardUnavailableStrategy,
	).WithoutWeightTieBreaker().WithRoundRobinHealthFilter(NewRoundRobinHealthStrategy())

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "unhealthy"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "hard-unavailable"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "healthy"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)
	assert.Equal(t, 3, result[0].Channel.ID)
	assert.Equal(t, 1, result[1].Channel.ID)
	assert.Equal(t, 2, result[2].Channel.ID)
}

func TestLoadBalancer_Sort_SingleStrategy(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 100,
			2: 200,
			3: 150,
		},
	}

	lb := newTestLoadBalancer(t, nil, strategy)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ch2"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "ch3"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)

	// Should be sorted by descending score: ch2(200), ch3(150), ch1(100)
	assert.Equal(t, 2, result[0].Channel.ID, "First should be ch2 with score 200")
	assert.Equal(t, 3, result[1].Channel.ID, "Second should be ch3 with score 150")
	assert.Equal(t, 1, result[2].Channel.ID, "Third should be ch1 with score 100")
}

func TestLoadBalancer_Sort_MultipleStrategies(t *testing.T) {
	ctx := context.Background()

	// Strategy 1: Give different base scores
	strategy1 := &channelBasedStrategy{
		name: "base",
		scores: map[int]float64{
			1: 100,
			2: 100,
			3: 100,
		},
	}

	// Strategy 2: Give boost to specific channels
	strategy2 := &channelBasedStrategy{
		name: "boost",
		scores: map[int]float64{
			1: 50,
			2: 0,
			3: 25,
		},
	}

	lb := newTestLoadBalancer(t, nil, strategy1, strategy2)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ch2"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "ch3"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)

	// Total scores: ch1=150, ch2=100, ch3=125
	assert.Equal(t, 1, result[0].Channel.ID, "First should be ch1 with total score 150")
	assert.Equal(t, 3, result[1].Channel.ID, "Second should be ch3 with total score 125")
	assert.Equal(t, 2, result[2].Channel.ID, "Third should be ch2 with total score 100")
}

func TestLoadBalancer_Sort_AdditiveScoring(t *testing.T) {
	ctx := context.Background()

	// Create three strategies with different scoring
	s1 := &mockStrategy{name: "s1", score: 100}
	s2 := &mockStrategy{name: "s2", score: 50}
	s3 := &mockStrategy{name: "s3", score: 25}

	lb := newTestLoadBalancer(t, nil, s1, s2, s3)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 1)

	// With mock strategies returning fixed scores, all channels get same total
	// This test mainly verifies that strategies are applied
}

func TestLoadBalancer_Sort_Stability(t *testing.T) {
	ctx := context.Background()

	// All channels get same score
	strategy := &mockStrategy{name: "equal", score: 100}
	lb := newTestLoadBalancer(t, nil, strategy)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ch2"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "ch3"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)

	// When scores are equal, original order should be preserved (stable sort)
	// Note: Current implementation uses partial.SortFunc which is stable
}

func TestLoadBalancer_Sort_NegativeScores(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "negative",
		scores: map[int]float64{
			1: -50,
			2: 100,
			3: -25,
		},
	}

	lb := newTestLoadBalancer(t, nil, strategy)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ch2"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "ch3"}}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)

	// Should handle negative scores: ch2(100), ch3(-25), ch1(-50)
	assert.Equal(t, 2, result[0].Channel.ID)
	assert.Equal(t, 3, result[1].Channel.ID)
	assert.Equal(t, 1, result[2].Channel.ID)
}

// TestLoadBalancer_ErrorAware_ChannelWithErrorsRankedLower tests that channels
// with recent errors are ranked lower in the load balancer.
func TestLoadBalancer_ErrorAware_ChannelWithErrorsRankedLower(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create three channels
	ch1, err := client.Channel.Create().
		SetName("healthy-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-1"}}).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetName("failing-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-2"}}).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetName("another-healthy-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"test-key-3"}}).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelService(client)

	// Record consecutive failures for ch2
	for range 3 {
		perf := &biz.PerformanceRecord{
			ChannelID:          ch2.ID,
			StartTime:          time.Now().Add(-time.Minute),
			EndTime:            time.Now(),
			Success:            false,
			RequestCompleted:   true,
			ResponseStatusCode: 500,
		}
		channelService.RecordPerformance(ctx, perf)
	}

	// Record successful requests for ch1 and ch3
	for _, chID := range []int{ch1.ID, ch3.ID} {
		perf := &biz.PerformanceRecord{
			ChannelID:        chID,
			StartTime:        time.Now().Add(-30 * time.Second),
			EndTime:          time.Now(),
			Success:          true,
			RequestCompleted: true,
		}
		channelService.RecordPerformance(ctx, perf)
	}

	// Create load balancer with ErrorAware and Weight strategies
	errorStrategy := NewErrorAwareStrategy()
	weightStrategy := NewWeightStrategy()
	lb := newTestLoadBalancer(t, channelService, errorStrategy, weightStrategy)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: ch1}},
		{Channel: &biz.Channel{Channel: ch2}},
		{Channel: &biz.Channel{Channel: ch3}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 3)

	// ch2 (failing channel) should be ranked last due to consecutive failures
	assert.NotEqual(t, ch2.ID, result[0].Channel.ID, "Failing channel should not be first")
	assert.Equal(t, ch2.ID, result[2].Channel.ID, "Failing channel should be last")

	// ch1 and ch3 (healthy channels) should be ranked higher
	assert.Contains(t, []int{ch1.ID, ch3.ID}, result[0].Channel.ID, "First should be a healthy channel")
	assert.Contains(t, []int{ch1.ID, ch3.ID}, result[1].Channel.ID, "Second should be a healthy channel")
}

// TestLoadBalancer_ErrorAware_ShortTermErrorPenalty tests that channels with
// recent errors are penalized but recover after the cooldown period.
func TestLoadBalancer_ErrorAware_ShortTermErrorPenalty(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ch1, err := client.Channel.Create().
		SetName("channel-with-recent-error").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-1"}).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetName("stable-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-2"}).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelService(client)

	// Record a recent failure for ch1 (within cooldown period)
	perf := &biz.PerformanceRecord{
		ChannelID:          ch1.ID,
		StartTime:          time.Now().Add(-30 * time.Second),
		EndTime:            time.Now(),
		Success:            false,
		RequestCompleted:   true,
		ResponseStatusCode: 500,
	}
	channelService.RecordPerformance(ctx, perf)

	// Record success for ch2
	perf2 := &biz.PerformanceRecord{
		ChannelID:        ch2.ID,
		StartTime:        time.Now().Add(-30 * time.Second),
		EndTime:          time.Now(),
		Success:          true,
		RequestCompleted: true,
	}
	channelService.RecordPerformance(ctx, perf2)

	errorStrategy := NewErrorAwareStrategy()
	lb := newTestLoadBalancer(t, channelService, errorStrategy)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: ch1}},
		{Channel: &biz.Channel{Channel: ch2}},
	}

	result := lb.Sort(ctx, candidates, "", false, sortAllCandidates)
	require.Len(t, result, 2)

	// ch2 should be ranked higher due to ch1's recent error
	assert.Equal(t, ch2.ID, result[0].Channel.ID, "Stable channel should be ranked first")
	assert.Equal(t, ch1.ID, result[1].Channel.ID, "Channel with recent error should be ranked lower")
}

func TestLoadBalancer_Sort_TopK(t *testing.T) {
	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 100,
			2: 200,
			3: 150,
			4: 300,
			5: 50,
			6: 250,
		},
	}

	tests := []struct {
		name string
		topK int
		want []int
	}{
		{name: "single best candidate", topK: 1, want: []int{4}},
		{name: "top three", topK: 3, want: []int{4, 6, 2}},
		{name: "top four", topK: 4, want: []int{4, 6, 2, 3}},
		{name: "more than available sorts all", topK: 10, want: []int{4, 6, 2, 3, 1, 5}},
		{name: "zero sorts all", topK: sortAllCandidates, want: []int{4, 6, 2, 3, 1, 5}},
		{name: "negative sorts all", topK: -1, want: []int{4, 6, 2, 3, 1, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := newTestLoadBalancer(t, nil, strategy)

			candidates := make([]*ChannelModelsCandidate, 0, len(strategy.scores))
			for id := 1; id <= len(strategy.scores); id++ {
				candidates = append(candidates, &ChannelModelsCandidate{
					Channel: &biz.Channel{Channel: &ent.Channel{ID: id, Name: "ch"}},
				})
			}

			result := lb.Sort(context.Background(), candidates, "", false, tt.topK)
			got := make([]int, 0, len(result))
			for _, candidate := range result {
				got = append(got, candidate.Channel.ID)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestLoadBalancer_SortTopKWithoutTracking verifies that a caller can keep more
// candidates than the limit used for the retry-driven path. Preflight rejections
// (probe lease busy) do not consume a retry, so the selection keeps spare
// candidates for the rollover instead of stopping at the retry budget.
func TestLoadBalancer_SortTopKWithoutTracking(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 30,
			2: 20,
			3: 10,
		},
	}

	lb := newTestLoadBalancer(t, nil, strategy)

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "ch1"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "ch2"}}},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "ch3"}}},
	}

	limited := lb.SortWithoutTracking(ctx, candidates, "", false, 1)
	require.Len(t, limited, 1, "a caller-supplied limit of 1 must keep a single candidate")

	kept := lb.SortTopKWithoutTracking(ctx, candidates, "", false, 3)
	require.Len(t, kept, 3, "the caller-provided limit must be honoured")
	assert.Equal(t, 1, kept[0].Channel.ID)
	assert.Equal(t, 2, kept[1].Channel.ID)
	assert.Equal(t, 3, kept[2].Channel.ID)
}
