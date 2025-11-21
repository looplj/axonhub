package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelperformance"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
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

	// sliding window of metrics for the last N minutes (key = timestamp rounded to second)
	// TODO: use circular buffer instead of map.
	window map[int64]*timeSlotMetrics

	// windowMu protects window map access
	windowMu sync.Mutex

	// aggreatedMetrics holds accumulated metrics for the flush period
	aggreatedMetrics *AggretagedMetrics

	// lock for concurrent access to aggreatedMetrics
	mu sync.RWMutex
}

// InitializeAllChannelPerformances ensures every channel has a corresponding performance record.
func (svc *ChannelService) InitializeAllChannelPerformances(ctx context.Context) error {
	client := svc.Ent
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
		creates[i] = client.ChannelPerformance.Create().SetChannelID(id)
	}

	if err := client.ChannelPerformance.CreateBulk(creates...).Exec(ctx); err != nil {
		return fmt.Errorf("failed to bulk initialize performance for channels: %w", err)
	}

	log.Info(ctx, "Initialized channel performance records for missing channels",
		log.Int("count", len(missingIDs)),
	)

	return nil
}

// InitializeChannelPerformance initializes performance record for a newly created channel.
func (svc *ChannelService) InitializeChannelPerformance(ctx context.Context, channelID int) error {
	log.Info(ctx, "Initializing channel performance record",
		log.Int("channel_id", channelID),
	)

	client := ent.FromContext(ctx)
	if client == nil {
		client = svc.Ent
	}

	_, err := client.ChannelPerformance.Create().
		SetChannelID(channelID).
		SetSuccessRate(0).
		SetAvgLatencyMs(0).
		SetAvgTokenPerSecond(0).
		SetAvgStreamFirstTokenLatencyMs(0).
		SetAvgStreamTokenPerSecond(0).
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

	StreamTotalTokenCount          int64
	StreamTotalFirstTokenLatencyMs int64
	StreamSuccessCount             int64

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
	if m.RequestCount > 0 {
		return float64(m.TotalTokenCount) / float64(m.RequestCount)
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
	if m.StreamSuccessCount > 0 {
		return float64(m.StreamTotalTokenCount) / float64(m.StreamSuccessCount)
	}

	return 0
}

// AggretagedMetrics holds accumulated metrics for the flush period.
type AggretagedMetrics struct {
	metricsRecord

	LastSuccessAt *time.Time
	LastFailureAt *time.Time
}

// newChannelMetrics creates a new channelMetrics instance.
func newChannelMetrics(channelID int) *channelMetrics {
	cm := &channelMetrics{
		channelID: channelID,
		window:    make(map[int64]*timeSlotMetrics),
		aggreatedMetrics: &AggretagedMetrics{
			metricsRecord: metricsRecord{},
		},
	}

	return cm
}

// recordSuccess records a successful request to the channel metrics.
func (cm *channelMetrics) recordSuccess(slot *timeSlotMetrics, perf *PerformanceRecord, firstTokenLatencyMs, requestLatencyMs int64) {
	slot.SuccessCount++
	cm.aggreatedMetrics.SuccessCount++
	cm.aggreatedMetrics.LastSuccessAt = &perf.EndTime

	// Reset consecutive failures on success
	cm.aggreatedMetrics.ConsecutiveFailures = 0

	slot.TotalRequestLatencyMs += requestLatencyMs
	cm.aggreatedMetrics.TotalRequestLatencyMs += requestLatencyMs

	if perf.Stream {
		slot.StreamSuccessCount++
		slot.StreamTotalTokenCount += perf.TokenCount
		slot.StreamTotalFirstTokenLatencyMs += firstTokenLatencyMs

		cm.aggreatedMetrics.StreamSuccessCount++
		cm.aggreatedMetrics.StreamTotalTokenCount += perf.TokenCount
		cm.aggreatedMetrics.StreamTotalFirstTokenLatencyMs += firstTokenLatencyMs
	}

	slot.TotalTokenCount += perf.TokenCount
	cm.aggreatedMetrics.TotalTokenCount += perf.TokenCount
}

// recordFailure records a failed request to the channel metrics.
func (cm *channelMetrics) recordFailure(slot *timeSlotMetrics, perf *PerformanceRecord) {
	slot.FailureCount++
	cm.aggreatedMetrics.FailureCount++
	cm.aggreatedMetrics.LastFailureAt = &perf.EndTime

	// Increment consecutive failures
	cm.aggreatedMetrics.ConsecutiveFailures++
}

// getOrCreateTimeSlot gets or creates a time slot for the given timestamp.
func (cm *channelMetrics) getOrCreateTimeSlot(ts int64, endTime time.Time, windowSize int64) *timeSlotMetrics {
	cm.windowMu.Lock()
	defer cm.windowMu.Unlock()

	if slot, ok := cm.window[ts]; ok {
		return slot
	}

	// Clean old entries to prevent memory leak
	if len(cm.window) >= int(windowSize) {
		cm.cleanupExpiredSlots(endTime.Add(-time.Duration(windowSize) * time.Second))
	}

	slot := &timeSlotMetrics{
		timestamp:     ts,
		metricsRecord: metricsRecord{},
	}
	cm.window[ts] = slot

	return slot
}

// RecordMetrics records performance metrics for a channel.
// This directly saves the period metrics to database.
func (svc *ChannelService) RecordMetrics(ctx context.Context, channelID int, metrics *AggretagedMetrics) error {
	if metrics == nil {
		return nil
	}

	now := time.Now()

	// Calculate metrics using the new methods
	successRate := metrics.CalculateSuccessRate()
	avgLatencyMs := metrics.CalculateAvgLatencyMs()
	avgTokensPerSecond := metrics.CalculateAvgTokensPerSecond()
	avgFirstTokenLatencyMs := metrics.CalculateAvgFirstTokenLatencyMs()
	avgStreamTokensPerSecond := metrics.CalculateAvgStreamTokensPerSecond()

	// Ensure ChannelPerformance record exists
	perf, err := svc.Ent.ChannelPerformance.Query().
		Where(channelperformance.ChannelID(channelID)).
		First(ctx)
	if err != nil {
		log.Error(ctx, "Failed to query channel performance", log.Cause(err))
		return err
	}

	// Update metrics
	update := svc.Ent.ChannelPerformance.UpdateOneID(perf.ID).
		SetSuccessRate(int(successRate)).
		SetAvgLatencyMs(int(avgLatencyMs)).
		SetAvgTokenPerSecond(int(avgTokensPerSecond)).
		SetAvgStreamFirstTokenLatencyMs(int(avgFirstTokenLatencyMs)).
		SetAvgStreamTokenPerSecond(avgStreamTokensPerSecond).
		SetNillableLastSuccessAt(metrics.LastSuccessAt).
		SetNillableLastFailureAt(metrics.LastFailureAt).
		SetUpdatedAt(now)

	_, err = update.Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update channel performance", log.Cause(err))
		return err
	}

	log.Debug(ctx, "Recorded channel performance metrics",
		log.Int("channel_id", channelID),
		log.Int("success_rate", int(successRate)),
		log.Int("avg_latency_ms", int(avgLatencyMs)),
		log.Int("avg_token_per_second", int(avgTokensPerSecond)),
		log.Int("avg_stream_first_token_ms", int(avgFirstTokenLatencyMs)),
		log.Float64("avg_stream_token_per_second", avgStreamTokensPerSecond),
	)

	return nil
}

func (svc *ChannelService) markChannelUnavaiable(ctx context.Context, channelID int, errorStatusCode int) {
	ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	_, err := svc.Ent.Channel.UpdateOneID(channelID).
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

// RecordPerformance records performance metrics to in-memory cache.
func (svc *ChannelService) RecordPerformance(ctx context.Context, perf *PerformanceRecord) {
	if perf == nil || !perf.IsValid() {
		return
	}

	// Check for unrecoverable errors and disable channel immediately
	if !perf.Success && !isRecoverable(perf.ErrorStatusCode) {
		svc.markChannelUnavaiable(ctx, perf.ChannelID, perf.ErrorStatusCode)
		return
	}

	// Get or create channel metrics
	svc.perfMetricsLock.Lock()

	cm, exists := svc.channelPerfMetrics[perf.ChannelID]
	if !exists {
		cm = newChannelMetrics(perf.ChannelID)
		svc.channelPerfMetrics[perf.ChannelID] = cm
	}

	svc.perfMetricsLock.Unlock()

	// Determine window size
	var windowSize int64 = defaultPerformanceWindowSize
	if svc.PerformanceWindowSeconds > 0 {
		windowSize = svc.PerformanceWindowSeconds
	}

	ts := perf.EndTime.Unix()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Get or create time slot for this second
	slot := cm.getOrCreateTimeSlot(ts, perf.EndTime, windowSize)

	firstTokenLatencyMs, requestLatencyMs, tokensPerSecond := perf.Calculate()

	// Update request counts.
	slot.RequestCount++
	cm.aggreatedMetrics.RequestCount++

	// Record success or failure
	if perf.Success {
		cm.recordSuccess(slot, perf, firstTokenLatencyMs, requestLatencyMs)
	} else {
		cm.recordFailure(slot, perf)
	}

	log.Debug(ctx, "Recorded performance metrics to cache",
		log.Int("channel_id", perf.ChannelID),
		log.Bool("success", perf.Success),
		log.Int64("first_token_latency_ms", firstTokenLatencyMs),
		log.Int64("total_duration_ms", requestLatencyMs),
		log.Float64("tokens_per_second", tokensPerSecond),
		log.Any("token_count", perf.TokenCount),
		log.Any("error_code", perf.ErrorStatusCode),
	)
}

// cleanupExpiredSlots removes time slots older than the cutoff time.
func (cm *channelMetrics) cleanupExpiredSlots(cutoff time.Time) {
	cutoffTs := cutoff.Unix()
	for ts := range cm.window {
		if ts < cutoffTs {
			delete(cm.window, ts)
		}
	}
}

// startPerformanceFlush starts the background goroutine to flush metrics to database.
func (svc *ChannelService) startPerformanceFlush() {
	svc.perfWg.Add(1)

	go func() {
		defer svc.perfWg.Done()

		ticker := time.NewTicker(performanceFlushInterval * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				svc.flushPerformanceMetrics(context.Background())
			case <-svc.perfStopCh:
				// Flush one last time before stopping
				svc.flushPerformanceMetrics(context.Background())
				return
			}
		}
	}()
}

// flushPerformanceMetrics flushes accumulated metrics to database.
func (svc *ChannelService) flushPerformanceMetrics(ctx context.Context) {
	svc.perfMetricsLock.RLock()

	metricsToFlush := make([]*channelMetrics, 0, len(svc.channelPerfMetrics))
	for _, cm := range svc.channelPerfMetrics {
		metricsToFlush = append(metricsToFlush, cm)
	}

	svc.perfMetricsLock.RUnlock()

	for _, cm := range metricsToFlush {
		cm.mu.Lock()

		if cm.aggreatedMetrics.RequestCount == 0 {
			cm.mu.Unlock()
			continue
		}

		// Reset current period
		periodMetrics := cm.aggreatedMetrics
		cm.aggreatedMetrics = &AggretagedMetrics{}
		cm.mu.Unlock()

		// Record to database (non-blocking, use separate goroutine)
		go func(channelID int, metrics *AggretagedMetrics) {
			defer func() {
				if r := recover(); r != nil {
					log.Error(ctx, "panic in flush performance metrics", log.Any("panic", r))
				}
			}()

			if err := svc.RecordMetrics(ctx, channelID, metrics); err != nil {
				log.Error(ctx, "Failed to flush performance metrics to database", log.Cause(err))
			}
		}(cm.channelID, periodMetrics)
	}
}

// GetChannelMetrics returns performance metrics for the last N minutes (configurable).
func (svc *ChannelService) GetChannelMetrics(ctx context.Context, channelID int) (*AggretagedMetrics, error) {
	svc.perfMetricsLock.RLock()
	cm, exists := svc.channelPerfMetrics[channelID]
	svc.perfMetricsLock.RUnlock()

	if !exists {
		return &AggretagedMetrics{}, nil
	}

	// Get window size (configurable, default to 10 minutes)
	windowSize := int(defaultPerformanceWindowSize)
	if svc.PerformanceWindowSeconds > 0 {
		windowSize = int(svc.PerformanceWindowSeconds)
	}

	now := time.Now()
	cutoff := now.Add(-time.Duration(windowSize) * time.Second).Unix()

	cm.windowMu.Lock()
	defer cm.windowMu.Unlock()

	metrics := &AggretagedMetrics{
		metricsRecord: metricsRecord{},
	}

	for ts, slot := range cm.window {
		if ts >= cutoff {
			metrics.RequestCount += slot.RequestCount
			metrics.SuccessCount += slot.SuccessCount
			metrics.FailureCount += slot.FailureCount
			metrics.TotalTokenCount += slot.TotalTokenCount
			metrics.TotalRequestLatencyMs += slot.TotalRequestLatencyMs
			metrics.StreamSuccessCount += slot.StreamSuccessCount
			metrics.StreamTotalTokenCount += slot.StreamTotalTokenCount
			metrics.StreamTotalFirstTokenLatencyMs += slot.StreamTotalFirstTokenLatencyMs
		}
	}

	// Copy last success/failure times from aggregated metrics
	cm.mu.RLock()
	metrics.LastSuccessAt = cm.aggreatedMetrics.LastSuccessAt
	metrics.LastFailureAt = cm.aggreatedMetrics.LastFailureAt
	metrics.ConsecutiveFailures = cm.aggreatedMetrics.ConsecutiveFailures
	cm.mu.RUnlock()

	return metrics, nil
}

// isRecoverable determines the health status based on error code.
func isRecoverable(errorCode int) bool {
	switch errorCode {
	case 401, 403, 404:
		return false
	default:
		return true
	}
}

func deriveErrorMessage(errorCode int) string {
	switch errorCode {
	case 401, 403:
		return "Unauthorized, please check your channel API key configuration."
	case 404:
		return "Not Found, please check your channel base URL configuration."
	default:
		return "Unable to access channel service, please check your channel configuration."
	}
}

// Stop gracefully stops the performance metrics collection.
func (svc *ChannelService) Stop() {
	close(svc.perfStopCh)
	svc.perfWg.Wait()
}

// PerformanceRecord contains performance metrics collected during request processing.
type PerformanceRecord struct {
	ChannelID        int
	StartTime        time.Time
	FirstTokenTime   *time.Time
	EndTime          time.Time
	Stream           bool
	Success          bool
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

// IsValid checks if metrics are valid for recording.
func (m *PerformanceRecord) IsValid() bool {
	return m.ChannelID > 0 && m.RequestCompleted
}

func (m *PerformanceRecord) MarkFirstToken() {
	if m.FirstTokenTime == nil {
		m.FirstTokenTime = lo.ToPtr(time.Now())
	}
}
