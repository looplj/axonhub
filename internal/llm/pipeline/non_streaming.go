package pipeline

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// Process executes the non-streaming LLM pipeline
// Steps: apply decorators -> outbound transform -> HTTP request -> outbound response transform -> inbound response transform.
func (p *pipeline) notStream(
	ctx context.Context,
	request *llm.Request,
) (*httpclient.Response, error) {
	// Step 1: Apply decorators to the request
	if len(p.decorators) > 0 {
		for _, dec := range p.decorators {
			var err error

			request, err = dec.Decorate(ctx, request)
			if err != nil {
				log.Error(ctx, "Failed to apply decorator", log.Cause(err))
				return nil, err
			}
		}
	}

	// Step 2: Transform to provider-specific HTTP request using outbound transformer
	httpReq, err := p.Outbound.TransformRequest(ctx, request)
	if err != nil {
		log.Error(ctx, "Failed to transform request", log.Cause(err))
		return nil, err
	}

	executor := p.Executor
	if c, ok := p.Outbound.(ChannelCustomizedExecutor); ok {
		executor = c.CustomizeExecutor(executor)
	}

	// Step 3: Execute HTTP request
	httpResp, err := executor.Do(ctx, httpReq)
	if err != nil {
		log.Error(ctx, "HTTP request failed", log.Cause(err))

		var responseErr *llm.ResponseError
		if httpErr, ok := xerrors.As[*httpclient.Error](err); ok {
			responseErr = p.Outbound.TransformError(ctx, httpErr)
		} else {
			responseErr = &llm.ResponseError{
				StatusCode: http.StatusInternalServerError,
				Detail: llm.ErrorDetail{
					Message: err.Error(),
					Type:    "",
				},
			}
		}

		return nil, p.Inbound.TransformError(ctx, responseErr)
	}

	// Step 4: Transform HTTP response to unified LLM response using outbound transformer
	llmResp, err := p.Outbound.TransformResponse(ctx, httpResp)
	if err != nil {
		log.Error(ctx, "Failed to transform response", log.Cause(err))
		return nil, err
	}

	log.Debug(ctx, "LLM response", log.Any("response", llmResp))

	// Step 5: Transform LLM response to final HTTP response using inbound transformer
	finalResp, err := p.Inbound.TransformResponse(ctx, llmResp)
	if err != nil {
		log.Error(ctx, "Failed to transform final response", log.Cause(err))
		return nil, err
	}

	return finalResp, nil
}
