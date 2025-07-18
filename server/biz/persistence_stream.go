package biz

import (
	"context"

	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/objects"
	"github.com/looplj/axonhub/pkg/streams"
)

// TrackedStream wraps a stream and tracks all responses for final saving
type TrackedStream struct {
	ctx                 context.Context
	stream              streams.Stream[*llm.GenericHttpResponse]
	request             *ent.Request
	requestExec         *ent.RequestExecution
	requestService      *RequestService
	outboundTransformer transformer.Outbound
	responseChunks      []objects.JSONRawMessage
	closed              bool
}

// Ensure TrackedStream implements Stream interface
var _ streams.Stream[*llm.GenericHttpResponse] = (*TrackedStream)(nil)

func NewTrackedStream(
	ctx context.Context,
	stream streams.Stream[*llm.GenericHttpResponse],
	request *ent.Request,
	requestExec *ent.RequestExecution,
	requestService *RequestService,
	outboundTransformer transformer.Outbound,
) *TrackedStream {
	return &TrackedStream{
		ctx:                 ctx,
		stream:              stream,
		request:             request,
		requestExec:         requestExec,
		requestService:      requestService,
		outboundTransformer: outboundTransformer,
	}
}

func (ts *TrackedStream) Next() bool {
	return ts.stream.Next()
}

func (ts *TrackedStream) Current() *llm.GenericHttpResponse {
	resp := ts.stream.Current()
	if resp != nil && resp.Body != nil {
		// Save each chunk to response_chunks field
		chunk := objects.JSONRawMessage(resp.Body)
		ts.responseChunks = append(ts.responseChunks, chunk)

		// Add options to control if save chunk to database
		err := ts.requestService.AppendRequestExecutionChunk(ts.ctx, ts.requestExec.ID, chunk)
		if err != nil {
			log.Warn(ts.ctx, "Failed to save response chunk", log.Cause(err))
		}
	}
	return resp
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
		err := ts.requestService.UpdateRequestExecutionFailed(ctx, ts.requestExec.ID, ts.stream.Err().Error())
		if err != nil {
			log.Warn(ctx, "Failed to update request execution status", log.Cause(err))
		}
		err = ts.requestService.UpdateRequestFailed(ctx, ts.request.ID)
		if err != nil {
			log.Warn(ctx, "Failed to update request status", log.Cause(err))
		}
	} else {
		// Use the new method to aggregate chunks and update status
		err := ts.requestService.UpdateRequestExecutionCompletedWithChunks(ctx, ts.requestExec.ID, ts.responseChunks, ts.outboundTransformer)
		if err != nil {
			log.Warn(ctx, "Failed to update request execution with aggregated chunks", log.Cause(err))
		}

		// For the main request, we need to get the aggregated response using transformer
		aggregatedResponse, err := ts.requestService.AggregateChunksToResponseWithTransformer(ctx, ts.responseChunks, ts.outboundTransformer)
		if err != nil {
			log.Warn(ctx, "Failed to aggregate chunks for request update", log.Cause(err))
			aggregatedResponse = objects.JSONRawMessage("{}")
		}
		err = ts.requestService.UpdateRequestCompleted(ctx, ts.request.ID, aggregatedResponse)
		if err != nil {
			log.Warn(ctx, "Failed to update request status to completed", log.Cause(err))
		}
	}

	return ts.stream.Close()
}
