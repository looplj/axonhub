package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
)

// Enhanced PersistenceState holds shared state with channel management and retry capabilities.
type PersistenceState struct {
	APIKey *ent.APIKey
	User   *ent.User

	RequestService  *biz.RequestService
	UsageLogService *biz.UsageLogService
	ChannelSelector ChannelSelector
	ModelMapper     *ModelMapper

	Request     *ent.Request
	RequestExec *ent.RequestExecution

	Channels       []*biz.Channel
	CurrentChannel *biz.Channel
	ChannelIndex   int
}

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
	httpRequest *httpclient.Request,
	modelMapper *ModelMapper,
) (*PersistentInboundTransformer, *PersistentOutboundTransformer) {
	return NewPersistentTransformersWithSelector(
		ctx,
		inbound,
		requestService,
		apiKey,
		user,
		httpRequest,
		modelMapper,
		NewDefaultChannelSelector(channelService),
	)
}

// NewPersistentTransformersWithSelector creates enhanced persistent transformers with custom channel selector.
func NewPersistentTransformersWithSelector(
	ctx context.Context,
	inbound transformer.Inbound,
	requestService *biz.RequestService,
	apiKey *ent.APIKey,
	user *ent.User,
	httpRequest *httpclient.Request,
	modelMapper *ModelMapper,
	channelSelector ChannelSelector,
) (*PersistentInboundTransformer, *PersistentOutboundTransformer) {
	state := &PersistenceState{
		RequestService:  requestService,
		ChannelSelector: channelSelector,
		APIKey:          apiKey,
		User:            user,
		ChannelIndex:    0,
		ModelMapper:     modelMapper,
	}

	return &PersistentInboundTransformer{
			wrapped: inbound,
			state:   state,
		}, &PersistentOutboundTransformer{
			wrapped: nil, // Will be set when channel is selected
			state:   state,
		}
}
