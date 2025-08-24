package chat

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

type ChannelSelector interface {
	Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error)
}

// DefaultChannelSelector selects only enabled channels.
type DefaultChannelSelector struct {
	ChannelService *biz.ChannelService
}

func NewDefaultChannelSelector(channelService *biz.ChannelService) *DefaultChannelSelector {
	return &DefaultChannelSelector{
		ChannelService: channelService,
	}
}

func (s *DefaultChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	return s.ChannelService.ChooseChannels(ctx, req)
}

// SpecifiedChannelSelector allows selecting specific channels (including disabled ones) for testing.
type SpecifiedChannelSelector struct {
	ChannelService *biz.ChannelService
	ChannelID      objects.GUID
}

func NewSpecifiedChannelSelector(channelService *biz.ChannelService, channelID objects.GUID) *SpecifiedChannelSelector {
	return &SpecifiedChannelSelector{
		ChannelService: channelService,
		ChannelID:      channelID,
	}
}

func (s *SpecifiedChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	channel, err := s.ChannelService.GetChannelForTest(ctx, s.ChannelID.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel for test: %w", err)
	}

	if !channel.IsModelSupported(req.Model) {
		return nil, fmt.Errorf("model %s not supported in channel %s", req.Model, channel.Name)
	}

	return []*biz.Channel{channel}, nil
}
