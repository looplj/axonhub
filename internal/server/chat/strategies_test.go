package chat

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

// mockMetricsProvider is a mock implementation of ChannelMetricsProvider for testing.
type mockMetricsProvider struct {
	metrics map[int]*biz.AggretagedMetrics
	err     error
}

func (m *mockMetricsProvider) GetChannelMetrics(ctx context.Context, channelID int) (*biz.AggretagedMetrics, error) {
	if m.err != nil {
		return nil, m.err
	}

	if metrics, ok := m.metrics[channelID]; ok {
		return metrics, nil
	}

	return &biz.AggretagedMetrics{}, nil
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
		metrics: make(map[int]*biz.AggretagedMetrics),
	}
	strategy := NewErrorAwareStrategy(mockProvider)
	assert.Equal(t, "ErrorAware", strategy.Name())
}

func TestErrorAwareStrategy_Score_NoMetrics(t *testing.T) {
	ctx := context.Background()

	// Mock provider returns empty metrics
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggretagedMetrics),
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
	metrics := &biz.AggretagedMetrics{}
	metrics.ConsecutiveFailures = 3

	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggretagedMetrics{
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
		metrics: map[int]*biz.AggretagedMetrics{
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

	traceService := newTestTraceService(client)
	strategy := NewTraceAwareStrategy(traceService)

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

	traceService := newTestTraceService(client)
	strategy := NewTraceAwareStrategy(traceService)

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
		metrics *biz.AggretagedMetrics
	}{
		{
			name: "no metrics",
			metrics: func() *biz.AggretagedMetrics {
				m := &biz.AggretagedMetrics{}
				return m
			}(),
		},
		{
			name: "consecutive failures",
			metrics: func() *biz.AggretagedMetrics {
				m := &biz.AggretagedMetrics{}
				m.ConsecutiveFailures = 3

				return m
			}(),
		},
		{
			name: "recent failure",
			metrics: &biz.AggretagedMetrics{
				LastFailureAt: &recentFailure,
			},
		},
		{
			name: "old failure",
			metrics: &biz.AggretagedMetrics{
				LastFailureAt: &oldFailure,
			},
		},
		{
			name: "recent success",
			metrics: &biz.AggretagedMetrics{
				LastSuccessAt: &recentSuccess,
			},
		},
		{
			name: "low success rate",
			metrics: func() *biz.AggretagedMetrics {
				m := &biz.AggretagedMetrics{}
				m.RequestCount = 20
				m.SuccessCount = 8 // 40% success rate

				return m
			}(),
		},
		{
			name: "high success rate",
			metrics: func() *biz.AggretagedMetrics {
				m := &biz.AggretagedMetrics{}
				m.RequestCount = 20
				m.SuccessCount = 19 // 95% success rate

				return m
			}(),
		},
		{
			name: "complex scenario",
			metrics: func() *biz.AggretagedMetrics {
				m := &biz.AggretagedMetrics{}
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
				metrics: map[int]*biz.AggretagedMetrics{
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
