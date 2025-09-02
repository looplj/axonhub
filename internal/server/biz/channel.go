package biz

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/llm/transformer/zai"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

type Channel struct {
	*ent.Channel

	// Outbound is the outbound transformer for the channel.
	Outbound transformer.Outbound
}

func (c Channel) IsModelSupported(model string) bool {
	if slices.Contains(c.SupportedModels, model) {
		return true
	}

	if c.Settings == nil {
		return false
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

	for _, mapping := range c.Settings.ModelMappings {
		if mapping.From == model && slices.Contains(c.SupportedModels, mapping.To) {
			return mapping.To, nil
		}
	}

	return "", fmt.Errorf("model %s not supported in channel %s", model, c.Name)
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
	Channels  []*Channel
	Executors executors.ScheduledExecutor
	Ent       *ent.Client
	// latestUpdate 记录最新的 channel 更新时间，用于优化定时加载
	latestUpdate time.Time
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
		//nolint:exhaustive // TODO SUPPORT.
		switch c.Type {
		case channel.TypeOpenai, channel.TypeDeepseek, channel.TypeDoubao, channel.TypeKimi:
			transformer, err := openai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
			if err != nil {
				log.Warn(ctx, "failed to create outbound transformer",
					log.String("channel", c.Name),
					log.String("type", c.Type.String()),
					log.Cause(err),
				)

				continue
			}

			log.Debug(ctx, "created openai outbound transformer", log.String("channel", c.Name))

			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
			})
		case channel.TypeZai, channel.TypeZhipu:
			transformer, err := zai.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
			if err != nil {
				log.Warn(ctx, "failed to create zai outbound transformer", log.Cause(err))
				continue
			}

			log.Debug(ctx, "created zai outbound transformer", log.String("channel", c.Name))

			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
			})
		case channel.TypeAnthropic, channel.TypeDeepseekAnthropic, channel.TypeKimiAnthropic, channel.TypeZhipuAnthropic, channel.TypeZaiAnthropic:
			transformer, err := anthropic.NewOutboundTransformer(c.BaseURL, c.Credentials.APIKey)
			if err != nil {
				log.Warn(ctx, "failed to create anthropic outbound transformer", log.Cause(err))
				continue
			}

			log.Debug(ctx, "created anthropic outbound transformer", log.String("channel", c.Name))

			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
			})
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
				log.Warn(ctx, "failed to create anthropic aws outbound transformer", log.Cause(err))
				continue
			}

			log.Debug(ctx, "created anthropic aws outbound transformer", log.String("channel", c.Name))

			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
			})
		case channel.TypeAnthropicGcp:
			// For anthropic_vertex, we need to create a VertexTransformer with GCP credentials
			// The transformer will handle Google Vertex AI integration
			if c.Credentials.GCP == nil {
				log.Warn(ctx, "GCP credentials are required for anthropic_vertex channel")
				continue
			}

			transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
				Type:      anthropic.PlatformVertex,
				Region:    c.Credentials.GCP.Region,
				ProjectID: c.Credentials.GCP.ProjectID,
				JSONData:  c.Credentials.GCP.JSONData,
			})
			if err != nil {
				log.Warn(ctx, "failed to create anthropic vertex outbound transformer", log.Cause(err))
				continue
			}

			log.Debug(ctx, "created anthropic vertex outbound transformer", log.String("channel", c.Name))

			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
			})
		case channel.TypeAnthropicFake:
			// For anthropic_fake, we use the fake transformer for testing
			fakeTransformer := anthropic.NewFakeTransformer()

			log.Debug(ctx, "created anthropic fake outbound transformer", log.String("channel", c.Name))
			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: fakeTransformer,
			})
		case channel.TypeOpenaiFake:
			fakeTransformer := openai.NewFakeTransformer()
			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: fakeTransformer,
			})
			log.Debug(ctx, "created openai fake outbound transformer", log.String("channel", c.Name))
		}
	}

	svc.Channels = channels

	return nil
}

func (svc *ChannelService) ChooseChannels(
	ctx context.Context,
	chatReq *llm.Request,
) ([]*Channel, error) {
	var channels []*Channel

	for _, channel := range svc.Channels {
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

	// Create the appropriate transformer based on channel type
	var outboundTransformer transformer.Outbound

	//nolint:exhaustive // TODO SUPPORT.
	switch entity.Type {
	case channel.TypeOpenai, channel.TypeDeepseek, channel.TypeDoubao, channel.TypeKimi:
		transformer, err := openai.NewOutboundTransformer(entity.BaseURL, entity.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}

		outboundTransformer = transformer
	case channel.TypeAnthropic, channel.TypeDeepseekAnthropic, channel.TypeKimiAnthropic, channel.TypeZhipuAnthropic:
		transformer, err := anthropic.NewOutboundTransformer(entity.BaseURL, entity.Credentials.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create anthropic outbound transformer: %w", err)
		}

		outboundTransformer = transformer
	case channel.TypeAnthropicAWS:
		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:            anthropic.PlatformBedrock,
			Region:          entity.Credentials.AWS.Region,
			AccessKeyID:     entity.Credentials.AWS.AccessKeyID,
			SecretAccessKey: entity.Credentials.AWS.SecretAccessKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create anthropic aws outbound transformer: %w", err)
		}

		outboundTransformer = transformer
	case channel.TypeAnthropicGcp:
		if entity.Credentials.GCP == nil {
			return nil, fmt.Errorf("GCP credentials are required for anthropic_vertex channel")
		}

		transformer, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:      anthropic.PlatformVertex,
			Region:    entity.Credentials.GCP.Region,
			ProjectID: entity.Credentials.GCP.ProjectID,
			JSONData:  entity.Credentials.GCP.JSONData,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create anthropic vertex outbound transformer: %w", err)
		}

		outboundTransformer = transformer
	case channel.TypeAnthropicFake:
		outboundTransformer = anthropic.NewFakeTransformer()
	case channel.TypeOpenaiFake:
		outboundTransformer = openai.NewFakeTransformer()
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", entity.Type)
	}

	return &Channel{
		Channel:  entity,
		Outbound: outboundTransformer,
	}, nil
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
