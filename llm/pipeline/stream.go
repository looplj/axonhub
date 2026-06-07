package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
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

		// Recognize both the shared sentinel pointer (llm.DoneResponse) and a
		// freshly-constructed "[DONE]" terminator: outbound transformers that emit
		// terminal events as new *llm.Response (e.g. OpenAI TTS binary streams) must
		// still trigger empty-response handling when no audio chunks were produced.
		if event == llm.DoneResponse || (event != nil && event.Object == "[DONE]") || hasFinishReason(event) {
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
	return p.streamWithOptions(ctx, executor, request, streamOptions{
		waitFirstEvent: true,
	})
}

func (p *pipeline) streamForAutoAggregation(
	ctx context.Context,
	executor Executor,
	request *httpclient.Request,
) (streams.Stream[*httpclient.StreamEvent], error) {
	return p.streamWithOptions(ctx, executor, request, streamOptions{})
}

type streamOptions struct {
	waitFirstEvent bool
}

type streamFirstByteAttempt struct {
	ctx      context.Context
	cancel   context.CancelFunc
	stop     func()
	timedOut func() bool
	enabled  bool
}

func (p *pipeline) streamWithOptions(
	ctx context.Context,
	executor Executor,
	request *httpclient.Request,
	opts streamOptions,
) (streams.Stream[*httpclient.StreamEvent], error) {
	attempt := p.streamFirstByteAttemptContext(ctx, opts.waitFirstEvent)
	streamCtx := attempt.ctx

	outboundStream, err := executor.DoStream(streamCtx, request)
	if err != nil {
		err = p.normalizeStreamFirstByteError(ctx, attempt, err)
		p.applyRawErrorResponseMiddlewares(streamCtx, err)

		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			transformedErr := p.Outbound.TransformError(streamCtx, httpErr)
			attempt.cancel()
			return nil, WrapUpstreamError(transformedErr)
		}

		attempt.cancel()
		return nil, WrapUpstreamError(err)
	}

	// Apply raw stream middlewares
	rawStream := outboundStream

	outboundStream, err = p.applyRawStreamMiddlewares(streamCtx, outboundStream)
	if err != nil {
		rawStream.Close()
		err = p.normalizeStreamFirstByteError(ctx, attempt, err)
		p.applyRawErrorResponseMiddlewares(streamCtx, err)
		attempt.cancel()

		return nil, fmt.Errorf("failed to apply raw stream middlewares: %w", err)
	}

	if slog.Default().Enabled(streamCtx, slog.LevelDebug) {
		outboundStream = streams.Map(outboundStream,
			func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
				slog.DebugContext(streamCtx, "Outbound stream event", slog.Any("event", event))
				return event
			},
		)
	}

	llmStream, err := p.Outbound.TransformStream(streamCtx, request, outboundStream)
	if err != nil {
		outboundStream.Close()
		err = p.normalizeStreamFirstByteError(ctx, attempt, err)
		p.applyRawErrorResponseMiddlewares(streamCtx, err)

		slog.ErrorContext(streamCtx, "Failed to transform streaming request", slog.Any("error", err))
		attempt.cancel()

		return nil, WrapUpstreamError(err)
	}

	rawLlmStream := llmStream

	// Apply LLM stream middlewares
	llmStream, err = p.applyLlmStreamMiddlewares(streamCtx, llmStream)
	if err != nil {
		rawLlmStream.Close()
		err = p.normalizeStreamFirstByteError(ctx, attempt, err)
		p.applyRawErrorResponseMiddlewares(streamCtx, err)
		attempt.cancel()

		return nil, fmt.Errorf("failed to apply llm stream middlewares: %w", err)
	}

	if slog.Default().Enabled(streamCtx, slog.LevelDebug) {
		llmStream = streams.Map(llmStream, func(event *llm.Response) *llm.Response {
			slog.DebugContext(streamCtx, "LLM stream event", slog.Any("event", event))
			return event
		})
	}

	// Check for empty response if detection is enabled
	if p.emptyResponseDetection {
		rawLlmStream := llmStream

		llmStream, err = p.checkEmptyResponse(streamCtx, llmStream)
		if err != nil {
			rawLlmStream.Close()
			err = p.normalizeStreamFirstByteError(ctx, attempt, err)
			p.applyRawErrorResponseMiddlewares(streamCtx, err)
			attempt.cancel()

			return nil, err
		}
	}

	inboundStream, err := p.Inbound.TransformStream(streamCtx, llmStream)
	if err != nil {
		llmStream.Close()
		err = p.normalizeStreamFirstByteError(ctx, attempt, err)
		p.applyRawErrorResponseMiddlewares(streamCtx, err)

		slog.ErrorContext(streamCtx, "Failed to transform streaming request", slog.Any("error", err))
		attempt.cancel()

		return nil, err
	}

	rawInboundStream := inboundStream

	inboundStream, err = p.applyInboundRawStreamMiddlewares(streamCtx, inboundStream)
	if err != nil {
		rawInboundStream.Close()
		err = p.normalizeStreamFirstByteError(ctx, attempt, err)
		p.applyRawErrorResponseMiddlewares(streamCtx, err)
		attempt.cancel()

		return nil, fmt.Errorf("failed to apply inbound raw stream middlewares: %w", err)
	}

	if opts.waitFirstEvent {
		inboundStream, err = p.waitFirstStreamEvent(streamCtx, inboundStream)
		if err != nil {
			err = p.normalizeStreamFirstByteError(ctx, attempt, err)
			p.applyRawErrorResponseMiddlewares(streamCtx, err)
			attempt.cancel()

			return nil, err
		}

		attempt.stop()
		inboundStream = &cancelOnCloseStream{Stream: inboundStream, cancel: attempt.cancel}
	}

	if slog.Default().Enabled(streamCtx, slog.LevelDebug) {
		inboundStream = streams.Map(
			inboundStream,
			func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
				slog.DebugContext(streamCtx, "Inbound stream event", slog.Any("event", event))
				return event
			},
		)
	}

	return inboundStream, nil
}

func (p *pipeline) streamFirstByteAttemptContext(ctx context.Context, waitFirstEvent bool) streamFirstByteAttempt {
	if !waitFirstEvent || p.streamFirstByteTimeout <= 0 {
		return streamFirstByteAttempt{
			ctx:      ctx,
			cancel:   func() {},
			stop:     func() {},
			timedOut: func() bool { return false },
		}
	}

	attemptCtx, cancel := context.WithCancel(ctx)

	var timedOut atomic.Bool

	timer := time.AfterFunc(p.streamFirstByteTimeout, func() {
		timedOut.Store(true)
		cancel()
	})

	return streamFirstByteAttempt{
		ctx:     attemptCtx,
		cancel:  cancel,
		stop:    func() { timer.Stop() },
		enabled: true,
		timedOut: func() bool {
			return timedOut.Load()
		},
	}
}

func (p *pipeline) normalizeStreamFirstByteError(parentCtx context.Context, attempt streamFirstByteAttempt, err error) error {
	if err == nil {
		return nil
	}

	if attempt.enabled && attempt.timedOut() && parentCtx.Err() == nil {
		return p.streamFirstByteTimeoutError()
	}

	return err
}

func (p *pipeline) streamFirstByteTimeoutError() error {
	return fmt.Errorf("%w after %s", ErrStreamFirstByteTimeout, p.streamFirstByteTimeout)
}

type cancelOnCloseStream struct {
	streams.Stream[*httpclient.StreamEvent]
	cancel context.CancelFunc
}

func (s *cancelOnCloseStream) Close() error {
	err := s.Stream.Close()
	s.cancel()

	return err
}

type firstStreamEventResult struct {
	ok    bool
	event *httpclient.StreamEvent
	err   error
}

func (p *pipeline) waitFirstStreamEvent(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*httpclient.StreamEvent], error) {
	if p.streamFirstByteTimeout <= 0 {
		return stream, nil
	}

	resultCh := make(chan firstStreamEventResult, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("panic while waiting for stream first event: %v", recovered)
				slog.ErrorContext(ctx, "panic while waiting for stream first event", slog.Any("panic", recovered))

				select {
				case resultCh <- firstStreamEventResult{err: err}:
				default:
				}
			}
		}()

		if stream.Next() {
			resultCh <- firstStreamEventResult{ok: true, event: stream.Current()}
			return
		}

		resultCh <- firstStreamEventResult{err: stream.Err()}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			_ = stream.Close()

			return nil, result.err
		}

		if !result.ok {
			return stream, nil
		}

		return streams.PrependStream(stream, result.event), nil
	case <-ctx.Done():
		_ = stream.Close()

		return nil, ctx.Err()
	}
}
