package chat

import (
	"context"
	"errors"

	"entgo.io/ent/privacy"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
)

// OutboundPersistentStream wraps a stream and tracks all responses for final saving to database.
// It implements the streams.Stream interface and handles persistence in the Close method.
//
//nolint:containedctx // Checked.
type OutboundPersistentStream struct {
	ctx            context.Context
	stream         streams.Stream[*httpclient.StreamEvent]
	request        *ent.Request
	requestExec    *ent.RequestExecution
	requestService *biz.RequestService
	transformer    transformer.Outbound
	responseChunks []*httpclient.StreamEvent
	closed         bool
}

var _ streams.Stream[*httpclient.StreamEvent] = (*OutboundPersistentStream)(nil)

func NewOutboundPersistentStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *biz.RequestService,
	outboundTransformer transformer.Outbound,
) *OutboundPersistentStream {
	return &OutboundPersistentStream{
		ctx:            ctx,
		stream:         stream,
		request:        request,
		requestExec:    requestExec,
		requestService: requestService,
		transformer:    outboundTransformer,
		responseChunks: make([]*httpclient.StreamEvent, 0),
		closed:         false,
	}
}

func (ts *OutboundPersistentStream) Next() bool {
	return ts.stream.Next()
}

func (ts *OutboundPersistentStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil {
		ts.responseChunks = append(ts.responseChunks, event)

		err := ts.requestService.AppendRequestExecutionChunk(
			ts.ctx,
			ts.requestExec.ID,
			event,
		)
		if err != nil {
			log.Warn(ts.ctx, "Failed to append request execution chunk", log.Cause(err))
		}
	}

	return event
}

func (ts *OutboundPersistentStream) Err() error {
	return ts.stream.Err()
}

func (ts *OutboundPersistentStream) Close() error {
	if ts.closed {
		return nil
	}

	ts.closed = true
	ctx := ts.ctx

	log.Debug(ctx, "Closing persistent stream", log.Int("chunk_count", len(ts.responseChunks)))

	if streamErr := ts.stream.Err(); streamErr != nil {
		if ts.requestExec != nil {
			err := ts.requestService.UpdateRequestExecutionFailed(
				ctx,
				ts.requestExec.ID,
				streamErr.Error(),
			)
			if err != nil {
				log.Warn(ctx, "Failed to update request execution status to failed", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	// Stream completed successfully - perform final persistence
	log.Debug(ctx, "Stream completed successfully, performing final persistence")

	// Update request execution with aggregated chunks
	if ts.requestExec != nil {
		responseBody, err := ts.transformer.AggregateStreamChunks(ctx, ts.responseChunks)
		if err != nil {
			log.Warn(ctx, "Failed to aggregate chunks using transformer", log.Cause(err))
			return ts.stream.Close()
		}

		err = ts.requestService.UpdateRequestExecutionCompletd(
			ctx,
			ts.requestExec.ID,
			responseBody,
		)
		if err != nil {
			log.Warn(
				ctx,
				"Failed to update request execution with chunks, trying basic completion",
				log.Cause(err),
			)
		}
	}

	return ts.stream.Close()
}

// PersistentOutboundTransformer wraps an outbound transformer with enhanced capabilities.
type PersistentOutboundTransformer struct {
	wrapped transformer.Outbound
	state   *PersistenceState
}

// Outbound transformer methods for enhanced version.
func (p *PersistentOutboundTransformer) TransformRequest(
	ctx context.Context,
	request *llm.Request,
) (*httpclient.Request, error) {
	// TODO fix the privacy context
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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

		log.Debug(ctx, "choose channels", log.Any("channels", channels),
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

		request.Model = requestExec.ModelID
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

func (p *PersistentOutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	persistentStream := NewOutboundPersistentStream(
		ctx,
		stream,
		p.state.Request,
		p.state.RequestExec,
		p.state.RequestService,
		p.wrapped, // Pass the wrapped outbound transformer for chunk aggregation
	)

	return p.wrapped.TransformStream(ctx, persistentStream)
}

func (p *PersistentOutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	return p.wrapped.TransformStreamChunk(ctx, event)
}

func (p *PersistentOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, error) {
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
