package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
)

// Process executes the non-streaming LLM pipeline
// Steps: outbound transform -> HTTP request -> outbound response transform -> inbound response transform.
func (p *pipeline) notStream(
	ctx context.Context,
	executor Executor,
	request *httpclient.Request,
) (*httpclient.Response, error) {
	httpResp, err := executor.Do(ctx, request)
	if err != nil {
		// Apply error response middlewares
		p.applyRawErrorResponseMiddlewares(ctx, err)

		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			return nil, p.Outbound.TransformError(ctx, httpErr)
		}

		return nil, fmt.Errorf("failed to do request: %w", err)
	}

	// Apply raw response middlewares
	httpResp, err = p.applyRawResponseMiddlewares(ctx, httpResp)
	if err != nil {
		return nil, fmt.Errorf("failed to apply raw response middlewares: %w", err)
	}

	llmResp, err := p.Outbound.TransformResponse(ctx, httpResp)
	if err != nil {
		return nil, fmt.Errorf("failed to transform response: %w", err)
	}

	// Apply LLM response middlewares
	llmResp, err = p.applyLlmResponseMiddlewares(ctx, llmResp)
	if err != nil {
		return nil, fmt.Errorf("failed to apply llm response middlewares: %w", err)
	}

	log.Debug(ctx, "LLM response", log.Any("response", llmResp))

	finalResp, err := p.Inbound.TransformResponse(ctx, llmResp)
	if err != nil {
		return nil, fmt.Errorf("failed to transform final response: %w", err)
	}

	// Apply inbound raw response middlewares after final response transformation
	finalResp, err = p.applyInboundRawResponseMiddlewares(ctx, finalResp)
	if err != nil {
		return nil, fmt.Errorf("failed to apply inbound raw response middlewares: %w", err)
	}

	return finalResp, nil
}
