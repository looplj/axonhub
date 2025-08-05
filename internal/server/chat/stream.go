package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
)

// PersistentStream wraps a stream and tracks all responses for final saving to database.
//
//nolint:containedctx // Checked.
type PersistentStream struct {
	ctx                 context.Context
	stream              streams.Stream[*httpclient.StreamEvent]
	request             *ent.Request
	requestExec         *ent.RequestExecution
	requestService      *biz.RequestService
	outboundTransformer transformer.Outbound
	responseChunks      []*httpclient.StreamEvent
	closed              bool
}

var _ streams.Stream[*httpclient.StreamEvent] = (*PersistentStream)(nil)

func NewPersistentStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *biz.RequestService,
	outboundTransformer transformer.Outbound,
) *PersistentStream {
	return &PersistentStream{
		ctx:                 ctx,
		stream:              stream,
		request:             request,
		requestExec:         requestExec,
		requestService:      requestService,
		outboundTransformer: outboundTransformer,
		responseChunks:      make([]*httpclient.StreamEvent, 0),
		closed:              false,
	}
}

func (ts *PersistentStream) Next() bool {
	return ts.stream.Next()
}

func (ts *PersistentStream) Current() *httpclient.StreamEvent {
	event := ts.stream.Current()
	if event != nil && event.Data != nil {
		// Collect chunks for final aggregation (chunks are already saved by persistent transformer)
		ts.responseChunks = append(ts.responseChunks, event)
	}

	return event
}

func (ts *PersistentStream) Err() error {
	return ts.stream.Err()
}

func (ts *PersistentStream) Close() error {
	if ts.closed {
		return nil
	}

	ts.closed = true

	// Save final response body and update status
	ctx := ts.ctx

	// Update request execution
	if streamErr := ts.stream.Err(); streamErr != nil {
		// Stream had an error
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

		// Update main request
		if ts.request != nil {
			err := ts.requestService.UpdateRequestFailed(ctx, ts.request.ID)
			if err != nil {
				log.Warn(ctx, "Failed to update request status to failed", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	// Stream completed successfully, aggregate chunks and update status
	if ts.requestExec != nil && len(ts.responseChunks) > 0 {
		err := ts.requestService.UpdateRequestExecutionCompletedWithChunks(
			ctx,
			ts.requestExec.ID,
			ts.responseChunks,
			ts.outboundTransformer,
		)
		if err != nil {
			log.Warn(ctx, "Failed to update request execution status to completed", log.Cause(err))
		}

		// Aggregate chunks for main request
		if ts.request != nil {
			aggregatedResponse, err := ts.requestService.AggregateChunksToResponseWithTransformer(
				ctx,
				ts.responseChunks,
				ts.outboundTransformer,
			)
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

	return ts.stream.Close()
}
