package biz

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func TestMetricsRecord_CalculateSuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		metrics  metricsRecord
		expected int64
	}{
		{
			name: "100% success rate",
			metrics: metricsRecord{
				RequestCount: 100,
				SuccessCount: 100,
			},
			expected: 100,
		},
		{
			name: "50% success rate",
			metrics: metricsRecord{
				RequestCount: 100,
				SuccessCount: 50,
			},
			expected: 50,
		},
		{
			name: "0% success rate",
			metrics: metricsRecord{
				RequestCount: 100,
				SuccessCount: 0,
			},
			expected: 0,
		},
		{
			name: "no requests",
			metrics: metricsRecord{
				RequestCount: 0,
				SuccessCount: 0,
			},
			expected: 0,
		},
		{
			name: "75% success rate",
			metrics: metricsRecord{
				RequestCount: 80,
				SuccessCount: 60,
			},
			expected: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.CalculateSuccessRate()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsRecord_CalculateAvgLatencyMs(t *testing.T) {
	tests := []struct {
		name     string
		metrics  metricsRecord
		expected int64
	}{
		{
			name: "average latency with successful requests",
			metrics: metricsRecord{
				SuccessCount:          10,
				TotalRequestLatencyMs: 1000,
			},
			expected: 100,
		},
		{
			name: "no successful requests",
			metrics: metricsRecord{
				SuccessCount:          0,
				TotalRequestLatencyMs: 1000,
			},
			expected: 0,
		},
		{
			name: "single successful request",
			metrics: metricsRecord{
				SuccessCount:          1,
				TotalRequestLatencyMs: 250,
			},
			expected: 250,
		},
		{
			name: "high latency",
			metrics: metricsRecord{
				SuccessCount:          5,
				TotalRequestLatencyMs: 10000,
			},
			expected: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.CalculateAvgLatencyMs()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsRecord_CalculateAvgTokensPerSecond(t *testing.T) {
	tests := []struct {
		name     string
		metrics  metricsRecord
		expected float64
	}{
		{
			name: "average tokens per second",
			metrics: metricsRecord{
				RequestCount:    10,
				TotalTokenCount: 1000,
			},
			expected: 100.0,
		},
		{
			name: "no requests",
			metrics: metricsRecord{
				RequestCount:    0,
				TotalTokenCount: 1000,
			},
			expected: 0,
		},
		{
			name: "fractional average",
			metrics: metricsRecord{
				RequestCount:    3,
				TotalTokenCount: 100,
			},
			expected: 33.333333333333336,
		},
		{
			name: "zero tokens",
			metrics: metricsRecord{
				RequestCount:    10,
				TotalTokenCount: 0,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.CalculateAvgTokensPerSecond()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsRecord_CalculateAvgFirstTokenLatencyMs(t *testing.T) {
	tests := []struct {
		name     string
		metrics  metricsRecord
		expected int64
	}{
		{
			name: "average first token latency",
			metrics: metricsRecord{
				StreamSuccessCount:             10,
				StreamTotalFirstTokenLatencyMs: 500,
			},
			expected: 50,
		},
		{
			name: "no stream requests",
			metrics: metricsRecord{
				StreamSuccessCount:             0,
				StreamTotalFirstTokenLatencyMs: 500,
			},
			expected: 0,
		},
		{
			name: "single stream request",
			metrics: metricsRecord{
				StreamSuccessCount:             1,
				StreamTotalFirstTokenLatencyMs: 150,
			},
			expected: 150,
		},
		{
			name: "high first token latency",
			metrics: metricsRecord{
				StreamSuccessCount:             5,
				StreamTotalFirstTokenLatencyMs: 5000,
			},
			expected: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.CalculateAvgFirstTokenLatencyMs()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsRecord_CalculateAvgStreamTokensPerSecond(t *testing.T) {
	tests := []struct {
		name     string
		metrics  metricsRecord
		expected float64
	}{
		{
			name: "average stream tokens per second",
			metrics: metricsRecord{
				StreamSuccessCount:    10,
				StreamTotalTokenCount: 1000,
			},
			expected: 100.0,
		},
		{
			name: "no stream requests",
			metrics: metricsRecord{
				StreamSuccessCount:    0,
				StreamTotalTokenCount: 1000,
			},
			expected: 0,
		},
		{
			name: "fractional average",
			metrics: metricsRecord{
				StreamSuccessCount:    3,
				StreamTotalTokenCount: 100,
			},
			expected: 33.333333333333336,
		},
		{
			name: "zero tokens",
			metrics: metricsRecord{
				StreamSuccessCount:    10,
				StreamTotalTokenCount: 0,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.CalculateAvgStreamTokensPerSecond()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestChannelService_RecordMetrics(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	svc := &ChannelService{
		AbstractService: &AbstractService{
			db: client,
		},
	}

	// Create a test channel
	ch, err := client.Channel.Create().
		SetName("test-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	// Initialize performance record
	err = svc.InitializeChannelPerformance(ctx, ch.ID)
	require.NoError(t, err)

	tests := []struct {
		name         string
		metrics      *AggretagedMetrics
		channelID    int
		validateFunc func(t *testing.T)
	}{
		{
			name: "record metrics with all fields",
			metrics: &AggretagedMetrics{
				metricsRecord: metricsRecord{
					RequestCount:                   100,
					SuccessCount:                   90,
					FailureCount:                   10,
					TotalTokenCount:                9000,
					TotalRequestLatencyMs:          45000,
					StreamTotalTokenCount:          5000,
					StreamTotalFirstTokenLatencyMs: 2500,
					StreamSuccessCount:             50,
				},
				LastSuccessAt: func() *time.Time { t := time.Now(); return &t }(),
				LastFailureAt: func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
			},
			channelID: ch.ID,
			validateFunc: func(t *testing.T) {
				perf, err := client.ChannelPerformance.Query().First(ctx)
				require.NoError(t, err)
				require.Equal(t, 90, perf.SuccessRate)
				require.Equal(t, 500, perf.AvgLatencyMs)
				require.Equal(t, 90, perf.AvgTokenPerSecond)
				require.Equal(t, 50, perf.AvgStreamFirstTokenLatencyMs)
				require.Equal(t, 100.0, perf.AvgStreamTokenPerSecond)
			},
		},
		{
			name: "record metrics with zero success",
			metrics: &AggretagedMetrics{
				metricsRecord: metricsRecord{
					RequestCount:          10,
					SuccessCount:          0,
					FailureCount:          10,
					TotalTokenCount:       0,
					TotalRequestLatencyMs: 0,
				},
			},
			channelID: ch.ID,
			validateFunc: func(t *testing.T) {
				perf, err := client.ChannelPerformance.Query().First(ctx)
				require.NoError(t, err)
				require.Equal(t, 0, perf.SuccessRate)
				require.Equal(t, 0, perf.AvgLatencyMs)
			},
		},
		{
			name:      "nil metrics",
			metrics:   nil,
			channelID: ch.ID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc.RecordMetrics(ctx, tt.channelID, tt.metrics)

			if tt.validateFunc != nil {
				tt.validateFunc(t)
			}
		})
	}
}

func TestAggretagedMetrics_AllCalculations(t *testing.T) {
	// Test all calculation methods together
	metrics := &AggretagedMetrics{
		metricsRecord: metricsRecord{
			RequestCount:                   100,
			SuccessCount:                   80,
			FailureCount:                   20,
			TotalTokenCount:                8000,
			TotalRequestLatencyMs:          40000,
			StreamTotalTokenCount:          4000,
			StreamTotalFirstTokenLatencyMs: 2000,
			StreamSuccessCount:             40,
		},
		LastSuccessAt: lo.ToPtr(time.Now()),
		LastFailureAt: lo.ToPtr(time.Now().Add(-1 * time.Hour)),
	}

	// Test all calculations
	require.Equal(t, int64(80), metrics.CalculateSuccessRate())
	require.Equal(t, int64(500), metrics.CalculateAvgLatencyMs())
	require.Equal(t, float64(80), metrics.CalculateAvgTokensPerSecond())
	require.Equal(t, int64(50), metrics.CalculateAvgFirstTokenLatencyMs())
	require.Equal(t, float64(100), metrics.CalculateAvgStreamTokensPerSecond())
}

func TestMetricsRecord_EdgeCases(t *testing.T) {
	t.Run("all zeros", func(t *testing.T) {
		metrics := &metricsRecord{}
		require.Equal(t, int64(0), metrics.CalculateSuccessRate())
		require.Equal(t, int64(0), metrics.CalculateAvgLatencyMs())
		require.Equal(t, float64(0), metrics.CalculateAvgTokensPerSecond())
		require.Equal(t, int64(0), metrics.CalculateAvgFirstTokenLatencyMs())
		require.Equal(t, float64(0), metrics.CalculateAvgStreamTokensPerSecond())
	})

	t.Run("large numbers", func(t *testing.T) {
		metrics := &metricsRecord{
			RequestCount:                   1000000,
			SuccessCount:                   999999,
			TotalTokenCount:                999999000000,
			TotalRequestLatencyMs:          999999000000,
			StreamSuccessCount:             500000,
			StreamTotalTokenCount:          500000000000,
			StreamTotalFirstTokenLatencyMs: 500000000000,
		}
		require.Equal(t, int64(99), metrics.CalculateSuccessRate())
		require.Equal(t, int64(1000000), metrics.CalculateAvgLatencyMs())
		require.Equal(t, float64(999999), metrics.CalculateAvgTokensPerSecond())
		require.Equal(t, int64(1000000), metrics.CalculateAvgFirstTokenLatencyMs())
		require.Equal(t, float64(1000000), metrics.CalculateAvgStreamTokensPerSecond())
	})
}

func TestChannelMetrics_RecordSuccess(t *testing.T) {
	cm := newChannelMetrics(1)
	now := time.Now()

	slot := &timeSlotMetrics{
		timestamp:     now.Unix(),
		metricsRecord: metricsRecord{},
	}

	tests := []struct {
		name                string
		perf                *PerformanceRecord
		firstTokenLatencyMs int64
		requestLatencyMs    int64
		validateFunc        func(t *testing.T)
	}{
		{
			name: "record non-stream success",
			perf: &PerformanceRecord{
				ChannelID:  1,
				StartTime:  now.Add(-100 * time.Millisecond),
				EndTime:    now,
				Stream:     false,
				Success:    true,
				TokenCount: 100,
			},
			firstTokenLatencyMs: 0,
			requestLatencyMs:    100,
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(1), slot.SuccessCount)
				require.Equal(t, int64(1), cm.aggreatedMetrics.SuccessCount)
				require.Equal(t, int64(100), slot.TotalRequestLatencyMs)
				require.Equal(t, int64(100), cm.aggreatedMetrics.TotalRequestLatencyMs)
				require.Equal(t, int64(100), slot.TotalTokenCount)
				require.Equal(t, int64(100), cm.aggreatedMetrics.TotalTokenCount)
				require.Equal(t, int64(0), slot.StreamSuccessCount)
				require.NotNil(t, cm.aggreatedMetrics.LastSuccessAt)
			},
		},
		{
			name: "record stream success",
			perf: &PerformanceRecord{
				ChannelID:      2,
				StartTime:      now.Add(-200 * time.Millisecond),
				FirstTokenTime: func() *time.Time { t := now.Add(-150 * time.Millisecond); return &t }(),
				EndTime:        now,
				Stream:         true,
				Success:        true,
				TokenCount:     500,
			},
			firstTokenLatencyMs: 50,
			requestLatencyMs:    200,
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(2), slot.SuccessCount)
				require.Equal(t, int64(2), cm.aggreatedMetrics.SuccessCount)
				require.Equal(t, int64(300), slot.TotalRequestLatencyMs)
				require.Equal(t, int64(300), cm.aggreatedMetrics.TotalRequestLatencyMs)
				require.Equal(t, int64(1), slot.StreamSuccessCount)
				require.Equal(t, int64(1), cm.aggreatedMetrics.StreamSuccessCount)
				require.Equal(t, int64(500), slot.StreamTotalTokenCount)
				require.Equal(t, int64(50), slot.StreamTotalFirstTokenLatencyMs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.recordSuccess(slot, tt.perf, tt.firstTokenLatencyMs, tt.requestLatencyMs)

			if tt.validateFunc != nil {
				tt.validateFunc(t)
			}
		})
	}
}

func TestChannelMetrics_RecordFailure(t *testing.T) {
	cm := newChannelMetrics(1)
	now := time.Now()

	slot := &timeSlotMetrics{
		timestamp:     now.Unix(),
		metricsRecord: metricsRecord{},
	}

	tests := []struct {
		name         string
		perf         *PerformanceRecord
		validateFunc func(t *testing.T)
	}{
		{
			name: "record first failure",
			perf: &PerformanceRecord{
				ChannelID:       1,
				StartTime:       now.Add(-100 * time.Millisecond),
				EndTime:         now,
				Success:         false,
				ErrorStatusCode: 500,
			},
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(1), slot.FailureCount)
				require.Equal(t, int64(1), cm.aggreatedMetrics.FailureCount)
				require.Equal(t, int64(1), cm.aggreatedMetrics.ConsecutiveFailures)
				require.NotNil(t, cm.aggreatedMetrics.LastFailureAt)
			},
		},
		{
			name: "record second consecutive failure",
			perf: &PerformanceRecord{
				ChannelID:       1,
				StartTime:       now.Add(-100 * time.Millisecond),
				EndTime:         now,
				Success:         false,
				ErrorStatusCode: 429,
			},
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(2), slot.FailureCount)
				require.Equal(t, int64(2), cm.aggreatedMetrics.FailureCount)
				require.Equal(t, int64(2), cm.aggreatedMetrics.ConsecutiveFailures)
			},
		},
		{
			name: "record third consecutive failure",
			perf: &PerformanceRecord{
				ChannelID:       1,
				StartTime:       now.Add(-100 * time.Millisecond),
				EndTime:         now,
				Success:         false,
				ErrorStatusCode: 500,
			},
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(3), slot.FailureCount)
				require.Equal(t, int64(3), cm.aggreatedMetrics.FailureCount)
				require.Equal(t, int64(3), cm.aggreatedMetrics.ConsecutiveFailures)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.recordFailure(slot, tt.perf)

			if tt.validateFunc != nil {
				tt.validateFunc(t)
			}
		})
	}
}

func TestChannelMetrics_ConsecutiveFailures(t *testing.T) {
	cm := newChannelMetrics(1)
	now := time.Now()

	slot := &timeSlotMetrics{
		timestamp:     now.Unix(),
		metricsRecord: metricsRecord{},
	}

	// Record 3 consecutive failures
	for i := 0; i < 3; i++ {
		perf := &PerformanceRecord{
			ChannelID:       1,
			StartTime:       now.Add(-100 * time.Millisecond),
			EndTime:         now,
			Success:         false,
			ErrorStatusCode: 500,
		}
		cm.recordFailure(slot, perf)
	}

	require.Equal(t, int64(3), cm.aggreatedMetrics.ConsecutiveFailures)

	// Record a success - should reset consecutive failures
	successPerf := &PerformanceRecord{
		ChannelID:  1,
		StartTime:  now.Add(-100 * time.Millisecond),
		EndTime:    now,
		Success:    true,
		TokenCount: 100,
	}
	cm.recordSuccess(slot, successPerf, 0, 100)
	require.Equal(t, int64(0), cm.aggreatedMetrics.ConsecutiveFailures)

	// Record another failure - should start from 1 again
	failPerf := &PerformanceRecord{
		ChannelID:       1,
		StartTime:       now.Add(-100 * time.Millisecond),
		EndTime:         now,
		Success:         false,
		ErrorStatusCode: 429,
	}
	cm.recordFailure(slot, failPerf)
	require.Equal(t, int64(1), cm.aggreatedMetrics.ConsecutiveFailures)
}

func TestChannelMetrics_GetOrCreateTimeSlot(t *testing.T) {
	cm := newChannelMetrics(1)
	now := time.Now()
	ts := now.Unix()

	t.Run("create new slot", func(t *testing.T) {
		slot := cm.getOrCreateTimeSlot(ts, now, 600)
		require.NotNil(t, slot)
		require.Equal(t, ts, slot.timestamp)
		require.Equal(t, 1, cm.window.Len())
	})

	t.Run("get existing slot", func(t *testing.T) {
		slot := cm.getOrCreateTimeSlot(ts, now, 600)
		require.NotNil(t, slot)
		require.Equal(t, ts, slot.timestamp)
		require.Equal(t, 1, cm.window.Len()) // Should still be 1
	})

	t.Run("cleanup old slots when window is full", func(t *testing.T) {
		cm := newChannelMetrics(1)
		windowSize := int64(10)

		// Fill the window
		for i := int64(0); i < windowSize; i++ {
			ts := now.Add(-time.Duration(i) * time.Second).Unix()
			cm.getOrCreateTimeSlot(ts, now.Add(-time.Duration(i)*time.Second), windowSize)
		}

		require.Equal(t, int(windowSize), cm.window.Len())

		// Add one more with a much older timestamp - should trigger cleanup
		// The new slot is far in the future, so old slots should be cleaned
		futureTime := now.Add(time.Duration(windowSize+5) * time.Second)
		newTs := futureTime.Unix()
		cm.getOrCreateTimeSlot(newTs, futureTime, windowSize)

		// After cleanup, only the new slot should remain (all old ones are outside the window)
		require.Equal(t, 1, cm.window.Len())
	})
}

func TestChannelService_RecordPerformance_UnrecoverableError(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	svc := &ChannelService{
		AbstractService: &AbstractService{
			db: client,
		},
		channelPerfMetrics: make(map[int]*channelMetrics),
		perfWindowSeconds:  600,
	}

	// Create a test channel
	ch, err := client.Channel.Create().
		SetName("test-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now()

	tests := []struct {
		name          string
		errorCode     int
		shouldDisable bool
	}{
		{
			name:          "401 unauthorized - should disable",
			errorCode:     401,
			shouldDisable: true,
		},
		{
			name:          "403 forbidden - should disable",
			errorCode:     403,
			shouldDisable: true,
		},
		{
			name:          "404 not found - should disable",
			errorCode:     404,
			shouldDisable: true,
		},
		{
			name:          "500 server error - should not disable",
			errorCode:     500,
			shouldDisable: false,
		},
		{
			name:          "429 rate limit - should not disable",
			errorCode:     429,
			shouldDisable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset channel status to enabled
			_, err := client.Channel.UpdateOneID(ch.ID).
				SetStatus(channel.StatusEnabled).
				ClearErrorMessage().
				Save(ctx)
			require.NoError(t, err)

			perf := &PerformanceRecord{
				ChannelID:        ch.ID,
				StartTime:        now.Add(-100 * time.Millisecond),
				EndTime:          now,
				Success:          false,
				RequestCompleted: true,
				ErrorStatusCode:  tt.errorCode,
			}

			svc.RecordPerformance(ctx, perf)

			// Give goroutine time to complete
			time.Sleep(100 * time.Millisecond)

			// Check channel status
			updatedCh, err := client.Channel.Get(ctx, ch.ID)
			require.NoError(t, err)

			if tt.shouldDisable {
				require.Equal(t, channel.StatusDisabled, updatedCh.Status)
				require.NotNil(t, updatedCh.ErrorMessage)
			} else {
				require.Equal(t, channel.StatusEnabled, updatedCh.Status)
			}
		})
	}
}

func TestChannelService_RecordPerformance(t *testing.T) {
	ctx := context.Background()
	svc := &ChannelService{
		channelPerfMetrics: make(map[int]*channelMetrics),
		perfWindowSeconds:  600,
	}

	now := time.Now()

	tests := []struct {
		name         string
		perf         *PerformanceRecord
		validateFunc func(t *testing.T)
	}{
		{
			name: "record successful non-stream request",
			perf: &PerformanceRecord{
				ChannelID:        1,
				StartTime:        now.Add(-100 * time.Millisecond),
				EndTime:          now,
				Stream:           false,
				Success:          true,
				RequestCompleted: true,
				TokenCount:       100,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(1), cm.aggreatedMetrics.RequestCount)
				require.Equal(t, int64(1), cm.aggreatedMetrics.SuccessCount)
				require.Equal(t, int64(0), cm.aggreatedMetrics.FailureCount)
				require.Equal(t, int64(100), cm.aggreatedMetrics.TotalTokenCount)
				require.Equal(t, int64(100), cm.aggreatedMetrics.TotalRequestLatencyMs)
			},
		},
		{
			name: "record successful stream request",
			perf: &PerformanceRecord{
				ChannelID:        1,
				StartTime:        now.Add(-200 * time.Millisecond),
				FirstTokenTime:   func() *time.Time { t := now.Add(-150 * time.Millisecond); return &t }(),
				EndTime:          now,
				Stream:           true,
				Success:          true,
				RequestCompleted: true,
				TokenCount:       500,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(2), cm.aggreatedMetrics.RequestCount)
				require.Equal(t, int64(2), cm.aggreatedMetrics.SuccessCount)
				require.Equal(t, int64(1), cm.aggreatedMetrics.StreamSuccessCount)
				require.Equal(t, int64(500), cm.aggreatedMetrics.StreamTotalTokenCount)
				require.Equal(t, int64(300), cm.aggreatedMetrics.TotalRequestLatencyMs)
			},
		},
		{
			name: "record failed request with error code",
			perf: &PerformanceRecord{
				ChannelID:        1,
				StartTime:        now.Add(-100 * time.Millisecond),
				EndTime:          now,
				Stream:           false,
				Success:          false,
				RequestCompleted: true,
				ErrorStatusCode:  500,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(3), cm.aggreatedMetrics.RequestCount)
				require.Equal(t, int64(1), cm.aggreatedMetrics.FailureCount)
				require.Equal(t, int64(1), cm.aggreatedMetrics.ConsecutiveFailures)
				require.Equal(t, int64(300), cm.aggreatedMetrics.TotalRequestLatencyMs)
			},
		},
		{
			name: "record multiple errors with different codes",
			perf: &PerformanceRecord{
				ChannelID:        1,
				StartTime:        now.Add(-100 * time.Millisecond),
				EndTime:          now,
				Stream:           false,
				Success:          false,
				RequestCompleted: true,
				ErrorStatusCode:  429,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(2), cm.aggreatedMetrics.FailureCount)
				require.Equal(t, int64(2), cm.aggreatedMetrics.ConsecutiveFailures)
				require.Equal(t, int64(300), cm.aggreatedMetrics.TotalRequestLatencyMs)
			},
		},
		{
			name: "record success after failure resets consecutive failures",
			perf: &PerformanceRecord{
				ChannelID:        1,
				StartTime:        now.Add(-100 * time.Millisecond),
				EndTime:          now,
				Stream:           false,
				Success:          true,
				RequestCompleted: true,
				TokenCount:       200,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(3), cm.aggreatedMetrics.SuccessCount)
				require.Equal(t, int64(0), cm.aggreatedMetrics.ConsecutiveFailures)
				require.Equal(t, int64(400), cm.aggreatedMetrics.TotalRequestLatencyMs)
			},
		},
		{
			name: "ignore invalid performance record",
			perf: &PerformanceRecord{
				ChannelID:        0, // Invalid channel ID
				StartTime:        now,
				EndTime:          now,
				RequestCompleted: false,
			},
			validateFunc: func(t *testing.T) {
				// Should not create metrics for invalid record
				_, exists := svc.channelPerfMetrics[0]
				require.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc.RecordPerformance(ctx, tt.perf)

			if tt.validateFunc != nil {
				tt.validateFunc(t)
			}
		})
	}
}
