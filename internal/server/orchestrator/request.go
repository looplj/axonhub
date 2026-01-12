package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

type persistRequestMiddleware struct {
	pipeline.DummyMiddleware

	inbound     *PersistentInboundTransformer
	llmResponse *llm.Response
}

func persistRequest(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return &persistRequestMiddleware{
		inbound: inbound,
	}
}

func (m *persistRequestMiddleware) Name() string {
	return "persist-request"
}

func (m *persistRequestMiddleware) OnInboundLlmRequest(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
	if m.inbound.state.Request != nil {
		return llmRequest, nil
	}

	request, err := m.inbound.state.RequestService.CreateRequest(
		ctx,
		llmRequest,
		m.inbound.state.RawRequest,
		m.inbound.APIFormat(),
	)
	if err != nil {
		return nil, err
	}

	m.inbound.state.Request = request

	return llmRequest, nil
}

func (m *persistRequestMiddleware) OnOutboundLlmResponse(ctx context.Context, llmResp *llm.Response) (*llm.Response, error) {
	state := m.inbound.state
	if state.Request == nil || llmResp == nil {
		return llmResp, nil
	}

	// Store LLM response locally for use in OnInboundRawResponse
	m.llmResponse = llmResp

	// Use context without cancellation to ensure persistence even if client canceled
	persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*10)
	defer cancel()

	_, err := state.UsageLogService.CreateUsageLogFromRequest(persistCtx, state.Request, state.RequestExec, llmResp.Usage)
	if err != nil {
		log.Warn(persistCtx, "Failed to create usage log from request", log.Cause(err))
	}

	return llmResp, nil
}

func (m *persistRequestMiddleware) OnInboundRawResponse(ctx context.Context, httpResp *httpclient.Response) (*httpclient.Response, error) {
	state := m.inbound.state
	if state.Request == nil || httpResp == nil {
		return httpResp, nil
	}

	llmResp := m.llmResponse
	if llmResp == nil {
		log.Warn(ctx, "LLM response not found in middleware, cannot update request completed status")
		return httpResp, nil
	}

	// Use context without cancellation to ensure persistence even if client canceled
	persistCtx, cancel := xcontext.DetachWithTimeout(ctx, time.Second*10)
	defer cancel()

	// Build latency metrics from performance record
	var metrics *biz.LatencyMetrics

	if state.Perf != nil {
		firstTokenLatencyMs, requestLatencyMs, _ := state.Perf.Calculate()

		metrics = &biz.LatencyMetrics{
			LatencyMs: &requestLatencyMs,
		}
		if state.Perf.Stream && state.Perf.FirstTokenTime != nil {
			metrics.FirstTokenLatencyMs = &firstTokenLatencyMs
		}
	}

	err := state.RequestService.UpdateRequestCompleted(persistCtx, state.Request.ID, llmResp.ID, httpResp.Body, metrics)
	if err != nil {
		log.Warn(persistCtx, "Failed to update request status to completed", log.Cause(err))
	}

	return httpResp, nil
}
