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

func NewChannelService(ent *ent.Client) *ChannelService {
	svc := &ChannelService{
		Ent:       ent,
		Executors: executors.NewPoolScheduleExecutor(),
	}
	if err := svc.loadProviders(context.Background()); err != nil {
		panic(err)
	}
	return svc
}

type ChannelService struct {
	Ent *ent.Client
	// TODO refresh registry periodically
	Executors executors.ScheduledExecutor
}

func (s *ChannelService) loadProviders(ctx context.Context) error {
	providers, err := s.Ent.Channel.Query().All(ctx)
	if err != nil {
		return err
	}
	for _, p := range providers {
		switch p.Type {
		case "openai":
			// TODO use transformer
		}
	}
	return nil
}

func (s *ChannelService) ChooseChannels(ctx context.Context, _ *llm.ChatCompletionRequest) ([]*ent.Channel, error) {
	channels, err := s.Ent.Channel.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	// TODO add cache
	// TODO choose by model and user
	return channels, nil
}

func (s *ChannelService) GetOutboundTransformer(ctx context.Context, channel *ent.Channel) (transformer.Outbound, error) {
	switch channel.Type {
	case "openai":
		return openaiTransformer.NewOutboundTransformer(channel.BaseURL, channel.APIKey), nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", channel.Type)
	}
}
