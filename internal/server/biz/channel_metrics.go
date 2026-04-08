package biz

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"entgo.io/ent/dialect"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/ringbuffer"
	"github.com/looplj/axonhub/internal/pkg/xtime"
)

const (
	// defaultPerformanceWindowSize is the default size of the sliding window in seconds (10 minutes).
	defaultPerformanceWindowSize = 600

	// MinLatencyMs is the minimum latency value (10ms) used for tokens/second calculations.
	// This matches the frontend standard MINIMUM_LATENCY_MS_FOR_CACHE_HITS.
	MinLatencyMs = 10

	// metricsBatchSize is the number of records to fetch per batch when loading channel metrics.
	// This prevents memory issues when loading large datasets (>10k records).
	// Note: Batch loading is not yet implemented; this constant is reserved for future use.
	metricsBatchSize = 1000

	// modelMetricsIdleTimeout is the duration after which unused model metrics are cleaned up.
	// This prevents memory leaks from accumulating unique model IDs over time.
	modelMetricsIdleTimeout = 24 * time.Hour
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

	// mu protects all fields from concurrent access
	mu sync.RWMutex

	// lastAccessTime tracks when this metrics was last accessed for cleanup
	lastAccessTime time.Time

	// sliding window of metrics for the last N minutes using ring buffer for O(1) cleanup
	window *ringbuffer.RingBuffer[*timeSlotMetrics]

	// aggregatedMetrics holds accumulated metrics for the flush period
	aggregatedMetrics *AggregatedMetrics
}

// loadChannelPerformances loads channel performance metrics from request_execution table.
// It queries historical data based on windowDuration to initialize in-memory metrics for load balancing.
// Uses a single GROUP BY query to fetch all channel-model metrics at once for better performance.
func (svc *ChannelService) loadChannelPerformances(ctx context.Context, windowDuration time.Duration) error {
	client := svc.entFromContext(ctx)

	// Query request execution data based on configurable window duration
	since := xtime.UTCNow().Add(-windowDuration)

	// Fetch all channel-model metrics in a single GROUP BY query
	metrics, err := svc.loadAllChannelMetricsFromExecutions(ctx, client, since)
	if err != nil {
		return fmt.Errorf("failed to load channel metrics: %w", err)
	}

	if len(metrics) == 0 {
		log.Info(ctx, "No request execution data found in the historical window")
		return nil
	}

	svc.channelPerfMetricsLock.Lock()
	defer svc.channelPerfMetricsLock.Unlock()

	if svc.channelPerfMetrics == nil {
		svc.channelPerfMetrics = make(map[int]map[string]*channelMetrics)
	}

	for key, m := range metrics {
		channelID, modelID := key.ChannelID, key.ModelID
		cm := svc.getOrCreateModelMetricsInternal(channelID, modelID)
		svc.populateChannelMetrics(cm, m)
	}

	log.Info(ctx, "Loaded channel performance metrics from request executions",
		log.Int("count", len(metrics)),
	)

	// Clean up any idle model metrics to prevent memory leaks
	svc.cleanupIdleModelMetrics()

	return nil
}

// cleanupIdleModelMetrics removes model metrics that haven't been accessed recently.
// This prevents memory leaks from accumulating unique model IDs over time.
// Should be called periodically (e.g., after loading historical data).
// MUST be called with channelPerfMetricsLock already held.
func (svc *ChannelService) cleanupIdleModelMetrics() {
	if svc.channelPerfMetrics == nil {
		return
	}

	cutoff := time.Now().Add(-modelMetricsIdleTimeout)
	totalRemoved := 0

	for channelID, channelMap := range svc.channelPerfMetrics {
		for modelID, cm := range channelMap {
			cm.mu.RLock()
			idle := cm.lastAccessTime.Before(cutoff)
			cm.mu.RUnlock()

			if idle {
				delete(channelMap, modelID)

				totalRemoved++
			}
		}

		// Remove channel entry if no models left
		if len(channelMap) == 0 {
			delete(svc.channelPerfMetrics, channelID)
		}
	}

	if totalRemoved > 0 {
		log.Info(context.Background(), "cleaned up idle model metrics", log.Int("removed", totalRemoved))
	}
}

// getOrCreateModelMetrics initializes the nested map structure for channel-model metrics.
// Uses empty string "" as default model key when modelID is empty.
func (svc *ChannelService) getOrCreateModelMetrics(channelID int, modelID string) *channelMetrics {
	svc.channelPerfMetricsLock.Lock()
	defer svc.channelPerfMetricsLock.Unlock()

	return svc.getOrCreateModelMetricsInternal(channelID, modelID)
}

// getOrCreateModelMetricsInternal is the unlocked version for internal use.
// Must be called with channelPerfMetricsLock already held.
func (svc *ChannelService) getOrCreateModelMetricsInternal(channelID int, modelID string) *channelMetrics {
	if svc.channelPerfMetrics == nil {
		svc.channelPerfMetrics = make(map[int]map[string]*channelMetrics)
	}

	channelMap, exists := svc.channelPerfMetrics[channelID]
	if !exists {
		channelMap = make(map[string]*channelMetrics)
		svc.channelPerfMetrics[channelID] = channelMap
	}

	// Use modelID directly as the key (empty string is valid for default)
	cm, exists := channelMap[modelID]
	if !exists {
		cm = newChannelMetrics(channelID)
		channelMap[modelID] = cm
	} else {
		// Update last access time for existing metrics
		cm.mu.Lock()
		cm.lastAccessTime = time.Now()
		cm.mu.Unlock()
	}

	return cm
}

// channelMetricsResult holds aggregated metrics for a single channel-model combination.
// Only includes fields needed for load balancing.
type channelMetricsResult struct {
	ChannelID            int        `json:"channel_id"`
	ModelID              string     `json:"model_id"`
	RequestCount         int64      `json:"request_count"`
	SuccessCount         int64      `json:"success_count"`
	LastFailureAt        *time.Time `json:"last_failure_at"`
	AvgFirstTokenLatency *float64   `json:"avg_first_token_latency"` // Average TTFT in milliseconds
	AvgThroughput        *float64   `json:"avg_throughput"`          // Average tokens per second
}

// modelKey is a composite key for channel-model metrics lookup.
type modelKey struct {
	ChannelID int
	ModelID   string
}

// loadAllChannelMetricsFromExecutions loads metrics for all channels.
// Uses a single GROUP BY query to fetch all channel-model metrics at once for accurate aggregation.
func (svc *ChannelService) loadAllChannelMetricsFromExecutions(ctx context.Context, client *ent.Client, since time.Time) (map[modelKey]*channelMetricsResult, error) {
	return svc.loadAllChannelMetricsSingleQuery(ctx, client, since)
}

// loadAllChannelMetricsSingleQuery loads metrics using a single GROUP BY query.
// Used for small datasets (<= metricsBatchSize records).
// Uses raw SQL to join request_executions with usage_logs for throughput calculation.
func (svc *ChannelService) loadAllChannelMetricsSingleQuery(ctx context.Context, client *ent.Client, since time.Time) (map[modelKey]*channelMetricsResult, error) {
	type queryResult struct {
		ChannelID              int        `json:"channel_id"`
		ModelID                string     `json:"model_id"`
		RequestCount           int64      `json:"request_count"`
		SuccessCount           int64      `json:"success_count"`
		LastFailureAt          *time.Time `json:"last_failure_at"`
		TotalFirstTokenLatency int64      `json:"total_first_token_latency"`
		TotalEffectiveLatency  int64      `json:"total_effective_latency"`
		TotalTokens            int64      `json:"total_tokens"`
		StreamingRequestCount  int64      `json:"streaming_request_count"`
	}

	// Get the underlying SQL driver from the ent client
	dbDriver := client.Driver()

	sqlDB, ok := dbDriver.(*entsql.Driver)
	if !ok {
		return nil, fmt.Errorf("failed to get underlying SQL driver")
	}

	// Detect dialect to use appropriate placeholder syntax
	dialectName := sqlDB.Dialect()
	useDollarPlaceholders := dialectName == dialect.Postgres

	// Build placeholder for the since parameter
	placeholder := "?"
	if useDollarPlaceholders {
		placeholder = "$1"
	}

	query := fmt.Sprintf(`
SELECT
    se.channel_id,
    COALESCE(se.model_id, '') as model_id,
    COUNT(*) as request_count,
    SUM(CASE WHEN se.status = 'completed' THEN 1 ELSE 0 END) as success_count,
    MAX(CASE WHEN se.status = 'failed' THEN se.created_at END) as last_failure_at,
    COALESCE(SUM(CASE WHEN se.status = 'completed' AND se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
        THEN se.metrics_first_token_latency_ms ELSE 0 END), 0) as total_first_token_latency,
    COALESCE(SUM(CASE WHEN se.status = 'completed' THEN
        CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
             THEN CASE WHEN se.metrics_first_token_latency_ms >= se.metrics_latency_ms
                  THEN 0
                  ELSE se.metrics_latency_ms - se.metrics_first_token_latency_ms END
             ELSE se.metrics_latency_ms END
        ELSE 0 END), 0) as total_effective_latency,
    COALESCE(SUM(CASE WHEN se.status = 'completed' THEN ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0) ELSE 0 END), 0) as total_tokens,
    COALESCE(SUM(CASE WHEN se.status = 'completed' AND se.stream AND se.metrics_first_token_latency_ms IS NOT NULL THEN 1 ELSE 0 END), 0) as streaming_request_count
FROM request_executions se
LEFT JOIN usage_logs ul ON se.request_id = ul.request_id
WHERE se.channel_id IS NOT NULL
    AND se.status NOT IN ('pending', 'processing')
    AND se.created_at >= %s
GROUP BY se.channel_id, se.model_id
`, placeholder)

	rows, err := sqlDB.DB().QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel metrics: %w", err)
	}

	defer func() { _ = rows.Close() }()

	metricsMap := make(map[modelKey]*channelMetricsResult)

	for rows.Next() {
		var r queryResult
		if err := rows.Scan(
			&r.ChannelID,
			&r.ModelID,
			&r.RequestCount,
			&r.SuccessCount,
			&r.LastFailureAt,
			&r.TotalFirstTokenLatency,
			&r.TotalEffectiveLatency,
			&r.TotalTokens,
			&r.StreamingRequestCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan channel metrics: %w", err)
		}

		m := &channelMetricsResult{
			ChannelID:    r.ChannelID,
			ModelID:      r.ModelID,
			RequestCount: r.RequestCount,
			SuccessCount: r.SuccessCount,
		}
		if r.LastFailureAt != nil {
			m.LastFailureAt = r.LastFailureAt
		}

		// Calculate average TTFT from streaming requests that have the metric
		if r.StreamingRequestCount > 0 && r.TotalFirstTokenLatency > 0 {
			avgTTFT := float64(r.TotalFirstTokenLatency) / float64(r.StreamingRequestCount)
			m.AvgFirstTokenLatency = &avgTTFT
		}

		// Calculate average throughput: total_tokens / effective_latency (in seconds)
		if r.TotalTokens > 0 && r.TotalEffectiveLatency > 0 {
			tps := float64(r.TotalTokens) / (float64(r.TotalEffectiveLatency) / 1000.0)
			m.AvgThroughput = &tps
		}

		key := modelKey{ChannelID: r.ChannelID, ModelID: r.ModelID}
		metricsMap[key] = m
	}

	return metricsMap, nil
}

// populateChannelMetrics populates channelMetrics from the aggregated result.
// Only populates fields needed for load balancing.
// Sets SuccessCount and cumulative totals so historical data integrates with the sliding window logic.
func (svc *ChannelService) populateChannelMetrics(cm *channelMetrics, m *channelMetricsResult) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Populate aggregated metrics - only fields needed for load balancing
	cm.aggregatedMetrics.RequestCount = m.RequestCount
	cm.aggregatedMetrics.SuccessCount = m.SuccessCount

	if m.LastFailureAt != nil {
		cm.aggregatedMetrics.LastFailureAt = m.LastFailureAt
	}

	// Populate cumulative performance totals for sliding window averaging
	// These integrate with GetChannelMetrics which calculates:
	// avgFirstTokenLatencyMs = TotalFirstTokenLatencyMs / SuccessCount
	// avgTokensPerSecond = TotalTokensPerSecond / SuccessCount
	if m.AvgFirstTokenLatency != nil && m.SuccessCount > 0 {
		// Convert average back to cumulative total
		cm.aggregatedMetrics.TotalFirstTokenLatencyMs = int64(*m.AvgFirstTokenLatency * float64(m.SuccessCount))
	}

	if m.AvgThroughput != nil && m.SuccessCount > 0 {
		// Convert average back to cumulative total
		cm.aggregatedMetrics.TotalTokensPerSecond = *m.AvgThroughput * float64(m.SuccessCount)
	}

	// Note: ConsecutiveFailures is not loaded from historical data.
	// It will be tracked in real-time as requests are processed.
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

	// Cumulative sums for performance averages (computed from sliding window)
	// TotalFirstTokenLatencyMs tracks the cumulative first token latency for all successful requests
	TotalFirstTokenLatencyMs int64
	// TotalTokensPerSecond tracks the cumulative tokens/second for all successful requests
	TotalTokensPerSecond float64
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
	if m.AvgFirstTokenLatencyMs != nil {
		clone.AvgFirstTokenLatencyMs = new(float64)
		*clone.AvgFirstTokenLatencyMs = *m.AvgFirstTokenLatencyMs
	}
	if m.AvgTokensPerSecond != nil {
		clone.AvgTokensPerSecond = new(float64)
		*clone.AvgTokensPerSecond = *m.AvgTokensPerSecond
	}
	return clone
}

// newChannelMetrics creates a new channelMetrics instance.
func newChannelMetrics(channelID int) *channelMetrics {
	cm := &channelMetrics{
		channelID:      channelID,
		window:         ringbuffer.New[*timeSlotMetrics](defaultPerformanceWindowSize),
		lastAccessTime: time.Now(),
		aggregatedMetrics: &AggregatedMetrics{
			metricsRecord: metricsRecord{},
		},
	}

	return cm
}

const latencyEWMAAlpha = 0.3

// recordSuccess records a successful request to the channel metrics.
// Must be called with cm.mu already held.
func (cm *channelMetrics) recordSuccess(slot *timeSlotMetrics, perf *PerformanceRecord) {
	slot.SuccessCount++
	cm.aggregatedMetrics.SuccessCount++
	cm.aggregatedMetrics.LastSelectedAt = &perf.EndTime

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
	cm.aggregatedMetrics.LastFailureAt = &perf.EndTime

	// Increment consecutive failures
	cm.aggregatedMetrics.ConsecutiveFailures++
}

// getOrCreateTimeSlot gets or creates a time slot for the given timestamp.
// Must be called with cm.mu already held.
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

	// Get or create channel-model metrics using composite key
	// Get or create channel-model metrics using model from PerformanceRecord (empty string is valid for default)
	cm := svc.getOrCreateModelMetrics(perf.ChannelID, perf.Model)

	// Lock the channel metrics for the duration of the updates
	cm.mu.Lock()
	defer cm.mu.Unlock()

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
		cm.aggregatedMetrics.RequestCount--
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
// Must be called with cm.mu already held.
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
		// Subtract performance totals for sliding window averaging
		cm.aggregatedMetrics.TotalFirstTokenLatencyMs -= metrics.TotalFirstTokenLatencyMs
		cm.aggregatedMetrics.TotalTokensPerSecond -= metrics.TotalTokensPerSecond
	}

	// Cleanup old entries from ringbuffer
	cm.window.CleanupBefore(cutoffTs)
}

// GetChannelMetrics returns performance metrics for the channel and model.
// If in-memory metrics are not available (e.g., after restart), it falls back to database values.
// Uses empty string as model key when model is not specified.
func (svc *ChannelService) GetChannelMetrics(ctx context.Context, channelID int, model string) (*AggregatedMetrics, error) {
	svc.channelPerfMetricsLock.RLock()
	defer svc.channelPerfMetricsLock.RUnlock()

	channelMap, exists := svc.channelPerfMetrics[channelID]
	if !exists {
		return &AggregatedMetrics{}, nil
	}

	cm, exists := channelMap[model]
	if !exists {
		return &AggregatedMetrics{}, nil
	}

	// Return a full copy of the aggregated metrics to avoid concurrent modification
	// while preserving all load-balancing signals, including latency EWMA.
	return cm.aggregatedMetrics.Clone(), nil
}

// IncrementChannelSelection increments the request count for a channel-model at selection time.
// This is called when a channel is selected by the load balancer to ensure immediate
// impact on subsequent selections, preventing the same channel from being selected
// repeatedly during burst/concurrent requests.
func (svc *ChannelService) IncrementChannelSelection(channelID int, model string) {
	// Use model directly as the key (empty string is valid for default)
	cm := svc.getOrCreateModelMetrics(channelID, model)

	cm.mu.Lock()
	defer cm.mu.Unlock()

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
			log.String("model", model),
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
	ChannelID        int
	Model            string // Model ID for model-specific performance tracking
	APIKey           string // API key used for the request (sensitive, do not log full value)
	StartTime           time.Time
	FirstTokenTime      *time.Time
	ReasoningStartTime  *time.Time
	ReasoningEndTime    *time.Time
	EndTime             time.Time
	Stream              bool
	Success          bool
	Canceled         bool
	RequestCompleted bool

	// If response status code is 0, it means the request is successful.
	ResponseStatusCode int
	CompletionTokens   int64
}

// Calculate calculates performance metrics from collected data.
// It enforces minimum latency to prevent extreme TPS calculations.
func (m *PerformanceRecord) Calculate() (firstTokenLatencyMs int64, requestLatencyMs int64, tokensPerSecond float64) {
	totalDuration := m.EndTime.Sub(m.StartTime)
	requestLatencyMs = totalDuration.Milliseconds()

	// Calculate first token latency
	if m.Stream && m.FirstTokenTime != nil {
		firstTokenLatency := m.FirstTokenTime.Sub(m.StartTime)
		firstTokenLatencyMs = firstTokenLatency.Milliseconds()
	}

	// Calculate tokens per second
	if totalDuration > 0 {
		tokensPerSecond = float64(m.TotalTokens) / totalDuration.Seconds()
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
