package orchestrator

import (
	"context"
	"regexp"
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

// Precompiled regex patterns for sanitizeResponseBody to avoid recompiling on each call.
var (
	tokenRegex  = regexp.MustCompile(`(?i)(bearer[\s:=]+)[a-zA-Z0-9_\-\.]+`)
	apiKeyRegex = regexp.MustCompile(`(api[keyK]ey|API[keyK]ey)["']?\s*[:=]\s*["']?([a-zA-Z0-9_\-\.]{8,})["']?`)
	emailRegex  = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
)

// sanitizeResponseBody redacts obvious secrets and truncates the body for safe logging.
func sanitizeResponseBody(body []byte, maxLen int) []byte {
	if len(body) == 0 {
		return body
	}

	str := string(body)

	// Redact bearer tokens (case-insensitive), preserving the bearer prefix
	str = tokenRegex.ReplaceAllString(str, "${1}[REDACTED]")

	// Redact API keys (common patterns)
	str = apiKeyRegex.ReplaceAllString(str, "$1=[REDACTED]")

	// Redact email addresses
	str = emailRegex.ReplaceAllString(str, "[EMAIL REDACTED]")

	// Truncate if too long
	if len(str) > maxLen {
		str = str[:maxLen] + "..."
	}

	return []byte(str)
}

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

			requestLatencyMs = biz.ClampLatency(requestLatencyMs)
			firstTokenLatencyMs = biz.ClampLatency(firstTokenLatencyMs)
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
		// Add response body for HTTP errors to help debug 400 errors (sanitized for PII)
		if httpErr, ok := xerrors.As[*httpclient.Error](err); ok && len(httpErr.Body) > 0 {
			sanitizedBody := sanitizeResponseBody(httpErr.Body, 1024)
			logFields = append(logFields, log.ByteString("response_body", sanitizedBody))
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

	// Other compatible error format.
	// Try errors.0.message first, then fall back to errors.message
	message1 := gjson.GetBytes(httpErr.Body, "errors.0.message")
	message2 := gjson.GetBytes(httpErr.Body, "errors.message")

	if message1.Exists() && message1.Type == gjson.String && message1.String() != "" {
		return message1.String()
	}

	if message2.Exists() && message2.Type == gjson.String {
		return message2.String()
	}

	return httpErr.Error()
}
