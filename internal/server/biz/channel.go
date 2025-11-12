package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/llm/transformer/doubao"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/llm/transformer/openrouter"
	"github.com/looplj/axonhub/internal/llm/transformer/xai"
	"github.com/looplj/axonhub/internal/llm/transformer/zai"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

type Channel struct {
	*ent.Channel

	// Outbound is the outbound transformer for the channel.
	Outbound transformer.Outbound

	// cachedOverrideParams stores the parsed override parameters to avoid repeated JSON parsing
	cachedOverrideParams map[string]any
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
	if c.cachedOverrideParams != nil {
		return c.cachedOverrideParams
	}

	if c.Settings == nil || c.Settings.OverrideParameters == "" {
		c.cachedOverrideParams = make(map[string]any)
		return c.cachedOverrideParams
	}

	var overrideParams map[string]any
	if err := json.Unmarshal([]byte(c.Settings.OverrideParameters), &overrideParams); err != nil {
		// If parsing fails, return empty map and log the error
		log.Warn(context.Background(), "failed to parse override parameters",
			log.String("channel", c.Name),
			log.Cause(err),
		)
		c.cachedOverrideParams = make(map[string]any)

		return c.cachedOverrideParams
	}

	c.cachedOverrideParams = overrideParams

	return c.cachedOverrideParams
}

type ChannelServiceParams struct {
	fx.In

	Executor executors.ScheduledExecutor
	Client   *ent.Client
}

func NewChannelService(params ChannelServiceParams) *ChannelService {
	svc := &ChannelService{
		Executors: params.Executor,
		Ent:       params.Client,
	}

	xerrors.NoErr(svc.loadChannels(context.Background()))
	xerrors.NoErr2(
		params.Executor.ScheduleFuncAtCronRate(
			svc.loadChannelsPeriodic,
			executors.CRONRule{Expr: "*/1 * * * *"},
		),
	)

	return svc
}

type ChannelService struct {
	Executors executors.ScheduledExecutor
	Ent       *ent.Client

	// latestUpdate 记录最新的 channel 更新时间，用于优化定时加载
	EnabledChannels []*Channel
	latestUpdate    time.Time
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

func (svc *ChannelService) buildChannel(c *ent.Channel) (*Channel, error) {
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
			Channel:  c,
			Outbound: transformer,
		}, nil
	case channel.TypeOpenrouter:
		transformer, err := openrouter.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:  c,
			Outbound: transformer,
		}, nil
	case channel.TypeZai, channel.TypeZhipu:
		transformer, err := zai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:  c,
			Outbound: transformer,
		}, nil
	case channel.TypeXai:
		transformer, err := xai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:  c,
			Outbound: transformer,
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
			Channel:  c,
			Outbound: transformer,
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
			Channel:  c,
			Outbound: transformer,
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
			Channel:  c,
			Outbound: transformer,
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
			Channel:  c,
			Outbound: transformer,
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
			Channel:  c,
			Outbound: transformer,
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
			Channel:  c,
			Outbound: transformer,
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
			Channel:  c,
			Outbound: transformer,
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
	case channel.TypeOpenai,
		channel.TypeDeepseek, channel.TypeMoonshot, channel.TypeLongcat, channel.TypeMinimax,
		channel.TypeGeminiOpenai,
		channel.TypePpio, channel.TypeSiliconflow, channel.TypeVolcengine,
		channel.TypeVercel, channel.TypeAihubmix:
		transformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
			Type:    openai.PlatformOpenAI,
			BaseURL: c.BaseURL,
			APIKey:  c.Credentials.APIKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		return &Channel{
			Channel:  c,
			Outbound: transformer,
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

// BulkUpdateChannelOrdering updates the ordering weight for multiple channels in a single transaction.
func (svc *ChannelService) BulkUpdateChannelOrdering(ctx context.Context, updates []struct {
	ID             int
	OrderingWeight int
},
) ([]*ent.Channel, error) {
	tx, err := svc.Ent.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	updatedChannels := make([]*ent.Channel, 0, len(updates))

	for _, update := range updates {
		channel, err := tx.Channel.
			UpdateOneID(update.ID).
			SetOrderingWeight(update.OrderingWeight).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to update channel %d: %w", update.ID, err)
		}

		updatedChannels = append(updatedChannels, channel)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Reload channels to ensure the in-memory cache reflects the new ordering
	go func() {
		if reloadErr := svc.loadChannels(context.Background()); reloadErr != nil {
			log.Error(context.Background(), "failed to reload channels after ordering update", log.Cause(reloadErr))
		}
	}()

	return updatedChannels, nil
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

// CreateChannel creates a new channel with the provided input.
func (svc *ChannelService) CreateChannel(ctx context.Context, input *ent.CreateChannelInput) (*ent.Channel, error) {
	// Check if a channel with the same name already exists
	existing, err := svc.Ent.Channel.Query().
		Where(channel.Name(input.Name)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check channel name: %w", err)
	}

	if existing != nil {
		return nil, fmt.Errorf("channel with name '%s' already exists", input.Name)
	}

	channel, err := svc.Ent.Channel.Create().
		SetType(input.Type).
		SetNillableBaseURL(input.BaseURL).
		SetName(input.Name).
		SetCredentials(input.Credentials).
		SetSupportedModels(input.SupportedModels).
		SetDefaultTestModel(input.DefaultTestModel).
		SetSettings(input.Settings).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

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

	return channel, nil
}

// BulkArchiveChannels updates the status of multiple channels to archived.
func (svc *ChannelService) BulkArchiveChannels(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	client := ent.FromContext(ctx)
	if client == nil {
		client = svc.Ent
	}

	// Verify all channels exist
	count, err := client.Channel.Query().
		Where(channel.IDIn(ids...)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to query channels: %w", err)
	}

	if count != len(ids) {
		return fmt.Errorf("expected to find %d channels, but found %d", len(ids), count)
	}

	// Update status to archived
	if _, err = client.Channel.Update().
		Where(channel.IDIn(ids...)).
		SetStatus(channel.StatusArchived).
		Save(ctx); err != nil {
		return fmt.Errorf("failed to archive channels: %w", err)
	}

	// Reload enabled channels to refresh in-memory cache
	go func() {
		if reloadErr := svc.loadChannels(context.Background()); reloadErr != nil {
			log.Error(context.Background(), "failed to reload channels after bulk archive", log.Cause(reloadErr))
		}
	}()

	return nil
}

// BulkImportChannelItem represents a single channel to be imported.
type BulkImportChannelItem struct {
	Type             string
	Name             string
	BaseURL          *string
	APIKey           *string
	SupportedModels  []string
	DefaultTestModel string
}

// BulkImportChannelsResult represents the result of bulk importing channels.
type BulkImportChannelsResult struct {
	Success  bool
	Created  int
	Failed   int
	Errors   []string
	Channels []*ent.Channel
}

// BulkImportChannels imports multiple channels at once.
func (svc *ChannelService) BulkImportChannels(ctx context.Context, items []BulkImportChannelItem) (*BulkImportChannelsResult, error) {
	var (
		createdChannels []*ent.Channel
		errors          []string
	)

	created := 0
	failed := 0

	for i, item := range items {
		// Validate channel type
		channelType := channel.Type(item.Type)
		if err := channel.TypeValidator(channelType); err != nil {
			errors = append(errors, fmt.Sprintf("Row %d: Invalid channel type '%s'", i+1, item.Type))
			failed++

			continue
		}

		// Validate required fields
		if item.BaseURL == nil || *item.BaseURL == "" {
			errors = append(errors, fmt.Sprintf("Row %d (%s): Base URL is required", i+1, item.Name))
			failed++

			continue
		}

		if item.APIKey == nil || *item.APIKey == "" {
			errors = append(errors, fmt.Sprintf("Row %d (%s): API Key is required", i+1, item.Name))
			failed++

			continue
		}

		// Prepare credentials (API key is now required)
		credentials := &objects.ChannelCredentials{
			APIKey: *item.APIKey,
		}

		// Create the channel (baseURL is now required)
		channelBuilder := svc.Ent.Channel.Create().
			SetType(channelType).
			SetName(item.Name).
			SetBaseURL(*item.BaseURL).
			SetCredentials(credentials).
			SetSupportedModels(item.SupportedModels).
			SetDefaultTestModel(item.DefaultTestModel)

		ch, err := channelBuilder.Save(ctx)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Row %d (%s): %s", i+1, item.Name, err.Error()))
			failed++

			continue
		}

		createdChannels = append(createdChannels, ch)
		created++
	}

	success := failed == 0

	return &BulkImportChannelsResult{
		Success:  success,
		Created:  created,
		Failed:   failed,
		Errors:   errors,
		Channels: createdChannels,
	}, nil
}

// TestChannelResult represents the result of a channel test.
type TestChannelResult struct {
	Latency float64
	Success bool
	Error   *string
}
