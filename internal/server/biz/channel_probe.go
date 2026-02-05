package biz

import (
	"context"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelprobe"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xtime"
)

// ChannelProbePoint represents a single probe data point for a channel.
type ChannelProbePoint struct {
	Timestamp             int64    `json:"timestamp"`
	TotalRequestCount     int      `json:"total_request_count"`
	SuccessRequestCount   int      `json:"success_request_count"`
	AvgTokensPerSecond    *float64 `json:"avg_tokens_per_second,omitempty"`
	AvgTimeToFirstTokenMs *float64 `json:"avg_time_to_first_token_ms,omitempty"`
}

// ChannelProbeData represents probe data for a single channel.
type ChannelProbeData struct {
	ChannelID int                  `json:"channel_id"`
	Points    []*ChannelProbePoint `json:"points"`
}

// ChannelProbeServiceParams contains dependencies for ChannelProbeService.
type ChannelProbeServiceParams struct {
	fx.In

	Ent           *ent.Client
	SystemService *SystemService
}

// ChannelProbeService handles channel probe operations.
type ChannelProbeService struct {
	*AbstractService

	SystemService     *SystemService
	Executor          executors.ScheduledExecutor
	mu                sync.Mutex
	lastExecutionTime time.Time
}

// NewChannelProbeService creates a new ChannelProbeService.
func NewChannelProbeService(params ChannelProbeServiceParams) *ChannelProbeService {
	svc := &ChannelProbeService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		SystemService:     params.SystemService,
		Executor:          executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1)),
		lastExecutionTime: time.Time{},
	}

	return svc
}

// Start starts the channel probe service with scheduled task.
func (svc *ChannelProbeService) Start(ctx context.Context) error {
	_, err := svc.Executor.ScheduleFuncAtCronRate(
		svc.runProbe,
		executors.CRONRule{Expr: "* * * * *"},
	)

	return err
}

// Stop stops the channel probe service.
func (svc *ChannelProbeService) Stop(ctx context.Context) error {
	return svc.Executor.Shutdown(ctx)
}

// shouldRunProbe determines if a probe should be executed based on frequency, current time, and last execution time.
// It returns true if the current aligned time is different from the last execution time.
// This is a pure function that does not depend on any external state.
func shouldRunProbe(frequency ProbeFrequency, now time.Time, lastExecution time.Time) bool {
	intervalMinutes := getIntervalMinutesFromFrequency(frequency)
	alignedTime := now.Truncate(time.Duration(intervalMinutes) * time.Minute)

	return !lastExecution.Equal(alignedTime)
}

// getIntervalMinutesFromFrequency returns the interval in minutes based on the probe frequency.
func getIntervalMinutesFromFrequency(frequency ProbeFrequency) int {
	switch frequency {
	case ProbeFrequency1Min:
		return 1
	case ProbeFrequency5Min:
		return 5
	case ProbeFrequency30Min:
		return 30
	case ProbeFrequency1Hour:
		return 60
	default:
		return 1
	}
}

type channelProbeStats struct {
	total                 int
	success               int
	avgTokensPerSecond    *float64
	avgTimeToFirstTokenMs *float64
}

// computeAllChannelProbeStats computes probe stats for all channels in a single batch query.
// This uses request_execution table for more accurate per-channel execution metrics,
// including retry attempts. This aligns with channel_performance data source.
func (svc *ChannelProbeService) computeAllChannelProbeStats(
	ctx context.Context,
	channelIDs []int,
	startTime time.Time,
	endTime time.Time,
) (map[int]*channelProbeStats, error) {
	if len(channelIDs) == 0 {
		return nil, nil
	}

	// Query 1: Get total and success counts per channel from request_execution
	type countResult struct {
		ChannelID    int `json:"channel_id"`
		TotalCount   int `json:"total_count"`
		SuccessCount int `json:"success_count"`
	}

	var countRes []countResult

	err := svc.db.RequestExecution.Query().
		Where(
			requestexecution.ChannelIDIn(channelIDs...),
			requestexecution.CreatedAtGTE(startTime),
			requestexecution.CreatedAtLT(endTime),
			requestexecution.StatusNotIn(requestexecution.StatusPending, requestexecution.StatusProcessing),
		).
		Modify(func(s *sql.Selector) {
			s.Select(
				s.C(requestexecution.FieldChannelID),
				sql.As(sql.Count("*"), "total_count"),
				sql.As("SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)", "success_count"),
			).GroupBy(s.C(requestexecution.FieldChannelID))
		}).
		Scan(ctx, &countRes)
	if err != nil {
		return nil, err
	}

	// Initialize result map
	result := make(map[int]*channelProbeStats)
	for _, r := range countRes {
		result[r.ChannelID] = &channelProbeStats{
			total:   r.TotalCount,
			success: r.SuccessCount,
		}
	}

	// Query 2: Get latency and first token latency per channel from request_execution
	type latencyResult struct {
		ChannelID                int   `json:"channel_id"`
		TotalLatencyMs           int64 `json:"total_latency_ms"`
		TotalFirstTokenLatencyMs int64 `json:"total_first_token_latency_ms"`
		StreamingCount           int   `json:"streaming_count"`
	}

	var latencyRes []latencyResult

	err = svc.db.RequestExecution.Query().
		Where(
			requestexecution.ChannelIDIn(channelIDs...),
			requestexecution.CreatedAtGTE(startTime),
			requestexecution.CreatedAtLT(endTime),
			requestexecution.StatusEQ(requestexecution.StatusCompleted),
		).
		Modify(func(s *sql.Selector) {
			s.Select(
				s.C(requestexecution.FieldChannelID),
				sql.As(sql.Sum(s.C(requestexecution.FieldMetricsLatencyMs)), "total_latency_ms"),
				sql.As(sql.Sum(s.C(requestexecution.FieldMetricsFirstTokenLatencyMs)), "total_first_token_latency_ms"),
				sql.As("SUM(CASE WHEN metrics_first_token_latency_ms IS NOT NULL THEN 1 ELSE 0 END)", "streaming_count"),
			).GroupBy(s.C(requestexecution.FieldChannelID))
		}).
		Scan(ctx, &latencyRes)
	if err != nil {
		return nil, err
	}

	// Build latency map for later use
	latencyMap := make(map[int]*latencyResult)
	for i := range latencyRes {
		latencyMap[latencyRes[i].ChannelID] = &latencyRes[i]
	}

	// Query 3: Get completion tokens per channel from usage_log
	// Note: usage_log is linked to request, not request_execution, so we filter by channel_id and time range
	type tokenResult struct {
		ChannelID        int   `json:"channel_id"`
		CompletionTokens int64 `json:"completion_tokens"`
	}

	var tokenRes []tokenResult

	err = svc.db.UsageLog.Query().
		Where(
			usagelog.ChannelIDIn(channelIDs...),
			usagelog.CreatedAtGTE(startTime),
			usagelog.CreatedAtLT(endTime),
		).
		Modify(func(s *sql.Selector) {
			s.Select(
				s.C(usagelog.FieldChannelID),
				sql.As(sql.Sum(s.C(usagelog.FieldCompletionTokens)), "completion_tokens"),
			).GroupBy(s.C(usagelog.FieldChannelID))
		}).
		Scan(ctx, &tokenRes)
	if err != nil {
		return nil, err
	}

	// Build completion token map
	completionTokenMap := make(map[int]int64)
	for _, r := range tokenRes {
		completionTokenMap[r.ChannelID] = r.CompletionTokens
	}

	// Compute derived metrics
	for channelID, stats := range result {
		latency := latencyMap[channelID]
		completionTokens := completionTokenMap[channelID]

		// Calculate avg tokens per second
		if latency != nil && latency.TotalLatencyMs > 0 && completionTokens > 0 {
			tps := float64(completionTokens) / (float64(latency.TotalLatencyMs) / 1000.0)
			stats.avgTokensPerSecond = &tps
		}

		// Calculate avg time to first token
		if latency != nil && latency.StreamingCount > 0 {
			avgTTFT := float64(latency.TotalFirstTokenLatencyMs) / float64(latency.StreamingCount)
			stats.avgTimeToFirstTokenMs = &avgTTFT
		}
	}

	return result, nil
}

// runProbe executes the probe task.
func (svc *ChannelProbeService) runProbe(ctx context.Context) {
	// Check if probe is enabled
	setting := svc.SystemService.ChannelSettingOrDefault(ctx)
	if !setting.Probe.Enabled {
		log.Debug(ctx, "Channel probe is disabled, skipping")
		return
	}

	intervalMinutes := setting.Probe.GetIntervalMinutes()
	now := xtime.UTCNow()
	// Align current time to interval boundary
	alignedTime := now.Truncate(time.Duration(intervalMinutes) * time.Minute)
	timestamp := alignedTime.Unix()

	// Check if we should execute based on last execution time
	svc.mu.Lock()

	lastExecution := svc.lastExecutionTime
	if !lastExecution.IsZero() && !shouldRunProbe(setting.Probe.Frequency, now, lastExecution) {
		// Already executed for this interval
		svc.mu.Unlock()
		log.Debug(ctx, "Skipping probe, already executed for this interval",
			log.Int64("timestamp", timestamp),
		)

		return
	}
	// Update last execution time
	svc.lastExecutionTime = alignedTime
	svc.mu.Unlock()

	ctx = ent.NewContext(ctx, svc.db)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	log.Debug(ctx, "Starting channel probe",
		log.Int("interval_minutes", intervalMinutes),
		log.Int64("timestamp", timestamp),
	)

	// Get all enabled channels
	channels, err := svc.db.Channel.Query().
		Where(channel.StatusEQ(channel.StatusEnabled)).
		Select(channel.FieldID).
		All(ctx)
	if err != nil {
		log.Error(ctx, "Failed to query enabled channels", log.Cause(err))
		return
	}

	if len(channels) == 0 {
		log.Debug(ctx, "No enabled channels to probe")
		return
	}

	// Calculate time range based on frequency
	startTime := alignedTime.Add(-time.Duration(intervalMinutes) * time.Minute)

	// Extract channel IDs for batch query
	channelIDs := make([]int, len(channels))
	for i, ch := range channels {
		channelIDs[i] = ch.ID
	}

	// Batch compute all channel stats in 3 queries instead of N*4 queries
	allStats, err := svc.computeAllChannelProbeStats(ctx, channelIDs, startTime, alignedTime)
	if err != nil {
		log.Error(ctx, "Failed to compute channel probe stats", log.Cause(err))
		return
	}

	// Collect probe data for each channel
	var probes []*ent.ChannelProbeCreate

	for _, ch := range channels {
		stats, ok := allStats[ch.ID]
		if !ok || stats.total == 0 {
			continue
		}

		probes = append(probes, svc.db.ChannelProbe.Create().
			SetChannelID(ch.ID).
			SetTotalRequestCount(stats.total).
			SetSuccessRequestCount(stats.success).
			SetNillableAvgTokensPerSecond(stats.avgTokensPerSecond).
			SetNillableAvgTimeToFirstTokenMs(stats.avgTimeToFirstTokenMs).
			SetTimestamp(timestamp),
		)
	}

	if len(probes) == 0 {
		log.Debug(ctx, "No probe data to store (all channels have 0 requests)")
		return
	}

	// Bulk create probes
	if err := svc.db.ChannelProbe.CreateBulk(probes...).Exec(ctx); err != nil {
		log.Error(ctx, "Failed to create channel probes", log.Cause(err))
		return
	}

	log.Debug(ctx, "Channel probe completed",
		log.Int("channels_probed", len(probes)),
		log.Int64("timestamp", timestamp),
	)
}

// generateTimestamps generates a slice of Unix timestamps from startTime to endTime
// with the given interval in minutes.
func generateTimestamps(setting ChannelProbeSetting, currentTime time.Time) []int64 {
	intervalMinutes := setting.GetIntervalMinutes()
	rangeMinutes := setting.GetQueryRangeMinutes()
	endTime := currentTime.Truncate(time.Duration(intervalMinutes) * time.Minute)
	startTime := endTime.Add(-time.Duration(rangeMinutes) * time.Minute)

	var timestamps []int64
	for t := startTime.Unix(); t <= endTime.Unix(); t += int64(intervalMinutes * 60) {
		timestamps = append(timestamps, t)
	}

	return timestamps
}

// QueryChannelProbes queries probe data for multiple channels with time range alignment.
func (svc *ChannelProbeService) QueryChannelProbes(ctx context.Context, channelIDs []int) ([]*ChannelProbeData, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	setting := svc.SystemService.ChannelSettingOrDefault(ctx)
	rangeMinutes := setting.Probe.GetQueryRangeMinutes()
	intervalMinutes := setting.Probe.GetIntervalMinutes()
	now := xtime.UTCNow()
	// Align end time to interval boundary
	endTime := now.Truncate(time.Duration(intervalMinutes) * time.Minute)
	startTime := endTime.Add(-time.Duration(rangeMinutes) * time.Minute)

	// Query all probes for the given channels in the time range
	probes, err := svc.db.ChannelProbe.Query().
		Where(
			channelprobe.ChannelIDIn(channelIDs...),
			channelprobe.TimestampGTE(startTime.Unix()),
			channelprobe.TimestampLTE(endTime.Unix()),
		).
		Order(ent.Asc(channelprobe.FieldTimestamp)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// Build a map of channel_id -> timestamp -> probe
	probeMap := make(map[int]map[int64]*ent.ChannelProbe)
	for _, p := range probes {
		if probeMap[p.ChannelID] == nil {
			probeMap[p.ChannelID] = make(map[int64]*ent.ChannelProbe)
		}

		probeMap[p.ChannelID][p.Timestamp] = p
	}

	// Generate all expected timestamps
	timestamps := generateTimestamps(setting.Probe, now)

	// Build result with aligned data (fill missing points with 0)
	result := make([]*ChannelProbeData, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		points := make([]*ChannelProbePoint, 0, len(timestamps))
		channelProbes := probeMap[channelID]

		for _, ts := range timestamps {
			if p, ok := channelProbes[ts]; ok {
				points = append(points, &ChannelProbePoint{
					Timestamp:             ts,
					TotalRequestCount:     p.TotalRequestCount,
					SuccessRequestCount:   p.SuccessRequestCount,
					AvgTokensPerSecond:    p.AvgTokensPerSecond,
					AvgTimeToFirstTokenMs: p.AvgTimeToFirstTokenMs,
				})
			} else {
				// Fill missing point with 0
				points = append(points, &ChannelProbePoint{
					Timestamp:           ts,
					TotalRequestCount:   0,
					SuccessRequestCount: 0,
				})
			}
		}

		result = append(result, &ChannelProbeData{
			ChannelID: channelID,
			Points:    points,
		})
	}

	return result, nil
}

// RunProbeNow manually triggers the probe task.
func (svc *ChannelProbeService) RunProbeNow(ctx context.Context) {
	svc.runProbe(ctx)
}

// GetProbesByChannelID returns probe data for a single channel.
func (svc *ChannelProbeService) GetProbesByChannelID(ctx context.Context, channelID int) ([]*ChannelProbePoint, error) {
	data, err := svc.QueryChannelProbes(ctx, []int{channelID})
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return []*ChannelProbePoint{}, nil
	}

	return data[0].Points, nil
}

// GetChannelProbeDataInput is the input for batch query.
type GetChannelProbeDataInput struct {
	ChannelIDs []int `json:"channel_ids"`
}

// BatchQueryChannelProbes is an alias for QueryChannelProbes for GraphQL.
func (svc *ChannelProbeService) BatchQueryChannelProbes(ctx context.Context, input GetChannelProbeDataInput) ([]*ChannelProbeData, error) {
	if len(input.ChannelIDs) == 0 {
		return []*ChannelProbeData{}, nil
	}

	return svc.QueryChannelProbes(ctx, lo.Map(input.ChannelIDs, func(id int, _ int) int {
		return id
	}))
}
