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
) (*httpclient.Response, http.Header, error) {
	httpResp, err := executor.Do(ctx, request)
	if err != nil {
		// Apply error response middlewares
		p.applyRawErrorResponseMiddlewares(ctx, err)

		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			return nil, nil, WrapUpstreamError(p.Outbound.TransformError(ctx, httpErr))
		}

		return nil, nil, WrapUpstreamError(fmt.Errorf("failed to do request: %w", err))
	}

	// Apply raw response middlewares
	httpResp, err = p.applyRawResponseMiddlewares(ctx, httpResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(ctx, err)

		return nil, nil, fmt.Errorf("failed to apply raw response middlewares: %w", err)
	}
	responseHeaders := httpclient.ForwardResponseHeaders(httpResp.Headers)

	llmResp, err := p.Outbound.TransformResponse(ctx, httpResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(ctx, err)

		return nil, nil, WrapUpstreamError(fmt.Errorf("failed to transform response: %w", err))
	}

	// Apply LLM response middlewares
	llmResp, err = p.applyLlmResponseMiddlewares(ctx, llmResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(ctx, err)

		return nil, nil, fmt.Errorf("failed to apply llm response middlewares: %w", err)
	}

	if p.emptyResponseDetection && !hasResponseContent(llmResp) {
		p.applyRawErrorResponseMiddlewares(ctx, ErrEmptyResponse)

		return nil, nil, ErrEmptyResponse
	}

	slog.DebugContext(ctx, "LLM response", slog.Any("response", llmResp))

	finalResp, err := p.Inbound.TransformResponse(ctx, llmResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(ctx, err)

		return nil, nil, fmt.Errorf("failed to transform final response: %w", err)
	}

	// Apply inbound raw response middlewares after final response transformation
	finalResp, err = p.applyInboundRawResponseMiddlewares(ctx, finalResp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(ctx, err)

		return nil, nil, fmt.Errorf("failed to apply inbound raw response middlewares: %w", err)
	}

	return finalResp, responseHeaders, nil
}

func (p *pipeline) autoAggregateStream(
	ctx context.Context,
	executor Executor,
	request *httpclient.Request,
) (*httpclient.Response, http.Header, error) {
	inboundStream, responseHeaders, err := p.stream(ctx, executor, request, 0)
	if err != nil {
		return nil, nil, err
	}
	defer inboundStream.Close()

	chunks := make([]*httpclient.StreamEvent, 0, 8)
	for inboundStream.Next() {
		event := inboundStream.Current()
		if event != nil {
			chunks = append(chunks, event)
		}
	}

	if err := inboundStream.Err(); err != nil {
		p.applyRawErrorResponseMiddlewares(ctx, err)
		return nil, nil, err
	}

	if len(chunks) == 0 {
		p.applyRawErrorResponseMiddlewares(ctx, ErrEmptyStreamChunks)
		return nil, nil, ErrEmptyStreamChunks
	}

	body, _, err := p.Inbound.AggregateStreamChunks(ctx, chunks)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(ctx, err)
		return nil, nil, err
	}

	if len(body) == 0 {
		p.applyRawErrorResponseMiddlewares(ctx, ErrEmptyAggregatedBody)
		return nil, nil, ErrEmptyAggregatedBody
	}

	resp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
		Body: body,
	}

	resp, err = p.applyInboundRawResponseMiddlewares(ctx, resp)
	if err != nil {
		p.applyRawErrorResponseMiddlewares(ctx, err)
		return nil, nil, fmt.Errorf("failed to apply inbound raw response middlewares: %w", err)
	}

	return resp, responseHeaders, nil
}
