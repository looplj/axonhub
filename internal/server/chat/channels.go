package chat

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

type ChannelSelector interface {
	Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error)
}

// DefaultChannelSelector selects only enabled channels and sorts them using load balancing.
type DefaultChannelSelector struct {
	ChannelService    *biz.ChannelService
	LoadBalancer      *LoadBalancer
	ConnectionTracker *DefaultConnectionTracker
}

// NewDefaultChannelSelector creates a selector with optional connection tracking.
func NewDefaultChannelSelector(
	channelService *biz.ChannelService,
	systemService *biz.SystemService,
	requestService *biz.RequestService,
	connectionTracker *DefaultConnectionTracker,
) *DefaultChannelSelector {
	// Build strategies
	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(requestService), // Priority 1: Last successful channel from trace
		NewErrorAwareStrategy(channelService), // Priority 2: Health and error rate
		NewWeightStrategy(),                   // Priority 3: Admin-configured weight
		NewConnectionAwareStrategy(channelService, connectionTracker),
	}

	loadBalancer := NewLoadBalancer(systemService, strategies...)

	return &DefaultChannelSelector{
		ChannelService:    channelService,
		LoadBalancer:      loadBalancer,
		ConnectionTracker: connectionTracker,
	}
}

func (s *DefaultChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*biz.Channel, error) {
	// The request model has already been mapped by the inbound transformer if needed
	// Channel selection will use the mapped model for finding compatible channels
	channels, err := s.ChannelService.ChooseChannels(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(channels) == 1 {
		return channels, nil
	}

	// Apply load balancing to sort channels by priority
	sortedChannels := s.LoadBalancer.Sort(ctx, channels, req.Model)

	log.Debug(ctx, "Selected and sorted channels for model",
		log.String("model", req.Model),
		log.Int("total_channels", len(channels)),
		log.Int("selected_channels", len(sortedChannels)))

	return sortedChannels, nil
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
