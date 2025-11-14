package pipeline

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
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
		if httpErr, ok := xerrors.As[*httpclient.Error](err); ok {
			return nil, p.Outbound.TransformError(ctx, httpErr)
		}

		return nil, fmt.Errorf("failed to do request: %w", err)
	}

	llmResp, err := p.Outbound.TransformResponse(ctx, httpResp)
	if err != nil {
		return nil, fmt.Errorf("failed to transform response: %w", err)
	}

	log.Debug(ctx, "LLM response", log.Any("response", llmResp))

	finalResp, err := p.Inbound.TransformResponse(ctx, llmResp)
	if err != nil {
		return nil, fmt.Errorf("failed to transform final response: %w", err)
	}

	return finalResp, nil
}
