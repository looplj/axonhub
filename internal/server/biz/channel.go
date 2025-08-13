package biz

import (
	"context"
	"fmt"
	"slices"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/zhenzou/executors"
	"go.uber.org/fx"
)

type Channel struct {
	*ent.Channel

	// Outbound is the outbound transformer for the channel.
	Outbound transformer.Outbound

	// Executor is the executor for the channel.
	Executor pipeline.Executor
}

func (c Channel) ChooseModel(model string) (string, error) {
	if slices.Contains(c.SupportedModels, model) {
		return model, nil
	}

	for _, mapping := range c.Settings.ModelMappings {
		if mapping.From == model {
			return mapping.To, nil
		}
	}

	return "", fmt.Errorf("model %s not supported in channel %s", model, c.Name)
}

type ChannelServiceParams struct {
	fx.In

	Ent        *ent.Client
	Executor   executors.ScheduledExecutor
	HttpClient *httpclient.HttpClient
}

func NewChannelService(params ChannelServiceParams) *ChannelService {
	svc := &ChannelService{
		Ent:       params.Ent,
		Executors: params.Executor,
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
	Ent       *ent.Client
	Channels  []*Channel
	Executors executors.ScheduledExecutor
}

func (svc *ChannelService) loadChannelsPeriodic(ctx context.Context) {
	err := svc.loadChannels(ctx)
	if err != nil {
		log.Error(ctx, "failed to load channels", log.Cause(err))
	}
}

func (svc *ChannelService) loadChannels(ctx context.Context) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	entities, err := svc.Ent.Channel.Query().All(ctx)
	if err != nil {
		return err
	}

	var channels []*Channel

	for _, c := range entities {
		//nolint:exhaustive // TODO SUPPORT.
		switch c.Type {
		case "openai":
			transformer, err := openai.NewOutboundTransformer(c.BaseURL, c.APIKey)
			if err != nil {
				log.Warn(ctx, "failed to create openai outbound transformer", log.Cause(err))
				continue
			}

			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
			})
		case "anthropic":
			transformer := anthropic.NewOutboundTransformer(c.BaseURL, c.APIKey)
			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
				// TODO: support aws.bedrock/gcp.vertex
			})
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
		if slices.Contains(channel.SupportedModels, chatReq.Model) {
			channels = append(channels, channel)
			continue
		}

		if slices.ContainsFunc(
			channel.Settings.ModelMappings,
			func(model objects.ModelMapping) bool {
				return model.From == chatReq.Model
			},
		) {
			channels = append(channels, channel)
		}
	}

	return channels, nil
}
