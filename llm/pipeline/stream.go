package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// hasFinishReason checks if an llm.Response event contains a finish reason.
func hasFinishReason(resp *llm.Response) bool {
	if resp == nil {
		return false
	}

	for _, choice := range resp.Choices {
		if choice.FinishReason != nil {
			return true
		}
	}

	return false
}

// checkEmptyResponse pre-reads up to 3 events from the LLM stream to detect empty responses.
// If the stream contains content, it returns a new stream with the pre-read events prepended.
// If the stream is empty (finish reason reached without content), it returns ErrEmptyResponse.
func (p *pipeline) checkEmptyResponse(
	ctx context.Context,
	llmStream streams.Stream[*llm.Response],
) (streams.Stream[*llm.Response], error) {
	const maxPreReadEvents = 3

	var buffered []*llm.Response

	for range maxPreReadEvents {
		if !llmStream.Next() {
			break
		}

		event := llmStream.Current()
		buffered = append(buffered, event)

		if hasResponseContent(event) {
			// Has content, not empty — prepend buffered events back
			return streams.PrependStream(llmStream, buffered...), nil
		}

		if event == llm.DoneResponse || hasFinishReason(event) {
			// Reached end without content — empty response
			slog.WarnContext(ctx, "empty response detected",
				slog.Int("events_read", len(buffered)),
			)

			llmStream.Close()

			return nil, ErrEmptyResponse
		}
	}

	if err := llmStream.Err(); err != nil {
		llmStream.Close()

		return nil, err
	}

	// Didn't find content or finish in 3 events — treat as non-empty (safe default)
	if len(buffered) > 0 {
		return streams.PrependStream(llmStream, buffered...), nil
	}

	return llmStream, nil
}

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
	rawStream := outboundStream
	outboundStream, err = p.applyRawStreamMiddlewares(ctx, outboundStream)
	if err != nil {
		rawStream.Close()
		p.applyRawErrorResponseMiddlewares(ctx, err)

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

	if request != nil && request.Metadata != nil {
		ctx = shared.ContextWithTransportScope(ctx, shared.ScopeFromMetadata(request.Metadata))
	}

	llmStream, err := p.Outbound.TransformStream(ctx, outboundStream)
	if err != nil {
		outboundStream.Close()
		p.applyRawErrorResponseMiddlewares(ctx, err)

		slog.ErrorContext(ctx, "Failed to transform streaming request", slog.Any("error", err))
		return nil, err
	}

	rawLlmStream := llmStream

	// Apply LLM stream middlewares
	llmStream, err = p.applyLlmStreamMiddlewares(ctx, llmStream)
	if err != nil {
		rawLlmStream.Close()
		p.applyRawErrorResponseMiddlewares(ctx, err)

		return nil, fmt.Errorf("failed to apply llm stream middlewares: %w", err)
	}

	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		llmStream = streams.Map(llmStream, func(event *llm.Response) *llm.Response {
			slog.DebugContext(ctx, "LLM stream event", slog.Any("event", event))
			return event
		})
	}

	// Check for empty response if detection is enabled
	if p.emptyResponseDetection {
		rawLlmStream := llmStream
		llmStream, err = p.checkEmptyResponse(ctx, llmStream)
		if err != nil {
			rawLlmStream.Close()
			p.applyRawErrorResponseMiddlewares(ctx, err)

			return nil, err
		}
	}

	inboundStream, err := p.Inbound.TransformStream(ctx, llmStream)
	if err != nil {
		llmStream.Close()
		p.applyRawErrorResponseMiddlewares(ctx, err)

		slog.ErrorContext(ctx, "Failed to transform streaming request", slog.Any("error", err))
		return nil, err
	}

	rawInboundStream := inboundStream

	inboundStream, err = p.applyInboundRawStreamMiddlewares(ctx, inboundStream)
	if err != nil {
		rawInboundStream.Close()
		p.applyRawErrorResponseMiddlewares(ctx, err)

		return nil, fmt.Errorf("failed to apply inbound raw stream middlewares: %w", err)
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
