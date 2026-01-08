package biz

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelperformance"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/ringbuffer"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

const (
	// defaultPerformanceWindowSize is the default size of the sliding window in seconds (10 minutes).
	defaultPerformanceWindowSize = 600

	// performanceFlushInterval is the interval to flush data to database (1 minute).
	performanceFlushInterval = 60
)

// channelMetrics holds the performance metrics for a channel in memory.
type channelMetrics struct {
	channelID int

	// sliding window of metrics for the last N minutes using ring buffer for O(1) cleanup
	window *ringbuffer.RingBuffer[*timeSlotMetrics]

	// aggregatedMetrics holds accumulated metrics for the flush period
	aggregatedMetrics *AggregatedMetrics
}

// InitializeAllChannelPerformances ensures every channel has a corresponding performance record.
func (svc *ChannelService) InitializeAllChannelPerformances(ctx context.Context) error {
	client := svc.entFromContext(ctx)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	channelIDs, err := client.Channel.Query().IDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to query channels: %w", err)
	}

	if len(channelIDs) == 0 {
		return nil
	}

	existingRecords, err := client.ChannelPerformance.Query().Select(channelperformance.FieldChannelID).All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query channel performances: %w", err)
	}

	existingSet := lo.SliceToMap(existingRecords, func(perf *ent.ChannelPerformance) (int, bool) {
		return perf.ChannelID, true
	})

	var missingIDs []int

	for _, id := range channelIDs {
		if _, ok := existingSet[id]; !ok {
			missingIDs = append(missingIDs, id)
		}
	}

	if len(missingIDs) == 0 {
		return nil
	}

	creates := make([]*ent.ChannelPerformanceCreate, len(missingIDs))
	for i, id := range missingIDs {
		creates[i] = client.ChannelPerformance.Create().
			SetChannelID(id).
			SetSuccessRate(0).
			SetAvgLatencyMs(0).
			SetAvgTokenPerSecond(0).
			SetAvgStreamFirstTokenLatencyMs(0).
			SetAvgStreamTokenPerSecond(0).
			SetRequestCount(0).
			SetSuccessCount(0).
			SetFailureCount(0).
			SetTotalTokenCount(0).
			SetTotalRequestLatencyMs(0).
			SetStreamSuccessCount(0).
			SetStreamTotalRequestCount(0).
			SetStreamTotalTokenCount(0).
			SetStreamTotalRequestLatencyMs(0).
			SetStreamTotalFirstTokenLatencyMs(0).
			SetConsecutiveFailures(0)
	}

	if err := client.ChannelPerformance.CreateBulk(creates...).Exec(ctx); err != nil {
		return fmt.Errorf("failed to bulk initialize performance for channels: %w", err)
	}

	log.Info(ctx, "Initialized channel performance records for missing channels",
		log.Int("count", len(missingIDs)),
	)

	return nil
}

// LoadChannelPerformances loads all channel performance metrics from database into memory.
// This is called after channels are loaded to restore historical metrics.
func (svc *ChannelService) LoadChannelPerformances(ctx context.Context) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Query all channel performance records
	performances, err := svc.entFromContext(ctx).ChannelPerformance.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query channel performances: %w", err)
	}

	svc.channelPerfMetricsLock.Lock()
	defer svc.channelPerfMetricsLock.Unlock()

	for _, perf := range performances {
		// Initialize channel metrics for this channel
		cm := newChannelMetrics(perf.ChannelID)

		// Restore aggregated metrics counters from database
		cm.aggregatedMetrics.RequestCount = perf.RequestCount % 150
		cm.aggregatedMetrics.SuccessCount = perf.SuccessCount % 150
		cm.aggregatedMetrics.FailureCount = perf.FailureCount % 150
		cm.aggregatedMetrics.TotalTokenCount = perf.TotalTokenCount
		cm.aggregatedMetrics.TotalRequestLatencyMs = perf.TotalRequestLatencyMs
		cm.aggregatedMetrics.StreamSuccessCount = perf.StreamSuccessCount
		cm.aggregatedMetrics.StreamTotalRequestCount = perf.StreamTotalRequestCount
		cm.aggregatedMetrics.StreamTotalTokenCount = perf.StreamTotalTokenCount
		cm.aggregatedMetrics.StreamTotalRequestLatencyMs = perf.StreamTotalRequestLatencyMs
		cm.aggregatedMetrics.StreamTotalFirstTokenLatencyMs = perf.StreamTotalFirstTokenLatencyMs
		cm.aggregatedMetrics.ConsecutiveFailures = perf.ConsecutiveFailures

		// Restore last success/failure timestamps
		cm.aggregatedMetrics.LastSuccessAt = perf.LastSuccessAt
		cm.aggregatedMetrics.LastFailureAt = perf.LastFailureAt

		// Store in memory map
		svc.channelPerfMetrics[perf.ChannelID] = cm

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "loaded channel performance metrics",
				log.Int("channel_id", perf.ChannelID),
				log.Int("success_rate", perf.SuccessRate),
				log.Int("avg_latency_ms", perf.AvgLatencyMs),
				log.Float64("avg_token_per_second", float64(perf.AvgTokenPerSecond)),
				log.Any("last_success_at", perf.LastSuccessAt),
				log.Any("last_failure_at", perf.LastFailureAt),
				log.Int64("request_count", perf.RequestCount),
				log.Int64("success_count", perf.SuccessCount),
			)
		}
	}

	log.Info(ctx, "Loaded channel performance metrics from database",
		log.Int("count", len(performances)),
	)

	return nil
}

// InitializeChannelPerformance initializes performance record for a newly created channel.
func (svc *ChannelService) InitializeChannelPerformance(ctx context.Context, channelID int) error {
	log.Info(ctx, "initializing channel performance record", log.Int("channel_id", channelID))

	client := svc.entFromContext(ctx)

	_, err := client.ChannelPerformance.Create().
		SetChannelID(channelID).
		SetSuccessRate(0).
		SetAvgLatencyMs(0).
		SetAvgTokenPerSecond(0).
		SetAvgStreamFirstTokenLatencyMs(0).
		SetAvgStreamTokenPerSecond(0).
		SetRequestCount(0).
		SetSuccessCount(0).
		SetFailureCount(0).
		SetTotalTokenCount(0).
		SetTotalRequestLatencyMs(0).
		SetStreamSuccessCount(0).
		SetStreamTotalRequestCount(0).
		SetStreamTotalTokenCount(0).
		SetStreamTotalRequestLatencyMs(0).
		SetStreamTotalFirstTokenLatencyMs(0).
		SetConsecutiveFailures(0).
		Save(ctx)

	return err
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

	TotalTokenCount       int64
	TotalRequestLatencyMs int64

	StreamSuccessCount             int64
	StreamTotalRequestCount        int64
	StreamTotalTokenCount          int64
	StreamTotalRequestLatencyMs    int64
	StreamTotalFirstTokenLatencyMs int64

	// ConsecutiveFailures tracks the number of consecutive failures
	// Reset to 0 on success, incremented on failure
	ConsecutiveFailures int64
}

// CalculateSuccessRate calculates the success rate percentage.
func (m *metricsRecord) CalculateSuccessRate() int64 {
	if m.RequestCount > 0 {
		return (m.SuccessCount * 100) / m.RequestCount
	}

	return 0
}

// CalculateAvgLatencyMs calculates the average latency in milliseconds.
func (m *metricsRecord) CalculateAvgLatencyMs() int64 {
	if m.SuccessCount > 0 {
		return m.TotalRequestLatencyMs / m.SuccessCount
	}

	return 0
}

// CalculateAvgTokensPerSecond calculates the average tokens per second.
func (m *metricsRecord) CalculateAvgTokensPerSecond() float64 {
	if m.TotalRequestLatencyMs > 0 {
		return float64(m.TotalTokenCount) / (float64(m.TotalRequestLatencyMs) / 1000)
	}

	return 0
}

// CalculateAvgFirstTokenLatencyMs calculates the average first token latency in milliseconds for stream requests.
func (m *metricsRecord) CalculateAvgFirstTokenLatencyMs() int64 {
	if m.StreamSuccessCount > 0 {
		return m.StreamTotalFirstTokenLatencyMs / m.StreamSuccessCount
	}

	return 0
}

// CalculateAvgStreamTokensPerSecond calculates the average tokens per second for stream requests.
func (m *metricsRecord) CalculateAvgStreamTokensPerSecond() float64 {
	if m.StreamTotalRequestLatencyMs > 0 {
		return float64(m.StreamTotalTokenCount) / (float64(m.StreamTotalRequestLatencyMs) / 1000)
	}

	return 0
}

// AggregatedMetrics holds accumulated metrics for the flush period.
type AggregatedMetrics struct {
	metricsRecord

	LastSuccessAt *time.Time
	LastFailureAt *time.Time
}

func (m *AggregatedMetrics) Clone() *AggregatedMetrics {
	return &AggregatedMetrics{
		metricsRecord: m.metricsRecord,
		LastSuccessAt: m.LastSuccessAt,
		LastFailureAt: m.LastFailureAt,
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
func (cm *channelMetrics) recordSuccess(slot *timeSlotMetrics, perf *PerformanceRecord, firstTokenLatencyMs, requestLatencyMs int64) {
	slot.SuccessCount++
	cm.aggregatedMetrics.SuccessCount++
	cm.aggregatedMetrics.LastSuccessAt = &perf.EndTime

	// Reset consecutive failures on success
	cm.aggregatedMetrics.ConsecutiveFailures = 0

	slot.TotalRequestLatencyMs += requestLatencyMs
	cm.aggregatedMetrics.TotalRequestLatencyMs += requestLatencyMs

	if perf.Stream {
		slot.StreamSuccessCount++
		slot.StreamTotalRequestCount++
		slot.StreamTotalTokenCount += perf.TokenCount
		slot.StreamTotalRequestLatencyMs += requestLatencyMs
		slot.StreamTotalFirstTokenLatencyMs += firstTokenLatencyMs

		cm.aggregatedMetrics.StreamSuccessCount++
		cm.aggregatedMetrics.StreamTotalRequestCount++
		cm.aggregatedMetrics.StreamTotalTokenCount += perf.TokenCount
		cm.aggregatedMetrics.StreamTotalRequestLatencyMs += requestLatencyMs
		cm.aggregatedMetrics.StreamTotalFirstTokenLatencyMs += firstTokenLatencyMs
	}

	slot.TotalTokenCount += perf.TokenCount
	cm.aggregatedMetrics.TotalTokenCount += perf.TokenCount
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

// RecordMetrics records performance metrics for a channel.
// This directly saves the period metrics to database.
func (svc *ChannelService) RecordMetrics(ctx context.Context, channelID int, metrics *AggregatedMetrics) {
	if metrics == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Error(ctx, "panic in flush performance metrics", log.Any("panic", r))
		}
	}()

	now := time.Now()

	// Calculate metrics using the new methods
	successRate := metrics.CalculateSuccessRate()
	avgLatencyMs := metrics.CalculateAvgLatencyMs()
	avgTokensPerSecond := metrics.CalculateAvgTokensPerSecond()
	avgFirstTokenLatencyMs := metrics.CalculateAvgFirstTokenLatencyMs()
	avgStreamTokensPerSecond := metrics.CalculateAvgStreamTokensPerSecond()

	// Ensure ChannelPerformance record exists
	perf, err := svc.db.ChannelPerformance.Query().
		Where(channelperformance.ChannelID(channelID)).
		First(ctx)
	if err != nil {
		log.Error(ctx, "Failed to query channel performance", log.Cause(err))
		return
	}

	// Update metrics with both calculated averages and raw counters
	update := svc.db.ChannelPerformance.UpdateOneID(perf.ID).
		SetSuccessRate(int(successRate)).
		SetAvgLatencyMs(int(avgLatencyMs)).
		SetAvgTokenPerSecond(int(avgTokensPerSecond)).
		SetAvgStreamFirstTokenLatencyMs(int(avgFirstTokenLatencyMs)).
		SetAvgStreamTokenPerSecond(avgStreamTokensPerSecond).
		SetNillableLastSuccessAt(metrics.LastSuccessAt).
		SetNillableLastFailureAt(metrics.LastFailureAt).
		SetRequestCount(metrics.RequestCount).
		SetSuccessCount(metrics.SuccessCount).
		SetFailureCount(metrics.FailureCount).
		SetTotalTokenCount(metrics.TotalTokenCount).
		SetTotalRequestLatencyMs(metrics.TotalRequestLatencyMs).
		SetStreamSuccessCount(metrics.StreamSuccessCount).
		SetStreamTotalRequestCount(metrics.StreamTotalRequestCount).
		SetStreamTotalTokenCount(metrics.StreamTotalTokenCount).
		SetStreamTotalRequestLatencyMs(metrics.StreamTotalRequestLatencyMs).
		SetStreamTotalFirstTokenLatencyMs(metrics.StreamTotalFirstTokenLatencyMs).
		SetConsecutiveFailures(metrics.ConsecutiveFailures).
		SetUpdatedAt(now)

	_, err = update.Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update channel performance", log.Cause(err))
		return
	}

	log.Debug(ctx, "Recorded channel performance metrics",
		log.Int("channel_id", channelID),
		log.Int("success_rate", int(successRate)),
		log.Int("avg_latency_ms", int(avgLatencyMs)),
		log.Int("avg_token_per_second", int(avgTokensPerSecond)),
		log.Int("avg_stream_first_token_ms", int(avgFirstTokenLatencyMs)),
		log.Float64("avg_stream_token_per_second", avgStreamTokensPerSecond),
	)
}

func (svc *ChannelService) markChannelUnavailable(ctx context.Context, channelID int, errorStatusCode int) {
	ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	_, err := svc.db.Channel.UpdateOneID(channelID).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(deriveErrorMessage(errorStatusCode)).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to disable channel on unrecoverable error",
			log.Int("channel_id", channelID),
			log.Int("error_code", errorStatusCode),
			log.Cause(err),
		)

		return
	}

	log.Warn(ctx, "Channel disabled due to unrecoverable error",
		log.Int("channel_id", channelID),
		log.Int("error_code", errorStatusCode),
	)
}

// checkAndHandleChannelError checks if the channel should be disabled based on the error status code.
func (svc *ChannelService) checkAndHandleChannelError(ctx context.Context, perf *PerformanceRecord, policy *RetryPolicy) bool {
	for _, statusConfig := range policy.AutoDisableChannel.Statuses {
		if statusConfig.Status != perf.ErrorStatusCode {
			continue
		}

		svc.channelErrorCountsLock.Lock()

		if svc.channelErrorCounts[perf.ChannelID] == nil {
			svc.channelErrorCounts[perf.ChannelID] = make(map[int]int)
		}

		svc.channelErrorCounts[perf.ChannelID][perf.ErrorStatusCode]++
		count := svc.channelErrorCounts[perf.ChannelID][perf.ErrorStatusCode]
		svc.channelErrorCountsLock.Unlock()

		if count >= statusConfig.Times {
			svc.markChannelUnavailable(ctx, perf.ChannelID, perf.ErrorStatusCode)
			svc.channelErrorCountsLock.Lock()
			delete(svc.channelErrorCounts, perf.ChannelID)
			svc.channelErrorCountsLock.Unlock()

			return true
		}
	}

	return false
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
	} else if !perf.Canceled {
		policy := svc.SystemService.RetryPolicyOrDefault(ctx)
		if policy.AutoDisableChannel.Enabled {
			if svc.checkAndHandleChannelError(ctx, perf, policy) {
				return
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

	firstTokenLatencyMs, requestLatencyMs, tokensPerSecond := perf.Calculate()

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
		cm.recordSuccess(slot, perf, firstTokenLatencyMs, requestLatencyMs)
	} else if !perf.Canceled {
		cm.recordFailure(slot, perf)
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "recorded performance metrics",
			log.Int("channel_id", perf.ChannelID),
			log.Bool("success", perf.Success),
			log.Int64("first_token_latency_ms", firstTokenLatencyMs),
			log.Int64("total_duration_ms", requestLatencyMs),
			log.Float64("tokens_per_second", tokensPerSecond),
			log.Any("token_count", perf.TokenCount),
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
		cm.aggregatedMetrics.TotalTokenCount -= metrics.TotalTokenCount
		cm.aggregatedMetrics.TotalRequestLatencyMs -= metrics.TotalRequestLatencyMs
		cm.aggregatedMetrics.StreamTotalRequestCount -= metrics.StreamTotalRequestCount
		cm.aggregatedMetrics.StreamTotalTokenCount -= metrics.StreamTotalTokenCount
		cm.aggregatedMetrics.StreamTotalRequestLatencyMs -= metrics.StreamTotalRequestLatencyMs
		cm.aggregatedMetrics.StreamTotalFirstTokenLatencyMs -= metrics.StreamTotalFirstTokenLatencyMs
		cm.aggregatedMetrics.StreamSuccessCount -= metrics.StreamSuccessCount
	}

	// Cleanup old entries from ringbuffer
	cm.window.CleanupBefore(cutoffTs)
}

// startPerformanceProcess starts the background goroutine to flush metrics to database.
func (svc *ChannelService) startPerformanceProcess() {
	ticker := time.NewTicker(performanceFlushInterval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case perf := <-svc.perfCh:
			svc.RecordPerformance(context.Background(), perf)
		case <-ticker.C:
			err := svc.Executors.ExecuteFunc(func(ctx context.Context) {
				svc.flushPerformanceMetrics(ctx)
			})
			if err != nil {
				log.Error(context.Background(), "failed to execute flush performance metrics", log.Cause(err))
			}
		}
	}
}

// flushPerformanceMetrics flushes accumulated metrics to database.
func (svc *ChannelService) flushPerformanceMetrics(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(ctx, "panic in flush performance metrics", log.Any("panic", r))
		}
	}()

	svc.channelPerfMetricsLock.RLock()

	metricsToFlush := map[int]*AggregatedMetrics{}
	for _, cm := range svc.channelPerfMetrics {
		metricsToFlush[cm.channelID] = cm.aggregatedMetrics.Clone()
	}

	svc.channelPerfMetricsLock.RUnlock()

	for channelID, aggregatedMetrics := range metricsToFlush {
		// Skip if no data in the sliding window (no requests in the last 10 minutes)
		// This prevents overwriting database values with zeros when there's no recent activity
		if aggregatedMetrics.RequestCount == 0 {
			continue
		}

		svc.RecordMetrics(ctx, channelID, aggregatedMetrics)
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
		metricsRecord: cm.aggregatedMetrics.metricsRecord,
		LastSuccessAt: cm.aggregatedMetrics.LastSuccessAt,
		LastFailureAt: cm.aggregatedMetrics.LastFailureAt,
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
	if cm.aggregatedMetrics.LastSuccessAt == nil || cm.aggregatedMetrics.LastSuccessAt.Before(now) {
		cm.aggregatedMetrics.LastSuccessAt = &now
	}

	log.Debug(context.Background(), "IncrementChannelSelection: incremented request count",
		log.Int("channel_id", channelID),
		log.Int64("old_count", oldCount),
		log.Int64("new_count", cm.aggregatedMetrics.RequestCount),
	)
}

func deriveErrorMessage(errorCode int) string {
	return http.StatusText(errorCode)
}

// PerformanceRecord contains performance metrics collected during request processing.
type PerformanceRecord struct {
	ChannelID        int
	StartTime        time.Time
	FirstTokenTime   *time.Time
	EndTime          time.Time
	Stream           bool
	Success          bool
	Canceled         bool
	RequestCompleted bool

	// If token count is 0, it means the provider response without token count.
	TokenCount int64

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

	// Calculate tokens per second
	if m.TokenCount > 0 && totalDuration.Seconds() > 0 {
		tokensPerSecond = float64(m.TokenCount) / totalDuration.Seconds()
	}

	return firstTokenLatencyMs, requestLatencyMs, tokensPerSecond
}

// MarkSuccess marks the request as completed.
func (m *PerformanceRecord) MarkSuccess(tokenCount int64) {
	m.Success = true
	m.TokenCount = tokenCount
	m.RequestCompleted = true
	m.EndTime = time.Now()
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

func (m *PerformanceRecord) MarkFirstToken() {
	if m.FirstTokenTime == nil {
		m.FirstTokenTime = lo.ToPtr(time.Now())
	}
}
