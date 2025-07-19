package biz

import (
	"context"
	"slices"

	"github.com/zhenzou/executors"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/pkg/xerrors"
)

type Channel struct {
	*ent.Channel
	Outbound transformer.Outbound
}

type ChannelServiceParams struct {
	fx.In

	Ent      *ent.Client
	Executor executors.ScheduledExecutor
}

func NewChannelService(params ChannelServiceParams) *ChannelService {
	svc := &ChannelService{
		Ent:       params.Ent,
		Executors: params.Executor,
	}

	xerrors.NoErr(svc.loadChannels(context.Background()))
	xerrors.NoErr2(params.Executor.ScheduleFuncAtCronRate(svc.loadChannelsPeriodic, executors.CRONRule{Expr: "*/1 * * * *"}))
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
	entities, err := svc.Ent.Channel.Query().All(ctx)
	if err != nil {
		return err
	}
	var channels []*Channel
	for _, c := range entities {
		switch c.Type {
		case "openai":
			transformer := openai.NewOutboundTransformer(c.BaseURL, c.APIKey)
			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
			})
		case "anthropic":
			transformer := anthropic.NewOutboundTransformer(c.BaseURL, c.APIKey)
			channels = append(channels, &Channel{
				Channel:  c,
				Outbound: transformer,
			})
		}
	}
	svc.Channels = channels
	return nil
}

func (svc *ChannelService) ChooseChannels(ctx context.Context, chatReq *llm.ChatCompletionRequest) ([]*Channel, error) {
	var channels []*Channel
	for _, channel := range svc.Channels {
		if slices.Contains(channel.SupportedModels, chatReq.Model) {
			channels = append(channels, channel)
		}
	}
	return channels, nil
}
