package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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

	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		outboundStream = streams.Map(outboundStream,
			func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
				slog.DebugContext(ctx, "Outbound stream event", slog.Any("event", event))
				return event
			},
		)
	}

	llmStream, err := p.Outbound.TransformStream(ctx, outboundStream)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to transform streaming request", slog.Any("error", err))
		return nil, err
	}

	// Apply LLM stream middlewares
	llmStream, err = p.applyLlmStreamMiddlewares(ctx, llmStream)
	if err != nil {
		return nil, fmt.Errorf("failed to apply llm stream middlewares: %w", err)
	}

	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		llmStream = streams.Map(llmStream, func(event *llm.Response) *llm.Response {
			slog.DebugContext(ctx, "LLM stream event", slog.Any("event", event))
			return event
		})
	}

	inboundStream, err := p.Inbound.TransformStream(ctx, llmStream)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to transform streaming request", slog.Any("error", err))
		return nil, err
	}

	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		inboundStream = streams.Map(
			inboundStream,
			func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
				slog.DebugContext(ctx, "Inbound stream event", slog.Any("event", event))
				return event
			},
		)
	}

	return inboundStream, nil
}
