package biz

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/ringbuffer"
	"github.com/looplj/axonhub/internal/pkg/xtime"
)

const (
	// defaultPerformanceWindowSize is the default size of the sliding window in seconds (10 minutes).
	defaultPerformanceWindowSize = 600

	// channelMetricsLoadBatchSize bounds the number of lightweight execution rows
	// held in memory while rebuilding the sliding window at startup.
	channelMetricsLoadBatchSize = 1000

	// MinLatencyMs is the minimum latency value (10ms) used for tokens/second calculations.
	// This matches the frontend standard MINIMUM_LATENCY_MS_FOR_CACHE_HITS.
	MinLatencyMs = 10
)

// ClampLatency enforces the minimum latency value to prevent extreme TPS calculations.
// Returns the latency if it's >= MinLatencyMs, otherwise returns MinLatencyMs.
func ClampLatency(latencyMs int64) int64 {
	if latencyMs < MinLatencyMs {
		return MinLatencyMs
	}

	return latencyMs
}

// channelMetrics holds the performance metrics for a channel in memory.
type channelMetrics struct {
	channelID int
	// mu protects the window and aggregated metrics after publication in the
	// service map. Release the map lock before acquiring this lock.
	mu sync.Mutex

	// sliding window of metrics for the last N minutes using ring buffer for O(1) cleanup
	window *ringbuffer.RingBuffer[*timeSlotMetrics]
	// Neither delayed outcomes nor clock adjustments may move the window back.
	windowCutoff int64
	latestSlot   int64

	// aggregatedMetrics holds accumulated metrics for the flush period
	aggregatedMetrics *AggregatedMetrics
}

// loadChannelPerformances rebuilds the in-memory load-balancing window from
// recent request executions.
func (svc *ChannelService) loadChannelPerformances(ctx context.Context) error {
	client := svc.entFromContext(ctx)

	windowSize := svc.performanceWindowSeconds()
	since := xtime.UTCNow().Add(-time.Duration(windowSize) * time.Second)

	// Rebuild each channel's recent time slots from lightweight execution rows.
	metrics, err := svc.loadAllChannelMetricsFromExecutions(ctx, client, since)
	if err != nil {
		return fmt.Errorf("failed to load channel metrics: %w", err)
	}

	if len(metrics) == 0 {
		log.Info(ctx, "No request execution data found in the performance window")
		return nil
	}

	restored := make(map[int]*channelMetrics, len(metrics))
	for channelID, m := range metrics {
		cm := newChannelMetricsWithWindow(channelID, windowSize)
		svc.populateChannelMetrics(cm, m)
		restored[channelID] = cm
	}

	// Publish fully initialized channels without holding the map lock while
	// rebuilding their windows.
	svc.channelPerfMetricsLock.Lock()
	if svc.channelPerfMetrics == nil {
		svc.channelPerfMetrics = make(map[int]*channelMetrics)
	}
	for channelID, cm := range restored {
		svc.channelPerfMetrics[channelID] = cm
	}
	svc.channelPerfMetricsLock.Unlock()

	log.Info(ctx, "Loaded channel performance metrics from request executions",
		log.Int("count", len(metrics)),
	)

	return nil
}

// channelMetricsResult holds aggregated metrics for a single channel.
// Only includes fields needed for load balancing.
type channelMetricsResult struct {
	ChannelID           int        `json:"channel_id"`
	RequestCount        int64      `json:"request_count"`
	ConsecutiveFailures int64      `json:"consecutive_failures"`
	LastSelectedAt      *time.Time `json:"last_selected_at"`
	LastFailureAt       *time.Time `json:"last_failure_at"`
	Slots               []*timeSlotMetrics
}

// channelMetricExecution contains only the fields needed to restore metrics.
// Terminal updated_at is the persisted approximation of completion time.
type channelMetricExecution struct {
	ID          int
	ChannelID   int
	Status      requestexecution.Status
	SelectedAt  time.Time
	CompletedAt time.Time
}

// scanChannelMetricExecutions reads executions created within the window in
// bounded batches. Recent updates do not bring older executions into recovery.
func scanChannelMetricExecutions(ctx context.Context, client *ent.Client, since time.Time, statuses []requestexecution.Status, visit func(channelMetricExecution)) error {
	type queryResult struct {
		ID        int    `json:"id"`
		ChannelID int    `json:"channel_id"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	afterID := 0
	for {
		var results []queryResult
		err := client.RequestExecution.Query().
			Where(
				requestexecution.IDGT(afterID),
				requestexecution.CreatedAtGTE(since),
				requestexecution.ChannelIDNotNil(),
				requestexecution.StatusIn(statuses...),
			).
			Order(ent.Asc(requestexecution.FieldID)).
			Limit(channelMetricsLoadBatchSize).
			Select(requestexecution.FieldID, requestexecution.FieldChannelID,
				requestexecution.FieldStatus, requestexecution.FieldCreatedAt, requestexecution.FieldUpdatedAt).
			Scan(ctx, &results)
		if err != nil {
			return fmt.Errorf("failed to query channel metrics: %w", err)
		}
		for _, r := range results {
			selectedAt, err := parseDBTime(r.CreatedAt)
			if err != nil {
				log.Warn(ctx, "failed to parse execution created_at while loading channel metrics",
					log.Int("request_execution_id", r.ID), log.Cause(err))
				continue
			}
			completedAt, err := parseDBTime(r.UpdatedAt)
			if err != nil {
				log.Warn(ctx, "failed to parse execution updated_at while loading channel metrics",
					log.Int("request_execution_id", r.ID), log.Cause(err))
				continue
			}
			visit(channelMetricExecution{
				ID: r.ID, ChannelID: r.ChannelID, Status: requestexecution.Status(r.Status),
				SelectedAt: selectedAt, CompletedAt: completedAt,
			})
		}
		if len(results) < channelMetricsLoadBatchSize {
			return nil
		}
		afterID = results[len(results)-1].ID
	}
}

// loadAllChannelMetricsFromExecutions restores metrics from executions created
// within the window, using selection time for load and completion time for health.
// Two bounded passes avoid retaining every outcome merely to sort concurrent
// requests by their completion order.
func (svc *ChannelService) loadAllChannelMetricsFromExecutions(ctx context.Context, client *ent.Client, since time.Time) (map[int]*channelMetricsResult, error) {
	metricsMap := make(map[int]*channelMetricsResult)
	slotsByChannel := make(map[int]map[int64]*timeSlotMetrics)
	lastSuccess := make(map[int]channelMetricExecution)
	getSlot := func(channelID int, at time.Time) *timeSlotMetrics {
		ts := at.Unix()
		slot := slotsByChannel[channelID][ts]
		if slot == nil {
			slot = &timeSlotMetrics{timestamp: ts}
			slotsByChannel[channelID][ts] = slot
		}
		return slot
	}
	after := func(a, b channelMetricExecution) bool {
		return a.CompletedAt.After(b.CompletedAt) || a.CompletedAt.Equal(b.CompletedAt) && a.ID > b.ID
	}
	statuses := []requestexecution.Status{requestexecution.StatusCompleted, requestexecution.StatusFailed, requestexecution.StatusCanceled}
	err := scanChannelMetricExecutions(ctx, client, since, statuses, func(r channelMetricExecution) {
		m := metricsMap[r.ChannelID]
		if m == nil {
			m = &channelMetricsResult{ChannelID: r.ChannelID}
			metricsMap[r.ChannelID] = m
			slotsByChannel[r.ChannelID] = make(map[int64]*timeSlotMetrics)
		}
		if !r.SelectedAt.Before(since) {
			getSlot(r.ChannelID, r.SelectedAt).RequestCount++
			m.RequestCount++
			if m.LastSelectedAt == nil || r.SelectedAt.After(*m.LastSelectedAt) {
				m.LastSelectedAt = &r.SelectedAt
			}
		}
		if r.CompletedAt.Before(since) {
			return
		}
		switch r.Status {
		case requestexecution.StatusCompleted:
			getSlot(r.ChannelID, r.CompletedAt).SuccessCount++
			if previous, ok := lastSuccess[r.ChannelID]; !ok || after(r, previous) {
				lastSuccess[r.ChannelID] = r
			}
		case requestexecution.StatusFailed:
			getSlot(r.ChannelID, r.CompletedAt).FailureCount++
			if m.LastFailureAt == nil || r.CompletedAt.After(*m.LastFailureAt) {
				m.LastFailureAt = &r.CompletedAt
			}
		}
	})
	if err != nil {
		return nil, err
	}

	// IDs describe creation order, not completion order. Count only failures
	// completed after the latest success, even across pagination boundaries.
	err = scanChannelMetricExecutions(ctx, client, since, []requestexecution.Status{requestexecution.StatusFailed}, func(r channelMetricExecution) {
		m := metricsMap[r.ChannelID]
		if m == nil || r.CompletedAt.Before(since) {
			return
		}
		if success, ok := lastSuccess[r.ChannelID]; !ok || after(r, success) {
			m.ConsecutiveFailures++
		}
	})
	if err != nil {
		return nil, err
	}

	for channelID, slots := range slotsByChannel {
		m := metricsMap[channelID]
		m.Slots = make([]*timeSlotMetrics, 0, len(slots))
		for _, slot := range slots {
			m.Slots = append(m.Slots, slot)
		}
		slices.SortFunc(m.Slots, func(a, b *timeSlotMetrics) int {
			switch {
			case a.timestamp < b.timestamp:
				return -1
			case a.timestamp > b.timestamp:
				return 1
			default:
				return 0
			}
		})
	}
	return metricsMap, nil
}

// dbTimeFormats covers the time formats written by historical SQLite drivers:
//   - time.Time.String() format (current modernc driver default, e.g. "2026-08-10 13:22:10.251164681 +0000 UTC")
//   - the legacy fixed 9-digit fraction format (e.g. "2026-04-18 07:41:37.000000000 +0000 UTC",
//     matched by the same layout's .999999999)
//   - RFC3339 / RFC3339Nano as a fallback
var dbTimeFormats = []string{
	"2006-01-02 15:04:05.999999999 -0700 MST",
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
}

// parseDBTime parses a time stored as text in SQLite into time.Time.
func parseDBTime(value string) (time.Time, error) {
	for _, format := range dbTimeFormats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized time format: %q", value)
}

// populateChannelMetrics populates channelMetrics from the aggregated result.
// Only populates fields needed for load balancing.
func (svc *ChannelService) populateChannelMetrics(cm *channelMetrics, m *channelMetricsResult) {
	for _, slot := range m.Slots {
		target := cm.getOrCreateTimeSlot(slot.timestamp, time.Unix(slot.timestamp, 0), int64(cm.window.Capacity()-1))
		if target == nil {
			continue
		}
		*target = *slot
		cm.aggregatedMetrics.RequestCount += slot.RequestCount
		cm.aggregatedMetrics.SuccessCount += slot.SuccessCount
		cm.aggregatedMetrics.FailureCount += slot.FailureCount
	}

	cm.aggregatedMetrics.LastSelectedAt = m.LastSelectedAt
	cm.aggregatedMetrics.ConsecutiveFailures = m.ConsecutiveFailures

	if m.LastFailureAt != nil {
		cm.aggregatedMetrics.LastFailureAt = m.LastFailureAt
	}

}

// timeSlotMetrics holds metrics for a specific second.
type timeSlotMetrics struct {
	metricsRecord

	timestamp int64
}

type metricsRecord struct {
	RequestCount int64
	SuccessCount int64
	FailureCount int64

	// ConsecutiveFailures tracks the number of consecutive failures
	// Reset to 0 on success, incremented on failure
	ConsecutiveFailures int64
}

// AggregatedMetrics holds accumulated metrics for the flush period.
type AggregatedMetrics struct {
	metricsRecord

	LastSelectedAt *time.Time
	LastFailureAt  *time.Time

	// StreamingFirstTokenLatencyEWMA is the EWMA of first-token latency for streaming requests.
	StreamingFirstTokenLatencyEWMA float64
	// StreamingTokensPerSecondEWMA is the EWMA of completion throughput for streaming requests.
	StreamingTokensPerSecondEWMA float64
	// StreamingSampleCount tracks streaming samples recorded for latency-aware scoring.
	StreamingSampleCount int64
	// NonStreamingLatencyEWMA is the EWMA of total request latency for non-streaming requests.
	NonStreamingLatencyEWMA float64
	// NonStreamingSampleCount tracks non-streaming samples recorded for latency-aware scoring.
	NonStreamingSampleCount int64
}

func (m *AggregatedMetrics) Clone() *AggregatedMetrics {
	return &AggregatedMetrics{
		metricsRecord:                  m.metricsRecord,
		LastSelectedAt:                 m.LastSelectedAt,
		LastFailureAt:                  m.LastFailureAt,
		StreamingFirstTokenLatencyEWMA: m.StreamingFirstTokenLatencyEWMA,
		StreamingTokensPerSecondEWMA:   m.StreamingTokensPerSecondEWMA,
		StreamingSampleCount:           m.StreamingSampleCount,
		NonStreamingLatencyEWMA:        m.NonStreamingLatencyEWMA,
		NonStreamingSampleCount:        m.NonStreamingSampleCount,
	}
}

// newChannelMetrics creates a new channelMetrics instance.
func newChannelMetrics(channelID int) *channelMetrics {
	return newChannelMetricsWithWindow(channelID, defaultPerformanceWindowSize)
}

func newChannelMetricsWithWindow(channelID int, windowSize int64) *channelMetrics {
	if windowSize <= 0 {
		windowSize = defaultPerformanceWindowSize
	}

	cm := &channelMetrics{
		channelID: channelID,
		window:    ringbuffer.New[*timeSlotMetrics](int(windowSize) + 1),
		aggregatedMetrics: &AggregatedMetrics{
			metricsRecord: metricsRecord{},
		},
	}

	return cm
}

func (svc *ChannelService) performanceWindowSeconds() int64 {
	if svc.perfWindowSeconds > 0 {
		return svc.perfWindowSeconds
	}

	return defaultPerformanceWindowSize
}

func (svc *ChannelService) getChannelMetrics(channelID int) *channelMetrics {
	svc.channelPerfMetricsLock.RLock()
	defer svc.channelPerfMetricsLock.RUnlock()

	return svc.channelPerfMetrics[channelID]
}

func (svc *ChannelService) getOrCreateChannelMetrics(channelID int, windowSize int64) *channelMetrics {
	if cm := svc.getChannelMetrics(channelID); cm != nil {
		return cm
	}

	svc.channelPerfMetricsLock.Lock()
	defer svc.channelPerfMetricsLock.Unlock()

	// Another writer may have created the channel since the initial lookup.
	cm := svc.channelPerfMetrics[channelID]
	if cm == nil {
		cm = newChannelMetricsWithWindow(channelID, windowSize)
		svc.channelPerfMetrics[channelID] = cm
	}

	return cm
}

const latencyEWMAAlpha = 0.3

// recordSuccess records a successful request to the channel metrics.
func (cm *channelMetrics) recordSuccess(slot *timeSlotMetrics, perf *PerformanceRecord) {
	slot.SuccessCount++
	cm.aggregatedMetrics.SuccessCount++

	// Reset consecutive failures on success
	cm.aggregatedMetrics.ConsecutiveFailures = 0

	firstTokenLatencyMs, requestLatencyMs, tokensPerSecond := perf.Calculate()

	if perf.Stream && perf.FirstTokenTime != nil {
		firstTokenLatency := float64(firstTokenLatencyMs)
		if cm.aggregatedMetrics.StreamingSampleCount == 0 {
			cm.aggregatedMetrics.StreamingFirstTokenLatencyEWMA = firstTokenLatency
		} else {
			cm.aggregatedMetrics.StreamingFirstTokenLatencyEWMA = latencyEWMAAlpha*firstTokenLatency + (1-latencyEWMAAlpha)*cm.aggregatedMetrics.StreamingFirstTokenLatencyEWMA
		}

		if tokensPerSecond > 0 {
			if cm.aggregatedMetrics.StreamingSampleCount == 0 {
				cm.aggregatedMetrics.StreamingTokensPerSecondEWMA = tokensPerSecond
			} else {
				cm.aggregatedMetrics.StreamingTokensPerSecondEWMA = latencyEWMAAlpha*tokensPerSecond + (1-latencyEWMAAlpha)*cm.aggregatedMetrics.StreamingTokensPerSecondEWMA
			}
		}

		cm.aggregatedMetrics.StreamingSampleCount++

		return
	}

	latency := float64(requestLatencyMs)
	if cm.aggregatedMetrics.NonStreamingSampleCount == 0 {
		cm.aggregatedMetrics.NonStreamingLatencyEWMA = latency
	} else {
		cm.aggregatedMetrics.NonStreamingLatencyEWMA = latencyEWMAAlpha*latency + (1-latencyEWMAAlpha)*cm.aggregatedMetrics.NonStreamingLatencyEWMA
	}

	cm.aggregatedMetrics.NonStreamingSampleCount++
}

// recordFailure records a failed request to the channel metrics.
func (cm *channelMetrics) recordFailure(slot *timeSlotMetrics, perf *PerformanceRecord) {
	slot.FailureCount++
	cm.aggregatedMetrics.FailureCount++
	if cm.aggregatedMetrics.LastFailureAt == nil || cm.aggregatedMetrics.LastFailureAt.Before(perf.EndTime) {
		cm.aggregatedMetrics.LastFailureAt = &perf.EndTime
	}

	// Increment consecutive failures
	cm.aggregatedMetrics.ConsecutiveFailures++
}

// getOrCreateTimeSlot returns nil for timestamps that have already expired.
func (cm *channelMetrics) getOrCreateTimeSlot(ts int64, endTime time.Time, windowSize int64) *timeSlotMetrics {
	cm.cleanupExpiredSlots(endTime.Add(-time.Duration(windowSize) * time.Second))
	if ts < cm.windowCutoff {
		return nil
	}

	if slot, ok := cm.window.Get(ts); ok {
		return slot
	}

	slot := &timeSlotMetrics{
		timestamp:     ts,
		metricsRecord: metricsRecord{},
	}
	// The monotonic cutoff leaves at most windowSize+1 distinct seconds,
	// including both endpoints, so insertion cannot silently evict a slot.
	if ts >= cm.latestSlot {
		cm.window.Push(ts, slot)
		cm.latestSlot = ts
	} else {
		// The async outcome worker can lag behind selection-time writes. Keep
		// chronological order so cleanup cannot miss an expired slot behind a
		// newer one. Only late insertions need to rebuild this bounded buffer.
		items := cm.window.GetAll()
		cm.window.Clear()
		inserted := false
		for _, item := range items {
			if !inserted && ts < item.Timestamp {
				cm.window.Push(ts, slot)
				inserted = true
			}
			cm.window.Push(item.Timestamp, item.Value)
		}
		if !inserted {
			cm.window.Push(ts, slot)
		}
	}

	return slot
}

// RecordPerformance records performance metrics to the in-memory cache.
func (svc *ChannelService) RecordPerformance(ctx context.Context, perf *PerformanceRecord) {
	if perf == nil || !perf.IsValid() {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Error(ctx, "panic in record performance", log.Any("panic", r))
		}
	}()

	if perf.Success {
		svc.clearAutoDisableCountsOnSuccess(perf)

		// Let the round_robin_success strategy count this success and move
		// its cursor once the key has served its share of successful requests.
		svc.onAPIKeySuccess(perf.ChannelID, perf.APIKey)
	} else if !perf.Canceled {
		svc.evaluateAutoDisableForFailure(ctx, perf)
	}

	windowSize := svc.performanceWindowSeconds()

	cm := svc.getOrCreateChannelMetrics(perf.ChannelID, windowSize)
	cm.mu.Lock()
	defer cm.mu.Unlock()

	ts := perf.EndTime.Unix()

	// Get or create time slot for this second
	slot := cm.getOrCreateTimeSlot(ts, perf.EndTime, windowSize)
	if slot == nil {
		return
	}

	// Record success or failure
	if perf.Success {
		cm.recordSuccess(slot, perf)
	} else if !perf.Canceled {
		cm.recordFailure(slot, perf)
	}

	if log.DebugEnabled(ctx) {
		keySuffix := ""
		if len(perf.APIKey) >= 4 {
			keySuffix = perf.APIKey[len(perf.APIKey)-4:]
		}
		log.Debug(ctx, "recorded performance metrics",
			log.Int("channel_id", perf.ChannelID),
			log.String("key_suffix", keySuffix), // Only log last 4 chars for security
			log.Bool("success", perf.Success),
			log.Any("error_code", perf.ResponseStatusCode),
		)
	}
}

// AsyncRecordPerformance records performance metrics to in-memory cache asynchronously.
func (svc *ChannelService) AsyncRecordPerformance(ctx context.Context, perr *PerformanceRecord) {
	svc.perfCh <- perr
}

// cleanupExpiredSlots removes time slots older than the cutoff time.
// This is now O(k) where k is the number of items to remove, instead of O(n) for the entire map.
func (cm *channelMetrics) cleanupExpiredSlots(cutoff time.Time) {
	cutoffTs := max(cutoff.Unix(), cm.windowCutoff)
	cm.windowCutoff = cutoffTs

	// Collect metrics to subtract before cleanup
	var metricsToRemove []*timeSlotMetrics

	cm.window.Range(func(ts int64, metrics *timeSlotMetrics) bool {
		if ts < cutoffTs {
			metricsToRemove = append(metricsToRemove, metrics)
			return true
		}
		// Since ringbuffer is ordered by timestamp, we can stop here
		return false
	})

	// Subtract removed metrics from aggregated metrics
	for _, metrics := range metricsToRemove {
		cm.aggregatedMetrics.RequestCount -= metrics.RequestCount
		cm.aggregatedMetrics.SuccessCount -= metrics.SuccessCount
		cm.aggregatedMetrics.FailureCount -= metrics.FailureCount
	}

	// Cleanup old entries from ringbuffer
	cm.window.CleanupBefore(cutoffTs)
}

// GetChannelMetrics returns performance metrics for the channel.
// If in-memory metrics are not available, it returns an empty snapshot.
func (svc *ChannelService) GetChannelMetrics(ctx context.Context, channelID int) (*AggregatedMetrics, error) {
	cm := svc.getChannelMetrics(channelID)
	if cm == nil {
		return &AggregatedMetrics{}, nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Idle channels have no writes to expire their load. Clean on reads too,
	// using only this channel's lock so other channels can proceed.
	windowSize := svc.performanceWindowSeconds()
	cm.cleanupExpiredSlots(time.Now().Add(-time.Duration(windowSize) * time.Second))

	// Return a full copy of the aggregated metrics to avoid concurrent modification
	// while preserving all load-balancing signals, including latency EWMA.
	return cm.aggregatedMetrics.Clone(), nil
}

// IncrementChannelSelection increments the request count for a channel at selection time.
// This is called when a channel is selected by the load balancer to ensure immediate
// impact on subsequent selections, preventing the same channel from being selected
// repeatedly during burst/concurrent requests.
func (svc *ChannelService) IncrementChannelSelection(channelID int) {
	windowSize := svc.performanceWindowSeconds()
	cm := svc.getOrCreateChannelMetrics(channelID, windowSize)
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	slot := cm.getOrCreateTimeSlot(now.Unix(), now, windowSize)
	if slot == nil {
		return
	}
	oldCount := cm.aggregatedMetrics.RequestCount

	// Record the selection in its own time slot so the request count expires from
	// the same selection-time window used by startup recovery. Completion metrics
	// may land in a later slot without extending the selection's lifetime.
	slot.RequestCount++
	cm.aggregatedMetrics.RequestCount++

	// Update last activity time to current time
	if cm.aggregatedMetrics.LastSelectedAt == nil || cm.aggregatedMetrics.LastSelectedAt.Before(now) {
		cm.aggregatedMetrics.LastSelectedAt = &now
	}

	// Log debug message if enabled
	if log.DebugEnabled(context.Background()) {
		log.Debug(context.Background(), "IncrementChannelSelection: incremented request count",
			log.Int("channel_id", channelID),
			log.Int64("old_count", oldCount),
			log.Int64("new_count", cm.aggregatedMetrics.RequestCount),
		)
	}
}

func deriveErrorMessage(errorCode int) string {
	if text := http.StatusText(errorCode); text != "" {
		return text
	}

	return fmt.Sprintf("Error %d", errorCode)
}

// PerformanceRecord contains performance metrics collected during request processing.
type PerformanceRecord struct {
	ChannelID          int
	APIKey             string // API key used for the request (sensitive, do not log full value)
	StartTime          time.Time
	FirstTokenTime     *time.Time
	ReasoningStartTime *time.Time
	ReasoningEndTime   *time.Time
	EndTime            time.Time
	Stream             bool
	Success            bool
	Canceled           bool
	RequestCompleted   bool

	// If response status code is 0, it means the request is successful.
	ResponseStatusCode int
	ErrorMessage       string
	CompletionTokens   int64
}

// Calculate calculates performance metrics from collected data.
// It enforces minimum latency to prevent extreme TPS calculations.
func (m *PerformanceRecord) Calculate() (firstTokenLatencyMs int64, requestLatencyMs int64, tokensPerSecond float64) {
	endTime := m.EndTime
	if endTime.IsZero() {
		// Streaming metrics can be calculated while the stream is being
		// finalized, before a terminal event has marked the record complete.
		endTime = time.Now()
	}

	totalDuration := endTime.Sub(m.StartTime)
	requestLatencyMs = totalDuration.Milliseconds()

	// Calculate first token latency
	if m.Stream && m.FirstTokenTime != nil {
		firstTokenLatency := m.FirstTokenTime.Sub(m.StartTime)
		firstTokenLatencyMs = firstTokenLatency.Milliseconds()
	}

	// Enforce minimum latency to prevent extreme TPS calculations
	requestLatencyMs = ClampLatency(requestLatencyMs)
	firstTokenLatencyMs = ClampLatency(firstTokenLatencyMs)

	if m.CompletionTokens > 0 {
		effectiveLatencyMs := requestLatencyMs
		if m.Stream && m.FirstTokenTime != nil {
			effectiveLatencyMs = requestLatencyMs - firstTokenLatencyMs
			effectiveLatencyMs = ClampLatency(effectiveLatencyMs)
		}

		tokensPerSecond = float64(m.CompletionTokens) / (float64(effectiveLatencyMs) / 1000.0)
	}

	return firstTokenLatencyMs, requestLatencyMs, tokensPerSecond
}

// CalculateReasoningDurationMs calculates the reasoning duration.
func (m *PerformanceRecord) CalculateReasoningDurationMs() int64 {
	if m.ReasoningStartTime == nil || m.ReasoningEndTime == nil {
		return 0
	}
	duration := m.ReasoningEndTime.Sub(*m.ReasoningStartTime)
	return duration.Milliseconds()
}

// MarkSuccess marks the request as completed.
func (m *PerformanceRecord) MarkSuccess() {
	m.Success = true
	m.RequestCompleted = true
	m.EndTime = time.Now()
}

// MarkFirstToken marks the first token time.
func (m *PerformanceRecord) MarkFirstToken() {
	if m.FirstTokenTime == nil {
		now := time.Now()
		m.FirstTokenTime = &now
	}
}

// MarkReasoningStart marks the reasoning start time.
func (m *PerformanceRecord) MarkReasoningStart() {
	if m.ReasoningStartTime == nil {
		now := time.Now()
		m.ReasoningStartTime = &now
	}
}

// MarkReasoningEnd marks the reasoning end time.
func (m *PerformanceRecord) MarkReasoningEnd() {
	if m.ReasoningEndTime == nil {
		now := time.Now()
		m.ReasoningEndTime = &now
	}
}

// MarkFailed marks the request as failed.
func (m *PerformanceRecord) MarkFailed(errorCode int) {
	m.Success = false
	m.ResponseStatusCode = errorCode
	m.RequestCompleted = true
	m.EndTime = time.Now()
}

// MarkFailedWithMessage records the provider error text used by keyword rules.
func (m *PerformanceRecord) MarkFailedWithMessage(errorCode int, errorMessage string) {
	m.MarkFailed(errorCode)
	m.ErrorMessage = errorMessage
}

// MarkCanceled marks the request as canceled by context.
func (m *PerformanceRecord) MarkCanceled() {
	m.Success = false
	m.Canceled = true
	m.RequestCompleted = true
	m.EndTime = time.Now()
}

// IsValid checks if metrics are valid for recording.
func (m *PerformanceRecord) IsValid() bool {
	return m.ChannelID > 0 && m.RequestCompleted
}
