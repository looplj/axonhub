package orchestrator

import (
	"context"
	"time"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
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

	candidate := state.ChannelModelsCandidates[state.CurrentCandidateIndex]
	entry := candidate.Models[state.CurrentModelIndex]

	requestExec, err := state.RequestService.CreateRequestExecution(
		ctx,
		channel,
		entry.ActualModel,
		state.Request,
		*request,
		m.outbound.APIFormat(),
	)
	if err != nil {
		return nil, err
	}

	// Update request with channel ID after channel selection
	if state.Request != nil && state.Request.ChannelID != channel.ID {
		err := state.RequestService.UpdateRequestChannelID(ctx, state.Request.ID, channel.ID)
		if err != nil {
			return nil, err
		}
		// Update the in-memory state to prevent duplicate updates and ensure consistency
		state.Request.ChannelID = channel.ID
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

	// Build latency metrics from performance record
	var metrics *biz.LatencyMetrics

	if state.Perf != nil && !state.Perf.StartTime.IsZero() {
		var (
			firstTokenLatencyMs int64
			requestLatencyMs    int64
		)

		if state.Perf.RequestCompleted && !state.Perf.EndTime.IsZero() {
			firstTokenLatencyMs, requestLatencyMs, _ = state.Perf.Calculate()
		} else {
			requestLatencyMs = time.Since(state.Perf.StartTime).Milliseconds()
			if state.Perf.Stream && state.Perf.FirstTokenTime != nil {
				firstTokenLatencyMs = state.Perf.FirstTokenTime.Sub(state.Perf.StartTime).Milliseconds()
			}
		}

		if requestLatencyMs < 0 {
			requestLatencyMs = 0
		}

		if firstTokenLatencyMs < 0 {
			firstTokenLatencyMs = 0
		}

		metrics = &biz.LatencyMetrics{
			LatencyMs: &requestLatencyMs,
		}
		if state.Perf.Stream && state.Perf.FirstTokenTime != nil {
			metrics.FirstTokenLatencyMs = &firstTokenLatencyMs
		}
	}

	err := state.RequestService.UpdateRequestExecutionCompleted(
		persistCtx,
		state.RequestExec.ID,
		llmResp.ID,
		m.rawResponse.Body,
		metrics,
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

	// Log error with channel information for better debugging
	channel := m.outbound.GetCurrentChannel()
	if channel != nil {
		logFields := []log.Field{
			log.Cause(err),
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
		}
		if modelID := m.outbound.GetCurrentModelID(); modelID != "" {
			logFields = append(logFields, log.String("model_id", modelID))
		}

		log.Warn(ctx, "request process failed", logFields...)
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
