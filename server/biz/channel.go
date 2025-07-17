package biz

import (
	"context"
	"fmt"

	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
	openaiTransformer "github.com/looplj/axonhub/llm/transformer/openai"
)

type Channel struct {
	*ent.Channel
	Transformer transformer.Outbound
}

func NewChannelService(ent *ent.Client) *ChannelService {
	svc := &ChannelService{
		Ent:       ent,
		Executors: executors.NewPoolScheduleExecutor(),
	}
	if err := svc.loadChannels(context.Background()); err != nil {
		panic(err)
	}
	return svc
}

type ChannelService struct {
	Ent *ent.Client
	// TODO refresh registry periodically
	Channels []*Channel

	Executors executors.ScheduledExecutor
}

func (s *ChannelService) loadChannels(ctx context.Context) error {
	channels, err := s.Ent.Channel.Query().All(ctx)
	if err != nil {
		return err
	}
	for _, p := range channels {
		switch p.Type {
		case "openai":
			transformer := openaiTransformer.NewOutboundTransformer(p.BaseURL, p.APIKey)
			s.Channels = append(s.Channels, &Channel{
				Channel:     p,
				Transformer: transformer,
			})
		}
	}
	return nil
}

func (s *ChannelService) ChooseChannels(ctx context.Context, _ *llm.ChatCompletionRequest) ([]*Channel, error) {
	return s.Channels, nil
}

func (s *ChannelService) GetOutboundTransformer(ctx context.Context, channel *Channel) (transformer.Outbound, error) {
	switch channel.Type {
	case "openai":
		return openaiTransformer.NewOutboundTransformer(channel.BaseURL, channel.APIKey), nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", channel.Type)
	}
}
