package biz

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"entgo.io/ent/dialect"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelprobe"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/gql/qb"
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
		svc.runProbePeriodically,
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
// Uses CTE with ROW_NUMBER to get only successful execution per request, includes all token types,
// and applies different TPS formulas for streaming vs non-streaming.
func (svc *ChannelProbeService) computeAllChannelProbeStats(
	ctx context.Context,
	channelIDs []int,
	startTime time.Time,
	endTime time.Time,
) (map[int]*channelProbeStats, error) {
	if len(channelIDs) == 0 {
		return nil, nil
	}

	// Use raw SQL query with CTE pattern (same as Task 1 FastestChannels)
	type probeResult struct {
		ChannelID              int   `json:"channel_id"`
		TotalCount             int   `json:"total_count"`
		SuccessCount           int   `json:"success_count"`
		TotalTokens            int64 `json:"total_tokens"`
		EffectiveLatencyMs     int64 `json:"effective_latency_ms"`
		TotalFirstTokenLatency int64 `json:"total_first_token_latency"`
		RequestCount           int   `json:"request_count"`
		StreamingRequestCount  int   `json:"streaming_request_count"`
	}

	dbDriver := svc.db.Driver()
	sqlDB, ok := dbDriver.(*entsql.Driver)
	if !ok {
		return nil, fmt.Errorf("failed to get underlying SQL driver")
	}

	// Detect dialect to use appropriate placeholder syntax
	// PostgreSQL uses $1, $2, etc. while SQLite uses ? placeholders
	dialectName := sqlDB.Dialect()
	useDollarPlaceholders := dialectName == dialect.Postgres

	// Build args slice for parameterized query
	args := make([]interface{}, 0, len(channelIDs)+2)
	args = append(args, startTime.UTC(), endTime.UTC())

	// Build channel ID filter with dialect-aware parameterized placeholders
	// Note: Placeholders start at $3 because $1 and $2 are reserved for startTime and endTime timestamps.
	// The args slice is constructed with timestamps first (lines 155-156), then channel IDs appended,
	// so placeholder numbering must match this ordering to bind values correctly.
	channelIDFilter := ""
	if len(channelIDs) > 0 {
		placeholders := make([]string, len(channelIDs))
		for i, id := range channelIDs {
			if useDollarPlaceholders {
				placeholders[i] = fmt.Sprintf("$%d", i+3) // $3, $4, etc. for PostgreSQL (offset by 2 for timestamps)
			} else {
				placeholders[i] = "?" // ? for SQLite
			}
			args = append(args, id)
		}
		channelIDFilter = fmt.Sprintf("AND se.channel_id IN (%s)", strings.Join(placeholders, ","))
	}

	queryMode := qb.ThroughputModeRowNumber
	if !useDollarPlaceholders {
		queryMode = qb.ThroughputModeMaxID
	}

	query := qb.BuildProbeStatsQuery(useDollarPlaceholders, channelIDFilter, queryMode)

	rows, err := sqlDB.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query channel probe stats: %w", err)
	}

	defer func() { _ = rows.Close() }()

	result := make(map[int]*channelProbeStats)
	for rows.Next() {
		var r probeResult
		if err := rows.Scan(
			&r.ChannelID,
			&r.TotalCount,
			&r.SuccessCount,
			&r.TotalTokens,
			&r.EffectiveLatencyMs,
			&r.TotalFirstTokenLatency,
			&r.RequestCount,
			&r.StreamingRequestCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan probe result: %w", err)
		}

		stats := &channelProbeStats{
			total:   r.TotalCount,
			success: r.SuccessCount,
		}

		// Calculate avg tokens per second using effective latency
		// For streaming: tokens / ((latency - first_token_latency) / 1000)
		// For non-streaming: tokens / (latency / 1000)
		if r.TotalTokens > 0 && r.EffectiveLatencyMs > 0 {
			tps := float64(r.TotalTokens) / (float64(r.EffectiveLatencyMs) / 1000.0)
			stats.avgTokensPerSecond = &tps
		}

		// Calculate avg time to first token (only for streaming requests)
		if r.TotalFirstTokenLatency > 0 && r.StreamingRequestCount > 0 {
			avgTTFT := float64(r.TotalFirstTokenLatency) / float64(r.StreamingRequestCount)
			stats.avgTimeToFirstTokenMs = &avgTTFT
		}

		result[r.ChannelID] = stats
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating probe results: %w", err)
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
	setting, err := authz.RunWithScopeDecision(ctx, scopes.ScopeReadChannels, func(ctx context.Context) (*SystemChannelSettings, error) {
		return svc.SystemService.ChannelSettingOrDefault(ctx), nil
	})
	if err != nil {
		return nil, err
	}
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

	return svc.QueryChannelProbes(ctx, input.ChannelIDs)
}
