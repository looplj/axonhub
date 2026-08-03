package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent/request"
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
		llmRequest.APIFormat,
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

	// Determine usage to log - unified in Response.Usage for all request types.
	usageToLog := llmResp.Usage

	_, err := state.UsageLogService.CreateUsageLogFromRequest(persistCtx, state.Request, state.RequestExec, usageToLog)
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

	// Video generation is async: initial response contains provider task id, but task may not be completed.
	// Keep request in processing status and store provider task id in external_id.
	if llmResp.RequestType == llm.RequestTypeVideo {
		err := state.RequestService.UpdateRequestStatusExternalIDAndResponseBody(
			persistCtx,
			state.Request.ID,
			request.StatusProcessing,
			llmResp.ID,
			httpResp.Body,
			metrics,
		)
		if err != nil {
			log.Warn(persistCtx, "Failed to update video request status to processing", log.Cause(err))
		}

		return httpResp, nil
	}

	// Speech (TTS) responses are binary audio. xjson.Marshal stores []byte verbatim, so raw
	// audio bytes would corrupt the JSON response_body column. Store a compact metadata
	// placeholder there and offload the audio payload to external storage (when configured),
	// mirroring how video artifacts are stored.
	if llmResp.RequestType == llm.RequestTypeSpeech {
		contentType := httpResp.Headers.Get("Content-Type")
		placeholder := audioSafeResponseBody(llm.RequestTypeSpeech, contentType, httpResp.Body)
		filename := audioFilenameForContentType(contentType)

		err := state.RequestService.UpdateRequestCompletedWithAudio(
			persistCtx,
			state.Request.ID,
			llmResp.ID,
			placeholder,
			httpResp.Body,
			filename,
			metrics,
		)
		if err != nil {
			log.Warn(persistCtx, "Failed to update speech request status to completed", log.Cause(err))
		}

		return httpResp, nil
	}

	// STT text/srt/vtt responses are non-JSON; wrap them so the JSON response_body column accepts them.
	respBody := audioSafeResponseBody(llmResp.RequestType, httpResp.Headers.Get("Content-Type"), httpResp.Body)

	err := state.RequestService.UpdateRequestCompleted(persistCtx, state.Request.ID, llmResp.ID, respBody, metrics)
	if err != nil {
		log.Warn(persistCtx, "Failed to update request status to completed", log.Cause(err))
	}

	return httpResp, nil
}

// audioSafeResponseBody converts audio response bodies into JSON-safe payloads for persistence:
// binary TTS audio becomes a compact metadata placeholder, and non-JSON STT bodies (text/srt/vtt)
// are wrapped into a JSON object. Other request types are returned unchanged.
func audioSafeResponseBody(requestType llm.RequestType, contentType string, body []byte) []byte {
	switch requestType {
	case llm.RequestTypeSpeech:
		return fmt.Appendf(nil, `{"object":"audio.speech","content_type":%q,"bytes":%d}`, contentType, len(body))
	case llm.RequestTypeTranscription, llm.RequestTypeTranslation:
		// Prefer the declared Content-Type, consistent with the outbound response parsing:
		// a text/srt/vtt transcript may coincidentally be valid JSON (e.g. "true", "123")
		// and must still be wrapped. Only sniff when Content-Type is absent.
		isJSON := strings.Contains(strings.ToLower(contentType), "application/json")
		if !isJSON && contentType == "" {
			isJSON = json.Valid(body)
		}

		if isJSON {
			return body
		}

		wrapped, err := json.Marshal(map[string]string{
			"object":       "audio.transcription",
			"content_type": contentType,
			"text":         string(body),
		})
		if err != nil {
			return body
		}

		return wrapped
	default:
		return body
	}
}

// audioFilenameForContentType derives a storage filename (with extension) from the audio
// response Content-Type, defaulting to mp3.
func audioFilenameForContentType(contentType string) string {
	ext := "mp3"

	switch {
	case strings.Contains(contentType, "wav"):
		ext = "wav"
	case strings.Contains(contentType, "opus"):
		ext = "opus"
	case strings.Contains(contentType, "aac"):
		ext = "aac"
	case strings.Contains(contentType, "flac"):
		ext = "flac"
	case strings.Contains(contentType, "pcm"):
		ext = "pcm"
	case strings.Contains(contentType, "mpeg"), strings.Contains(contentType, "mp3"):
		ext = "mp3"
	}

	return "audio." + ext
}
