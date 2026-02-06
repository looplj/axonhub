package biz

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/ringbuffer"
	"github.com/looplj/axonhub/internal/pkg/xtime"
)

const (
	// defaultPerformanceWindowSize is the default size of the sliding window in seconds (10 minutes).
	defaultPerformanceWindowSize = 600
)

// channelMetrics holds the performance metrics for a channel in memory.
type channelMetrics struct {
	channelID int

	// sliding window of metrics for the last N minutes using ring buffer for O(1) cleanup
	window *ringbuffer.RingBuffer[*timeSlotMetrics]

	// aggregatedMetrics holds accumulated metrics for the flush period
	aggregatedMetrics *AggregatedMetrics
}

// LoadChannelPerformances loads channel performance metrics from request_execution table.
// It queries the last 6 hours of data to initialize in-memory metrics for load balancing.
// Uses a single GROUP BY query to fetch all channel metrics at once for better performance.
func (svc *ChannelService) LoadChannelPerformances(ctx context.Context) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := svc.entFromContext(ctx)

	// Query last 6 hours of request execution data
	since := xtime.UTCNow().Add(-6 * time.Hour)

	// Fetch all channel metrics in a single GROUP BY query
	metrics, err := svc.loadAllChannelMetricsFromExecutions(ctx, client, since)
	if err != nil {
		return fmt.Errorf("failed to load channel metrics: %w", err)
	}

	if len(metrics) == 0 {
		log.Info(ctx, "No request execution data found in the last 6 hours")
		return nil
	}

	svc.channelPerfMetricsLock.Lock()
	defer svc.channelPerfMetricsLock.Unlock()

	if svc.channelPerfMetrics == nil {
		svc.channelPerfMetrics = make(map[int]*channelMetrics)
	}

	for channelID, m := range metrics {
		cm := newChannelMetrics(channelID)
		svc.populateChannelMetrics(cm, m)
		svc.channelPerfMetrics[channelID] = cm
	}

	log.Info(ctx, "Loaded channel performance metrics from request executions",
		log.Int("count", len(metrics)),
	)

	return nil
}

// channelMetricsResult holds aggregated metrics for a single channel.
// Only includes fields needed for load balancing.
type channelMetricsResult struct {
	ChannelID     int        `json:"channel_id"`
	RequestCount  int64      `json:"request_count"`
	LastFailureAt *time.Time `json:"last_failure_at"`
}

// loadAllChannelMetricsFromExecutions loads metrics for all channels using a single GROUP BY query.
// Uses raw SQL via Modify to get request count and last failure time in one query.
func (svc *ChannelService) loadAllChannelMetricsFromExecutions(ctx context.Context, client *ent.Client, since time.Time) (map[int]*channelMetricsResult, error) {
	// Single query to get request count and last failure time for all channels
	type queryResult struct {
		ChannelID     int       `json:"channel_id"`
		RequestCount  int64     `json:"request_count"`
		LastFailureAt time.Time `json:"last_failure_at"`
	}

	var results []queryResult

	err := client.RequestExecution.Query().
		Where(
			requestexecution.CreatedAtGTE(since),
			requestexecution.ChannelIDNotNil(),
			requestexecution.StatusNotIn(requestexecution.StatusPending, requestexecution.StatusProcessing),
		).
		Modify(func(s *sql.Selector) {
			// Use a subquery or join to get last failure time per channel
			// For simplicity, we use MAX(CASE WHEN status = 'failed' THEN created_at END) to get last failure
			s.Select(
				s.C(requestexecution.FieldChannelID),
				sql.As(sql.Count("*"), "request_count"),
				sql.As(fmt.Sprintf("MAX(CASE WHEN status = '%s' THEN %s END)", requestexecution.StatusFailed, s.C(requestexecution.FieldCreatedAt)), "last_failure_at"),
			).
				GroupBy(s.C(requestexecution.FieldChannelID))
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel metrics: %w", err)
	}

	metricsMap := make(map[int]*channelMetricsResult)

	for _, r := range results {
		m := &channelMetricsResult{
			ChannelID:    r.ChannelID,
			RequestCount: r.RequestCount,
		}
		if !r.LastFailureAt.IsZero() {
			m.LastFailureAt = &r.LastFailureAt
		}

		metricsMap[r.ChannelID] = m
	}

	return metricsMap, nil
}

// populateChannelMetrics populates channelMetrics from the aggregated result.
// Only populates fields needed for load balancing.
func (svc *ChannelService) populateChannelMetrics(cm *channelMetrics, m *channelMetricsResult) {
	// Populate aggregated metrics - only fields needed for load balancing
	cm.aggregatedMetrics.RequestCount = m.RequestCount

	if m.LastFailureAt != nil {
		cm.aggregatedMetrics.LastFailureAt = m.LastFailureAt
	}

	// Note: ConsecutiveFailures is not loaded from historical data.
	// It will be tracked in real-time as requests are processed.
}

// InitializeChannelPerformance initializes in-memory performance metrics for a newly created channel.
// Note: Performance metrics are no longer persisted to database, only kept in memory.
func (svc *ChannelService) InitializeChannelPerformance(ctx context.Context, channelID int) error {
	log.Info(ctx, "initializing in-memory channel performance metrics", log.Int("channel_id", channelID))

	svc.channelPerfMetricsLock.Lock()
	defer svc.channelPerfMetricsLock.Unlock()

	if svc.channelPerfMetrics == nil {
		svc.channelPerfMetrics = make(map[int]*channelMetrics)
	}

	if _, exists := svc.channelPerfMetrics[channelID]; !exists {
		svc.channelPerfMetrics[channelID] = newChannelMetrics(channelID)
	}

	return nil
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
}

func (m *AggregatedMetrics) Clone() *AggregatedMetrics {
	return &AggregatedMetrics{
		metricsRecord:  m.metricsRecord,
		LastSelectedAt: m.LastSelectedAt,
		LastFailureAt:  m.LastFailureAt,
	}
}

// newChannelMetrics creates a new channelMetrics instance.
func newChannelMetrics(channelID int) *channelMetrics {
	cm := &channelMetrics{
		channelID: channelID,
		window:    ringbuffer.New[*timeSlotMetrics](defaultPerformanceWindowSize),
		aggregatedMetrics: &AggregatedMetrics{
			metricsRecord: metricsRecord{},
		},
	}

	return cm
}

// recordSuccess records a successful request to the channel metrics.
func (cm *channelMetrics) recordSuccess(slot *timeSlotMetrics, perf *PerformanceRecord) {
	slot.SuccessCount++
	cm.aggregatedMetrics.SuccessCount++
	cm.aggregatedMetrics.LastSelectedAt = &perf.EndTime

	// Reset consecutive failures on success
	cm.aggregatedMetrics.ConsecutiveFailures = 0
}

// recordFailure records a failed request to the channel metrics.
func (cm *channelMetrics) recordFailure(slot *timeSlotMetrics, perf *PerformanceRecord) {
	slot.FailureCount++
	cm.aggregatedMetrics.FailureCount++
	cm.aggregatedMetrics.LastFailureAt = &perf.EndTime

	// Increment consecutive failures
	cm.aggregatedMetrics.ConsecutiveFailures++
}

// getOrCreateTimeSlot gets or creates a time slot for the given timestamp.
func (cm *channelMetrics) getOrCreateTimeSlot(ts int64, endTime time.Time, windowSize int64) *timeSlotMetrics {
	if slot, ok := cm.window.Get(ts); ok {
		return slot
	}

	// Clean old entries to prevent memory leak
	if cm.window.Len() >= int(windowSize) {
		cm.cleanupExpiredSlots(endTime.Add(-time.Duration(windowSize) * time.Second))
	}

	slot := &timeSlotMetrics{
		timestamp:     ts,
		metricsRecord: metricsRecord{},
	}
	cm.window.Push(ts, slot)

	return slot
}

// RecordPerformance records performance metrics to in-memory cache.
// This function is not thread-safe.
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
		svc.channelErrorCountsLock.Lock()
		delete(svc.channelErrorCounts, perf.ChannelID)
		svc.channelErrorCountsLock.Unlock()

		// Also clear API key error counts on success
		if perf.APIKey != "" {
			svc.apiKeyErrorCountsLock.Lock()

			if svc.apiKeyErrorCounts[perf.ChannelID] != nil {
				delete(svc.apiKeyErrorCounts[perf.ChannelID], perf.APIKey)
			}

			svc.apiKeyErrorCountsLock.Unlock()
		}
	} else if !perf.Canceled {
		policy := svc.SystemService.RetryPolicyOrDefault(ctx)

		if policy.AutoDisableChannel.Enabled {
			// Check API key error first if available.
			if perf.APIKey != "" {
				if svc.checkAndHandleAPIKeyError(ctx, perf, policy) {
					return
				}
			} else {
				if svc.checkAndHandleChannelError(ctx, perf, policy) {
					return
				}
			}
		}
	}

	// Get or create channel metrics
	svc.channelPerfMetricsLock.Lock()

	cm, exists := svc.channelPerfMetrics[perf.ChannelID]
	if !exists {
		cm = newChannelMetrics(perf.ChannelID)
		svc.channelPerfMetrics[perf.ChannelID] = cm
	}

	svc.channelPerfMetricsLock.Unlock()

	// Determine window size
	var windowSize int64 = defaultPerformanceWindowSize
	if svc.perfWindowSeconds > 0 {
		windowSize = svc.perfWindowSeconds
	}

	ts := perf.EndTime.Unix()

	// Get or create time slot for this second
	slot := cm.getOrCreateTimeSlot(ts, perf.EndTime, windowSize)

	// Update slot request count for sliding window metrics.
	// Note: aggregatedMetrics.RequestCount is NOT incremented here because it was already
	// incremented in IncrementChannelSelection() at selection time for immediate load balancing effect.
	// The cleanup logic will subtract slot.RequestCount from aggregatedMetrics when the slot expires.
	if !perf.Canceled {
		slot.RequestCount++
	} else {
		// If canceled, decrement the aggregated request count that was incremented at selection time.
		// We don't increment slot.RequestCount, so it won't be subtracted later.
		svc.channelPerfMetricsLock.Lock()

		cm.aggregatedMetrics.RequestCount--

		svc.channelPerfMetricsLock.Unlock()
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
			log.Any("error_code", perf.ErrorStatusCode),
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
	cutoffTs := cutoff.Unix()

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

// startPerformanceProcess starts the background goroutine to flush metrics to database.
func (svc *ChannelService) startPerformanceProcess() {
	for perf := range svc.perfCh {
		svc.RecordPerformance(context.Background(), perf)
	}
}

// GetChannelMetrics returns performance metrics for the channel.
// If in-memory metrics are not available (e.g., after restart), it falls back to database values.
func (svc *ChannelService) GetChannelMetrics(ctx context.Context, channelID int) (*AggregatedMetrics, error) {
	svc.channelPerfMetricsLock.RLock()
	cm, exists := svc.channelPerfMetrics[channelID]
	svc.channelPerfMetricsLock.RUnlock()

	if !exists {
		return &AggregatedMetrics{}, nil
	}

	// Return a copy of the aggregated metrics to avoid concurrent modification
	return &AggregatedMetrics{
		metricsRecord:  cm.aggregatedMetrics.metricsRecord,
		LastSelectedAt: cm.aggregatedMetrics.LastSelectedAt,
		LastFailureAt:  cm.aggregatedMetrics.LastFailureAt,
	}, nil
}

// IncrementChannelSelection increments the request count for a channel at selection time.
// This is called when a channel is selected by the load balancer to ensure immediate
// impact on subsequent selections, preventing the same channel from being selected
// repeatedly during burst/concurrent requests.
func (svc *ChannelService) IncrementChannelSelection(channelID int) {
	svc.channelPerfMetricsLock.Lock()
	defer svc.channelPerfMetricsLock.Unlock()

	cm, exists := svc.channelPerfMetrics[channelID]
	if !exists {
		cm = newChannelMetrics(channelID)
		svc.channelPerfMetrics[channelID] = cm
	}

	oldCount := cm.aggregatedMetrics.RequestCount

	// Increment request count immediately to affect subsequent load balancing decisions
	cm.aggregatedMetrics.RequestCount++

	// Update last activity time to current time
	now := time.Now()
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
	return http.StatusText(errorCode)
}

// PerformanceRecord contains performance metrics collected during request processing.
type PerformanceRecord struct {
	ChannelID        int
	APIKey           string // API key used for the request (sensitive, do not log full value)
	StartTime        time.Time
	FirstTokenTime   *time.Time
	EndTime          time.Time
	Stream           bool
	Success          bool
	Canceled         bool
	RequestCompleted bool

	// If error status code is 0, it means the request is successful.
	ErrorStatusCode int
}

// Calculate calculates performance metrics from collected data.
func (m *PerformanceRecord) Calculate() (firstTokenLatencyMs int64, requestLatencyMs int64, tokensPerSecond float64) {
	totalDuration := m.EndTime.Sub(m.StartTime)
	requestLatencyMs = totalDuration.Milliseconds()

	// Calculate first token latency
	if m.Stream && m.FirstTokenTime != nil {
		firstTokenLatency := m.FirstTokenTime.Sub(m.StartTime)
		firstTokenLatencyMs = firstTokenLatency.Milliseconds()
	}

	return firstTokenLatencyMs, requestLatencyMs, tokensPerSecond
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

// MarkFailed marks the request as failed.
func (m *PerformanceRecord) MarkFailed(errorCode int) {
	m.Success = false
	m.ErrorStatusCode = errorCode
	m.RequestCompleted = true
	m.EndTime = time.Now()
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
