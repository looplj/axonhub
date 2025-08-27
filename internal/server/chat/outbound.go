package chat

import (
	"context"
	"errors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
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
	ctx context.Context

	RequestService  *biz.RequestService
	UsageLogService *biz.UsageLogService

	stream      streams.Stream[*httpclient.StreamEvent]
	request     *ent.Request
	requestExec *ent.RequestExecution

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
	usageLogService *biz.UsageLogService,
	outboundTransformer transformer.Outbound,
) *OutboundPersistentStream {
	return &OutboundPersistentStream{
		ctx:             ctx,
		stream:          stream,
		request:         request,
		requestExec:     requestExec,
		RequestService:  requestService,
		UsageLogService: usageLogService,
		transformer:     outboundTransformer,
		responseChunks:  make([]*httpclient.StreamEvent, 0),
		closed:          false,
	}
}

func (ts *OutboundPersistentStream) Next() bool {
	return ts.stream.Next()
}

func (ts *OutboundPersistentStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil {
		ts.responseChunks = append(ts.responseChunks, event)

		err := ts.RequestService.AppendRequestExecutionChunk(
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

	streamErr := ts.stream.Err()
	if streamErr != nil {
		ctx = context.WithoutCancel(ctx)
		if ts.requestExec != nil {
			err := ts.RequestService.UpdateRequestExecutionFailed(
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
		ctx = context.WithoutCancel(ctx)
		responseBody, usage, err := ts.transformer.AggregateStreamChunks(ctx, ts.responseChunks)
		if err != nil {
			log.Warn(ctx, "Failed to aggregate chunks using transformer", log.Cause(err))
			return ts.stream.Close()
		}

		err = ts.RequestService.UpdateRequestExecutionCompletd(
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

		// Try to create usage log from aggregated response
		if usage != nil {
			_, err = ts.UsageLogService.CreateUsageLogFromRequest(ctx, ts.request, ts.requestExec, usage)
			if err != nil {
				log.Warn(ctx, "Failed to create usage log from request", log.Cause(err))
			}
		}
	}

	return ts.stream.Close()
}

// PersistentOutboundTransformer wraps an outbound transformer with enhanced capabilities.
type PersistentOutboundTransformer struct {
	wrapped transformer.Outbound
	state   *PersistenceState
}

// APIFormat returns the API format of the transformer.
func (p *PersistentOutboundTransformer) APIFormat() llm.APIFormat {
	return p.wrapped.APIFormat()
}

func (p *PersistentOutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return p.wrapped.TransformError(ctx, rawErr)
}

// Outbound transformer methods for enhanced version.
func (p *PersistentOutboundTransformer) TransformRequest(ctx context.Context, llmRequest *llm.Request) (*httpclient.Request, error) {
	if len(p.state.Channels) == 0 {
		channels, err := p.state.ChannelSelector.Select(ctx, llmRequest)
		if err != nil {
			return nil, err
		}

		log.Debug(ctx, "selected channels",
			log.Any("channels", channels),
			log.Any("model", llmRequest.Model),
		)

		if len(channels) == 0 {
			return nil, biz.ErrInvalidModel
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
		log.Any("model", llmRequest.Model),
	)

	model, err := p.state.CurrentChannel.ChooseModel(llmRequest.Model)
	if err != nil {
		log.Error(ctx, "Failed to choose model", log.Cause(err))
		return nil, err
	}

	llmRequest.Model = model

	channelRequest, err := p.wrapped.TransformRequest(ctx, llmRequest)
	if err != nil {
		return nil, err
	}

	if p.state.RequestExec == nil {
		requestExec, err := p.state.RequestService.CreateRequestExecution(
			ctx,
			p.state.CurrentChannel,
			model,
			p.state.Request,
			*channelRequest,
			p.APIFormat(),
		)
		if err != nil {
			return nil, err
		}

		p.state.RequestExec = requestExec
	}

	// Update request with channel ID after channel selection
	if p.state.Request != nil && p.state.Request.ChannelID == 0 {
		err := p.state.RequestService.UpdateRequestChannelID(
			ctx,
			p.state.Request.ID,
			p.state.CurrentChannel.ID,
		)
		if err != nil {
			log.Warn(ctx, "Failed to update request channel ID", log.Cause(err))
			// Continue processing even if channel ID update fails
		}
	}

	return channelRequest, nil
}

func (p *PersistentOutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	llmResp, err := p.wrapped.TransformResponse(ctx, response)
	if err != nil {
		if p.state.RequestExec != nil {
			innerErr := p.state.RequestService.UpdateRequestExecutionFailed(
				ctx,
				p.state.RequestExec.ID,
				err.Error(),
			)
			if innerErr != nil {
				log.Warn(ctx, "Failed to update request execution status to failed", log.Cause(innerErr))
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

	// Update request with usage log if we have a request and response with usage data
	if p.state.Request != nil && llmResp != nil {
		usage := llmResp.Usage
		_, err = p.state.UsageLogService.CreateUsageLogFromRequest(ctx, p.state.Request, p.state.RequestExec, usage)
		if err != nil {
			log.Warn(ctx, "Failed to create usage log from request", log.Cause(err))
		}
	}

	return llmResp, nil
}

func (p *PersistentOutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	persistentStream := NewOutboundPersistentStream(
		ctx,
		stream,
		p.state.Request,
		p.state.RequestExec,
		p.state.RequestService,
		p.state.UsageLogService,
		p.wrapped, // Pass the wrapped outbound transformer for chunk aggregation
	)

	return p.wrapped.TransformStream(ctx, persistentStream)
}

func (p *PersistentOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, *llm.Usage, error) {
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

// CustomizeExecutor customizes the executor for the current channel.
// If the current channel has an executor, it will be used.
// Otherwise, the default executor will be used.
//
// The customized executor will be used to execute the request.
// e.g. the aws bedrock process need a custom executor to handle the request.
func (p *PersistentOutboundTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	if customExecutor, ok := p.state.CurrentChannel.Outbound.(pipeline.ChannelCustomizedExecutor); ok {
		return customExecutor.CustomizeExecutor(executor)
	}

	return executor
}
