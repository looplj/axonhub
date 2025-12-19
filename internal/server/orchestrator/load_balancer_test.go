package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/server/biz"
)

// channelBasedStrategy returns different scores based on channel ID.
type channelBasedStrategy struct {
	name   string
	scores map[int]float64
}

func (c *channelBasedStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	if score, ok := c.scores[channel.ID]; ok {
		return score
	}

	return 0
}

func (c *channelBasedStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	score := c.Score(ctx, channel)

	return score, StrategyScore{
		StrategyName: c.name,
		Score:        score,
		Details:      map[string]interface{}{"channel_id": channel.ID},
	}
}

func (c *channelBasedStrategy) Name() string {
	return c.name
}

// newTestLoadBalancer creates a LoadBalancer with mock system service for testing.
func newTestLoadBalancer(t *testing.T, retryPolicy *biz.RetryPolicy, strategies ...LoadBalanceStrategy) *LoadBalancer {
	t.Helper()

	if retryPolicy == nil {
		// Pass nil to mockSystemService so it uses its own default (Enabled: false)
		mockSvc := &mockSystemService{retryPolicy: nil}
		return NewLoadBalancer(mockSvc, strategies...)
	}
	// Use the provided retry policy
	mockSvc := &mockSystemService{retryPolicy: retryPolicy}

	return NewLoadBalancer(mockSvc, strategies...)
}

func TestLoadBalancer_Sort_EmptyChannels(t *testing.T) {
	ctx := context.Background()
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3})

	result := lb.Sort(ctx, []*biz.Channel{}, "")
	assert.Empty(t, result)
}

func TestLoadBalancer_Sort_SingleChannel(t *testing.T) {
	ctx := context.Background()
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3})

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0].ID)
}

func TestLoadBalancer_Sort_NoStrategies(t *testing.T) {
	ctx := context.Background()
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3})

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)
	// Without strategies, order should remain unchanged (all score 0)
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

	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)

	// Should be sorted by descending score: ch2(200), ch3(150), ch1(100)
	assert.Equal(t, 2, result[0].ID, "First should be ch2 with score 200")
	assert.Equal(t, 3, result[1].ID, "Second should be ch3 with score 150")
	assert.Equal(t, 1, result[2].ID, "Third should be ch1 with score 100")
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

	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, strategy1, strategy2)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)

	// Total scores: ch1=150, ch2=100, ch3=125
	assert.Equal(t, 1, result[0].ID, "First should be ch1 with total score 150")
	assert.Equal(t, 3, result[1].ID, "Second should be ch3 with total score 125")
	assert.Equal(t, 2, result[2].ID, "Third should be ch2 with total score 100")
}

func TestLoadBalancer_Sort_AdditiveScoring(t *testing.T) {
	ctx := context.Background()

	// Create three strategies with different scoring
	s1 := &mockStrategy{name: "s1", score: 100}
	s2 := &mockStrategy{name: "s2", score: 50}
	s3 := &mockStrategy{name: "s3", score: 25}

	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, s1, s2, s3)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 1)

	// With mock strategies returning fixed scores, all channels get same total
	// This test mainly verifies that strategies are applied
}

func TestLoadBalancer_Sort_Stability(t *testing.T) {
	ctx := context.Background()

	// All channels get same score
	strategy := &mockStrategy{name: "equal", score: 100}
	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
	}

	result := lb.Sort(ctx, channels, "")
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

	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)

	// Should handle negative scores: ch2(100), ch3(-25), ch1(-50)
	assert.Equal(t, 2, result[0].ID)
	assert.Equal(t, 3, result[1].ID)
	assert.Equal(t, 1, result[2].ID)
}

// TestLoadBalancer_ErrorAware_ChannelWithErrorsRankedLower tests that channels
// with recent errors are ranked lower in the load balancer.
func TestLoadBalancer_ErrorAware_ChannelWithErrorsRankedLower(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create three channels
	ch1, err := client.Channel.Create().
		SetName("healthy-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetName("failing-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetName("another-healthy-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelService(client)

	// Record consecutive failures for ch2
	for i := 0; i < 3; i++ {
		perf := &biz.PerformanceRecord{
			ChannelID:        ch2.ID,
			StartTime:        time.Now().Add(-time.Minute),
			EndTime:          time.Now(),
			Success:          false,
			RequestCompleted: true,
			ErrorStatusCode:  500,
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
			TokenCount:       100,
		}
		channelService.RecordPerformance(ctx, perf)
	}

	// Create load balancer with ErrorAware and Weight strategies
	errorStrategy := NewErrorAwareStrategy(channelService)
	weightStrategy := NewWeightStrategy()
	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, errorStrategy, weightStrategy)

	channels := []*biz.Channel{
		{Channel: ch1},
		{Channel: ch2},
		{Channel: ch3},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)

	// ch2 (failing channel) should be ranked last due to consecutive failures
	assert.NotEqual(t, ch2.ID, result[0].ID, "Failing channel should not be first")
	assert.Equal(t, ch2.ID, result[2].ID, "Failing channel should be last")

	// ch1 and ch3 (healthy channels) should be ranked higher
	assert.Contains(t, []int{ch1.ID, ch3.ID}, result[0].ID, "First should be a healthy channel")
	assert.Contains(t, []int{ch1.ID, ch3.ID}, result[1].ID, "Second should be a healthy channel")
}

// TestLoadBalancer_ErrorAware_ShortTermErrorPenalty tests that channels with
// recent errors are penalized but recover after the cooldown period.
func TestLoadBalancer_ErrorAware_ShortTermErrorPenalty(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ch1, err := client.Channel.Create().
		SetName("channel-with-recent-error").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetName("stable-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelService(client)

	// Record a recent failure for ch1 (within cooldown period)
	perf := &biz.PerformanceRecord{
		ChannelID:        ch1.ID,
		StartTime:        time.Now().Add(-30 * time.Second),
		EndTime:          time.Now(),
		Success:          false,
		RequestCompleted: true,
		ErrorStatusCode:  500,
	}
	channelService.RecordPerformance(ctx, perf)

	// Record success for ch2
	perf2 := &biz.PerformanceRecord{
		ChannelID:        ch2.ID,
		StartTime:        time.Now().Add(-30 * time.Second),
		EndTime:          time.Now(),
		Success:          true,
		RequestCompleted: true,
		TokenCount:       100,
	}
	channelService.RecordPerformance(ctx, perf2)

	errorStrategy := NewErrorAwareStrategy(channelService)
	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, errorStrategy)

	channels := []*biz.Channel{
		{Channel: ch1},
		{Channel: ch2},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 2)

	// ch2 should be ranked higher due to ch1's recent error
	assert.Equal(t, ch2.ID, result[0].ID, "Stable channel should be ranked first")
	assert.Equal(t, ch1.ID, result[1].ID, "Channel with recent error should be ranked lower")
}

// TestLoadBalancer_TraceAware_SameChannelPrioritized tests that when a trace ID
// exists, the channel that last succeeded for that trace is prioritized.
func TestLoadBalancer_TraceAware_SameChannelPrioritized(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create project
	project, err := client.Project.Create().
		SetName("test-project").
		Save(ctx)
	require.NoError(t, err)

	// Create channels
	ch1, err := client.Channel.Create().
		SetName("channel-1").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetName("channel-2").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetName("channel-3").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	// Create trace
	trace, err := client.Trace.Create().
		SetProjectID(project.ID).
		SetTraceID("test-trace-abc").
		Save(ctx)
	require.NoError(t, err)

	// Create a successful request with ch2 in this trace
	_, err = client.Request.Create().
		SetProjectID(project.ID).
		SetTraceID(trace.ID).
		SetChannelID(ch2.ID).
		SetModelID("gpt-4").
		SetStatus("completed").
		SetSource("api").
		SetRequestBody([]byte(`{"model":"gpt-4","messages":[]}`)).
		Save(ctx)
	require.NoError(t, err)

	// Add trace entity to context and ent client
	ctx = contexts.WithTrace(ctx, trace) // Use the trace entity directly
	ctx = ent.NewContext(ctx, client)

	requestService := newTestRequestService(client)
	traceStrategy := NewTraceAwareStrategy(requestService)
	weightStrategy := NewWeightStrategy()
	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, traceStrategy, weightStrategy)

	channels := []*biz.Channel{
		{Channel: ch1},
		{Channel: ch2},
		{Channel: ch3},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)

	// ch2 should be ranked first because it was the last successful channel in this trace
	assert.Equal(t, ch2.ID, result[0].ID, "Channel from trace should be ranked first")
}

// TestLoadBalancer_Combined_ErrorAndTrace tests the combined behavior of
// error-aware and trace-aware strategies.
func TestLoadBalancer_Combined_ErrorAndTrace(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create project
	project, err := client.Project.Create().
		SetName("test-project").
		Save(ctx)
	require.NoError(t, err)

	// Create channels
	ch1, err := client.Channel.Create().
		SetName("healthy-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetName("trace-channel-with-errors").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetName("another-channel").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelService(client)

	// Record consecutive failures for ch2
	for i := 0; i < 2; i++ {
		perf := &biz.PerformanceRecord{
			ChannelID:        ch2.ID,
			StartTime:        time.Now().Add(-time.Minute),
			EndTime:          time.Now(),
			Success:          false,
			RequestCompleted: true,
			ErrorStatusCode:  500,
		}
		channelService.RecordPerformance(ctx, perf)
	}

	// Create trace with ch2 as last successful channel
	trace, err := client.Trace.Create().
		SetProjectID(project.ID).
		SetTraceID("test-trace-xyz").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Request.Create().
		SetProjectID(project.ID).
		SetTraceID(trace.ID).
		SetChannelID(ch2.ID).
		SetModelID("gpt-4").
		SetStatus("completed").
		SetSource("api").
		SetRequestBody([]byte(`{"model":"gpt-4","messages":[]}`)).
		Save(ctx)
	require.NoError(t, err)

	// Add trace entity to context and ent client
	ctx = contexts.WithTrace(ctx, trace) // Use the trace entity directly
	ctx = ent.NewContext(ctx, client)

	// Create load balancer with both strategies
	requestService := newTestRequestService(client)
	traceStrategy := NewTraceAwareStrategy(requestService)
	errorStrategy := NewErrorAwareStrategy(channelService)
	weightStrategy := NewWeightStrategy()
	// Mock SystemService for testing
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, traceStrategy, errorStrategy, weightStrategy)

	channels := []*biz.Channel{
		{Channel: ch1},
		{Channel: ch2},
		{Channel: ch3},
	}

	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)

	// ch2 should still be ranked first because trace boost (1000) outweighs error penalty
	// TraceAware gives +1000, ErrorAware gives penalty (around -100 to -150), Weight gives +50
	// Net score for ch2: ~900-950
	// ch1 and ch3: ErrorAware ~200, Weight ~50 = ~250
	assert.Equal(t, ch2.ID, result[0].ID, "Trace channel should be first despite errors (trace boost is stronger)")
}

// mockSystemService is a test mock for SystemService.
type mockSystemService struct {
	retryPolicy *biz.RetryPolicy
}

func (m *mockSystemService) RetryPolicyOrDefault(ctx context.Context) *biz.RetryPolicy {
	if m.retryPolicy != nil {
		return m.retryPolicy
	}
	// Return default policy if not set
	return &biz.RetryPolicy{
		Enabled:                 false,
		MaxChannelRetries:       3,
		MaxSingleChannelRetries: 2,
		RetryDelayMs:            1000,
	}
}

// TestLoadBalancer_TopK_OnlyOneChannel tests that only 1 channel is returned when retry is disabled.
func TestLoadBalancer_TopK_OnlyOneChannel(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 100,
			2: 200,
			3: 150,
			4: 300,
			5: 50,
		},
	}

	// Mock SystemService with retry disabled
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: false}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
		{Channel: &ent.Channel{ID: 4, Name: "ch4"}},
		{Channel: &ent.Channel{ID: 5, Name: "ch5"}},
	}

	// With retry disabled, should only return the highest scored channel
	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 1)
	assert.Equal(t, 4, result[0].ID, "Should return only ch4 with highest score 300")
}

// TestLoadBalancer_TopK_TopThreeChannels tests that only top 3 channels are returned.
func TestLoadBalancer_TopK_TopThreeChannels(t *testing.T) {
	ctx := context.Background()

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

	// Mock SystemService with 2 retries (topK=1+2=3)
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 2}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
		{Channel: &ent.Channel{ID: 4, Name: "ch4"}},
		{Channel: &ent.Channel{ID: 5, Name: "ch5"}},
		{Channel: &ent.Channel{ID: 6, Name: "ch6"}},
	}

	// With 2 retries, should return top 3 channels
	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)
	// Scores: ch4=300, ch6=250, ch2=200
	assert.Equal(t, 4, result[0].ID, "First should be ch4 with score 300")
	assert.Equal(t, 6, result[1].ID, "Second should be ch6 with score 250")
	assert.Equal(t, 2, result[2].ID, "Third should be ch2 with score 200")
}

// TestLoadBalancer_TopK_MoreThanAvailable tests when retry count exceeds available channels.
func TestLoadBalancer_TopK_MoreThanAvailable(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 100,
			2: 200,
			3: 150,
		},
	}

	// Mock SystemService with 10 retries (topK=1+10=11) but only 3 channels
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 10}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
	}

	// With 10 retries but only 3 channels, should return all 3
	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3)
	assert.Equal(t, 2, result[0].ID, "First should be ch2 with score 200")
	assert.Equal(t, 3, result[1].ID, "Second should be ch3 with score 150")
	assert.Equal(t, 1, result[2].ID, "Third should be ch1 with score 100")
}

// TestLoadBalancer_TopK_RetryDisabled simulates retry disabled.
func TestLoadBalancer_TopK_RetryDisabled(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 100,
			2: 200,
			3: 150,
			4: 300,
			5: 50,
		},
	}

	// Mock SystemService with retry disabled
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: false}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
		{Channel: &ent.Channel{ID: 4, Name: "ch4"}},
		{Channel: &ent.Channel{ID: 5, Name: "ch5"}},
	}

	// With retry disabled, should only get best channel
	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 1)
	assert.Equal(t, 4, result[0].ID, "With retry disabled, should get only best channel")
}

// TestLoadBalancer_TopK_RetryEnabled simulates retry enabled with max 3 retries.
func TestLoadBalancer_TopK_RetryEnabled(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 100,
			2: 200,
			3: 150,
			4: 300,
			5: 50,
		},
	}

	// Mock SystemService with 3 retries (topK=1+3=4)
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
		{Channel: &ent.Channel{ID: 4, Name: "ch4"}},
		{Channel: &ent.Channel{ID: 5, Name: "ch5"}},
	}

	// With 3 retries, should get top 4 channels
	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 4)
	// Top 4: ch4(300), ch2(200), ch3(150), ch1(100)
	assert.Equal(t, 4, result[0].ID)
	assert.Equal(t, 2, result[1].ID)
	assert.Equal(t, 3, result[2].ID)
	assert.Equal(t, 1, result[3].ID)
}

// TestLoadBalancer_TopK_FewChannelsManyRetries tests when retry count exceeds channel count.
func TestLoadBalancer_TopK_FewChannelsManyRetries(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 100,
			2: 200,
			3: 150,
		},
	}

	// Mock SystemService with 10 retries but only 3 channels
	lb := newTestLoadBalancer(t, &biz.RetryPolicy{Enabled: true, MaxChannelRetries: 10}, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
	}

	// With 10 retries but only 3 channels, should return all 3
	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 3, "Should return all 3 channels even though retry count is high")
	assert.Equal(t, 2, result[0].ID)
	assert.Equal(t, 3, result[1].ID)
	assert.Equal(t, 1, result[2].ID)
}

// TestLoadBalancer_TopK_DefaultPolicy tests that default retry policy works.
func TestLoadBalancer_TopK_DefaultPolicy(t *testing.T) {
	ctx := context.Background()

	strategy := &channelBasedStrategy{
		name: "test",
		scores: map[int]float64{
			1: 100,
			2: 200,
			3: 150,
		},
	}

	// Mock SystemService with nil policy (should use default)
	lb := newTestLoadBalancer(t, nil, strategy)

	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1"}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2"}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3"}},
	}

	// With default policy (retry disabled), should return 1 channel
	result := lb.Sort(ctx, channels, "")
	require.Len(t, result, 1)
	assert.Equal(t, 2, result[0].ID, "Should return only best channel with default policy")
}
