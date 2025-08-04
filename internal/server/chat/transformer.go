package chat

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
)

// Enhanced PersistenceState holds shared state with channel management and retry capabilities.
type PersistenceState struct {
	Request        *ent.Request
	RequestExec    *ent.RequestExecution
	Channels       []*biz.Channel
	CurrentChannel *biz.Channel
	ChannelIndex   int
	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	APIKey         *ent.APIKey
	ChatRequest    *llm.Request
	RequestBody    any
}

// PersistentInboundTransformer wraps an inbound transformer with enhanced capabilities.
type PersistentInboundTransformer struct {
	wrapped transformer.Inbound
	state   *PersistenceState
}

// PersistentOutboundTransformer wraps an outbound transformer with enhanced capabilities.
type PersistentOutboundTransformer struct {
	wrapped transformer.Outbound
	state   *PersistenceState
}

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

// Inbound transformer methods for enhanced version.
func (p *PersistentInboundTransformer) TransformRequest(
	ctx context.Context,
	request *httpclient.Request,
) (*llm.Request, error) {
	return p.wrapped.TransformRequest(ctx, request)
}

func (p *PersistentInboundTransformer) TransformResponse(
	ctx context.Context,
	response *llm.Response,
) (*httpclient.Response, error) {
	httpResp, err := p.wrapped.TransformResponse(ctx, response)
	if err != nil {
		return nil, err
	}

	if p.state.Request != nil {
		err = p.state.RequestService.UpdateRequestCompleted(ctx, p.state.Request.ID, httpResp.Body)
		if err != nil {
			log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
		}
	}

	return httpResp, nil
}

func (p *PersistentInboundTransformer) TransformStreamChunk(
	ctx context.Context,
	response *llm.Response,
) (*httpclient.StreamEvent, error) {
	return p.wrapped.TransformStreamChunk(ctx, response)
}

// Outbound transformer methods for enhanced version.
func (p *PersistentOutboundTransformer) TransformRequest(
	ctx context.Context,
	request *llm.Request,
) (*httpclient.Request, error) {
	// Initialize request and channels if not done yet
	if p.state.Request == nil {
		req, err := p.state.RequestService.CreateRequest(
			ctx,
			p.state.APIKey,
			p.state.ChatRequest,
			p.state.RequestBody,
		)
		if err != nil {
			return nil, err
		}
		p.state.Request = req

		// Choose channels
		channels, err := p.state.ChannelService.ChooseChannels(ctx, p.state.ChatRequest)
		if err != nil {
			return nil, err
		}
		log.Debug(
			ctx,
			"choose channels",
			log.Any("channels", channels),
			log.Any("model", p.state.ChatRequest.Model),
		)
		if len(channels) == 0 {
			return nil, errors.New("no provider available")
		}
		p.state.Channels = channels
	}

	// Select current channel for this attempt
	if p.state.ChannelIndex >= len(p.state.Channels) {
		return nil, errors.New("all channels exhausted")
	}

	p.state.CurrentChannel = p.state.Channels[p.state.ChannelIndex]
	p.wrapped = p.state.CurrentChannel.Outbound

	log.Debug(
		ctx,
		"using channel",
		log.Any("channel", p.state.CurrentChannel.Name),
		log.Any("model", p.state.ChatRequest.Model),
	)

	// Create request execution record before processing
	if p.state.RequestExec == nil {
		requestExec, err := p.state.RequestService.CreateRequestExecution(
			ctx,
			p.state.CurrentChannel,
			p.state.Request,
			request,
		)
		if err != nil {
			return nil, err
		}
		p.state.RequestExec = requestExec
	}

	return p.wrapped.TransformRequest(ctx, request)
}

func (p *PersistentOutboundTransformer) TransformResponse(
	ctx context.Context,
	response *httpclient.Response,
) (*llm.Response, error) {
	llmResp, err := p.wrapped.TransformResponse(ctx, response)
	if err != nil {
		if p.state.RequestExec != nil {
			err := p.state.RequestService.UpdateRequestExecutionFailed(
				ctx,
				p.state.RequestExec.ID,
				err.Error(),
			)
			if err != nil {
				log.Warn(ctx, "Failed to update request execution status to failed", log.Cause(err))
			}
		}
		return nil, err
	}

	if p.state.RequestExec != nil {
		err = p.state.RequestService.UpdateRequestExecutionCompleted(
			ctx,
			p.state.RequestExec.ID,
			response.Body,
		)
		if err != nil {
			log.Warn(ctx, "Failed to update request execution status to completed", log.Cause(err))
		}
	}

	return llmResp, nil
}

func (p *PersistentOutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	// Transform the stream chunk first
	llmResp, err := p.wrapped.TransformStreamChunk(ctx, event)
	if err != nil {
		return nil, err
	}

	if p.state.RequestExec != nil && event.Data != nil {
		err = p.state.RequestService.AppendRequestExecutionChunk(
			ctx,
			p.state.RequestExec.ID,
			event.Data,
		)
		if err != nil {
			log.Warn(ctx, "Failed to save response chunk", log.Cause(err))
		}
	}

	return llmResp, nil
}

func (p *PersistentOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks [][]byte,
) (*llm.Response, error) {
	return p.wrapped.AggregateStreamChunks(ctx, chunks)
}

// GetRequestExecution returns the current request execution.
func (p *PersistentOutboundTransformer) GetRequestExecution() *ent.RequestExecution {
	return p.state.RequestExec
}

// GetRequest returns the current request.
func (p *PersistentOutboundTransformer) GetRequest() *ent.Request {
	return p.state.Request
}

// GetCurrentChannelOutbound returns the current channel's outbound transformer.
func (p *PersistentOutboundTransformer) GetCurrentChannelOutbound() transformer.Outbound {
	if p.state.CurrentChannel != nil {
		return p.state.CurrentChannel.Outbound
	}
	return nil
}

// NextChannel moves to the next available channel for retry.
func (p *PersistentOutboundTransformer) NextChannel(ctx context.Context) error {
	p.state.ChannelIndex++
	if p.state.ChannelIndex >= len(p.state.Channels) {
		return errors.New("no more channels available for retry")
	}

	// Reset request execution for the new channel
	p.state.RequestExec = nil
	p.state.CurrentChannel = p.state.Channels[p.state.ChannelIndex]
	p.wrapped = p.state.CurrentChannel.Outbound

	log.Debug(ctx, "switching to next channel for retry",
		log.Any("channel", p.state.CurrentChannel.Name),
		log.Any("index", p.state.ChannelIndex))

	return nil
}

// HasMoreChannels returns true if there are more channels available for retry.
func (p *PersistentOutboundTransformer) HasMoreChannels() bool {
	return p.state.ChannelIndex+1 < len(p.state.Channels)
}
