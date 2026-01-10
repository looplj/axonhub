package biz

import (
	"context"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelprobe"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/log"
)

// ChannelProbePoint represents a single probe data point for a channel.
type ChannelProbePoint struct {
	Timestamp           int64 `json:"timestamp"`
	TotalRequestCount   int   `json:"total_request_count"`
	SuccessRequestCount int   `json:"success_request_count"`
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

	err := svc.Start(context.Background())
	if err != nil {
		panic(err)
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

// runProbe executes the probe task.
func (svc *ChannelProbeService) runProbe(ctx context.Context) {
	// Check if probe is enabled
	setting := svc.SystemService.ChannelSettingOrDefault(ctx)
	if !setting.Probe.Enabled {
		log.Debug(ctx, "Channel probe is disabled, skipping")
		return
	}

	intervalMinutes := setting.Probe.GetIntervalMinutes()
	now := time.Now()
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

	// Collect probe data for each channel
	var probes []*ent.ChannelProbeCreate

	for _, ch := range channels {
		// Count total and success requests in the time range (excluding pending/processing requests)
		total, err := svc.db.RequestExecution.Query().
			Where(
				requestexecution.ChannelIDEQ(ch.ID),
				requestexecution.CreatedAtGTE(startTime),
				requestexecution.CreatedAtLT(alignedTime),
				requestexecution.StatusNotIn(requestexecution.StatusPending, requestexecution.StatusProcessing),
			).
			Count(ctx)
		if err != nil {
			log.Error(ctx, "Failed to count total requests",
				log.Int("channel_id", ch.ID),
				log.Cause(err),
			)

			continue
		}

		// Skip if total is 0 (optimization: don't store zero data)
		if total == 0 {
			continue
		}

		success, err := svc.db.RequestExecution.Query().
			Where(
				requestexecution.ChannelIDEQ(ch.ID),
				requestexecution.CreatedAtGTE(startTime),
				requestexecution.CreatedAtLT(alignedTime),
				requestexecution.StatusEQ(requestexecution.StatusCompleted),
			).
			Count(ctx)
		if err != nil {
			log.Error(ctx, "Failed to count success requests",
				log.Int("channel_id", ch.ID),
				log.Cause(err),
			)

			continue
		}

		probes = append(probes, svc.db.ChannelProbe.Create().
			SetChannelID(ch.ID).
			SetTotalRequestCount(total).
			SetSuccessRequestCount(success).
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
	for t := startTime.Unix(); t < endTime.Unix(); t += int64(intervalMinutes * 60) {
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

	now := time.Now()
	// Align end time to interval boundary
	endTime := now.Truncate(time.Duration(intervalMinutes) * time.Minute)
	startTime := endTime.Add(-time.Duration(rangeMinutes) * time.Minute)

	// Query all probes for the given channels in the time range
	probes, err := svc.db.ChannelProbe.Query().
		Where(
			channelprobe.ChannelIDIn(channelIDs...),
			channelprobe.TimestampGTE(startTime.Unix()),
			channelprobe.TimestampLT(endTime.Unix()),
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
					Timestamp:           ts,
					TotalRequestCount:   p.TotalRequestCount,
					SuccessRequestCount: p.SuccessRequestCount,
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
