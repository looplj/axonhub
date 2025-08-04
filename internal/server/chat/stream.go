package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
)

// TrackedStream wraps a stream and tracks all responses for final saving
// This version works with persistent transformers.
//
//nolint:containedctx // Checked.
type TrackedStream struct {
	ctx                 context.Context
	stream              streams.Stream[*httpclient.StreamEvent]
	request             *ent.Request
	requestExec         *ent.RequestExecution
	requestService      *biz.RequestService
	outboundTransformer transformer.Outbound
	responseChunks      []objects.JSONRawMessage
	closed              bool
}

// Ensure TrackedStreamV2 implements Stream interface.
var _ streams.Stream[*httpclient.StreamEvent] = (*TrackedStream)(nil)

func NewPersistentStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *biz.RequestService,
	outboundTransformer transformer.Outbound,
) *TrackedStream {
	return &TrackedStream{
		ctx:                 ctx,
		stream:              stream,
		request:             request,
		requestExec:         requestExec,
		requestService:      requestService,
		outboundTransformer: outboundTransformer,
		responseChunks:      make([]objects.JSONRawMessage, 0),
		closed:              false,
	}
}

func (ts *TrackedStream) Next() bool {
	return ts.stream.Next()
}

func (ts *TrackedStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil && event.Data != nil {
		// Collect chunks for final aggregation (chunks are already saved by persistent transformer)
		chunk := objects.JSONRawMessage(event.Data)
		ts.responseChunks = append(ts.responseChunks, chunk)
	}
	return event
}

func (ts *TrackedStream) Err() error {
	return ts.stream.Err()
}

func (ts *TrackedStream) Close() error {
	if ts.closed {
		return nil
	}
	ts.closed = true

	// Save final response body and update status
	ctx := ts.ctx

	// Update request execution
	if ts.stream.Err() != nil {
		// Stream had an error
		if ts.requestExec != nil {
			err := ts.requestService.UpdateRequestExecutionFailed(
				ctx,
				ts.requestExec.ID,
				ts.stream.Err().Error(),
			)
			if err != nil {
				log.Warn(ctx, "Failed to update request execution status to failed", log.Cause(err))
			}
		}

		// Update main request
		if ts.request != nil {
			err := ts.requestService.UpdateRequestFailed(ctx, ts.request.ID)
			if err != nil {
				log.Warn(ctx, "Failed to update request status to failed", log.Cause(err))
			}
		}
	} else {
		// Stream completed successfully, aggregate chunks and update status
		if ts.requestExec != nil && len(ts.responseChunks) > 0 {
			err := ts.requestService.UpdateRequestExecutionCompletedWithChunks(ctx, ts.requestExec.ID, ts.responseChunks, ts.outboundTransformer)
			if err != nil {
				log.Warn(ctx, "Failed to update request execution status to completed", log.Cause(err))
			}

			// Aggregate chunks for main request
			if ts.request != nil {
				aggregatedResponse, err := ts.requestService.AggregateChunksToResponseWithTransformer(ctx, ts.responseChunks, ts.outboundTransformer)
				if err != nil {
					log.Warn(ctx, "Failed to aggregate chunks for main request", log.Cause(err))
				} else {
					err = ts.requestService.UpdateRequestCompleted(ctx, ts.request.ID, aggregatedResponse)
					if err != nil {
						log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
					}
				}
			}
		}
	}

	return ts.stream.Close()
}
