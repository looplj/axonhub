package biz

import (
	"context"
	"database/sql"
	"path/filepath"
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
				require.NotNil(t, cm.aggregatedMetrics.LastSelectedAt)
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
			// It increments aggregatedMetrics.RequestCount before the request completes.
			// RecordPerformance only increments slot.RequestCount (for sliding window),
			// not aggregatedMetrics.RequestCount (to avoid double counting).
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

// TestChannelMetrics_LoadAllFromExecutions_TimeFormatCompat covers SQLite
// created_at stored as TEXT with formats from different driver generations:
// the MAX(...) aggregate is returned as a string and must parse both the
// current driver format and the legacy .000000000 format, and must not error
// when there are no failed records (NULL).
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

	// channel 1: 3 records, last_failure_at is the lexicographically largest failed
	// record (failed2 at -1h). Compare absolute instants (Unix) instead of
	// time.Time values, which are sensitive to the Location pointer.
	require.Contains(t, metrics, 1)
	require.Equal(t, int64(3), metrics[1].RequestCount)
	require.NotNil(t, metrics[1].LastFailureAt)
	require.Equal(t, now.Add(-1*time.Hour).Truncate(time.Second).Unix(), metrics[1].LastFailureAt.Unix())
	require.NotEqual(t, failed1.CreatedAt.Truncate(time.Second).Unix(), metrics[1].LastFailureAt.Unix())

	// channel 2: no failed records, LastFailureAt must be nil
	require.Contains(t, metrics, 2)
	require.Equal(t, int64(1), metrics[2].RequestCount)
	require.Nil(t, metrics[2].LastFailureAt)
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
