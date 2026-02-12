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
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/watcher"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
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

	startTokenProvider func()
	stopTokenProvider  func()

	// cachedOverrideOps stores the parsed override operations to avoid repeated JSON parsing
	cachedOverrideOps []objects.OverrideOperation

	// cachedOverrideHeaders stores the parsed override headers to avoid repeated JSON parsing
	cachedOverrideHeaders []objects.OverrideOperation

	// cachedModelEntries caches GetModelEntries results
	// RequestModel -> Entry
	cachedModelEntries map[string]ChannelModelEntry

	// cachedModelPrices caches model prices per request model id
	// RequestModel -> ChannelModelPrice entity (contains Price and ReferenceID)
	cachedModelPrices map[string]*ent.ChannelModelPrice

	// cachedEnabledAPIKeys caches enabled API keys (computed once when channel is loaded)
	cachedEnabledAPIKeys []string

	// cachedDisabledKeySet caches disabled key lookup set for O(1) check
	cachedDisabledKeySet map[string]struct{}
}

type ChannelServiceParams struct {
	fx.In

	CacheConfig   xcache.Config
	Executor      executors.ScheduledExecutor
	Ent           *ent.Client
	SystemService *SystemService
}

func NewChannelService(params ChannelServiceParams) *ChannelService {
	svc := &ChannelService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		Executors:          params.Executor,
		SystemService:      params.SystemService,
		channelPerfMetrics: make(map[int]*channelMetrics),
		channelErrorCounts: make(map[int]map[int]int),
		apiKeyErrorCounts:  make(map[int]map[string]map[int]int),
		perfCh:             make(chan *PerformanceRecord, 1024),
	}

	svc.initChannelPerformances(context.Background())

	watcherMode := params.CacheConfig.Mode
	if watcherMode == "" {
		watcherMode = xcache.ModeMemory
	}

	if watcherMode == xcache.ModeTwoLevel {
		watcherMode = watcher.ModeRedis
	}

	notifier, err := watcher.NewWatcherFromConfig[live.CacheEvent[struct{}]](watcher.Config{
		Mode:  watcherMode,
		Redis: params.CacheConfig.Redis,
	}, watcher.WatcherFromConfigOptions{
		RedisChannel: "axonhub:cache:channels",
		Buffer:       32,
	})
	if err != nil {
		panic(fmt.Errorf("channel watcher init failed: %w", err))
	}

	svc.channelNotifier = notifier

	svc.enabledChannelsCache = live.NewCache(live.Options[[]*Channel]{
		Name:            "axonhub:enabled_channels",
		InitialValue:    []*Channel{},
		RefreshInterval: time.Minute,
		RefreshFunc:     svc.onCacheRefreshed,
		OnSwap:          svc.onEnabledChannelsSwap,
		Watcher:         svc.channelNotifier,
	})
	xerrors.NoErr(svc.enabledChannelsCache.Load(context.Background(), true))

	// Schedule model sync every hour
	xerrors.NoErr2(svc.Executors.ScheduleFuncAtCronRate(svc.runSyncChannelModelsPeriodically, executors.CRONRule{Expr: "11 * * * *"}))

	// Start performance metrics background flush
	go svc.startPerformanceProcess()

	return svc
}

func (svc *ChannelService) Stop() {
	svc.enabledChannelsCache.Stop()
}

type ChannelService struct {
	*AbstractService

	Executors     executors.ScheduledExecutor
	SystemService *SystemService

	enabledChannelsCache *live.Cache[[]*Channel]
	channelNotifier      watcher.Notifier[live.CacheEvent[struct{}]]

	// perfWindowSeconds is the configurable sliding window size for performance metrics (in seconds)
	// If not set (0), uses defaultPerformanceWindowSize (600 seconds = 10 minutes)
	perfWindowSeconds int64

	// channelPerfMetrics stores the performance metrics for each channel
	// protected by channelPerfMetricsLock
	channelPerfMetrics     map[int]*channelMetrics
	channelPerfMetricsLock sync.RWMutex

	// channelErrorCounts stores the error counts for each channel and status code
	// channelID -> statusCode -> count
	channelErrorCounts     map[int]map[int]int
	channelErrorCountsLock sync.Mutex

	// apiKeyErrorCounts stores the error counts for each API key and status code
	// channelID -> apiKey -> statusCode -> count
	apiKeyErrorCounts     map[int]map[string]map[int]int
	apiKeyErrorCountsLock sync.Mutex

	// perfCh is the channel for performance records for async processing.
	perfCh chan *PerformanceRecord
}

func (svc *ChannelService) reloadEnabledChannels(ctx context.Context, current []*Channel, lastUpdate time.Time) ([]*Channel, time.Time, bool, error) {
	// Query latest updated channel including soft-deleted ones to detect deletions
	latestUpdatedChannel, err := svc.entFromContext(ctx).Channel.Query().
		Order(ent.Desc(channel.FieldUpdatedAt)).
		First(schematype.SkipSoftDelete(ctx))
	if err != nil && !ent.IsNotFound(err) {
		return current, lastUpdate, false, err
	}

	if latestUpdatedChannel == nil {
		if lastUpdate.IsZero() && len(current) == 0 {
			return current, time.Time{}, false, nil
		}
	} else if !latestUpdatedChannel.UpdatedAt.After(lastUpdate) {
		log.Debug(ctx, "no new channels updated")
		return current, lastUpdate, false, nil
	}

	entities, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusEQ(channel.StatusEnabled)).
		Order(ent.Desc(channel.FieldOrderingWeight)).
		All(ctx)
	if err != nil {
		return current, lastUpdate, false, err
	}

	var channels []*Channel

	for _, c := range entities {
		channel, err := svc.buildChannelWithTransformer(c)
		if err != nil {
			log.Warn(ctx, "failed to build channel",
				log.String("channel", c.Name),
				log.String("type", c.Type.String()),
				log.Cause(err),
			)

			continue
		}

		// Preload override parameters
		overrideParams := channel.GetBodyOverrideOperations()
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "created outbound transformer",
				log.String("channel", c.Name),
				log.String("type", c.Type.String()),
				log.Any("override_params", overrideParams),
			)
		}

		// Preload model prices
		svc.preloadModelPrices(ctx, channel)

		channels = append(channels, channel)
	}

	log.Info(ctx, "loaded channels", log.Int("count", len(channels)))

	updateTime := time.Time{}
	if latestUpdatedChannel != nil {
		updateTime = latestUpdatedChannel.UpdatedAt
	}

	return channels, updateTime, true, nil
}

func (svc *ChannelService) onEnabledChannelsSwap(old, new []*Channel) {
	for _, ch := range new {
		if ch != nil && ch.startTokenProvider != nil {
			ch.startTokenProvider()
		}
	}

	for _, ch := range old {
		if ch != nil && ch.stopTokenProvider != nil {
			ch.stopTokenProvider()
		}
	}
}

// GetEnabledChannels returns all enabled channels.
// This method hides the internal field and provides a stable interface.
//
// WARNING: The returned slice and its elements are internal cached state.
// DO NOT modify the returned slice or any of its Channel elements.
// Modifications will not persist and may cause data inconsistency.
func (svc *ChannelService) GetEnabledChannels() []*Channel {
	return svc.enabledChannelsCache.GetData()
}

// GetEnabledChannel returns the enabled channel by id, or nil if not found.
func (svc *ChannelService) GetEnabledChannel(id int) *Channel {
	for _, ch := range svc.GetEnabledChannels() {
		if ch.ID == id {
			return ch
		}
	}

	return nil
}

func (svc *ChannelService) SetEnabledChannelsForTest(channels []*Channel) {
	svc.enabledChannelsCache.Stop()

	svc.enabledChannelsCache = live.NewCache(live.Options[[]*Channel]{
		Name:            "enabled_channels_test",
		InitialValue:    channels,
		RefreshInterval: 24 * time.Hour,
		RefreshFunc: func(ctx context.Context, current []*Channel, lastUpdate time.Time) ([]*Channel, time.Time, bool, error) {
			return current, lastUpdate, false, nil
		},
	})
}

// GetChannel retrieves a specific channel by ID for testing purposes,
// including disabled channels. This bypasses the normal enabled-only filtering.
func (svc *ChannelService) GetChannel(ctx context.Context, channelID int) (*Channel, error) {
	// Get the channel entity from database (including disabled ones)
	entity, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	return svc.buildChannelWithTransformer(entity)
}

// ListModelsInput represents the input for listing models with filters.
type ListModelsInput struct {
	StatusIn                []channel.Status
	IncludeAllChannelModels bool
	IncludeMapping          bool
	IncludePrefix           bool
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
		if input.IncludeAllChannelModels {
			// Use GetModelEntries to get all model entries (including mapping, prefix, auto_trim)
			bizCh := &Channel{Channel: ch}

			entries := bizCh.GetModelEntries()
			for requestModel := range entries {
				setModelStatus(modelMap, requestModel, ch.Status)
			}
		} else {
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
	if input.Settings != nil {
		if input.Settings.BodyOverrideOperations != nil {
			if err := ValidateBodyOverrideOperations(input.Settings.BodyOverrideOperations); err != nil {
				return nil, fmt.Errorf("invalid body override operations: %w", err)
			}
		}

		if input.Settings.HeaderOverrideOperations != nil {
			if err := ValidateOverrideHeaders(input.Settings.HeaderOverrideOperations); err != nil {
				return nil, fmt.Errorf("invalid header override operations: %w", err)
			}
		}
	}

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

	if input.Policies != nil {
		createBuilder.SetPolicies(*input.Policies)
	}

	channel, err := createBuilder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
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

	if input.ManualModels != nil {
		mut.SetManualModels(input.ManualModels)
	}

	if input.Tags != nil {
		mut.SetTags(input.Tags)
	}

	if input.Settings != nil {
		// Always normalize and validate override settings.
		if input.Settings.BodyOverrideOperations != nil {
			if err := ValidateBodyOverrideOperations(input.Settings.BodyOverrideOperations); err != nil {
				return nil, fmt.Errorf("invalid body override operations: %w", err)
			}
		}

		if input.Settings.HeaderOverrideOperations != nil {
			if err := ValidateOverrideHeaders(input.Settings.HeaderOverrideOperations); err != nil {
				return nil, fmt.Errorf("invalid header override operations: %w", err)
			}
		}

		mut.SetSettings(input.Settings)
	}

	if input.Policies != nil {
		mut.SetPolicies(*input.Policies)
	}

	if input.Credentials != nil {
		mut.SetCredentials(*input.Credentials)
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

	if err := svc.channelNotifier.Notify(context.Background(), live.NewForceRefreshEvent[struct{}]()); err != nil {
		log.Warn(context.Background(), "channel cache watcher notify failed", log.Cause(err))
	}
}

// DeleteChannel deletes a channel by ID.
func (svc *ChannelService) DeleteChannel(ctx context.Context, id int) error {
	if err := svc.entFromContext(ctx).Channel.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	svc.asyncReloadChannels()

	return nil
}

// GetEnabledAPIKeys returns cached enabled API keys.
func (c *Channel) GetEnabledAPIKeys() []string {
	return c.cachedEnabledAPIKeys
}

// IsAPIKeyDisabled checks if a key is disabled (O(1) lookup).
func (c *Channel) IsAPIKeyDisabled(key string) bool {
	_, ok := c.cachedDisabledKeySet[key]
	return ok
}
