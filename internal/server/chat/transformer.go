package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
)

// Enhanced PersistenceState holds shared state with channel management and retry capabilities.
type PersistenceState struct {
	Channels       []*biz.Channel
	CurrentChannel *biz.Channel
	ChannelIndex   int

	Request     *ent.Request
	RequestExec *ent.RequestExecution

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	APIKey         *ent.APIKey
	ChatRequest    *llm.Request
	RequestBody    any
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
	httpRequest *httpclient.Request,
	requestBody any,
) (*PersistentInboundTransformer, *PersistentOutboundTransformer, error) {
	// Transform httpclient.Request to llm.Request using inbound transformer
	chatReq, err := inbound.TransformRequest(ctx, httpRequest)
	if err != nil {
		return nil, nil, err
	}

	log.Debug(ctx, "receive chat request", log.Any("request", chatReq))

	state := &PersistenceState{
		ChannelService: channelService,
		RequestService: requestService,
		APIKey:         apiKey,
		ChatRequest:    chatReq,
		RequestBody:    requestBody,
		ChannelIndex:   0,
	}

	return &PersistentInboundTransformer{
			wrapped: inbound,
			state:   state,
		}, &PersistentOutboundTransformer{
			wrapped: nil, // Will be set when channel is selected
			state:   state,
		}, nil
}
