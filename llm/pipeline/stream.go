package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// Process executes the streaming LLM pipeline
// Steps: outbound transform -> HTTP stream -> outbound stream transform -> inbound stream transform.
func (p *pipeline) stream(
	ctx context.Context,
	executor Executor,
	request *httpclient.Request,
) (streams.Stream[*httpclient.StreamEvent], error) {
	outboundStream, err := executor.DoStream(ctx, request)
	if err != nil {
		// Apply error response middlewares
		p.applyRawErrorResponseMiddlewares(ctx, err)

		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			return nil, p.Outbound.TransformError(ctx, httpErr)
		}

		return nil, err
	}

	// Apply raw stream middlewares
	outboundStream, err = p.applyRawStreamMiddlewares(ctx, outboundStream)
	if err != nil {
		return nil, fmt.Errorf("failed to apply raw stream middlewares: %w", err)
	}

	if log.DebugEnabled(ctx) {
		outboundStream = streams.Map(outboundStream,
			func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
				log.Debug(ctx, "Outbound stream event", log.Any("event", event))
				return event
			},
		)
	}

	llmStream, err := p.Outbound.TransformStream(ctx, outboundStream)
	if err != nil {
		log.Error(ctx, "Failed to transform streaming request", log.Cause(err))
		return nil, err
	}

	// Apply LLM stream middlewares
	llmStream, err = p.applyLlmStreamMiddlewares(ctx, llmStream)
	if err != nil {
		return nil, fmt.Errorf("failed to apply llm stream middlewares: %w", err)
	}

	if log.DebugEnabled(ctx) {
		llmStream = streams.Map(llmStream, func(event *llm.Response) *llm.Response {
			log.Debug(ctx, "LLM stream event", log.Any("event", event))
			return event
		})
	}

	inboundStream, err := p.Inbound.TransformStream(ctx, llmStream)
	if err != nil {
		log.Error(ctx, "Failed to transform streaming request", log.Cause(err))
		return nil, err
	}

	if log.DebugEnabled(ctx) {
		inboundStream = streams.Map(
			inboundStream,
			func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
				log.Debug(ctx, "Inbound stream event", log.Any("event", event))
				return event
			},
		)
	}

	return inboundStream, nil
}
