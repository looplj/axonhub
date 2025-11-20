package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

var (
	_ transformer.Inbound  = &PersistentInboundTransformer{}
	_ transformer.Outbound = &PersistentOutboundTransformer{}
)

// NewPersistentTransformers creates enhanced persistent transformers with channel management
// It accepts an httpclient.Request and transforms it to llm.Request internally.
func NewPersistentTransformers(
	ctx context.Context,
	inbound transformer.Inbound,
	channelService *biz.ChannelService,
	requestService *biz.RequestService,
	apiKey *ent.APIKey,
	user *ent.User,
	modelMapper *ModelMapper,
	proxy *objects.ProxyConfig,
) (*PersistentInboundTransformer, *PersistentOutboundTransformer) {
	return NewPersistentTransformersWithSelector(
		ctx,
		inbound,
		requestService,
		channelService,
		apiKey,
		user,
		modelMapper,
		proxy,
		NewDefaultChannelSelector(channelService),
	)
}

// NewPersistentTransformersWithSelector creates enhanced persistent transformers with custom channel selector.
func NewPersistentTransformersWithSelector(
	ctx context.Context,
	inbound transformer.Inbound,
	requestService *biz.RequestService,
	channelService *biz.ChannelService,
	apiKey *ent.APIKey,
	user *ent.User,
	modelMapper *ModelMapper,
	proxy *objects.ProxyConfig,
	channelSelector ChannelSelector,
) (*PersistentInboundTransformer, *PersistentOutboundTransformer) {
	state := &PersistenceState{
		APIKey:          apiKey,
		User:            user,
		RequestService:  requestService,
		ChannelService:  channelService,
		ChannelSelector: channelSelector,
		ChannelIndex:    0,
		ModelMapper:     modelMapper,
		Proxy:           proxy,
	}

	return &PersistentInboundTransformer{
			wrapped: inbound,
			state:   state,
		}, &PersistentOutboundTransformer{
			wrapped: nil, // Will be set when channel is selected
			state:   state,
		}
}
