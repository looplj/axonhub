package chat

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/server/biz"
)

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
