package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/llm/transformer/doubao"
	"github.com/looplj/axonhub/internal/llm/transformer/modelscope"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/llm/transformer/openrouter"
	"github.com/looplj/axonhub/internal/llm/transformer/xai"
	"github.com/looplj/axonhub/internal/llm/transformer/zai"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

type Channel struct {
	*ent.Channel

	// Outbound is the outbound transformer for the channel.
	Outbound transformer.Outbound

	// HTTPClient is the custom HTTP client for this channel with proxy support
	HTTPClient *httpclient.HttpClient

	// CachedOverrideParams stores the parsed override parameters to avoid repeated JSON parsing
	CachedOverrideParams map[string]any
}

func (c Channel) resolvePrefixedModel(model string) (string, bool) {
	if c.Settings == nil || c.Settings.ExtraModelPrefix == "" {
		return "", false
	}

	prefix := c.Settings.ExtraModelPrefix + "/"
	if !strings.HasPrefix(model, prefix) {
		return "", false
	}

	modelWithoutPrefix := model[len(prefix):]
	if !slices.Contains(c.SupportedModels, modelWithoutPrefix) {
		return "", false
	}

	return modelWithoutPrefix, true
}

func (c Channel) IsModelSupported(model string) bool {
	if slices.Contains(c.SupportedModels, model) {
		return true
	}

	if c.Settings == nil {
		return false
	}

	if _, ok := c.resolvePrefixedModel(model); ok {
		return true
	}

	for _, mapping := range c.Settings.ModelMappings {
		if mapping.From == model && slices.Contains(c.SupportedModels, mapping.To) {
			return true
		}
	}

	return false
}

// CustomizeExecutor implements pipeline.ChannelCustomizedExecutor interface
// This allows the channel to provide a custom HTTP client with proxy support.
func (c *Channel) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	if c.HTTPClient != nil {
		// Return the HTTP client as the executor for this channel
		return c.HTTPClient
	}
	// Fall back to the default executor if no custom HTTP client is configured
	return executor
}

func (c Channel) ChooseModel(model string) (string, error) {
	if slices.Contains(c.SupportedModels, model) {
		return model, nil
	}

	if c.Settings == nil {
		return "", fmt.Errorf("model %s not supported in channel %s", model, c.Name)
	}

	if resolved, ok := c.resolvePrefixedModel(model); ok {
		return resolved, nil
	}

	for _, mapping := range c.Settings.ModelMappings {
		if mapping.From == model && slices.Contains(c.SupportedModels, mapping.To) {
			return mapping.To, nil
		}
	}

	return "", fmt.Errorf("model %s not supported in channel %s", model, c.Name)
}

// GetOverrideParameters returns the cached override parameters for the channel.
// If the parameters haven't been parsed yet, it parses and caches them.
func (c *Channel) GetOverrideParameters() map[string]any {
	if c.CachedOverrideParams != nil {
		return c.CachedOverrideParams
	}

	if c.Settings == nil || c.Settings.OverrideParameters == "" {
		c.CachedOverrideParams = make(map[string]any)
		return c.CachedOverrideParams
	}

	var overrideParams map[string]any
	if err := json.Unmarshal([]byte(c.Settings.OverrideParameters), &overrideParams); err != nil {
		// If parsing fails, return empty map and log the error
		log.Warn(context.Background(), "failed to parse override parameters",
			log.String("channel", c.Name),
			log.Cause(err),
		)
		c.CachedOverrideParams = make(map[string]any)

		return c.CachedOverrideParams
	}

	c.CachedOverrideParams = overrideParams

	return c.CachedOverrideParams
}

type ChannelServiceParams struct {
	fx.In

	Executor executors.ScheduledExecutor
	Ent      *ent.Client
}

func NewChannelService(params ChannelServiceParams) *ChannelService {
	svc := &ChannelService{
		Executors:          params.Executor,
		Ent:                params.Ent,
		channelPerfMetrics: make(map[int]*channelMetrics),
		perfCh:             make(chan *PerformanceRecord, 1024),
	}

	xerrors.NoErr(svc.InitializeAllChannelPerformances(context.Background()))
	xerrors.NoErr(svc.loadChannels(context.Background()))
	xerrors.NoErr2(
		params.Executor.ScheduleFuncAtCronRate(
			svc.loadChannelsPeriodic,
			executors.CRONRule{Expr: "*/1 * * * *"},
		),
	)

	// Start performance metrics background flush
	go svc.startPerformanceProcess()

	return svc
}

type ChannelService struct {
	Ent       *ent.Client
	Executors executors.ScheduledExecutor

	// latestUpdate 记录最新的 channel 更新时间，用于优化定时加载
	EnabledChannels []*Channel
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
	latestUpdatedChannel, err := svc.Ent.Channel.Query().
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

	entities, err := svc.Ent.Channel.Query().
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
		log.Debug(ctx, "created outbound transformer",
			log.String("channel", c.Name),
			log.String("type", c.Type.String()),
			log.Any("override_params", overrideParams),
		)

		channels = append(channels, channel)
	}

	log.Info(ctx, "loaded channels", log.Int("count", len(channels)))

	svc.EnabledChannels = channels

	return nil
}

// getProxyConfig extracts proxy configuration from channel settings
// Returns nil if no proxy configuration is set (backward compatibility).
func getProxyConfig(channelSettings *objects.ChannelSettings) *objects.ProxyConfig {
	if channelSettings == nil || channelSettings.Proxy == nil {
		// Backward compatibility: default to environment proxy type
		return &objects.ProxyConfig{
			Type: objects.ProxyTypeEnvironment,
		}
	}

	return channelSettings.Proxy
}

//nolint:maintidx // Simple switch statement.
func (svc *ChannelService) buildChannel(c *ent.Channel) (*Channel, error) {
	httpClient := httpclient.NewHttpClientWithProxy(getProxyConfig(c.Settings))

	//nolint:exhaustive // TODO SUPPORT more providers.
	switch c.Type {
	case channel.TypeDoubao:
		transformer, err := doubao.NewOutboundTransformerWithConfig(&doubao.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeOpenrouter:
		transformer, err := openrouter.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeZai, channel.TypeZhipu:
		transformer, err := zai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeXai:
		transformer, err := xai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeAnthropic, channel.TypeLongcatAnthropic, channel.TypeMinimaxAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDirect,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeDeepseekAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDeepSeek,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeDoubaoAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformDoubao,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeMoonshotAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformMoonshot,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeZhipuAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformZhipu,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeZaiAnthropic:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformZai,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil

	case channel.TypeAnthropicAWS:
		// For anthropic_aws, we need to create a transformer with AWS credentials
		// The transformer will handle AWS Bedrock integration
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformBedrock,
			Region:          c.Credentials.AWS.Region,
			AccessKeyID:     c.Credentials.AWS.AccessKeyID,
			SecretAccessKey: c.Credentials.AWS.SecretAccessKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeAnthropicGcp:
		// For anthropic_vertex, we need to create a VertexTransformer with GCP credentials
		// The transformer will handle Google Vertex AI integration
		if c.Credentials.GCP == nil {
			return nil, errors.New("GCP credentials are required for anthropic_vertex channel")
		}

		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:      anthropic.PlatformVertex,
			Region:    c.Credentials.GCP.Region,
			ProjectID: c.Credentials.GCP.ProjectID,
			JSONData:  c.Credentials.GCP.JSONData,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeAnthropicFake:
		// For anthropic_fake, we use the fake transformer for testing
		fakeTransformer := anthropic.NewFakeTransformer()

		return &Channel{
			Channel:  c,
			Outbound: fakeTransformer,
		}, nil
	case channel.TypeOpenaiFake:
		fakeTransformer := openai.NewFakeTransformer()

		return &Channel{
			Channel:  c,
			Outbound: fakeTransformer,
		}, nil
	case channel.TypeModelscope:
		transformer, err := modelscope.NewOutboundTransformerWithConfig(&modelscope.Config{
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	case channel.TypeOpenai,
		channel.TypeDeepseek, channel.TypeMoonshot, channel.TypeLongcat, channel.TypeMinimax,
		channel.TypeGeminiOpenai,
		channel.TypePpio, channel.TypeSiliconflow, channel.TypeVolcengine,
		channel.TypeVercel, channel.TypeAihubmix, channel.TypeBurncloud:
		transformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
			Type:    openai.PlatformOpenAI,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:    c,
			Outbound:   transformer,
			HTTPClient: httpClient,
		}, nil
	default:
		return nil, errors.New("unknown channel type")
	}
}

func (svc *ChannelService) ChooseChannels(
	ctx context.Context,
	chatReq *llm.Request,
) ([]*Channel, error) {
	var channels []*Channel

	for _, channel := range svc.EnabledChannels {
		if channel.IsModelSupported(chatReq.Model) {
			channels = append(channels, channel)
		}
	}

	return channels, nil
}

// GetChannelForTest retrieves a specific channel by ID for testing purposes,
// including disabled channels. This bypasses the normal enabled-only filtering.
func (svc *ChannelService) GetChannelForTest(ctx context.Context, channelID int) (*Channel, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Get the channel entity from database (including disabled ones)
	entity, err := svc.Ent.Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	return svc.buildChannel(entity)
}

// ListAllModels returns all unique models across all enabled channels,
// considering model mappings. It returns both the original model names
// from SupportedModels and the "From" names from model mappings.
func (svc *ChannelService) ListAllModels(ctx context.Context) []objects.Model {
	modelSet := make(map[string]objects.Model)

	for _, ch := range svc.EnabledChannels {
		// Add all supported models
		for _, model := range ch.SupportedModels {
			if _, ok := modelSet[model]; ok {
				continue
			}

			modelSet[model] = objects.Model{
				ID:          model,
				DisplayName: model,
				CreatedAt:   ch.CreatedAt,
				Created:     ch.CreatedAt.Unix(),
				OwnedBy:     ch.Channel.Type.String(),
			}
		}

		// Add all "From" models from model mappings
		if ch.Settings != nil {
			for _, mapping := range ch.Settings.ModelMappings {
				// Only add the mapping if the target model is supported
				if slices.Contains(ch.SupportedModels, mapping.To) {
					if _, ok := modelSet[mapping.From]; ok {
						continue
					}

					modelSet[mapping.From] = objects.Model{
						ID:          mapping.From,
						DisplayName: mapping.From,
						CreatedAt:   ch.CreatedAt,
						Created:     ch.CreatedAt.Unix(),
						OwnedBy:     ch.Channel.Type.String(),
					}
				}
			}

			// Add models with extra prefix
			if ch.Settings.ExtraModelPrefix != "" {
				for _, model := range ch.SupportedModels {
					prefixedModel := ch.Settings.ExtraModelPrefix + "/" + model
					if _, ok := modelSet[prefixedModel]; ok {
						continue
					}

					modelSet[prefixedModel] = objects.Model{
						ID:          prefixedModel,
						DisplayName: prefixedModel,
						CreatedAt:   ch.CreatedAt,
						Created:     ch.CreatedAt.Unix(),
						OwnedBy:     ch.Channel.Type.String(),
					}
				}
			}
		}
	}

	return lo.Values(modelSet)
}

// createChannel creates a new channel without triggering a reload.
// This is useful for batch operations where reload should happen once at the end.
func (svc *ChannelService) createChannel(ctx context.Context, input ent.CreateChannelInput) (*ent.Channel, error) {
	createBuilder := ent.FromContext(ctx).Channel.Create().
		SetType(input.Type).
		SetNillableBaseURL(input.BaseURL).
		SetName(input.Name).
		SetCredentials(input.Credentials).
		SetSupportedModels(input.SupportedModels).
		SetDefaultTestModel(input.DefaultTestModel).
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
	existing, err := ent.FromContext(ctx).Channel.Query().
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
		existing, err := svc.Ent.Channel.Query().
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

	mut := svc.Ent.Channel.UpdateOneID(id).
		SetNillableBaseURL(input.BaseURL).
		SetNillableName(input.Name).
		SetNillableDefaultTestModel(input.DefaultTestModel)

	if input.SupportedModels != nil {
		mut.SetSupportedModels(input.SupportedModels)
	}

	if input.Tags != nil {
		mut.SetTags(input.Tags)
	}

	if input.Settings != nil {
		mut.SetSettings(input.Settings)
	}

	if input.Credentials != nil {
		mut.SetCredentials(input.Credentials)
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
	channel, err := svc.Ent.Channel.UpdateOneID(id).
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

		if reloadErr := svc.loadChannels(reloadCtx); reloadErr != nil {
			log.Error(reloadCtx, "failed to reload channels after bulk update", log.Cause(reloadErr))
		}
	}()
}

// DeleteChannel deletes a channel by ID.
func (svc *ChannelService) DeleteChannel(ctx context.Context, id int) error {
	if err := svc.Ent.Channel.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	svc.asyncReloadChannels()

	return nil
}
