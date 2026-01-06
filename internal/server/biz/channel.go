package biz

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// ChannelModelEntry represents a model that the channel can handle.
type ChannelModelEntry struct {
	// RequestModel is the model name that can be used in requests
	RequestModel string

	// ActualModel is the model that will be sent to the provider
	ActualModel string

	// Source indicates how this model is supported
	Source string // "direct", "prefix", "auto_trim", "mapping"
}

type Channel struct {
	*ent.Channel

	// Outbound is the outbound transformer for the channel.
	Outbound transformer.Outbound

	// HTTPClient is the custom HTTP client for this channel with proxy support
	HTTPClient *httpclient.HttpClient

	// cachedOverrideParams stores the parsed override parameters to avoid repeated JSON parsing
	cachedOverrideParams map[string]any

	// cachedOverrideHeaders stores the parsed override headers to avoid repeated JSON parsing
	cachedOverrideHeaders []objects.HeaderEntry

	// cachedModelEntries caches GetModelEntries results
	// RequestModel -> Entry
	cachedModelEntries map[string]ChannelModelEntry
}

type ChannelServiceParams struct {
	fx.In

	Executor executors.ScheduledExecutor
	Ent      *ent.Client
}

func NewChannelService(params ChannelServiceParams) *ChannelService {
	svc := &ChannelService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		Executors: executors.NewPoolScheduleExecutor(
			executors.WithMaxConcurrent(1),
		),
		channelPerfMetrics: make(map[int]*channelMetrics),
		perfCh:             make(chan *PerformanceRecord, 1024),
	}

	xerrors.NoErr(svc.InitializeAllChannelPerformances(context.Background()))
	// Load channel performance metrics after channels are loaded
	if err := svc.LoadChannelPerformances(context.Background()); err != nil {
		log.Error(context.Background(), "failed to load channel performances", log.Cause(err))
		// Continue loading channels even if metrics loading fails
	}

	xerrors.NoErr(svc.loadChannels(context.Background()))
	xerrors.NoErr2(
		params.Executor.ScheduleFuncAtCronRate(
			svc.loadChannelsPeriodic,
			executors.CRONRule{Expr: "*/1 * * * *"},
		),
	)

	// Schedule model sync every hour
	xerrors.NoErr2(
		params.Executor.ScheduleFuncAtCronRate(
			svc.syncChannelModels,
			executors.CRONRule{Expr: "11 * * * *"},
		),
	)

	// Start performance metrics background flush
	go svc.startPerformanceProcess()

	return svc
}

type ChannelService struct {
	*AbstractService

	Executors executors.ScheduledExecutor

	// latestUpdate 记录最新的 channel 更新时间，用于优化定时加载
	enabledChannels []*Channel
	latestUpdate    time.Time

	// perfWindowSeconds is the configurable sliding window size for performance metrics (in seconds)
	// If not set (0), uses defaultPerformanceWindowSize (600 seconds = 10 minutes)
	perfWindowSeconds int64

	// channelPerfMetrics stores the performance metrics for each channel
	// protected by channelPerfMetricsLock
	channelPerfMetrics     map[int]*channelMetrics
	channelPerfMetricsLock sync.RWMutex

	// perfCh is the channel for performance records for async processing.
	perfCh chan *PerformanceRecord
}

func (svc *ChannelService) loadChannelsPeriodic(ctx context.Context) {
	err := svc.loadChannels(ctx)
	if err != nil {
		log.Error(ctx, "failed to load channels", log.Cause(err))
	}
}

func (svc *ChannelService) loadChannels(ctx context.Context) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// 检查是否有 channels 被修改
	latestUpdatedChannel, err := svc.entFromContext(ctx).Channel.Query().
		Order(ent.Desc(channel.FieldUpdatedAt)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}

	// 如果没有找到任何 channels，latestUpdate 会是 nil
	if latestUpdatedChannel != nil {
		// 如果最新的更新时间早于或等于我们记录的时间，说明没有新的修改
		if !latestUpdatedChannel.UpdatedAt.After(svc.latestUpdate) {
			log.Debug(ctx, "no new channels updated")
			return nil
		}
		// 更新最新的修改时间记录
		svc.latestUpdate = latestUpdatedChannel.UpdatedAt
	} else {
		// 如果没有 channels，确保 latestUpdate 是零值时间
		svc.latestUpdate = time.Time{}
	}

	entities, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusEQ(channel.StatusEnabled)).
		Order(ent.Desc(channel.FieldOrderingWeight)).
		All(ctx)
	if err != nil {
		return err
	}

	var channels []*Channel

	for _, c := range entities {
		channel, err := svc.buildChannel(c)
		if err != nil {
			log.Warn(ctx, "failed to build channel",
				log.String("channel", c.Name),
				log.String("type", c.Type.String()),
				log.Cause(err),
			)

			continue
		}

		// Preload override parameters
		overrideParams := channel.GetOverrideParameters()
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "created outbound transformer",
				log.String("channel", c.Name),
				log.String("type", c.Type.String()),
				log.Any("override_params", overrideParams),
			)
		}

		channels = append(channels, channel)
	}

	log.Info(ctx, "loaded channels", log.Int("count", len(channels)))

	svc.enabledChannels = channels

	return nil
}

// GetEnabledChannels returns all enabled channels.
// This method hides the internal field and provides a stable interface.
//
// WARNING: The returned slice and its elements are internal cached state.
// DO NOT modify the returned slice or any of its Channel elements.
// Modifications will not persist and may cause data inconsistency.
func (svc *ChannelService) GetEnabledChannels() []*Channel {
	return svc.enabledChannels
}

// GetChannelForTest retrieves a specific channel by ID for testing purposes,
// including disabled channels. This bypasses the normal enabled-only filtering.
func (svc *ChannelService) GetChannelForTest(ctx context.Context, channelID int) (*Channel, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Get the channel entity from database (including disabled ones)
	entity, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	return svc.buildChannel(entity)
}

// ListModelsInput represents the input for listing models with filters.
type ListModelsInput struct {
	StatusIn       []channel.Status
	IncludeMapping bool
	IncludePrefix  bool
}

// ModelIdentityWithStatus represents a model with its status.
type ModelIdentityWithStatus struct {
	ID     string
	Status channel.Status
}

var statusPriority = map[channel.Status]int{
	channel.StatusEnabled:  3,
	channel.StatusDisabled: 2,
	channel.StatusArchived: 1,
}

// setModelStatus updates the model status in the map with priority logic
// Priority: enabled > disabled > archived.
func setModelStatus(models map[string]channel.Status, modelID string, newStatus channel.Status) {
	if existingStatus, exists := models[modelID]; !exists || statusPriority[newStatus] > statusPriority[existingStatus] {
		models[modelID] = newStatus
	}
}

// ListModels returns all unique models across channels matching the filter criteria.
// It supports filtering by status and optionally including model mappings and prefixes.
func (svc *ChannelService) ListModels(ctx context.Context, input ListModelsInput) ([]*ModelIdentityWithStatus, error) {
	// Build query for channels
	query := svc.entFromContext(ctx).Channel.Query()

	// Apply status filter if provided, otherwise default to enabled
	if len(input.StatusIn) > 0 {
		query = query.Where(channel.StatusIn(input.StatusIn...))
	} else {
		query = query.Where(channel.StatusEQ(channel.StatusEnabled))
	}

	// Get all channels matching the filter
	channels, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}

	// Collect all unique models from channels with their status
	modelMap := make(map[string]channel.Status)

	for _, ch := range channels {
		// Add all supported models
		for _, modelID := range ch.SupportedModels {
			setModelStatus(modelMap, modelID, ch.Status)
		}

		// Add model mappings if requested
		if input.IncludeMapping && ch.Settings != nil {
			for _, mapping := range ch.Settings.ModelMappings {
				// Only add the mapping if the target model is supported
				if slices.Contains(ch.SupportedModels, mapping.To) {
					setModelStatus(modelMap, mapping.From, ch.Status)
				}
			}
		}

		// Add models with extra prefix if requested
		if input.IncludePrefix && ch.Settings != nil && ch.Settings.ExtraModelPrefix != "" {
			for _, modelID := range ch.SupportedModels {
				prefixedModel := ch.Settings.ExtraModelPrefix + "/" + modelID
				setModelStatus(modelMap, prefixedModel, ch.Status)
			}
		}
	}

	// Convert map to slice
	models := make([]*ModelIdentityWithStatus, 0, len(modelMap))
	for modelID, status := range modelMap {
		models = append(models, &ModelIdentityWithStatus{
			ID:     modelID,
			Status: status,
		})
	}

	return models, nil
}

// createChannel creates a new channel without triggering a reload.
// This is useful for batch operations where reload should happen once at the end.
func (svc *ChannelService) createChannel(ctx context.Context, input ent.CreateChannelInput) (*ent.Channel, error) {
	createBuilder := svc.entFromContext(ctx).Channel.Create().
		SetType(input.Type).
		SetNillableBaseURL(input.BaseURL).
		SetNillableRemark(input.Remark).
		SetName(input.Name).
		SetCredentials(input.Credentials).
		SetSupportedModels(input.SupportedModels).
		SetDefaultTestModel(input.DefaultTestModel).
		SetNillableAutoSyncSupportedModels(input.AutoSyncSupportedModels).
		SetSettings(input.Settings)

	if input.Tags != nil {
		createBuilder.SetTags(input.Tags)
	}

	channel, err := createBuilder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	// Initialize ChannelPerformance record for the new channel
	if err := svc.InitializeChannelPerformance(ctx, channel.ID); err != nil {
		return nil, fmt.Errorf("failed to initialize channel performance record: %w", err)
	}

	return channel, nil
}

// CreateChannel creates a new channel with the provided input.
func (svc *ChannelService) CreateChannel(ctx context.Context, input ent.CreateChannelInput) (*ent.Channel, error) {
	// Check if a channel with the same name already exists
	existing, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.Name(input.Name)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check channel name: %w", err)
	}

	if existing != nil {
		return nil, fmt.Errorf("channel with name '%s' already exists", input.Name)
	}

	channel, err := svc.createChannel(ctx, input)
	if err != nil {
		return nil, err
	}

	svc.asyncReloadChannels()

	return channel, nil
}

// UpdateChannel updates an existing channel with the provided input.
func (svc *ChannelService) UpdateChannel(ctx context.Context, id int, input *ent.UpdateChannelInput) (*ent.Channel, error) {
	log.Debug(ctx, "UpdateChannel", log.Int("id", id), log.Any("input", input))

	// Check if name is being updated and if it conflicts with existing channels
	if input.Name != nil {
		existing, err := svc.entFromContext(ctx).Channel.Query().
			Where(
				channel.Name(*input.Name),
				channel.IDNEQ(id),
			).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, fmt.Errorf("failed to check channel name: %w", err)
		}

		if existing != nil {
			return nil, fmt.Errorf("channel with name '%s' already exists", *input.Name)
		}
	}

	mut := svc.entFromContext(ctx).Channel.UpdateOneID(id).
		SetNillableBaseURL(input.BaseURL).
		SetNillableName(input.Name).
		SetNillableDefaultTestModel(input.DefaultTestModel).
		SetNillableOrderingWeight(input.OrderingWeight).
		SetNillableAutoSyncSupportedModels(input.AutoSyncSupportedModels)

	if input.SupportedModels != nil {
		mut.SetSupportedModels(input.SupportedModels)
	}

	if input.Tags != nil {
		mut.SetTags(input.Tags)
	}

	if input.Settings != nil {
		// Always normalize and validate override parameters
		input.Settings.OverrideParameters = NormalizeOverrideParameters(input.Settings.OverrideParameters)
		if err := ValidateOverrideParameters(input.Settings.OverrideParameters); err != nil {
			return nil, fmt.Errorf("invalid override parameters: %w", err)
		}
		// Validate override headers
		if len(input.Settings.OverrideHeaders) > 0 {
			if err := ValidateOverrideHeaders(input.Settings.OverrideHeaders); err != nil {
				return nil, fmt.Errorf("invalid override headers: %w", err)
			}
		}

		mut.SetSettings(input.Settings)
	}

	if input.Credentials != nil {
		mut.SetCredentials(input.Credentials)
	}

	if input.Remark != nil {
		mut.SetRemark(*input.Remark)
	}

	if input.ClearRemark {
		mut.ClearRemark()
	}

	if input.ClearErrorMessage {
		mut.ClearErrorMessage()
	}

	channel, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update channel: %w", err)
	}

	svc.asyncReloadChannels()

	return channel, nil
}

// UpdateChannelStatus updates the status of a channel.
func (svc *ChannelService) UpdateChannelStatus(ctx context.Context, id int, status channel.Status) (*ent.Channel, error) {
	channel, err := svc.entFromContext(ctx).Channel.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update channel status: %w", err)
	}

	svc.asyncReloadChannels()

	return channel, nil
}

// For test, disable async reload.
var asyncReloadDisabled = false

func (svc *ChannelService) asyncReloadChannels() {
	if asyncReloadDisabled {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error(context.Background(), "panic in async reload channels", log.Any("panic", r))
			}
		}()

		reloadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Force reload by resetting latestUpdate timestamp
		svc.latestUpdate = time.Time{}

		if reloadErr := svc.loadChannels(reloadCtx); reloadErr != nil {
			log.Error(reloadCtx, "failed to reload channels after bulk update", log.Cause(reloadErr))
		}
	}()
}

// DeleteChannel deletes a channel by ID.
func (svc *ChannelService) DeleteChannel(ctx context.Context, id int) error {
	if err := svc.entFromContext(ctx).Channel.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	svc.asyncReloadChannels()

	return nil
}
