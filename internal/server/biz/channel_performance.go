package biz

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channelperformance"
	"github.com/looplj/axonhub/internal/log"
)

// PerformanceRecord contains performance metrics collected during request processing.
type PerformanceRecord struct {
	StartTime        time.Time
	FirstTokenTime   *time.Time
	EndTime          *time.Time
	ChannelID        int
	Stream           bool
	Success          bool
	RequestCompleted bool

	// If token count is nil, it means the provider response without token count.
	TokenCount *int64

	// If error status code is nil, it means the request is successful.
	ErrorStatusCode *int
}

// PerformanceMetrics contains metrics collected during a request.
type PerformanceMetrics struct {
	ChannelID           int
	Success             bool
	ErrorCode           *int
	FirstTokenLatencyMs int
	TotalDurationMs     int
	TokenCount          int
	TokensPerSecond     float64
}

// RecordMetrics records performance metrics for a channel.
func (svc *ChannelService) RecordMetrics(ctx context.Context, metrics *PerformanceMetrics) error {
	if metrics == nil {
		return nil
	}

	client := ent.FromContext(ctx)
	now := time.Now()

	// Get or create channel performance record
	perf, err := client.ChannelPerformance.Query().
		Where(channelperformance.ChannelID(metrics.ChannelID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// Create new record
			perf, err = client.ChannelPerformance.Create().
				SetChannelID(metrics.ChannelID).
				SetHealthStatus(channelperformance.HealthStatusGood).
				SetLastPeriodStart(now).
				SetLastPeriodEnd(now).
				Save(ctx)
			if err != nil {
				log.Error(ctx, "Failed to create channel performance record", log.Cause(err))
				return err
			}
		} else {
			log.Error(ctx, "Failed to query channel performance", log.Cause(err))
			return err
		}
	}

	// Update metrics
	update := client.ChannelPerformance.UpdateOneID(perf.ID)

	// Update total counters
	update = update.
		SetTotalCount(perf.TotalCount + 1).
		SetLastAttemptAt(now)

	if metrics.Success {
		update = update.
			SetTotalSuccessCount(perf.TotalSuccessCount + 1).
			SetLastSuccessAt(now)

		// Update token metrics if available
		if metrics.TokenCount > 0 {
			newTotalTokens := perf.TotalTokenCount + metrics.TokenCount
			update = update.SetTotalTokenCount(newTotalTokens)

			// Update average first token latency for streaming
			if metrics.FirstTokenLatencyMs > 0 {
				newAvgFirstToken := calculateNewAverage(
					perf.TotalAvgStreamFirstTokenLatenchMs,
					perf.TotalSuccessCount,
					metrics.FirstTokenLatencyMs,
				)
				update = update.SetTotalAvgStreamFirstTokenLatenchMs(newAvgFirstToken)
			}

			// Update tokens per second for streaming
			if metrics.TokensPerSecond > 0 {
				newAvgTPS := calculateNewAverageFloat(
					perf.TotalAvgStreamTokenPerSecond,
					perf.TotalSuccessCount,
					metrics.TokensPerSecond,
				)
				update = update.SetTotalAvgStreamTokenPerSecond(newAvgTPS)
			}
		}
	} else {
		update = update.SetLastFailureAt(now)
	}

	// Update last period counters
	update = update.
		SetLastPeriodCount(perf.LastPeriodCount + 1)

	if metrics.Success {
		update = update.SetLastPeriodSuccessCount(perf.LastPeriodSuccessCount + 1)

		if metrics.TokenCount > 0 {
			newPeriodTokens := perf.LastPeriodTokenCount + metrics.TokenCount
			update = update.SetLastPeriodTokenCount(newPeriodTokens)

			if metrics.FirstTokenLatencyMs > 0 {
				newPeriodAvgFirstToken := calculateNewAverage(
					perf.LastPeriodAvgStreamFirstTokenLatenchMs,
					perf.LastPeriodSuccessCount,
					metrics.FirstTokenLatencyMs,
				)
				update = update.SetLastPeriodAvgStreamFirstTokenLatenchMs(newPeriodAvgFirstToken)
			}

			if metrics.TokensPerSecond > 0 {
				newPeriodAvgTPS := calculateNewAverageFloat(
					perf.LastPeriodAvgStreamTokenPerSecond,
					perf.LastPeriodSuccessCount,
					metrics.TokensPerSecond,
				)
				update = update.SetLastPeriodAvgStreamTokenPerSecond(newPeriodAvgTPS)
			}
		}
	}

	// Update health status based on error codes
	if !metrics.Success && metrics.ErrorCode != nil {
		newHealthStatus := determineHealthStatus(*metrics.ErrorCode)
		if newHealthStatus != perf.HealthStatus {
			update = update.SetHealthStatus(newHealthStatus)
			log.Info(ctx, "Channel health status changed",
				log.Int("channel_id", metrics.ChannelID),
				log.String("old_status", string(perf.HealthStatus)),
				log.String("new_status", string(newHealthStatus)),
				log.Int("error_code", *metrics.ErrorCode),
			)
		}
	}

	_, err = update.Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to update channel performance", log.Cause(err))
		return err
	}

	log.Debug(ctx, "Recorded channel performance metrics",
		log.Int("channel_id", metrics.ChannelID),
		log.Bool("success", metrics.Success),
		log.Int("first_token_latency_ms", metrics.FirstTokenLatencyMs),
		log.Int("token_count", metrics.TokenCount),
	)

	return nil
}

// calculateNewAverage calculates a new cumulative average.
func calculateNewAverage(currentAvg int, count int, newValue int) int {
	if count == 0 {
		return newValue
	}

	return (currentAvg*count + newValue) / (count + 1)
}

// calculateNewAverageFloat calculates a new cumulative average for float values.
func calculateNewAverageFloat(currentAvg float64, count int, newValue float64) float64 {
	if count == 0 {
		return newValue
	}

	return (currentAvg*float64(count) + newValue) / float64(count+1)
}

// determineHealthStatus determines the health status based on error code.
func determineHealthStatus(errorCode int) channelperformance.HealthStatus {
	switch errorCode {
	case 401, 403, 404:
		return channelperformance.HealthStatusPanic
	case 429, 500, 502, 503, 504:
		return channelperformance.HealthStatusCritical
	default:
		return channelperformance.HealthStatusWarning
	}
}

// GetChannelPerformance retrieves performance metrics for a channel.
func (svc *ChannelService) GetChannelPerformance(ctx context.Context, channelID int) (*ent.ChannelPerformance, error) {
	client := ent.FromContext(ctx)

	perf, err := client.ChannelPerformance.Query().
		Where(channelperformance.ChannelID(channelID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return perf, nil
}

// ResetPeriod resets the last period counters for a channel.
func (svc *ChannelService) ResetPeriod(ctx context.Context, channelID int) error {
	client := ent.FromContext(ctx)
	now := time.Now()

	perf, err := client.ChannelPerformance.Query().
		Where(channelperformance.ChannelID(channelID)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil
		}

		return err
	}

	_, err = client.ChannelPerformance.UpdateOneID(perf.ID).
		SetLastPeriodCount(0).
		SetLastPeriodSuccessCount(0).
		SetLastPeriodTokenCount(0).
		SetLastPeriodAvgStreamFirstTokenLatenchMs(0).
		SetLastPeriodAvgStreamTokenPerSecond(0).
		SetLastPeriodStart(now).
		SetLastPeriodEnd(now).
		SetLastPeriodSeconds(0).
		Save(ctx)

	return err
}

// UpdateHealthStatus manually updates the health status of a channel.
func (svc *ChannelService) UpdateHealthStatus(ctx context.Context, channelID int, status channelperformance.HealthStatus) error {
	client := ent.FromContext(ctx)

	perf, err := client.ChannelPerformance.Query().
		Where(channelperformance.ChannelID(channelID)).
		First(ctx)
	if err != nil {
		return err
	}

	_, err = client.ChannelPerformance.UpdateOneID(perf.ID).
		SetHealthStatus(status).
		Save(ctx)
	if err == nil {
		log.Info(ctx, "Manually updated channel health status",
			log.Int("channel_id", channelID),
			log.String("status", string(status)),
		)
	}

	return err
}

// RecordPerformance records performance data for a channel request.
// Currently this only logs the performance data.
func (svc *ChannelService) RecordPerformance(ctx context.Context, perf *PerformanceRecord) {
	if perf == nil || !perf.IsValid() {
		return
	}

	firstTokenLatencyMs, totalDurationMs, tokensPerSecond := perf.Calculate()

	log.Info(ctx, "Channel performance recorded",
		log.Int("channel_id", perf.ChannelID),
		log.Bool("success", perf.Success),
		log.Bool("stream", perf.Stream),
		log.Int("first_token_latency_ms", firstTokenLatencyMs),
		log.Int("total_duration_ms", totalDurationMs),
		log.Float64("tokens_per_second", tokensPerSecond),
		log.Any("token_count", perf.TokenCount),
		log.Any("error_code", perf.ErrorStatusCode),
	)
}

// Calculate calculates performance metrics from collected data.
func (m *PerformanceRecord) Calculate() (firstTokenLatencyMs int, totalDurationMs int, tokensPerSecond float64) {
	if m.EndTime == nil {
		endTime := time.Now()
		m.EndTime = &endTime
	}

	totalDuration := m.EndTime.Sub(m.StartTime)
	totalDurationMs = int(totalDuration.Milliseconds())

	// Calculate first token latency
	if m.FirstTokenTime != nil {
		firstTokenLatency := m.FirstTokenTime.Sub(m.StartTime)
		firstTokenLatencyMs = int(firstTokenLatency.Milliseconds())
	}

	// Calculate tokens per second
	if m.TokenCount != nil && *m.TokenCount > 0 && totalDuration.Seconds() > 0 {
		tokensPerSecond = float64(*m.TokenCount) / totalDuration.Seconds()
	}

	return firstTokenLatencyMs, totalDurationMs, tokensPerSecond
}

// MarkSuccess marks the request as completed.
func (m *PerformanceRecord) MarkSuccess(tokenCount *int64) {
	m.Success = true
	m.TokenCount = tokenCount
	m.RequestCompleted = true
	m.EndTime = lo.ToPtr(time.Now())
}

// MarkFailed marks the request as failed.
func (m *PerformanceRecord) MarkFailed(errorCode int) {
	m.Success = false
	m.ErrorStatusCode = &errorCode
	m.RequestCompleted = true
	now := time.Now()
	m.EndTime = &now
}

// IsValid checks if metrics are valid for recording.
func (m *PerformanceRecord) IsValid() bool {
	return m.ChannelID > 0 && m.RequestCompleted
}

// MarkFirstToken records the time of the first token.
func (m *PerformanceRecord) MarkFirstToken() {
	if m.FirstTokenTime == nil {
		now := time.Now()
		m.FirstTokenTime = &now
	}
}
