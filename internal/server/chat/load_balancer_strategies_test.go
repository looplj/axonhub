package chat

import (
	"context"
	"fmt"
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

// mockMetricsProvider is a mock implementation of ChannelMetricsProvider for testing.
type mockMetricsProvider struct {
	metrics map[int]*biz.AggregatedMetrics
	err     error
}

func (m *mockMetricsProvider) GetChannelMetrics(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
	if m.err != nil {
		return nil, m.err
	}

	if metrics, ok := m.metrics[channelID]; ok {
		return metrics, nil
	}

	return &biz.AggregatedMetrics{}, nil
}

// mockTraceProvider is a mock implementation of ChannelTraceProvider for testing.
type mockTraceProvider struct {
	lastSuccessChannel map[int]int // traceID -> channelID
	err                error
}

func (m *mockTraceProvider) GetLastSuccessfulChannelID(ctx context.Context, traceID int) (int, error) {
	if m.err != nil {
		return 0, m.err
	}

	if channelID, ok := m.lastSuccessChannel[traceID]; ok {
		return channelID, nil
	}

	return 0, nil
}

func TestWeightStrategy_Score(t *testing.T) {
	ctx := context.Background()
	strategy := NewWeightStrategy()

	tests := []struct {
		name        string
		weight      int
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "zero weight",
			weight:      0,
			expectedMin: 0,
			expectedMax: 0,
		},
		{
			name:        "low weight",
			weight:      25,
			expectedMin: 24,
			expectedMax: 26,
		},
		{
			name:        "medium weight",
			weight:      50,
			expectedMin: 49,
			expectedMax: 51,
		},
		{
			name:        "high weight",
			weight:      100,
			expectedMin: 99,
			expectedMax: 101,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tt.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			assert.GreaterOrEqual(t, score, tt.expectedMin)
			assert.LessOrEqual(t, score, tt.expectedMax)
		})
	}
}

func TestWeightStrategy_Name(t *testing.T) {
	strategy := NewWeightStrategy()
	assert.Equal(t, "Weight", strategy.Name())
}

func TestErrorAwareStrategy_Name(t *testing.T) {
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}
	strategy := NewErrorAwareStrategy(mockProvider)
	assert.Equal(t, "ErrorAware", strategy.Name())
}

func TestErrorAwareStrategy_Score_NoMetrics(t *testing.T) {
	ctx := context.Background()

	// Mock provider returns empty metrics
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}
	strategy := NewErrorAwareStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 999, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	// Should return maxScore (200) when no failures
	assert.Equal(t, 200.0, score)
}

func TestErrorAwareStrategy_Score_WithMockConsecutiveFailures(t *testing.T) {
	ctx := context.Background()

	// Mock 3 consecutive failures
	metrics := &biz.AggregatedMetrics{}
	metrics.ConsecutiveFailures = 3

	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewErrorAwareStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	// Base 200 - (3 * 50) = 50
	assert.Equal(t, 50.0, score)
}

func TestErrorAwareStrategy_Score_WithMockRecentSuccess(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	recentSuccess := now.Add(-30 * time.Second)

	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: {
				LastSuccessAt: &recentSuccess,
			},
		},
	}
	strategy := NewErrorAwareStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	// Base 200 + 20 (recent success boost) = 220
	assert.Equal(t, 220.0, score)
}

func TestErrorAwareStrategy_Score_ConsecutiveFailures(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create channel
	ch, err := client.Channel.Create().
		SetName("test").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelService(client)

	// Record consecutive failures
	for i := 0; i < 3; i++ {
		perf := &biz.PerformanceRecord{
			ChannelID:        ch.ID,
			StartTime:        time.Now().Add(-time.Minute),
			EndTime:          time.Now(),
			Success:          false,
			RequestCompleted: true,
			ErrorStatusCode:  500,
		}
		channelService.RecordPerformance(ctx, perf)
	}

	strategy := NewErrorAwareStrategy(channelService)
	channel := &biz.Channel{Channel: ch}

	score := strategy.Score(ctx, channel)

	// Should have significant penalty for 3 consecutive failures
	// Base 200 - (3 * 50) = 50
	assert.Less(t, score, 100.0, "Score should be penalized for consecutive failures")
}

func TestErrorAwareStrategy_Score_RecentSuccess(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ch, err := client.Channel.Create().
		SetName("test").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelService(client)

	// Record a recent success
	perf := &biz.PerformanceRecord{
		ChannelID:        ch.ID,
		StartTime:        time.Now().Add(-10 * time.Second),
		EndTime:          time.Now(),
		Success:          true,
		RequestCompleted: true,
		TokenCount:       100,
	}
	channelService.RecordPerformance(ctx, perf)

	strategy := NewErrorAwareStrategy(channelService)
	channel := &biz.Channel{Channel: ch}

	score := strategy.Score(ctx, channel)

	// Should have boost for recent success
	// Base 200 + 20 (recent success) = 220
	assert.Greater(t, score, 200.0, "Score should be boosted for recent success")
}

func TestTraceAwareStrategy_Name(t *testing.T) {
	mockProvider := &mockTraceProvider{
		lastSuccessChannel: make(map[int]int),
	}
	strategy := NewTraceAwareStrategy(mockProvider)
	assert.Equal(t, "TraceAware", strategy.Name())
}

func TestTraceAwareStrategy_Score_NoTrace(t *testing.T) {
	ctx := context.Background()

	mockProvider := &mockTraceProvider{
		lastSuccessChannel: make(map[int]int),
	}
	strategy := NewTraceAwareStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 0.0, score, "Should return 0 when no trace in context")
}

func TestTraceAwareStrategy_Score_WithMockTrace(t *testing.T) {
	ctx := context.Background()

	// Mock: trace-123 last succeeded on channel 1
	mockProvider := &mockTraceProvider{
		lastSuccessChannel: map[int]int{
			123: 1,
		},
	}
	strategy := NewTraceAwareStrategy(mockProvider)

	// Add trace ID to context
	ctx = contexts.WithTrace(ctx, &ent.Trace{ID: 123})

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 1000.0, score, "Should return max boost for last successful channel")
}

func TestTraceAwareStrategy_Score_WithMockDifferentChannel(t *testing.T) {
	ctx := context.Background()

	// Mock: trace-456 last succeeded on channel 1
	mockProvider := &mockTraceProvider{
		lastSuccessChannel: map[int]int{
			456: 1,
		},
	}
	strategy := NewTraceAwareStrategy(mockProvider)

	// Add trace ID to context
	ctx = contexts.WithTrace(ctx, &ent.Trace{ID: 456})

	// Test with channel 2 (different from last success)
	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 2, Name: "test2"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 0.0, score, "Should return 0 for channels that weren't last successful")
}

func TestTraceAwareStrategy_Score_WithLastSuccessChannel(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create project
	project, err := client.Project.Create().
		SetName("test").
		Save(ctx)
	require.NoError(t, err)

	// Create channel
	ch, err := client.Channel.Create().
		SetName("test").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	// Create trace
	trace, err := client.Trace.Create().
		SetProjectID(project.ID).
		SetTraceID("test-trace-123").
		Save(ctx)
	require.NoError(t, err)

	// Create a successful request in this trace
	_, err = client.Request.Create().
		SetProjectID(project.ID).
		SetTraceID(trace.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetStatus("completed").
		SetSource("api").
		SetRequestBody([]byte(`{"model":"gpt-4","messages":[]}`)).
		Save(ctx)
	require.NoError(t, err)

	// Add trace ID to context and ent client
	ctx = contexts.WithTrace(ctx, &ent.Trace{ID: trace.ID})
	ctx = ent.NewContext(ctx, client)

	requestService := newTestRequestService(client)
	strategy := NewTraceAwareStrategy(requestService)

	channel := &biz.Channel{Channel: ch}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 1000.0, score, "Should return max boost for last successful channel")
}

func TestTraceAwareStrategy_Score_DifferentChannel(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	project, err := client.Project.Create().
		SetName("test").
		Save(ctx)
	require.NoError(t, err)

	ch1, err := client.Channel.Create().
		SetName("ch1").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetName("ch2").
		SetType("openai").
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	trace, err := client.Trace.Create().
		SetProjectID(project.ID).
		SetTraceID("test-trace-456").
		Save(ctx)
	require.NoError(t, err)

	// Create successful request with ch1
	_, err = client.Request.Create().
		SetProjectID(project.ID).
		SetTraceID(trace.ID).
		SetChannelID(ch1.ID).
		SetModelID("gpt-4").
		SetStatus("completed").
		SetSource("api").
		SetRequestBody([]byte(`{"model":"gpt-4","messages":[]}`)).
		Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithTrace(ctx, &ent.Trace{ID: trace.ID})

	requestService := newTestRequestService(client)
	strategy := NewTraceAwareStrategy(requestService)

	// Test with ch2 (different channel)
	channel := &biz.Channel{Channel: ch2}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 0.0, score, "Should return 0 for channels that weren't last successful")
}

func TestConnectionAwareStrategy_Name(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	channelService := newTestChannelService(client)
	tracker := NewDefaultConnectionTracker(10)
	strategy := NewConnectionAwareStrategy(channelService, tracker)
	assert.Equal(t, "ConnectionAware", strategy.Name())
}

func TestConnectionAwareStrategy_Score_NoTracker(t *testing.T) {
	ctx := context.Background()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	channelService := newTestChannelService(client)
	strategy := NewConnectionAwareStrategy(channelService, nil)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 25.0, score, "Should return neutral score when no tracker")
}

func TestConnectionAwareStrategy_Score_NoConnections(t *testing.T) {
	ctx := context.Background()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	channelService := newTestChannelService(client)
	tracker := NewDefaultConnectionTracker(10)
	strategy := NewConnectionAwareStrategy(channelService, tracker)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 50.0, score, "Should return max score when no active connections")
}

func TestConnectionAwareStrategy_Score_PartialUtilization(t *testing.T) {
	ctx := context.Background()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	channelService := newTestChannelService(client)
	tracker := NewDefaultConnectionTracker(10)
	strategy := NewConnectionAwareStrategy(channelService, tracker)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	// Simulate 5 active connections out of 10 max (50% utilization)
	for i := 0; i < 5; i++ {
		tracker.IncrementConnection(channel.ID)
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 25.0, score, "Should return half max score at 50% utilization")
}

func TestConnectionAwareStrategy_Score_FullUtilization(t *testing.T) {
	ctx := context.Background()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	channelService := newTestChannelService(client)
	tracker := NewDefaultConnectionTracker(10)
	strategy := NewConnectionAwareStrategy(channelService, tracker)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	// Simulate full utilization
	for i := 0; i < 10; i++ {
		tracker.IncrementConnection(channel.ID)
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 0.0, score, "Should return 0 at 100% utilization")
}

func TestCompositeStrategy_Score(t *testing.T) {
	ctx := context.Background()

	s1 := &mockStrategy{name: "s1", score: 100}
	s2 := &mockStrategy{name: "s2", score: 50}

	composite := NewCompositeStrategy(s1, s2)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := composite.Score(ctx, channel)
	assert.Equal(t, 150.0, score, "Should sum all strategy scores with default weights")
}

func TestCompositeStrategy_WithWeights(t *testing.T) {
	ctx := context.Background()

	s1 := &mockStrategy{name: "s1", score: 100}
	s2 := &mockStrategy{name: "s2", score: 50}

	composite := NewCompositeStrategy(s1, s2).WithWeights(2.0, 0.5)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score := composite.Score(ctx, channel)
	// (100 * 2.0) + (50 * 0.5) = 200 + 25 = 225
	assert.Equal(t, 225.0, score, "Should apply weights to strategy scores")
}

func TestCompositeStrategy_Name(t *testing.T) {
	composite := NewCompositeStrategy()
	assert.Equal(t, "Composite", composite.Name())
}

// Test that Score and ScoreWithDebug return identical scores

func TestWeightStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()
	strategy := NewWeightStrategy()

	testCases := []struct {
		name   string
		weight int
	}{
		{"zero weight", 0},
		{"low weight", 25},
		{"medium weight", 50},
		{"high weight", 100},
		{"negative weight", -10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tc.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for weight=%d", tc.weight)
		})
	}
}

func TestErrorAwareStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	recentFailure := now.Add(-2 * time.Minute)
	recentSuccess := now.Add(-30 * time.Second)
	oldFailure := now.Add(-10 * time.Minute)

	testCases := []struct {
		name    string
		metrics *biz.AggregatedMetrics
	}{
		{
			name: "no metrics",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				return m
			}(),
		},
		{
			name: "consecutive failures",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.ConsecutiveFailures = 3

				return m
			}(),
		},
		{
			name: "recent failure",
			metrics: &biz.AggregatedMetrics{
				LastFailureAt: &recentFailure,
			},
		},
		{
			name: "old failure",
			metrics: &biz.AggregatedMetrics{
				LastFailureAt: &oldFailure,
			},
		},
		{
			name: "recent success",
			metrics: &biz.AggregatedMetrics{
				LastSuccessAt: &recentSuccess,
			},
		},
		{
			name: "low success rate",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 20
				m.SuccessCount = 8 // 40% success rate

				return m
			}(),
		},
		{
			name: "high success rate",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 20
				m.SuccessCount = 19 // 95% success rate

				return m
			}(),
		},
		{
			name: "complex scenario",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.ConsecutiveFailures = 2
				m.LastFailureAt = &recentFailure
				m.LastSuccessAt = &recentSuccess
				m.RequestCount = 15
				m.SuccessCount = 10

				return m
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: tc.metrics,
				},
			}
			strategy := NewErrorAwareStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{ID: 1, Name: "test"},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			// Allow small tolerance for time-based calculations (time.Since() may differ slightly)
			assert.InDelta(t, score, debugScore, 0.01,
				"Score and ScoreWithDebug must return nearly identical scores for %s", tc.name)
		})
	}
}

func TestTraceAwareStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name               string
		traceID            int
		channelID          int
		lastSuccessChannel int
		hasTrace           bool
	}{
		{
			name:     "no trace",
			hasTrace: false,
		},
		{
			name:               "matching channel",
			traceID:            123,
			channelID:          1,
			lastSuccessChannel: 1,
			hasTrace:           true,
		},
		{
			name:               "different channel",
			traceID:            123,
			channelID:          2,
			lastSuccessChannel: 1,
			hasTrace:           true,
		},
		{
			name:               "no last success",
			traceID:            456,
			channelID:          1,
			lastSuccessChannel: 0,
			hasTrace:           true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockProvider := &mockTraceProvider{
				lastSuccessChannel: map[int]int{},
			}
			if tc.lastSuccessChannel > 0 {
				mockProvider.lastSuccessChannel[tc.traceID] = tc.lastSuccessChannel
			}

			strategy := NewTraceAwareStrategy(mockProvider)

			testCtx := ctx
			if tc.hasTrace {
				testCtx = contexts.WithTrace(ctx, &ent.Trace{ID: tc.traceID})
			}

			channel := &biz.Channel{
				Channel: &ent.Channel{ID: tc.channelID, Name: "test"},
			}

			score := strategy.Score(testCtx, channel)
			debugScore, _ := strategy.ScoreWithDebug(testCtx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for %s", tc.name)
		})
	}
}

func TestConnectionAwareStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	channelService := newTestChannelService(client)

	testCases := []struct {
		name              string
		tracker           ConnectionTracker
		channelID         int
		activeConnections int
	}{
		{
			name:      "no tracker",
			tracker:   nil,
			channelID: 1,
		},
		{
			name:              "no connections",
			tracker:           NewDefaultConnectionTracker(10),
			channelID:         1,
			activeConnections: 0,
		},
		{
			name:              "partial utilization",
			tracker:           NewDefaultConnectionTracker(10),
			channelID:         2,
			activeConnections: 5,
		},
		{
			name:              "full utilization",
			tracker:           NewDefaultConnectionTracker(10),
			channelID:         3,
			activeConnections: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := NewConnectionAwareStrategy(channelService, tc.tracker)

			if tc.tracker != nil {
				tracker := tc.tracker.(*DefaultConnectionTracker)
				for i := 0; i < tc.activeConnections; i++ {
					tracker.IncrementConnection(tc.channelID)
				}
			}

			channel := &biz.Channel{
				Channel: &ent.Channel{ID: tc.channelID, Name: "test"},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for %s", tc.name)
		})
	}
}

func TestCompositeStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	s1 := &mockStrategy{name: "s1", score: 100}
	s2 := &mockStrategy{name: "s2", score: 50}

	testCases := []struct {
		name    string
		weights []float64
	}{
		{
			name:    "default weights",
			weights: nil,
		},
		{
			name:    "custom weights",
			weights: []float64{2.0, 0.5},
		},
		{
			name:    "zero weights",
			weights: []float64{0, 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			composite := NewCompositeStrategy(s1, s2)
			if tc.weights != nil {
				composite = composite.WithWeights(tc.weights...)
			}

			channel := &biz.Channel{
				Channel: &ent.Channel{ID: 1, Name: "test"},
			}

			score := composite.Score(ctx, channel)
			debugScore, _ := composite.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for %s", tc.name)
		})
	}
}

func TestRoundRobinStrategy_Name(t *testing.T) {
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}
	strategy := NewRoundRobinStrategy(mockProvider)
	assert.Equal(t, "RoundRobin", strategy.Name())
}

func TestRoundRobinStrategy_Score_ZeroRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with zero requests
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 0
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "new-channel"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 150.0, score, "New channels with zero requests should get max score")
}

func TestRoundRobinStrategy_Score_LowRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with low request count (10 requests)
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 10
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "low-usage"},
	}

	score := strategy.Score(ctx, channel)
	// Should be high but less than maxScore
	assert.Greater(t, score, 100.0, "Low request channels should get high scores")
	assert.Less(t, score, 150.0, "Score should be less than maxScore")
}

func TestRoundRobinStrategy_Score_ModerateRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with moderate request count (100 requests)
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 100
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "moderate-usage"},
	}

	score := strategy.Score(ctx, channel)
	// With exponential decay (scaling factor 150), 100 requests scores ~77.0
	// This provides good differentiation while keeping 500 requests from hitting minimum too early
	assert.Greater(t, score, 70.0, "Moderate usage channels should get moderate-high scores")
	assert.Less(t, score, 80.0, "Score should reflect moderate usage")
}

func TestRoundRobinStrategy_Score_HighRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with high request count (500 requests)
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 500
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "high-usage"},
	}

	score := strategy.Score(ctx, channel)
	// With 500 requests, calculated score is ~5.35 which clamps to minScore (10.0)
	// This is expected behavior - heavily used channels get minimum priority
	assert.Equal(t, 10.0, score, "High usage channels should get minimum score when they exceed the decay curve")
}

func TestRoundRobinStrategy_Score_CappedRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with requests exceeding the cap (2000 requests, cap is 1000)
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 2000
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "very-high-usage"},
	}

	score := strategy.Score(ctx, channel)
	// Should be at or near minimum score
	assert.GreaterOrEqual(t, score, 10.0, "Score should not go below minScore")
	assert.LessOrEqual(t, score, 20.0, "Very high usage should result in very low score")
}

func TestRoundRobinStrategy_Score_MetricsError(t *testing.T) {
	ctx := context.Background()

	// Mock provider that returns error
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
		err:     assert.AnError,
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 999, Name: "error-channel"},
	}

	score := strategy.Score(ctx, channel)
	// Should return moderate score (max + min) / 2 = (150 + 10) / 2 = 80
	assert.Equal(t, 80.0, score, "Should return moderate score when metrics unavailable")
}

func TestRoundRobinStrategy_MultipleChannels(t *testing.T) {
	ctx := context.Background()

	// Create metrics for multiple channels
	metrics1 := &biz.AggregatedMetrics{}
	metrics1.RequestCount = 0
	metrics2 := &biz.AggregatedMetrics{}
	metrics2.RequestCount = 50
	metrics3 := &biz.AggregatedMetrics{}
	metrics3.RequestCount = 200
	metrics4 := &biz.AggregatedMetrics{}
	metrics4.RequestCount = 800

	// Multiple channels with different request counts
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics1, // New channel
			2: metrics2, // Low usage
			3: metrics3, // Moderate usage
			4: metrics4, // High usage
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channels := []struct {
		id   int
		name string
	}{
		{1, "channel-new"},
		{2, "channel-low"},
		{3, "channel-moderate"},
		{4, "channel-high"},
	}

	scores := make([]float64, len(channels))
	for i, ch := range channels {
		channel := &biz.Channel{
			Channel: &ent.Channel{ID: ch.id, Name: ch.name},
		}
		scores[i] = strategy.Score(ctx, channel)
	}

	// Verify ordering: new > low > moderate > high
	assert.Greater(t, scores[0], scores[1], "New channel should outrank low usage channel")
	assert.Greater(t, scores[1], scores[2], "Low usage channel should outrank moderate usage channel")
	assert.Greater(t, scores[2], scores[3], "Moderate usage channel should outrank high usage channel")

	// Verify specific values
	assert.Equal(t, 150.0, scores[0], "New channel should get max score")
	assert.Equal(t, 10.0, scores[3], "Very high usage channels should get minimum score")
}

func TestRoundRobinStrategy_ScoreWithDebug(t *testing.T) {
	ctx := context.Background()

	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 100
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score, strategyScore := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, "RoundRobin", strategyScore.StrategyName)
	assert.Greater(t, score, 0.0)
	assert.NotNil(t, strategyScore.Details)
	assert.Contains(t, strategyScore.Details, "request_count")
	assert.Contains(t, strategyScore.Details, "max_score")
	assert.Contains(t, strategyScore.Details, "calculated_score")
}

func TestRoundRobinStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		requestCount int64
	}{
		{"zero requests", 0},
		{"low requests", 10},
		{"moderate requests", 100},
		{"high requests", 500},
		{"capped requests", 1500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = tc.requestCount

			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: metrics,
				},
			}
			strategy := NewRoundRobinStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{ID: 1, Name: "test"},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for request_count=%d", tc.requestCount)
		})
	}
}

func TestRoundRobinStrategy_WithRealDatabase(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create multiple channels
	channels := make([]*ent.Channel, 4)
	for i := 0; i < 4; i++ {
		ch, err := client.Channel.Create().
			SetName(fmt.Sprintf("channel-%d", i)).
			SetType("openai").
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			Save(ctx)
		require.NoError(t, err)

		channels[i] = ch
	}

	channelService := newTestChannelService(client)

	// Record different numbers of requests for each channel
	requestCounts := []int64{0, 50, 200, 800}
	for i, ch := range channels {
		for j := int64(0); j < requestCounts[i]; j++ {
			perf := &biz.PerformanceRecord{
				ChannelID:        ch.ID,
				StartTime:        time.Now().Add(-time.Minute),
				EndTime:          time.Now(),
				Success:          true,
				RequestCompleted: true,
				TokenCount:       100,
			}
			channelService.RecordPerformance(ctx, perf)
		}
	}

	strategy := NewRoundRobinStrategy(channelService)

	// Score all channels
	scores := make([]float64, len(channels))
	for i, ch := range channels {
		channel := &biz.Channel{Channel: ch}
		scores[i] = strategy.Score(ctx, channel)
	}

	// Verify ordering based on request counts
	assert.Equal(t, 150.0, scores[0], "Channel with 0 requests should get max score")
	assert.Greater(t, scores[0], scores[1], "Lower request count should have higher score")
	assert.Greater(t, scores[1], scores[2], "Score should decrease with request count")
	assert.Greater(t, scores[2], scores[3], "Highest request count should have lowest score")
}

func TestWeightRoundRobinStrategy_Name(t *testing.T) {
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)
	assert.Equal(t, "WeightRoundRobin", strategy.Name())
}

func TestWeightRoundRobinStrategy_Score_ZeroRequests(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name   string
		weight int
		min    float64
		max    float64
	}{
		{"zero weight", 0, 150, 151},    // 150 (round-robin) + 0 (weight) = 150
		{"low weight", 25, 162, 163},    // 150 + ~12.5 = 162.5
		{"medium weight", 50, 174, 175}, // 150 + 25 = 175
		{"high weight", 100, 199, 200},  // 150 + 50 = 200
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = 0
			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: metrics,
				},
			}
			strategy := NewWeightRoundRobinStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "new-channel",
					OrderingWeight: tt.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			assert.GreaterOrEqual(t, score, tt.min)
			assert.LessOrEqual(t, score, tt.max)
		})
	}
}

func TestWeightRoundRobinStrategy_Score_ModerateRequests(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		requestCount int64
		weight       int
	}{
		{"100 requests, no weight", 100, 0},
		{"100 requests, low weight", 100, 25},
		{"100 requests, medium weight", 100, 50},
		{"100 requests, high weight", 100, 100},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = tt.requestCount
			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: metrics,
				},
			}
			strategy := NewWeightRoundRobinStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tt.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			// With 100 requests, round-robin component should be around 75
			// Weight component adds 0-50
			assert.Greater(t, score, 70.0, "Should get reasonable score with 100 requests")
			assert.Less(t, score, 200.0, "Total score should not exceed max")
		})
	}
}

func TestWeightRoundRobinStrategy_Score_HighRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with high request count (500 requests), medium weight
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 500
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             1,
			Name:           "high-usage",
			OrderingWeight: 50,
		},
	}

	score := strategy.Score(ctx, channel)
	// Round-robin component: 150 * e^(-500/150) = ~5.35
	// Weight component: (50/100) * 50 = 25
	// Total: ~30.35 (above minScore of 10, so not clamped)
	assert.InDelta(t, 30.35, score, 0.1, "High usage with weight should get score above min")
	assert.Greater(t, score, 25.0, "Weight component should contribute even with high requests")
}

func TestWeightRoundRobinStrategy_Score_MetricsError(t *testing.T) {
	ctx := context.Background()

	// Mock provider that returns error
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
		err:     assert.AnError,
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             999,
			Name:           "error-channel",
			OrderingWeight: 25,
		},
	}

	score := strategy.Score(ctx, channel)
	// Should return moderate score (maxRoundRobin+minScore)/2 + maxWeight/2
	// = (150+10)/2 + 50/2 = 80 + 25 = 105
	assert.GreaterOrEqual(t, score, 100.0, "Should return moderate score when metrics unavailable")
	assert.LessOrEqual(t, score, 110.0, "Should return moderate score when metrics unavailable")
}

func TestWeightRoundRobinStrategy_MultipleChannels(t *testing.T) {
	ctx := context.Background()

	// Create metrics for multiple channels with different combinations of requests and weights
	metrics1 := &biz.AggregatedMetrics{}
	metrics1.RequestCount = 0
	metrics2 := &biz.AggregatedMetrics{}
	metrics2.RequestCount = 50
	metrics3 := &biz.AggregatedMetrics{}
	metrics3.RequestCount = 200
	metrics4 := &biz.AggregatedMetrics{}
	metrics4.RequestCount = 800

	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics1, // Very new (0 requests)
			2: metrics2, // Low usage (50 requests)
			3: metrics3, // Medium usage (200 requests)
			4: metrics4, // High usage (800 requests)
		},
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)

	channels := []struct {
		id     int
		name   string
		weight int
	}{
		{1, "channel-new", 50},   // New but medium weight
		{2, "channel-low", 100},  // More requests but high weight
		{3, "channel-medium", 0}, // More requests but no weight
		{4, "channel-high", 75},  // Highest requests with medium-high weight
	}

	scores := make([]float64, len(channels))
	for i, ch := range channels {
		channel := &biz.Channel{
			Channel: &ent.Channel{
				ID:             ch.id,
				Name:           ch.name,
				OrderingWeight: ch.weight,
			},
		}
		scores[i] = strategy.Score(ctx, channel)
	}

	// Channel 1 should outrank channel 2 due to round-robin advantage (zero requests)
	assert.Greater(t, scores[0], scores[1], "New channel with 0 requests should outrank low usage channel even with lower weight")

	// Channel 2 should outrank channel 3 due to weight advantage (100 vs 0)
	assert.Greater(t, scores[1], scores[2], "Low usage channel with high weight should outrank medium usage channel with no weight")

	// Channel 3 should outrank channel 4 due to low request count
	assert.Greater(t, scores[2], scores[3], "Medium usage channel with no weight should outrank high usage channel with medium weight")
}

func TestWeightRoundRobinStrategy_WithRealDatabase(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create multiple channels with different weights
	channels := make([]*ent.Channel, 4)

	weights := []int{0, 25, 50, 100}
	for i := 0; i < 4; i++ {
		ch, err := client.Channel.Create().
			SetName(fmt.Sprintf("channel-%d", i)).
			SetType("openai").
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetOrderingWeight(weights[i]).
			Save(ctx)
		require.NoError(t, err)

		channels[i] = ch
	}

	channelService := newTestChannelService(client)

	// Record different numbers of requests for each channel
	requestCounts := []int64{0, 50, 200, 800}
	for i, ch := range channels {
		for j := int64(0); j < requestCounts[i]; j++ {
			perf := &biz.PerformanceRecord{
				ChannelID:        ch.ID,
				StartTime:        time.Now().Add(-time.Minute),
				EndTime:          time.Now(),
				Success:          true,
				RequestCompleted: true,
				TokenCount:       100,
			}
			channelService.RecordPerformance(ctx, perf)
		}
	}

	strategy := NewWeightRoundRobinStrategy(channelService)

	// Score all channels
	scores := make([]float64, len(channels))
	for i, ch := range channels {
		channel := &biz.Channel{Channel: ch}
		scores[i] = strategy.Score(ctx, channel)
	}

	// Verify ordering is affected by both request counts and weights
	// Channel 0: 0 requests, weight 0 = score ~150
	// Channel 1: 50 requests, weight 25 = score ~110 + 12.5 = 122.5
	// Channel 2: 200 requests, weight 50 = score ~70 + 25 = 95
	// Channel 3: 800 requests, weight 100 = score ~11 + 50 = 61 (clamped to 10)

	// New channel should have highest score
	assert.Greater(t, scores[0], scores[1], "Channel 0 (0 requests, 0 weight) should outrank channel 1 (50 requests, weight 25)")

	// Weight can compensate for request count differences
	assert.Greater(t, scores[2], scores[3], "Channel 2 (200 requests, weight 50) should outrank channel 3 (800 requests, weight 100)")
}

func TestWeightRoundRobinStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		requestCount int64
		weight       int
	}{
		{"zero requests, zero weight", 0, 0},
		{"zero requests, low weight", 0, 25},
		{"zero requests, high weight", 0, 100},
		{"low requests, low weight", 10, 25},
		{"moderate requests, medium weight", 100, 50},
		{"high requests, high weight", 500, 100},
		{"capped requests, medium weight", 1500, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = tc.requestCount

			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: metrics,
				},
			}
			strategy := NewWeightRoundRobinStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tc.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for %s", tc.name)
		})
	}
}
