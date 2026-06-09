package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/looplj/axonhub/llm/httpclient"
)

// Process executes the non-streaming LLM pipeline
// Steps: outbound transform -> HTTP request -> outbound response transform -> inbound response transform.
func (p *pipeline) notStream(
	ctx context.Context,
	executor Executor,
	request *httpclient.Request,
) (*httpclient.Response, error) {
	attemptCtx, cancel := p.nonStreamAttemptContext(ctx)
	defer cancel()

	httpResp, err := executor.Do(attemptCtx, request)
	if err != nil {
		if p.isNonStreamAttemptTimeout(ctx, attemptCtx) {
			err = p.nonStreamTimeoutError()
			p.applyRawErrorResponseMiddlewares(attemptCtx, err)

			return nil, err
		}

		// Apply error response middlewares
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			return nil, WrapUpstreamError(p.Outbound.TransformError(attemptCtx, httpErr))
		}

		return nil, WrapUpstreamError(fmt.Errorf("failed to do request: %w", err))
	}

	if p.isNonStreamAttemptTimeout(ctx, attemptCtx) {
		err := p.nonStreamTimeoutError()
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		return nil, err
	}

	// Apply raw response middlewares
	httpResp, err = p.applyRawResponseMiddlewares(attemptCtx, httpResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		return nil, fmt.Errorf("failed to apply raw response middlewares: %w", err)
	}

	llmResp, err := p.Outbound.TransformResponse(attemptCtx, httpResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		return nil, WrapUpstreamError(fmt.Errorf("failed to transform response: %w", err))
	}

	// Apply LLM response middlewares
	llmResp, err = p.applyLlmResponseMiddlewares(attemptCtx, llmResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		return nil, fmt.Errorf("failed to apply llm response middlewares: %w", err)
	}

	if p.emptyResponseDetection && !hasResponseContent(llmResp) {
		p.applyRawErrorResponseMiddlewares(attemptCtx, ErrEmptyResponse)

		return nil, ErrEmptyResponse
	}

	slog.DebugContext(attemptCtx, "LLM response", slog.Any("response", llmResp))

	finalResp, err := p.Inbound.TransformResponse(attemptCtx, llmResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		return nil, fmt.Errorf("failed to transform final response: %w", err)
	}

	// Apply inbound raw response middlewares after final response transformation
	finalResp, err = p.applyInboundRawResponseMiddlewares(attemptCtx, finalResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		return nil, fmt.Errorf("failed to apply inbound raw response middlewares: %w", err)
	}

	return finalResp, nil
}

func (p *pipeline) nonStreamAttemptContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if p.nonStreamResponseTimeout <= 0 {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, p.nonStreamResponseTimeout)
}

func (p *pipeline) isNonStreamAttemptTimeout(parentCtx, attemptCtx context.Context) bool {
	if p.nonStreamResponseTimeout <= 0 {
		return false
	}

	return errors.Is(attemptCtx.Err(), context.DeadlineExceeded) && parentCtx.Err() == nil
}

func (p *pipeline) nonStreamTimeoutError() error {
	return fmt.Errorf("%w after %s", ErrNonStreamResponseTimeout, p.nonStreamResponseTimeout)
}

func (p *pipeline) autoAggregateStream(
	ctx context.Context,
	executor Executor,
	request *httpclient.Request,
) (*httpclient.Response, error) {
	attemptCtx, cancel := p.nonStreamAttemptContext(ctx)
	defer cancel()

	inboundStream, err := p.streamForAutoAggregation(attemptCtx, executor, request, func(err error) error {
		if p.isNonStreamAttemptTimeout(ctx, attemptCtx) {
			return p.nonStreamTimeoutError()
		}

		return err
	})
	if err != nil {
		if p.isNonStreamAttemptTimeout(ctx, attemptCtx) {
			return nil, p.nonStreamTimeoutError()
		}

		return nil, err
	}
	defer inboundStream.Close()

	chunks := make([]*httpclient.StreamEvent, 0, 8)
	for inboundStream.Next() {
		event := inboundStream.Current()
		if event != nil {
			chunks = append(chunks, event)
		}
	}

	if p.isNonStreamAttemptTimeout(ctx, attemptCtx) {
		err := p.nonStreamTimeoutError()
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		return nil, err
	}

	if err := inboundStream.Err(); err != nil {
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)
		return nil, err
	}

	if len(chunks) == 0 {
		p.applyRawErrorResponseMiddlewares(attemptCtx, ErrEmptyStreamChunks)
		return nil, ErrEmptyStreamChunks
	}

	body, _, err := p.Inbound.AggregateStreamChunks(attemptCtx, chunks)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)
		return nil, err
	}

	if p.isNonStreamAttemptTimeout(ctx, attemptCtx) {
		err := p.nonStreamTimeoutError()
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)

		return nil, err
	}

	if len(body) == 0 {
		p.applyRawErrorResponseMiddlewares(attemptCtx, ErrEmptyAggregatedBody)
		return nil, ErrEmptyAggregatedBody
	}

	resp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
		Body: body,
	}

	resp, err = p.applyInboundRawResponseMiddlewares(attemptCtx, resp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(attemptCtx, err)
		return nil, fmt.Errorf("failed to apply inbound raw response middlewares: %w", err)
	}

	return resp, nil
}
