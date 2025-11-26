package chat

import (
	"context"
	"time"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// persistRequestExecutionMiddleware ensures a request execution exists and handles error updates.
type persistRequestExecutionMiddleware struct {
	pipeline.DummyMiddleware

	outbound *PersistentOutboundTransformer

	rawResponse *httpclient.Response
}

func persistRequestExecution(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return &persistRequestExecutionMiddleware{
		outbound: outbound,
	}
}

func (m *persistRequestExecutionMiddleware) Name() string {
	return "persist-request-execution"
}

func (m *persistRequestExecutionMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	state := m.outbound.state
	if state == nil || state.RequestExec != nil {
		return request, nil
	}

	channel := m.outbound.GetCurrentChannel()
	if channel == nil {
		return request, nil
	}

	llmRequest := state.LlmRequest
	if llmRequest == nil {
		return request, nil
	}

	requestExec, err := state.RequestService.CreateRequestExecution(
		ctx,
		channel,
		llmRequest.Model,
		state.Request,
		*request,
		m.outbound.APIFormat(),
	)
	if err != nil {
		return nil, err
	}

	state.RequestExec = requestExec

	return request, nil
}

func (m *persistRequestExecutionMiddleware) OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	m.rawResponse = response
	return response, nil
}

func (m *persistRequestExecutionMiddleware) OnOutboundLlmResponse(ctx context.Context, llmResp *llm.Response) (*llm.Response, error) {
	state := m.outbound.state
	if state == nil || state.RequestExec == nil {
		return llmResp, nil
	}

	// Use context without cancellation to ensure persistence even if client canceled
	persistCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := state.RequestService.UpdateRequestExecutionCompleted(
		persistCtx,
		state.RequestExec.ID,
		llmResp.ID,
		m.rawResponse.Body,
	)
	if err != nil {
		log.Warn(persistCtx, "Failed to update request execution status to completed", log.Cause(err))
	}

	return llmResp, nil
}

func (m *persistRequestExecutionMiddleware) OnOutboundRawError(ctx context.Context, err error) {
	// Update request execution with the real error message when request fails
	state := m.outbound.state
	if state == nil || state.RequestExec == nil {
		return
	}

	// Use context without cancellation to ensure persistence even if client canceled
	persistCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	updateErr := state.RequestService.UpdateRequestExecutionFailed(
		persistCtx,
		state.RequestExec.ID,
		ExtractErrorMessage(err),
	)
	if updateErr != nil {
		log.Warn(persistCtx, "Failed to update request execution status to failed", log.Cause(updateErr))
	}
}

// ExtractErrorMessage extracts HTTP error message from error.
func ExtractErrorMessage(err error) string {
	httpErr, ok := xerrors.As[*httpclient.Error](err)
	if !ok {
		return err.Error()
	}

	// Anthropic && OpenAI error format.
	message := gjson.GetBytes(httpErr.Body, "error.message")
	if message.Exists() && message.Type == gjson.String {
		return message.String()
	}

	// Other campatible error format.
	message = gjson.GetBytes(httpErr.Body, "errors.message")
	if message.Exists() && message.Type == gjson.String {
		return message.String()
	}

	return httpErr.Error()
}
