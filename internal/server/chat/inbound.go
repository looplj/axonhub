package chat

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/dumper"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
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

		// Use context without cancellation to ensure persistence even if client canceled
		if ts.request != nil {
			persistCtx := context.WithoutCancel(ctx)
			if err := ts.requestService.UpdateRequestStatusFromError(persistCtx, ts.request.ID, streamErr); err != nil {
				log.Warn(persistCtx, "Failed to update request status from error", log.Cause(err))
			}
		}

		return ts.stream.Close()
	}

	// Stream completed successfully - perform final persistence
	log.Debug(ctx, "Stream completed successfully, performing final persistence")

	ts.persistResponseChunks(ctx)

	return ts.stream.Close()
}

func (ts *InboundPersistentStream) persistResponseChunks(ctx context.Context) {
	defer func() {
		if cause := recover(); cause != nil {
			log.Warn(ctx, "Failed to persist inbound response chunks", log.Any("cause", cause))
		}
	}()

	// Update main request with aggregated response
	// Use context without cancellation to ensure persistence even if client canceled
	if ts.request != nil {
		persistCtx := context.WithoutCancel(ctx)

		responseBody, meta, err := ts.transformer.AggregateStreamChunks(persistCtx, ts.responseChunks)
		if err != nil {
			log.Warn(persistCtx, "Failed to aggregate chunks for main request", log.Cause(err))

			dumper.DumpStreamEvents(persistCtx, ts.responseChunks, "response_chunks.json")
		}

		err = ts.requestService.UpdateRequestCompleted(persistCtx, ts.request.ID, meta.ID, responseBody)
		if err != nil {
			log.Warn(persistCtx, "Failed to update request status to completed", log.Cause(err))
		}
	}
}

// PersistentInboundTransformer wraps an inbound transformer with enhanced capabilities.
type PersistentInboundTransformer struct {
	wrapped transformer.Inbound
	state   *PersistenceState
}

func (p *PersistentInboundTransformer) APIFormat() llm.APIFormat {
	return p.wrapped.APIFormat()
}

func (p *PersistentInboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	return p.wrapped.TransformError(ctx, rawErr)
}

func (p *PersistentInboundTransformer) TransformRequest(ctx context.Context, request *httpclient.Request) (*llm.Request, error) {
	llmRequest, err := p.wrapped.TransformRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	llmRequest.RawRequest = request
	p.state.RawRequest = request
	p.state.LlmRequest = llmRequest

	return llmRequest, nil
}

func (p *PersistentInboundTransformer) TransformResponse(ctx context.Context, response *llm.Response) (*httpclient.Response, error) {
	finalResp, err := p.wrapped.TransformResponse(ctx, response)
	if err != nil {
		return nil, err
	}

	if p.state.Request != nil {
		// Use context without cancellation to ensure persistence even if client canceled
		persistCtx := context.WithoutCancel(ctx)

		err = p.state.RequestService.UpdateRequestCompleted(persistCtx, p.state.Request.ID, response.ID, finalResp.Body)
		if err != nil {
			log.Warn(persistCtx, "Failed to update request status to completed", log.Cause(err))
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

func (p *PersistentInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return p.wrapped.AggregateStreamChunks(ctx, chunks)
}

// applyApiKeyModelMapping creates a middleware that applies model mapping from API key profiles.
// This is the first step in the inbound pipeline.
func applyApiKeyModelMapping(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("apply-model-mapping", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		if llmRequest.Model == "" {
			return nil, fmt.Errorf("%w: request model is empty", biz.ErrInvalidModel)
		}

		// Apply model mapping from API key profiles if active profile exists
		if inbound.state.APIKey == nil {
			return llmRequest, nil
		}

		originalModel := llmRequest.Model
		mappedModel := inbound.state.ModelMapper.MapModel(ctx, inbound.state.APIKey, originalModel)

		if mappedModel != originalModel {
			llmRequest.Model = mappedModel
			log.Debug(ctx, "applied model mapping from API key profile",
				log.String("api_key_name", inbound.state.APIKey.Name),
				log.String("original_model", originalModel),
				log.String("mapped_model", mappedModel))
		}

		// Save the model for later use, e.g. retry from next channels, should use the original model to choose channel model.
		// This should be done after the api key level model mapping.
		// This should be done before the request is created.
		// The outbound transformer will restore the original model if it was mapped.
		if inbound.state.OriginalModel == "" {
			inbound.state.OriginalModel = llmRequest.Model
		} else {
			// Restore original model if it was mapped
			// This should not happen, the inbound should not be called twice.
			// Just in case, restore the original model.
			llmRequest.Model = inbound.state.OriginalModel
		}

		return llmRequest, nil
	})
}

// selectChannels creates a middleware that selects available channels for the model.
// This is the second step in the inbound pipeline, moved from outbound transformer.
// If no valid channels are found, it returns ErrInvalidModel to fail fast.
func selectChannels(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("select-channels", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		// Only select channels once
		if len(inbound.state.Channels) > 0 {
			return llmRequest, nil
		}

		channels, err := inbound.state.ChannelSelector.Select(ctx, llmRequest)
		if err != nil {
			return nil, err
		}

		log.Debug(ctx, "selected channels",
			log.Any("channels", channels),
			log.Any("model", llmRequest.Model),
		)

		if len(channels) == 0 {
			return nil, fmt.Errorf("%w: no valid channels found for model %s", biz.ErrInvalidModel, llmRequest.Model)
		}

		inbound.state.Channels = channels

		return llmRequest, nil
	})
}

// createRequest creates a middleware that creates the request entity in the database.
// This is the third step in the inbound pipeline.
func createRequest(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("create-request", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		if inbound.state.Request != nil {
			return llmRequest, nil
		}

		request, err := inbound.state.RequestService.CreateRequest(
			ctx,
			llmRequest,
			inbound.state.RawRequest,
			inbound.APIFormat(),
		)
		if err != nil {
			return nil, err
		}

		inbound.state.Request = request

		return llmRequest, nil
	})
}
