package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
)

// InboundPersistentStream wraps a stream and tracks all responses for final saving to database.
// It implements the streams.Stream interface and handles persistence in the Close method.
//
//nolint:containedctx // Checked.
type InboundPersistentStream struct {
	ctx            context.Context
	stream         streams.Stream[*httpclient.StreamEvent]
	request        *ent.Request
	requestExec    *ent.RequestExecution
	requestService *biz.RequestService
	transformer    transformer.Inbound
	responseChunks []*httpclient.StreamEvent
	closed         bool
}

var _ streams.Stream[*httpclient.StreamEvent] = (*InboundPersistentStream)(nil)

func NewInboundPersistentStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *biz.RequestService,
	transformer transformer.Inbound,
) *InboundPersistentStream {
	return &InboundPersistentStream{
		ctx:            ctx,
		stream:         stream,
		request:        request,
		requestExec:    requestExec,
		requestService: requestService,
		transformer:    transformer,
		responseChunks: make([]*httpclient.StreamEvent, 0),
		closed:         false,
	}
}

func (ts *InboundPersistentStream) Next() bool {
	return ts.stream.Next()
}

func (ts *InboundPersistentStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil {
		ts.responseChunks = append(ts.responseChunks, event)

		err := ts.requestService.AppendRequestChunk(
			ts.ctx,
			ts.request.ID,
			event,
		)
		if err != nil {
			log.Warn(ts.ctx, "Failed to append request chunk", log.Cause(err))
		}
	}

	return event
}

func (ts *InboundPersistentStream) Err() error {
	return ts.stream.Err()
}

func (ts *InboundPersistentStream) Close() error {
	if ts.closed {
		return nil
	}

	ts.closed = true
	ctx := ts.ctx

	log.Debug(ctx, "Closing persistent stream", log.Int("chunk_count", len(ts.responseChunks)))

	streamErr := ts.stream.Err()
	if streamErr != nil {
		// Stream had an error - update both request execution and main request
		log.Warn(ctx, "Stream completed with error", log.Cause(streamErr))

		if ts.request != nil {
			err := ts.requestService.UpdateRequestFailed(ctx, ts.request.ID)
			if err != nil {
				log.Warn(ctx, "Failed to update request status to failed", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	// Stream completed successfully - perform final persistence
	log.Debug(ctx, "Stream completed successfully, performing final persistence")

	// Update main request with aggregated response
	if ts.request != nil {
		responseBody, err := ts.transformer.AggregateStreamChunks(ctx, ts.responseChunks)
		if err != nil {
			log.Warn(ctx, "Failed to aggregate chunks for main request", log.Cause(err))
		} else {
			err = ts.requestService.UpdateRequestCompleted(ctx, ts.request.ID, responseBody)
			if err != nil {
				log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
			}
		}
	}

	return ts.stream.Close()
}

// PersistentInboundTransformer wraps an inbound transformer with enhanced capabilities.
type PersistentInboundTransformer struct {
	wrapped transformer.Inbound
	state   *PersistenceState
}

// Name returns the name of the transformer.
func (p *PersistentInboundTransformer) Name() string {
	return p.wrapped.Name()
}

// Inbound transformer methods for enhanced version.
func (p *PersistentInboundTransformer) TransformRequest(ctx context.Context, request *httpclient.Request) (*llm.Request, error) {
	llmRequest, err := p.wrapped.TransformRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	if p.state.Request == nil {
		request, err := p.state.RequestService.CreateRequest(
			ctx,
			p.state.APIKey,
			llmRequest,
			request,
			p.Name(),
		)
		if err != nil {
			return nil, err
		}

		p.state.Request = request
	}

	return llmRequest, nil
}

func (p *PersistentInboundTransformer) TransformResponse(ctx context.Context, response *llm.Response) (*httpclient.Response, error) {
	finalResp, err := p.wrapped.TransformResponse(ctx, response)
	if err != nil {
		return nil, err
	}

	if p.state.Request != nil {
		err = p.state.RequestService.UpdateRequestCompleted(ctx, p.state.Request.ID, finalResp.Body)
		if err != nil {
			log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
		}
	}

	return finalResp, nil
}

func (p *PersistentInboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	channelStream, err := p.wrapped.TransformStream(ctx, stream)
	if err != nil {
		return nil, err
	}

	persistentStream := NewInboundPersistentStream(
		ctx,
		channelStream,
		p.state.Request,
		p.state.RequestExec,
		p.state.RequestService,
		p, // Use the PersistentInboundTransformer as the transformer
	)

	return persistentStream, nil
}

func (p *PersistentInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, error) {
	return p.wrapped.AggregateStreamChunks(ctx, chunks)
}
