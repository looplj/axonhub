package orchestrator

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
	// Should return maxScore (200) when no failures - no boosts applied
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

	// Create metrics with recent success - but no boost should be applied
	metrics := &biz.AggregatedMetrics{
		LastSuccessAt: &recentSuccess,
	}

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
	// Base 200 - no boosts applied (we removed success-based boosts)
	assert.Equal(t, 200.0, score)
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
	for range 3 {
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

	// No boost for recent success - we only apply penalties
	// Base 200, no penalties = 200
	assert.Equal(t, 200.0, score, "Score should be base score with no penalties")
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
	for range 5 {
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
	for range 10 {
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

// TestErrorAwareStrategy_NoPenaltyForHealthyChannels tests that healthy channels get base score.
func TestErrorAwareStrategy_NoPenaltyForHealthyChannels(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		requestCount int64
		successCount int64
	}{
		{"zero requests", 0, 0},
		{"1 request", 1, 1},
		{"4 requests", 4, 4},
		{"5 requests with 100% success", 5, 5},
		{"10 requests with 100% success", 10, 10},
		{"100 requests with 80% success", 100, 80}, // 80% > 50%, no penalty
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = tc.requestCount
			metrics.SuccessCount = tc.successCount

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

			// All healthy channels should get base score (200) - no boosts, no penalties
			assert.Equal(t, 200.0, score, "Healthy channel should get base score for %s", tc.name)
		})
	}
}

// TestErrorAwareStrategy_OnlyPenaltiesNoBoosts tests that only penalties are applied, no boosts.
func TestErrorAwareStrategy_OnlyPenaltiesNoBoosts(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	recentSuccess := now.Add(-30 * time.Second)

	// Test that recent success does NOT give a boost
	t.Run("no recent success boost", func(t *testing.T) {
		metrics := &biz.AggregatedMetrics{
			LastSuccessAt: &recentSuccess,
		}
		metrics.RequestCount = 10
		metrics.SuccessCount = 10 // 100% success rate

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
		// Base 200, no boosts applied
		assert.Equal(t, 200.0, score)
	})

	// Test that high success rate does NOT give a boost
	t.Run("no high success rate boost", func(t *testing.T) {
		metrics := &biz.AggregatedMetrics{}
		metrics.RequestCount = 10
		metrics.SuccessCount = 10 // 100% success rate

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
		// Base 200, no boosts applied
		assert.Equal(t, 200.0, score)
	})

	// Test that low success rate DOES apply penalty
	t.Run("low success rate penalty still applies", func(t *testing.T) {
		metrics := &biz.AggregatedMetrics{}
		metrics.RequestCount = 10
		metrics.SuccessCount = 4 // 40% success rate

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
		// Base 200 - 50 (low success rate penalty) = 150
		assert.Equal(t, 150.0, score)
	})
}

// TestErrorAwareStrategy_LowSuccessRatePenalty tests that low success rate penalty is applied.
func TestErrorAwareStrategy_LowSuccessRatePenalty(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                    string
		requestCount            int64
		successCount            int64
		expectLowSuccessPenalty bool
	}{
		{"4 requests - no check (below threshold)", 4, 1, false},
		{"5 requests - high rate, no penalty", 5, 5, false},
		{"5 requests - low rate, penalty", 5, 2, true},
		{"10 requests - high rate, no penalty", 10, 10, false},
		{"10 requests - 50% rate, no penalty", 10, 5, false}, // 50% is not < 50%
		{"10 requests - 40% rate, penalty", 10, 4, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = tc.requestCount
			metrics.SuccessCount = tc.successCount

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

			expectedScore := 200.0
			if tc.expectLowSuccessPenalty {
				expectedScore -= 50.0
			}

			assert.Equal(t, expectedScore, score, "Score should match expected for %s", tc.name)
		})
	}
}

// TestErrorAwareStrategy_FairDistribution tests that the strategy promotes fair distribution
// by NOT giving boosts to successful channels.
func TestErrorAwareStrategy_FairDistribution(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	recentSuccess := now.Add(-30 * time.Second)
	oldSuccess := now.Add(-30 * time.Minute)

	// Simulate the bug scenario:
	// Channel 8: heavily used (23 requests), recent success, 100% success rate
	// Channel 6: lightly used (5 requests), old success
	// Channel 4: never used (0 requests)
	// Channel 7: barely used (1 request), old success

	channelMetrics := map[int]*biz.AggregatedMetrics{
		8: func() *biz.AggregatedMetrics {
			m := &biz.AggregatedMetrics{LastSuccessAt: &recentSuccess}
			m.RequestCount = 23
			m.SuccessCount = 23

			return m
		}(),
		6: func() *biz.AggregatedMetrics {
			m := &biz.AggregatedMetrics{LastSuccessAt: &oldSuccess}
			m.RequestCount = 5
			m.SuccessCount = 5

			return m
		}(),
		4: func() *biz.AggregatedMetrics {
			m := &biz.AggregatedMetrics{}
			m.RequestCount = 0
			m.SuccessCount = 0

			return m
		}(),
		7: func() *biz.AggregatedMetrics {
			m := &biz.AggregatedMetrics{LastSuccessAt: &oldSuccess}
			m.RequestCount = 1
			m.SuccessCount = 1

			return m
		}(),
	}

	mockProvider := &mockMetricsProvider{metrics: channelMetrics}
	strategy := NewErrorAwareStrategy(mockProvider)

	scores := make(map[int]float64)

	for id := range channelMetrics {
		channel := &biz.Channel{
			Channel: &ent.Channel{ID: id, Name: "test"},
		}
		scores[id] = strategy.Score(ctx, channel)
	}

	// With no boosts, all healthy channels should get the same base score (200)
	// This ensures ErrorAwareStrategy doesn't interfere with WeightRoundRobinStrategy's distribution
	for id, score := range scores {
		assert.Equal(t, 200.0, score,
			"Channel %d should get base score (no boosts), got %.1f", id, score)
	}
}
