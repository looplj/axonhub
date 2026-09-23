package biz

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

func TestAggregatedMetrics_Clone(t *testing.T) {
	now := time.Now()
	metrics := &AggregatedMetrics{
		metricsRecord: metricsRecord{
			RequestCount:        100,
			SuccessCount:        80,
			FailureCount:        20,
			ConsecutiveFailures: 0,
		},
		LastSelectedAt:                 new(now),
		LastFailureAt:                  new(now.Add(-1 * time.Hour)),
		StreamingFirstTokenLatencyEWMA: 320,
		StreamingTokensPerSecondEWMA:   42,
		StreamingSampleCount:           3,
		NonStreamingLatencyEWMA:        1800,
		NonStreamingSampleCount:        4,
	}

	cloned := metrics.Clone()
	require.Equal(t, metrics.metricsRecord, cloned.metricsRecord)
	require.Equal(t, metrics.LastSelectedAt, cloned.LastSelectedAt)
	require.Equal(t, metrics.LastFailureAt, cloned.LastFailureAt)
	require.Equal(t, metrics.StreamingFirstTokenLatencyEWMA, cloned.StreamingFirstTokenLatencyEWMA)
	require.Equal(t, metrics.StreamingTokensPerSecondEWMA, cloned.StreamingTokensPerSecondEWMA)
	require.Equal(t, metrics.StreamingSampleCount, cloned.StreamingSampleCount)
	require.Equal(t, metrics.NonStreamingLatencyEWMA, cloned.NonStreamingLatencyEWMA)
	require.Equal(t, metrics.NonStreamingSampleCount, cloned.NonStreamingSampleCount)
}

func TestChannelMetrics_RecordSuccess(t *testing.T) {
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
			name: "record success",
			perf: &PerformanceRecord{
				ChannelID: 1,
				EndTime:   now,
				Success:   true,
			},
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(1), slot.SuccessCount)
				require.Equal(t, int64(1), cm.aggregatedMetrics.SuccessCount)
				require.Equal(t, int64(0), cm.aggregatedMetrics.ConsecutiveFailures)
				require.Nil(t, cm.aggregatedMetrics.LastSelectedAt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.recordSuccess(slot, tt.perf)

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
				ChannelID:          1,
				EndTime:            now,
				Success:            false,
				ResponseStatusCode: 500,
			},
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(1), slot.FailureCount)
				require.Equal(t, int64(1), cm.aggregatedMetrics.FailureCount)
				require.Equal(t, int64(1), cm.aggregatedMetrics.ConsecutiveFailures)
				require.NotNil(t, cm.aggregatedMetrics.LastFailureAt)
			},
		},
		{
			name: "record second consecutive failure",
			perf: &PerformanceRecord{
				ChannelID:          1,
				EndTime:            now,
				Success:            false,
				ResponseStatusCode: 429,
			},
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(2), slot.FailureCount)
				require.Equal(t, int64(2), cm.aggregatedMetrics.FailureCount)
				require.Equal(t, int64(2), cm.aggregatedMetrics.ConsecutiveFailures)
			},
		},
		{
			name: "record third consecutive failure",
			perf: &PerformanceRecord{
				ChannelID:          1,
				EndTime:            now,
				Success:            false,
				ResponseStatusCode: 500,
			},
			validateFunc: func(t *testing.T) {
				require.Equal(t, int64(3), slot.FailureCount)
				require.Equal(t, int64(3), cm.aggregatedMetrics.FailureCount)
				require.Equal(t, int64(3), cm.aggregatedMetrics.ConsecutiveFailures)
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
	for range 3 {
		perf := &PerformanceRecord{
			ChannelID:          1,
			EndTime:            now,
			Success:            false,
			ResponseStatusCode: 500,
		}
		cm.recordFailure(slot, perf)
	}

	require.Equal(t, int64(3), cm.aggregatedMetrics.ConsecutiveFailures)

	// Record a success - should reset consecutive failures
	successPerf := &PerformanceRecord{
		ChannelID: 1,
		EndTime:   now,
		Success:   true,
	}
	cm.recordSuccess(slot, successPerf)
	require.Equal(t, int64(0), cm.aggregatedMetrics.ConsecutiveFailures)

	// Record another failure - should start from 1 again
	failPerf := &PerformanceRecord{
		ChannelID:          1,
		EndTime:            now,
		Success:            false,
		ResponseStatusCode: 429,
	}
	cm.recordFailure(slot, failPerf)
	require.Equal(t, int64(1), cm.aggregatedMetrics.ConsecutiveFailures)
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
		for i := range windowSize {
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
	// Disabled the feature for now.
	t.SkipNow()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	svc := NewChannelServiceForTest(client)

	// Create a test channel
	ch, err := client.Channel.Create().
		SetName("test-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
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
				ChannelID:          ch.ID,
				EndTime:            now,
				Success:            false,
				RequestCompleted:   true,
				ResponseStatusCode: tt.errorCode,
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
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Auto-disable resolution reads the enabled-channel cache, so the fixture
	// needs the fully wired service rather than a bare struct literal.
	svc := newTestChannelService(client)

	now := time.Now()

	tests := []struct {
		name         string
		perf         *PerformanceRecord
		validateFunc func(t *testing.T)
	}{
		{
			name: "record successful request",
			perf: &PerformanceRecord{
				ChannelID:        1,
				EndTime:          now,
				Success:          true,
				RequestCompleted: true,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(1), cm.aggregatedMetrics.RequestCount)
				require.Equal(t, int64(1), cm.aggregatedMetrics.SuccessCount)
				require.Equal(t, int64(0), cm.aggregatedMetrics.FailureCount)
			},
		},
		{
			name: "record failed request with error code",
			perf: &PerformanceRecord{
				ChannelID:          1,
				EndTime:            now,
				Success:            false,
				RequestCompleted:   true,
				ResponseStatusCode: 500,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(2), cm.aggregatedMetrics.RequestCount)
				require.Equal(t, int64(1), cm.aggregatedMetrics.FailureCount)
				require.Equal(t, int64(1), cm.aggregatedMetrics.ConsecutiveFailures)
			},
		},
		{
			name: "record multiple errors with different codes",
			perf: &PerformanceRecord{
				ChannelID:          1,
				EndTime:            now,
				Success:            false,
				RequestCompleted:   true,
				ResponseStatusCode: 429,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(2), cm.aggregatedMetrics.FailureCount)
				require.Equal(t, int64(2), cm.aggregatedMetrics.ConsecutiveFailures)
			},
		},
		{
			name: "record success after failure resets consecutive failures",
			perf: &PerformanceRecord{
				ChannelID:        1,
				EndTime:          now,
				Success:          true,
				RequestCompleted: true,
			},
			validateFunc: func(t *testing.T) {
				cm := svc.channelPerfMetrics[1]
				require.NotNil(t, cm)
				require.Equal(t, int64(2), cm.aggregatedMetrics.SuccessCount)
				require.Equal(t, int64(0), cm.aggregatedMetrics.ConsecutiveFailures)
			},
		},
		{
			name: "ignore invalid performance record",
			perf: &PerformanceRecord{
				ChannelID:        0, // Invalid channel ID
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
			// IncrementChannelSelection is called at selection time in production.
			// It records RequestCount in the selection-time slot before the request
			// completes. RecordPerformance only adds outcome and latency signals.
			if tt.perf != nil && tt.perf.ChannelID > 0 {
				svc.IncrementChannelSelection(tt.perf.ChannelID)
			}

			svc.RecordPerformance(ctx, tt.perf)

			if tt.validateFunc != nil {
				tt.validateFunc(t)
			}
		})
	}
}

func TestPerformanceRecord_Methods(t *testing.T) {
	t.Run("MarkSuccess", func(t *testing.T) {
		perf := &PerformanceRecord{}
		perf.MarkSuccess()
		require.True(t, perf.Success)
		require.True(t, perf.RequestCompleted)
		require.False(t, perf.EndTime.IsZero())
	})

	t.Run("MarkFailed", func(t *testing.T) {
		perf := &PerformanceRecord{}
		perf.MarkFailed(500)
		require.False(t, perf.Success)
		require.True(t, perf.RequestCompleted)
		require.Equal(t, 500, perf.ResponseStatusCode)
		require.False(t, perf.EndTime.IsZero())
	})

	t.Run("MarkCanceled", func(t *testing.T) {
		perf := &PerformanceRecord{}
		perf.MarkCanceled()
		require.False(t, perf.Success)
		require.True(t, perf.Canceled)
		require.True(t, perf.RequestCompleted)
		require.False(t, perf.EndTime.IsZero())
	})

	t.Run("IsValid", func(t *testing.T) {
		validPerf := &PerformanceRecord{
			ChannelID:        1,
			RequestCompleted: true,
		}
		require.True(t, validPerf.IsValid())

		invalidPerf1 := &PerformanceRecord{
			ChannelID:        0,
			RequestCompleted: true,
		}
		require.False(t, invalidPerf1.IsValid())

		invalidPerf2 := &PerformanceRecord{
			ChannelID:        1,
			RequestCompleted: false,
		}
		require.False(t, invalidPerf2.IsValid())
	})
}

func TestChannelMetrics_GetExpiresIdleChannels(t *testing.T) {
	for _, window := range []int64{0, 10} {
		t.Run((time.Duration(window) * time.Second).String(), func(t *testing.T) {
			svc := &ChannelService{
				channelPerfMetrics: make(map[int]*channelMetrics),
				perfWindowSeconds:  window,
			}
			windowSize := svc.performanceWindowSeconds()
			cm := newChannelMetricsWithWindow(1, windowSize)
			at := time.Now().Add(-time.Duration(windowSize+5) * time.Second)
			slot := cm.getOrCreateTimeSlot(at.Unix(), at, windowSize)
			slot.metricsRecord = metricsRecord{RequestCount: 1000, SuccessCount: 900, FailureCount: 100}
			cm.aggregatedMetrics.metricsRecord = slot.metricsRecord
			svc.channelPerfMetrics[1] = cm

			// An idle channel must recover without being selected or recording
			// another outcome to trigger write-side cleanup.
			metrics, err := svc.GetChannelMetrics(context.Background(), 1)
			require.NoError(t, err)
			require.Zero(t, metrics.RequestCount)
			require.Zero(t, metrics.SuccessCount)
			require.Zero(t, metrics.FailureCount)
			require.Zero(t, cm.window.Len())
			require.NotSame(t, cm.aggregatedMetrics, metrics)
		})
	}
}

func TestChannelMetrics_ExistingChannelsDoNotRequireGlobalWriteLock(t *testing.T) {
	tests := []struct {
		name string
		run  func(*ChannelService)
	}{
		{
			name: "read",
			run: func(svc *ChannelService) {
				_, _ = svc.GetChannelMetrics(context.Background(), 1)
			},
		},
		{
			name: "selection",
			run: func(svc *ChannelService) {
				svc.IncrementChannelSelection(1)
			},
		},
		{
			name: "outcome",
			run: func(svc *ChannelService) {
				svc.RecordPerformance(context.Background(), &PerformanceRecord{
					ChannelID: 1, EndTime: time.Now(), Success: true, RequestCompleted: true,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ChannelService{channelPerfMetrics: map[int]*channelMetrics{1: newChannelMetrics(1)}}
			svc.channelPerfMetricsLock.RLock()
			done := make(chan struct{})
			defer func() {
				svc.channelPerfMetricsLock.RUnlock()
				<-done
			}()
			go func() {
				defer close(done)
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("metrics operation panicked: %v", recovered)
					}
				}()
				tt.run(svc)
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("existing channel operation blocked on the global map lock")
			}
		})
	}
}

func TestChannelMetrics_ConcurrentReadsAndWrites(t *testing.T) {
	const (
		channels          = 4
		workersPerChannel = 4
		requestsPerWorker = 100
	)
	ctx := context.Background()
	svc := &ChannelService{channelPerfMetrics: make(map[int]*channelMetrics)}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for worker := range channels * workersPerChannel {
		channelID := worker%channels + 1
		workers.Go(func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("metrics worker panicked: %v", recovered)
				}
			}()
			<-start
			for range requestsPerWorker {
				svc.IncrementChannelSelection(channelID)
				svc.RecordPerformance(ctx, &PerformanceRecord{
					ChannelID: channelID, EndTime: time.Now(), Success: true, RequestCompleted: true,
				})
				metrics, err := svc.GetChannelMetrics(ctx, channelID)
				if err != nil {
					t.Errorf("get channel metrics: %v", err)
					return
				}
				if metrics.SuccessCount > metrics.RequestCount || metrics.NonStreamingSampleCount != metrics.SuccessCount {
					t.Errorf("inconsistent channel snapshot: %+v", metrics)
					return
				}
			}
		})
	}
	close(start)
	workers.Wait()

	for channelID := 1; channelID <= channels; channelID++ {
		metrics, err := svc.GetChannelMetrics(ctx, channelID)
		require.NoError(t, err)
		require.EqualValues(t, workersPerChannel*requestsPerWorker, metrics.RequestCount)
		require.Equal(t, metrics.RequestCount, metrics.SuccessCount)
		require.Equal(t, metrics.SuccessCount, metrics.NonStreamingSampleCount)
	}
}

func TestChannelMetrics_SelectionCountExpiresFromSelectionTime(t *testing.T) {
	ctx := context.Background()
	svc := &ChannelService{
		channelPerfMetrics: make(map[int]*channelMetrics),
	}

	svc.IncrementChannelSelection(1)
	selectedAt := *svc.channelPerfMetrics[1].aggregatedMetrics.LastSelectedAt
	completedAt := selectedAt.Add(5 * time.Minute)
	svc.RecordPerformance(ctx, &PerformanceRecord{
		ChannelID:        1,
		EndTime:          completedAt,
		Success:          true,
		RequestCompleted: true,
	})

	cm := svc.channelPerfMetrics[1]
	require.Equal(t, int64(1), cm.aggregatedMetrics.RequestCount)
	require.Equal(t, int64(1), cm.aggregatedMetrics.SuccessCount)
	require.Equal(t, selectedAt, *cm.aggregatedMetrics.LastSelectedAt)

	// Expiring the selection slot must remove RequestCount even though the
	// completion outcome belongs to a newer slot.
	cm.cleanupExpiredSlots(selectedAt.Add(time.Second))
	require.Zero(t, cm.aggregatedMetrics.RequestCount)
	require.Equal(t, int64(1), cm.aggregatedMetrics.SuccessCount)
}

// TestChannelMetrics_LoadAllFromExecutions_TimeFormatCompat covers SQLite
// created_at stored as TEXT with formats from different driver generations.
func TestChannelMetrics_LoadAllFromExecutions_TimeFormatCompat(t *testing.T) {
	// Use a file database so legacy driver timestamp formats can be injected via raw SQL
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	client := enttest.NewEntClient(t, "sqlite3", "file:"+dbPath+"?_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	now := time.Now().UTC()
	svc := &ChannelService{}

	// failed record 1: current driver format (written via ent)
	failed1 := client.RequestExecution.Create().
		SetProjectID(1).
		SetRequestID(1001).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusFailed).
		SetCreatedAt(now.Add(-2 * time.Hour)).
		SetUpdatedAt(now.Add(-2 * time.Hour)).
		SaveX(ctx)

	// failed record 2: created_at rewritten via raw SQL to the legacy .000000000 format
	legacyAt := now.Add(-1 * time.Hour).Format("2006-01-02 15:04:05.000000000 -0700 MST")
	failed2 := client.RequestExecution.Create().
		SetProjectID(1).
		SetRequestID(1002).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusFailed).
		SetCreatedAt(now.Add(-1 * time.Hour)).
		SetUpdatedAt(now.Add(-1 * time.Hour)).
		SaveX(ctx)

	sqlDB, err := sql.Open("sqlite3", "file:"+dbPath+"?_fk=0")
	require.NoError(t, err)
	defer sqlDB.Close()

	_, err = sqlDB.Exec("UPDATE request_executions SET created_at = ? WHERE id = ?", legacyAt, failed2.ID)
	require.NoError(t, err)

	// completed record: counts toward request_count but not last_failure_at
	client.RequestExecution.Create().
		SetProjectID(1).
		SetRequestID(1003).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetCreatedAt(now.Add(-30 * time.Minute)).
		SetUpdatedAt(now.Add(-30 * time.Minute)).
		SaveX(ctx)

	// channel with only completed records: MAX(CASE ...) is NULL and must not error
	client.RequestExecution.Create().
		SetProjectID(1).
		SetRequestID(2001).
		SetChannelID(2).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetCreatedAt(now.Add(-1 * time.Hour)).
		SetUpdatedAt(now.Add(-1 * time.Hour)).
		SaveX(ctx)

	metrics, err := svc.loadAllChannelMetricsFromExecutions(ctx, client, now.Add(-6*time.Hour))
	require.NoError(t, err)

	// channel 1: 3 records, with failed2 as the latest failure at -1h.
	// Compare absolute instants (Unix) instead of
	// time.Time values, which are sensitive to the Location pointer.
	require.Contains(t, metrics, 1)
	require.Equal(t, int64(3), metrics[1].RequestCount)
	require.NotNil(t, metrics[1].LastSelectedAt)
	require.Equal(t, now.Add(-30*time.Minute).Truncate(time.Second).Unix(), metrics[1].LastSelectedAt.Unix())
	require.NotNil(t, metrics[1].LastFailureAt)
	require.Equal(t, now.Add(-1*time.Hour).Truncate(time.Second).Unix(), metrics[1].LastFailureAt.Unix())
	require.NotEqual(t, failed1.CreatedAt.Truncate(time.Second).Unix(), metrics[1].LastFailureAt.Unix())
	require.Zero(t, metrics[1].ConsecutiveFailures)

	// channel 2: no failed records, LastFailureAt must be nil
	require.Contains(t, metrics, 2)
	require.Equal(t, int64(1), metrics[2].RequestCount)
	require.NotNil(t, metrics[2].LastSelectedAt)
	require.Nil(t, metrics[2].LastFailureAt)
	require.Zero(t, metrics[2].ConsecutiveFailures)
}

func TestChannelMetrics_LoadAllFromExecutions_RebuildsExpiringWindow(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	now := time.Now().UTC().Truncate(time.Second)
	svc := &ChannelService{}

	createExecution := func(requestID int, status requestexecution.Status, createdAt time.Time) {
		t.Helper()
		client.RequestExecution.Create().
			SetProjectID(1).
			SetRequestID(requestID).
			SetChannelID(1).
			SetModelID("gpt-4").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStatus(status).
			SetCreatedAt(createdAt).
			SetUpdatedAt(createdAt).
			SaveX(ctx)
	}

	createExecution(1, requestexecution.StatusCompleted, now.Add(-11*time.Minute))
	createExecution(2, requestexecution.StatusCompleted, now.Add(-9*time.Minute))
	createExecution(3, requestexecution.StatusFailed, now.Add(-time.Minute))
	createExecution(4, requestexecution.StatusCanceled, now.Add(-30*time.Second))

	metrics, err := svc.loadAllChannelMetricsFromExecutions(ctx, client, now.Add(-10*time.Minute))
	require.NoError(t, err)
	require.Contains(t, metrics, 1)

	loaded := metrics[1]
	require.Equal(t, int64(3), loaded.RequestCount)
	require.Len(t, loaded.Slots, 3)
	require.NotNil(t, loaded.LastSelectedAt)
	require.Equal(t, now.Add(-30*time.Second).Unix(), loaded.LastSelectedAt.Unix())
	require.NotNil(t, loaded.LastFailureAt)
	require.Equal(t, now.Add(-time.Minute).Unix(), loaded.LastFailureAt.Unix())
	require.Equal(t, int64(1), loaded.ConsecutiveFailures)

	cm := newChannelMetricsWithWindow(1, defaultPerformanceWindowSize)
	svc.populateChannelMetrics(cm, loaded)
	require.Equal(t, int64(3), cm.aggregatedMetrics.RequestCount)
	require.Equal(t, int64(1), cm.aggregatedMetrics.SuccessCount)
	require.Equal(t, int64(1), cm.aggregatedMetrics.FailureCount)
	require.Equal(t, int64(1), cm.aggregatedMetrics.ConsecutiveFailures)

	cm.cleanupExpiredSlots(now.Add(-2 * time.Minute))
	require.Equal(t, int64(2), cm.aggregatedMetrics.RequestCount)
	require.Equal(t, int64(0), cm.aggregatedMetrics.SuccessCount)
	require.Equal(t, int64(1), cm.aggregatedMetrics.FailureCount)
	require.Len(t, cm.window.GetAll(), 2)
}

func TestParseDBTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		value   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "current driver String() format",
			value: now.Format("2006-01-02 15:04:05.999999999 -0700 MST"),
			want:  now,
		},
		{
			name:  "legacy fixed 9-digit format",
			value: "2026-04-18 07:41:37.000000000 +0000 UTC",
			want:  time.Date(2026, 4, 18, 7, 41, 37, 0, time.UTC),
		},
		{
			name:  "RFC3339Nano",
			value: "2026-08-10T13:22:10.251164681Z",
			want:  time.Date(2026, 8, 10, 13, 22, 10, 251164681, time.UTC),
		},
		{
			name:  "sqlite CURRENT_TIMESTAMP format",
			value: "2026-04-18 07:41:37",
			want:  time.Date(2026, 4, 18, 7, 41, 37, 0, time.UTC),
		},
		{
			name:  "fractional timestamp without timezone",
			value: "2026-04-18 07:41:37.123456",
			want:  time.Date(2026, 4, 18, 7, 41, 37, 123456000, time.UTC),
		},
		{
			name:    "unrecognized format",
			value:   "not-a-time",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDBTime(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			// Compare full instants (including fractional seconds) with Equal,
			// which ignores the Location pointer, unlike == / require.Equal.
			require.True(t, got.Equal(tt.want), "parseDBTime(%q) = %v, want %v", tt.value, got, tt.want)
		})
	}
}

func TestChannelMetrics_DelayedOutcomeDoesNotLeakSelectionCount(t *testing.T) {
	const window = int64(600)
	cm := newChannelMetricsWithWindow(1, window)
	base := time.Now().Truncate(time.Second)
	selectAt := func(second int64) {
		at := base.Add(time.Duration(second) * time.Second)
		slot := cm.getOrCreateTimeSlot(at.Unix(), at, window)
		require.NotNil(t, slot)
		slot.RequestCount++
		cm.aggregatedMetrics.RequestCount++
	}
	selectAt(0)
	selectAt(2)
	svc := &ChannelService{channelPerfMetrics: map[int]*channelMetrics{1: cm}}
	// A one-second async delay inserts a completion behind a newer selection.
	svc.RecordPerformance(context.Background(), &PerformanceRecord{
		ChannelID: 1, EndTime: base.Add(time.Second), Success: true, RequestCompleted: true,
	})
	items := cm.window.GetAll()
	require.Len(t, items, 3)
	for i, item := range items {
		require.Equal(t, base.Add(time.Duration(i)*time.Second).Unix(), item.Timestamp)
	}
	for i := int64(3); i <= window+2; i++ {
		selectAt(i)
	}
	cm.cleanupExpiredSlots(base.Add(time.Duration(window+3) * time.Second))
	require.Zero(t, cm.window.Len())
	require.Zero(t, cm.aggregatedMetrics.RequestCount)
	require.Zero(t, cm.aggregatedMetrics.SuccessCount)

	// An already-expired completion must not revive the window or displace
	// another slot, even when its own timestamp would imply an older cutoff.
	svc.RecordPerformance(context.Background(), &PerformanceRecord{
		ChannelID: 1, EndTime: base.Add(time.Second), Success: true, RequestCompleted: true,
	})
	require.Zero(t, cm.window.Len())
	require.Zero(t, cm.aggregatedMetrics.SuccessCount)
}

func TestChannelMetrics_RecoveryUsesCompletionOrderWithinCreationWindow(t *testing.T) {
	for _, latestSuccess := range []bool{true, false} {
		name := "last completion failed"
		if latestSuccess {
			name = "last completion succeeded"
		}
		t.Run(name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:recovery-order?mode=memory&_fk=0")
			defer client.Close()
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			now := time.Now().UTC().Truncate(time.Second)
			create := func(id int, status requestexecution.Status, start, end time.Time) {
				t.Helper()
				client.RequestExecution.Create().SetProjectID(1).SetRequestID(id).
					SetChannelID(1).SetModelID("gpt-4").SetRequestBody(objects.JSONRawMessage(`{}`)).
					SetStatus(status).SetCreatedAt(start).SetUpdatedAt(end).SaveX(ctx)
			}
			longStatus, shortStatus := requestexecution.StatusFailed, requestexecution.StatusCompleted
			if latestSuccess {
				longStatus, shortStatus = shortStatus, longStatus
			}
			// Among executions created inside the window, completion order still
			// determines health even when it differs from creation order.
			create(1, longStatus, now.Add(-9*time.Minute), now.Add(-time.Second))
			for i := 2; i <= 4; i++ {
				at := now.Add(-time.Duration(5-i) * time.Minute)
				create(i, shortStatus, at, at.Add(time.Second))
			}
			// Cancellation never resets consecutive failures.
			create(5, requestexecution.StatusCanceled, now.Add(-time.Second), now)
			// An older execution is excluded even if its recent completion would
			// otherwise change the last failure or consecutive failure count.
			create(6, shortStatus, now.Add(-20*time.Minute), now)
			metrics, err := (&ChannelService{}).loadAllChannelMetricsFromExecutions(ctx, client, now.Add(-10*time.Minute))
			require.NoError(t, err)
			loaded := metrics[1]
			require.Equal(t, int64(5), loaded.RequestCount)
			if latestSuccess {
				require.Zero(t, loaded.ConsecutiveFailures)
				require.Equal(t, now.Add(-time.Minute+time.Second), *loaded.LastFailureAt)
			} else {
				require.Equal(t, int64(1), loaded.ConsecutiveFailures)
				require.Equal(t, now.Add(-time.Second), *loaded.LastFailureAt)
			}
			cm := newChannelMetrics(1)
			(&ChannelService{}).populateChannelMetrics(cm, loaded)
			cm.cleanupExpiredSlots(now.Add(-30 * time.Second))
			// Older selections/outcomes expire; the long request's recent
			// completion and the canceled selection remain.
			require.Equal(t, int64(1), cm.aggregatedMetrics.RequestCount)
			if latestSuccess {
				require.Equal(t, int64(1), cm.aggregatedMetrics.SuccessCount)
				require.Zero(t, cm.aggregatedMetrics.FailureCount)
			} else {
				require.Equal(t, int64(1), cm.aggregatedMetrics.FailureCount)
				require.Zero(t, cm.aggregatedMetrics.SuccessCount)
			}
		})
	}
}

func TestChannelMetrics_RecoveryCompletionOrderAcrossBatches(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:recovery-batches?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	now := time.Now().UTC().Truncate(time.Second)
	for i := range channelMetricsLoadBatchSize + 2 {
		status, end := requestexecution.StatusFailed, now.Add(-time.Minute)
		if i == 0 {
			status, end = requestexecution.StatusCompleted, now
		}
		client.RequestExecution.Create().SetProjectID(1).SetRequestID(i + 1).
			SetChannelID(1).SetModelID("gpt-4").SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStatus(status).SetCreatedAt(now.Add(-2 * time.Minute)).SetUpdatedAt(end).SaveX(ctx)
	}
	metrics, err := (&ChannelService{}).loadAllChannelMetricsFromExecutions(ctx, client, now.Add(-10*time.Minute))
	require.NoError(t, err)
	require.Equal(t, int64(channelMetricsLoadBatchSize+2), metrics[1].RequestCount)
	require.Zero(t, metrics[1].ConsecutiveFailures)
}
