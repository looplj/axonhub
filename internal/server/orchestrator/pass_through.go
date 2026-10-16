package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	responsestransformer "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// codexResponsesPassThroughHeaders contains Codex identity metadata that can
// accompany a pass-through body. Keep this as an explicit allowlist: inbound
// credentials, transport headers, and protocol-selection headers are never copied.
var codexResponsesPassThroughHeaders = []string{
	"X-Codex-Turn-Metadata",
	"X-Codex-Turn-State",
	"X-Codex-Window-Id",
	"X-Client-Request-Id",
	"X-Codex-Beta-Features",
	"Session-Id",
	"Originator",
	"Thread-Id",
}

// isPassThroughEnabled returns true when the effective pass-through flag for the current
// channel is enabled and both the inbound and outbound API formats are identical.
//
// The effective flag is the channel-level PassThroughBody when set, otherwise it falls back
// to the global system setting. systemService may be nil; in that case only the channel-level
// setting is consulted (used by tests that exercise per-channel behavior in isolation).
func (p *PersistentOutboundTransformer) isPassThroughEnabled(ctx context.Context, systemService *biz.SystemService) bool {
	channel := p.GetCurrentChannel()
	if channel == nil {
		return false
	}

	rawReq := p.state.RawProviderRequest
	if rawReq == nil || rawReq.APIFormat == "" {
		return false
	}

	llmReq := p.state.LlmRequest
	if llmReq == nil || string(llmReq.APIFormat) != rawReq.APIFormat {
		return false
	}

	if !passThroughStreamAligned(p.state.OriginalRequestStream, llmReq.Stream) {
		return false
	}

	var enabled bool

	switch {
	case channel.Settings != nil && channel.Settings.PassThroughBody != nil:
		enabled = *channel.Settings.PassThroughBody
	case systemService != nil:
		global, err := systemService.PassThrough(ctx)
		if err != nil {
			log.Warn(ctx, "failed to get global pass-through setting", log.Cause(err))

			return false
		}

		enabled = global
	}

	return enabled
}

func passThroughStreamAligned(originalStream, effectiveStream *bool) bool {
	originalEnabled := originalStream != nil && *originalStream
	effectiveEnabled := effectiveStream != nil && *effectiveStream

	return originalEnabled == effectiveEnabled
}

func responsesTransportAdaptationEnabled(_ context.Context, outbound *PersistentOutboundTransformer, _ *biz.SystemService) bool {
	if outbound == nil || outbound.state == nil || outbound.state.LlmRequest == nil || outbound.state.RawProviderRequest == nil {
		return false
	}

	source := outbound.state.LlmRequest.APIFormat
	target := llm.APIFormat(outbound.state.RawProviderRequest.APIFormat)
	if source == target || !responsestransformer.IsResponsesFormat(source) || !responsestransformer.IsResponsesFormat(target) {
		return false
	}

	return true
}

func responsesIdentityPreservationEnabled(outbound *PersistentOutboundTransformer) bool {
	if outbound == nil || outbound.state == nil || outbound.state.LlmRequest == nil || outbound.state.RawProviderRequest == nil {
		return false
	}

	source := outbound.state.LlmRequest.APIFormat
	target := llm.APIFormat(outbound.state.RawProviderRequest.APIFormat)

	return source == target && responsestransformer.IsResponsesFormat(source)
}

func responsesClientModel(outbound *PersistentOutboundTransformer) string {
	if outbound == nil || outbound.state == nil || outbound.state.LlmRequest == nil {
		return ""
	}

	request := outbound.state.LlmRequest.RawRequest
	if request == nil {
		request = outbound.state.RawRequest
	}
	if request == nil {
		return ""
	}

	body := request.Body
	if len(body) == 0 {
		body = request.JSONBody
	}

	return gjson.GetBytes(body, "model").String()
}

func preserveResponsesModel(body []byte, clientModel string) []byte {
	if clientModel == "" {
		return append([]byte(nil), body...)
	}

	patched := append([]byte(nil), body...)
	for _, path := range []string{"model", "response.model"} {
		if !gjson.GetBytes(patched, path).Exists() {
			continue
		}

		next, err := sjson.SetBytes(patched, path, clientModel)
		if err != nil {
			return append([]byte(nil), body...)
		}
		patched = next
	}

	return patched
}

func applyResponsesTransportRequestBody(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawRequest("responses-transport-request-body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		if !responsesTransportAdaptationEnabled(ctx, outbound, systemService) {
			return request, nil
		}

		llmRequest := outbound.state.LlmRequest
		if !outbound.allowPassThroughBody(ctx, llmRequest, request) {
			return request, nil
		}
		source := llmRequest.APIFormat
		target := llm.APIFormat(request.APIFormat)
		body, err := mergePassThroughRequestBodyWithPromptProtection(
			effectiveResponsesRequestBody(outbound.state),
			source,
			llmRequest.Model,
			outbound.state.PromptProtectionMaskRules,
		)
		if err != nil {
			log.Warn(ctx, "failed to merge Responses transport body, keeping outbound body", log.Cause(err))
			return request, nil
		}
		if outbound.state.PromptProtectionBodyCheck != nil {
			if err := outbound.state.PromptProtectionBodyCheck.validate(ctx, body); err != nil {
				log.Warn(ctx, "failed to validate Responses transport body, keeping outbound body", log.Cause(err))
				return request, nil
			}
		}

		body, err = responsestransformer.AdaptRequestBody(body, source, target)
		if err != nil {
			return nil, err
		}
		body, err = mergePassThroughRequestBody(body, target, llmRequest.Model)
		if err != nil {
			return nil, err
		}

		cloned := *request
		cloned.Body = body
		cloned.ContentType = "application/json"
		if cloned.Headers == nil {
			cloned.Headers = make(http.Header)
		}
		cloned.Headers.Set("Content-Type", "application/json")
		outbound.state.PassThroughApplied = true

		return &cloned, nil
	})
}

// applyPassThroughRequestBody creates a middleware that reuses the original inbound request body when
// the channel enables pass-through and the inbound and outbound API formats are identical.
// For formats that encode the selected model in the request body, the mapped llmReq.Model is
// written back into the copied raw payload so pass-through does not bypass model mapping.
// Save the actual outbound provider request so pass-through checks use the emitted API format.
func applyPassThroughRequestBody(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawRequest("pass-through-request-body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		outbound.state.RawProviderRequest = request

		if !outbound.isPassThroughEnabled(ctx, systemService) && !responsesTransportAdaptationEnabled(ctx, outbound, systemService) {
			return request, nil
		}

		channel := outbound.GetCurrentChannel()
		llmReq := outbound.state.LlmRequest
		if !outbound.allowPassThroughBody(ctx, llmReq, request) {
			return request, nil
		}

		// Multipart bodies cannot be reused: the outbound transformer rebuilds the
		// multipart payload with a new boundary in Content-Type, so replaying the inbound
		// bytes would mismatch the header, and form fields cannot be patched via sjson.
		if !passThroughBodySupported(llmReq) {
			return request, nil
		}

		log.Debug(ctx, "applying pass-through body",
			log.String("channel", channel.Name),
			log.String("api_format", request.APIFormat),
		)

		body, err := mergePassThroughRequestBodyWithPromptProtection(
			effectiveResponsesRequestBody(outbound.state),
			llmReq.APIFormat,
			llmReq.Model,
			outbound.state.PromptProtectionMaskRules,
		)
		if err == nil && outbound.state.PromptProtectionBodyCheck != nil {
			err = outbound.state.PromptProtectionBodyCheck.validate(ctx, body)
		}
		if err != nil {
			log.Warn(ctx, "failed to merge pass-through body, keeping outbound body",
				log.String("channel", channel.Name),
				log.Int("channel_id", channel.ID),
				log.Cause(err),
			)

			return request, nil
		}

		request.Body = body

		// The replayed body keeps the inbound media type: sync Content-Type so a JSON
		// image edit is not sent with the multipart header the outbound transformer built.
		if contentType := llmReq.RawRequest.Headers.Get("Content-Type"); contentType != "" {
			if request.Headers == nil {
				request.Headers = make(http.Header)
			}

			request.Headers.Set("Content-Type", contentType)
			request.ContentType = contentType
		}

		outbound.state.PassThroughApplied = true

		return request, nil
	})
}

func (p *PersistentOutboundTransformer) allowPassThroughBody(ctx context.Context, llmReq *llm.Request, providerReq *httpclient.Request) bool {
	policy, ok := p.wrapped.(transformer.PassThroughBodyPolicy)
	if !ok {
		return true
	}

	return policy.AllowPassThroughBody(ctx, llmReq, providerReq)
}

func (p *PersistentOutboundTransformer) allowPassThroughResponse(ctx context.Context) bool {
	policy, ok := p.wrapped.(transformer.PassThroughResponsePolicy)
	if !ok {
		return true
	}

	return policy.AllowPassThroughResponse(ctx, p.state.LlmRequest, p.state.RawProviderRequest)
}

// applyPassThroughRequestHeaders forwards Codex identity metadata paired with
// a pass-through body. Protocol-selection headers such as Responses Lite are
// deliberately excluded: the Codex transformer decides whether they apply.
func applyPassThroughRequestHeaders(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("pass-through-request-headers", func(_ context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		if !outbound.state.PassThroughApplied || outbound.state.LlmRequest == nil ||
			!responsestransformer.IsResponsesFormat(outbound.state.LlmRequest.APIFormat) ||
			outbound.state.LlmRequest.RawRequest == nil {
			return request, nil
		}

		if request.Headers == nil {
			request.Headers = make(http.Header)
		}

		inboundHeaders := outbound.state.LlmRequest.RawRequest.Headers
		for _, header := range codexResponsesPassThroughHeaders {
			values := inboundHeaders.Values(header)
			if len(values) == 0 {
				continue
			}

			request.Headers.Del(header)
			for _, value := range values {
				request.Headers.Add(header, value)
			}
		}

		return request, nil
	})
}

func mergePassThroughRequestBody(rawBody []byte, apiFormat llm.APIFormat, model string) ([]byte, error) {
	body := append([]byte(nil), rawBody...)

	if !passThroughBodyNeedsModelPatch(apiFormat) {
		return body, nil
	}

	if model == "" {
		return body, nil
	}

	nextBody, err := sjson.SetBytes(body, "model", model)
	if err != nil {
		return nil, fmt.Errorf("set model in pass-through body: %w", err)
	}

	return nextBody, nil
}

// passThroughBodySupported reports whether the raw inbound body can safely replace the
// outbound request body. Multipart bodies cannot be reused: the outbound transformer
// rebuilds the multipart payload with a new boundary in Content-Type, so replaying the
// inbound bytes would mismatch the header, and form fields cannot be patched via sjson.
// JSON image edits are replayable because their model field can be patched in place.
func passThroughBodySupported(llmReq *llm.Request) bool {
	//nolint:exhaustive // only multipart formats are excluded or content-type checked.
	switch llmReq.APIFormat {
	case llm.APIFormatOpenAITranscription,
		llm.APIFormatOpenAITranslation,
		llm.APIFormatOpenAIImageVariation:
		return false
	case llm.APIFormatOpenAIImageEdit:
		// Only the image edit inbound accepts JSON today; for the other formats this
		// branch is unreachable because their inbounds still require multipart.
		if llmReq.RawRequest == nil {
			return false
		}

		mediaType, _, err := mime.ParseMediaType(llmReq.RawRequest.Headers.Get("Content-Type"))
		return err == nil && strings.EqualFold(mediaType, "application/json")
	case llm.APIFormatOpenAIVideo:
		if llmReq.RawRequest == nil {
			return false
		}

		return !strings.HasPrefix(strings.ToLower(llmReq.RawRequest.Headers.Get("Content-Type")), "multipart/")
	default:
		return true
	}
}

func passThroughBodyNeedsModelPatch(apiFormat llm.APIFormat) bool {
	//nolint:exhaustive // other formats do not need a model field.
	switch apiFormat {
	case llm.APIFormatOpenAIChatCompletion,
		llm.APIFormatOpenAICompletion,
		llm.APIFormatOpenAIResponse,
		llm.APIFormatOpenAIResponseWebSocket,
		llm.APIFormatOpenAIResponseCompact,
		llm.APIFormatOpenAIEmbedding,
		llm.APIFormatOpenAIModeration,
		llm.APIFormatOpenAIAlphaSearch,
		llm.APIFormatOpenAIDecisions,
		llm.APIFormatOpenAIImageGeneration,
		llm.APIFormatOpenAIVideo,
		llm.APIFormatJinaEmbedding,
		llm.APIFormatJinaRerank,
		llm.APIFormatAnthropicMessage,
		// Speech (TTS) has a JSON body with a model field; transcription/translation
		// use multipart bodies that cannot be patched via sjson, so they are excluded.
		llm.APIFormatOpenAISpeech,
		// Image edits submitted as application/json carry a top-level model field.
		// Multipart edit bodies never reach this point (passThroughBodySupported
		// rejects them), so sjson patching only ever runs on JSON payloads.
		llm.APIFormatOpenAIImageEdit,
		llm.APIFormatTypeSafeSystemOne:
		return true
	default:
		return false
	}
}

// applyUserAgentPassThrough creates a middleware that applies the User-Agent pass-through setting.
func applyUserAgentPassThrough(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawRequest("user-agent-pass-through", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()
		if channel == nil {
			return request, nil
		}

		var passThroughEnabled bool
		if channel.Settings != nil && channel.Settings.PassThroughUserAgent != nil {
			passThroughEnabled = *channel.Settings.PassThroughUserAgent
		} else {
			globalPassThrough, err := systemService.UserAgentPassThrough(ctx)
			if err != nil {
				log.Warn(ctx, "failed to get global user agent pass through setting", log.Cause(err))

				passThroughEnabled = false
			} else {
				passThroughEnabled = globalPassThrough
			}
		}

		// Handle User-Agent header based on pass-through setting
		// This must be done here (before persistRequestExecution) to ensure
		// the correct User-Agent is logged in request execution records.
		if request.Headers == nil {
			request.Headers = make(http.Header)
		}

		if passThroughEnabled {
			// Pass-through enabled: use the original client's User-Agent
			if outbound.state.LlmRequest != nil && outbound.state.LlmRequest.RawRequest != nil {
				if clientUA := outbound.state.LlmRequest.RawRequest.Headers.Get("User-Agent"); clientUA != "" {
					request.Headers.Set("User-Agent", clientUA)
				}
			}
		} else if request.Headers.Get("User-Agent") == "" {
			// Pass-through disabled: use AxonHub's default User-Agent, unless the
			// outbound transformer already set a provider-required one (e.g.
			// GitHubCopilotChat on Copilot channels).
			request.Headers.Set("User-Agent", "axonhub/1.0")
		}

		return request, nil
	})
}

// captureRawProviderResponse stores the raw provider response on state for response pass-through.
func captureRawProviderResponse(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawResponse("capture-raw-provider-response", func(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
		if outbound.isPassThroughEnabled(ctx, systemService) || responsesTransportAdaptationEnabled(ctx, outbound, systemService) || responsesIdentityPreservationEnabled(outbound) {
			outbound.state.RawProviderResponse = response
		}

		return response, nil
	})
}

// applyPassThroughResponse replaces the transformed response with the raw provider response
// when PassThroughBody is enabled and the inbound/outbound API formats match, unless the
// outbound transformer vetoes it via transformer.PassThroughResponsePolicy.
func applyPassThroughResponse(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnInboundRawResponse("pass-through-response", func(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
		if !outbound.isPassThroughEnabled(ctx, systemService) && !responsesTransportAdaptationEnabled(ctx, outbound, systemService) && !responsesIdentityPreservationEnabled(outbound) {
			return response, nil
		}

		if !outbound.allowPassThroughResponse(ctx) {
			return response, nil
		}

		rawResp := outbound.state.RawProviderResponse
		if rawResp == nil {
			return response, nil
		}

		log.Debug(ctx, "applying pass-through response",
			log.String("channel", outbound.GetCurrentChannel().Name),
			log.String("api_format", outbound.state.RawProviderRequest.APIFormat),
		)

		if outbound.isPassThroughEnabled(ctx, systemService) || responsesIdentityPreservationEnabled(outbound) {
			cloned := *rawResp
			if responsesIdentityPreservationEnabled(outbound) {
				cloned.Body = preserveResponsesModel(rawResp.Body, responsesClientModel(outbound))
			}

			return &cloned, nil
		}

		source := outbound.state.LlmRequest.APIFormat
		target := llm.APIFormat(outbound.state.RawProviderRequest.APIFormat)
		body, err := responsestransformer.AdaptResponseBody(rawResp.Body, target, source)
		if err != nil {
			return nil, err
		}
		body = preserveResponsesModel(body, responsesClientModel(outbound))
		cloned := *rawResp
		cloned.Body = body
		cloned.Request = response.Request

		return &cloned, nil
	})
}

// captureRawProviderStream fans out raw provider stream events to both the pipeline
// (for transforms and LLM middlewares like connection tracking, performance recording)
// and the pass-through consumer. The pipeline receives events via pipelineCh. Raw events
// are held on state.RawStreamBacklog until applyPassThroughStream attaches the consumer,
// and are delivered via state.RawStreamCh afterwards.
func captureRawProviderStream(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnRawStream("capture-raw-provider-stream", func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
		adaptResponses := responsesTransportAdaptationEnabled(ctx, outbound, systemService)
		preserveResponses := responsesIdentityPreservationEnabled(outbound)
		if !outbound.isPassThroughEnabled(ctx, systemService) && !adaptResponses && !preserveResponses {
			return stream, nil
		}

		channel := outbound.GetCurrentChannel()

		backlog := &rawStreamBacklog{}
		rawQueue := newRawStreamQueue(backlog)
		pipelineCh := make(chan *httpclient.StreamEvent, 64)
		rawStreamCh := rawQueue.ch
		outbound.state.RawStreamCh = rawStreamCh
		outbound.state.RawStreamQueue = rawQueue
		outbound.state.RawStreamBacklog = backlog

		// Per-attempt local error storage: each attempt writes to its own variable so
		// concurrent defers from an abandoned goroutine and the new attempt's goroutine
		// never touch the same memory location, eliminating the data race on retries.
		var rawStreamErr error

		outbound.state.RawStreamErrRef = &rawStreamErr

		// Per-attempt cancelable context: PrepareForRetry / NextChannel call this cancel
		// to unblock the goroutine's channel sends and release the upstream HTTP connection
		// before the next attempt starts, preventing goroutine leaks.
		attemptCtx, cancel := context.WithCancel(ctx)
		var closeStreamOnce sync.Once
		closeStream := func() {
			closeStreamOnce.Do(func() {
				cancel()
				_ = stream.Close()
			})
		}
		outbound.state.RawStreamCancel = closeStream

		go func() {
			defer func() {
				backlog.mu.Lock()
				if r := recover(); r != nil {
					log.Warn(ctx, "captureRawProviderStream goroutine panicked, recovering",
						log.Any("panic", r),
						log.String("channel", channel.Name),
					)
					rawStreamErr = fmt.Errorf("passthrough stream panic: %v", r)
				} else if rawStreamErr == nil {
					rawStreamErr = stream.Err()
					if rawStreamErr == nil {
						rawStreamErr = ctx.Err()
					}
				}
				backlog.mu.Unlock()

				close(pipelineCh)
				close(rawStreamCh)
			}()
			// Ensure the context is cleaned up when the goroutine exits, regardless of
			// whether it finished naturally or was canceled by a retry.
			defer closeStream()

			for {
				select {
				case <-attemptCtx.Done():
					log.Debug(ctx, "context canceled before reading pass-through stream",
						log.String("channel", channel.Name))

					return
				default:
				}

				if !stream.Next() {
					return
				}

				event := stream.Current()
				held, err := backlog.hold(event)
				if err != nil {
					backlog.mu.Lock()
					rawStreamErr = err
					backlog.mu.Unlock()

					return
				}
				// Use blocking sends so events are not silently dropped when a
				// consumer is slower than the upstream provider. Bail out on
				// attempt cancellation (retry) or request cancellation to avoid
				// blocking forever.
				select {
				case pipelineCh <- event:
				case <-attemptCtx.Done():
					log.Debug(ctx, "context canceled while sending pipeline event",
						log.String("channel", channel.Name))

					return
				}

				// Nothing drains rawStreamCh until the pass-through consumer attaches.
				if held {
					continue
				}

				if err := rawQueue.enqueue(attemptCtx, event); err != nil {
					backlog.mu.Lock()
					rawStreamErr = err
					backlog.mu.Unlock()

					return
				}
			}
		}()

		return &passThroughChannelStream{ctx: ctx, ch: pipelineCh, errRef: &rawStreamErr, errMu: &backlog.mu, cancel: closeStream, owner: true, detach: outbound.state.RawStreamDetach}, nil
	})
}

// applyPassThroughStream returns a stream of raw provider events when PassThroughBody is enabled.
// A goroutine drains the transformed pipeline stream so that LLM middlewares (connection tracking,
// performance recording, rate limit tracking) still process events.
func applyPassThroughStream(outbound *PersistentOutboundTransformer, systemService *biz.SystemService) pipeline.Middleware {
	return pipeline.OnInboundRawStream("pass-through-response-stream", func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
		adaptResponses := responsesTransportAdaptationEnabled(ctx, outbound, systemService)
		preserveResponses := responsesIdentityPreservationEnabled(outbound)
		if !outbound.isPassThroughEnabled(ctx, systemService) && !adaptResponses && !preserveResponses {
			return stream, nil
		}

		rawCh := outbound.state.RawStreamCh
		if rawCh == nil {
			return stream, nil
		}

		// Snapshot the current attempt's error reference. If a future retry replaces
		// state.RawStreamErrRef, this stream still reads from the correct variable.
		errRef := outbound.state.RawStreamErrRef
		detach := outbound.state.RawStreamDetach

		// Events captured while the pipeline pre-read the attempt precede the channel.
		var held []*httpclient.StreamEvent
		var errMu *sync.Mutex
		queue := outbound.state.RawStreamQueue
		if queue != nil {
			held = queue.attach()
			errMu = &queue.backlog.mu
		} else if backlog := outbound.state.RawStreamBacklog; backlog != nil {
			held = backlog.attach()
			errMu = &backlog.mu
		}

		channel := outbound.GetCurrentChannel()

		log.Debug(ctx, "applying pass-through stream",
			log.String("channel", channel.Name),
			log.Int("held_events", len(held)),
		)

		observer := streamLifecycleFromContext(ctx)
		drainDone := make(chan struct{})
		go func() {
			defer close(drainDone)
			if observer == nil {
				defer stream.Close()
			}
			defer func() {
				if r := recover(); r != nil {
					log.Warn(ctx, "pass-through pipeline drain goroutine panicked, recovering",
						log.Any("panic", r),
						log.String("channel", channel.Name),
					)
				}
			}()
			for stream.Next() {
				_ = stream.Current()
			}
		}()

		rawStream := streams.Stream[*httpclient.StreamEvent](&passThroughChannelStream{ctx: ctx, ch: rawCh, queue: queue, errRef: errRef, errMu: errMu, detach: detach})
		if queue == nil {
			rawStream = streams.PrependStream(rawStream, held...)
		}

		if adaptResponses {
			rawStream = &responsesTransportStream{
				stream: rawStream,
				source: llm.APIFormat(outbound.state.RawProviderRequest.APIFormat),
				target: outbound.state.LlmRequest.APIFormat,
			}
		}
		if adaptResponses || preserveResponses {
			rawStream = &responsesIdentityStream{
				stream:      rawStream,
				clientModel: responsesClientModel(outbound),
			}
		}

		if observer != nil {
			return withPassThroughDeliveryGateAndDrain(rawStream, observer, stream.Close, drainDone), nil
		}

		return rawStream, nil
	})
}

type passThroughDeliveryStream struct {
	streams.Stream[*httpclient.StreamEvent]

	observer streams.DeliveryAcknowledger
	boundary streams.DeliveryBoundary
	close    func() error
	done     <-chan struct{}
}

func withPassThroughDeliveryGate(
	stream streams.Stream[*httpclient.StreamEvent],
	observer streams.DeliveryAcknowledger,
	closeFns ...func() error,
) streams.Stream[*httpclient.StreamEvent] {
	if observer == nil {
		return stream
	}

	boundary, _ := observer.(streams.DeliveryBoundary)
	var close func() error
	if len(closeFns) > 0 {
		close = closeFns[0]
	}
	return &passThroughDeliveryStream{Stream: stream, observer: observer, boundary: boundary, close: close}
}

func withPassThroughDeliveryGateAndDrain(
	stream streams.Stream[*httpclient.StreamEvent],
	observer streams.DeliveryAcknowledger,
	close func() error,
	done <-chan struct{},
) streams.Stream[*httpclient.StreamEvent] {
	if observer == nil {
		return stream
	}

	boundary, _ := observer.(streams.DeliveryBoundary)
	return &passThroughDeliveryStream{Stream: stream, observer: observer, boundary: boundary, close: close, done: done}
}

func (s *passThroughDeliveryStream) Close() error {
	// Stop the producer before waiting on the drain. A failed downstream write
	// stops the pass-through consumer, so nothing reads the raw queue, the
	// producer blocks on it, and the drain blocks on the producer. Only s.close
	// cancels the producer, so waiting first deadlocks. The drain still consumes
	// everything already buffered, preserving the background reader's accounting.
	var closeErr error
	producerClosed := false
	delivered := s.DeliveryAcknowledged()
	if !delivered && s.close != nil {
		closeErr = s.close()
		producerClosed = true
	}
	if s.done != nil {
		if delivered {
			timer := time.NewTimer(streamTailTimeout)
			select {
			case <-s.done:
			case <-timer.C:
				if s.close != nil {
					closeErr = s.close()
					producerClosed = true
				}
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		} else {
			<-s.done
		}
	}
	if delivered && !producerClosed && s.close != nil {
		closeErr = s.close()
	}
	return errors.Join(closeErr, s.Stream.Close())
}

func (s *passThroughDeliveryStream) AcknowledgeDelivery() {
	s.observer.AcknowledgeDelivery()
}

func (s *passThroughDeliveryStream) DeliveryAcknowledged() bool {
	return s.observer.DeliveryAcknowledged()
}

func (s *passThroughDeliveryStream) Deliver(ctx context.Context, write func() (bool, error)) error {
	if s.boundary != nil {
		return s.boundary.Deliver(ctx, write)
	}
	terminal, err := write()
	if err == nil && terminal {
		s.AcknowledgeDelivery()
	}
	return err
}

func (s *passThroughDeliveryStream) WaitForFinalization() {
	if waiter, ok := s.observer.(streams.FinalizationWaiter); ok {
		waiter.WaitForFinalization()
	}
}

type responsesTransportStream struct {
	stream  streams.Stream[*httpclient.StreamEvent]
	source  llm.APIFormat
	target  llm.APIFormat
	current *httpclient.StreamEvent
	err     error
}

type responsesIdentityStream struct {
	stream      streams.Stream[*httpclient.StreamEvent]
	clientModel string
	current     *httpclient.StreamEvent
}

func (s *responsesIdentityStream) Next() bool {
	if !s.stream.Next() {
		return false
	}

	s.current = preserveResponsesStreamEventModel(s.stream.Current(), s.clientModel)
	return true
}

func (s *responsesIdentityStream) Current() *httpclient.StreamEvent { return s.current }
func (s *responsesIdentityStream) Err() error                       { return s.stream.Err() }
func (s *responsesIdentityStream) Close() error                     { return s.stream.Close() }

func preserveResponsesStreamEventModel(event *httpclient.StreamEvent, clientModel string) *httpclient.StreamEvent {
	if event == nil || clientModel == "" {
		return event
	}

	cloned := *event
	cloned.Data = preserveResponsesModel(event.Data, clientModel)
	return &cloned
}

func (s *responsesTransportStream) Next() bool {
	if s.err != nil || !s.stream.Next() {
		return false
	}

	event, err := responsestransformer.AdaptStreamEvent(s.stream.Current(), s.source, s.target)
	if err != nil {
		s.err = err
		return false
	}
	s.current = event

	return true
}

func (s *responsesTransportStream) Current() *httpclient.StreamEvent { return s.current }

func (s *responsesTransportStream) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.stream.Err()
}

func (s *responsesTransportStream) Close() error { return s.stream.Close() }

// rawStreamBacklog holds the raw provider events of one attempt until the pass-through
// consumer attaches. Before that point the pipeline may still be pre-reading the attempt
// (first-event timeout, empty-response detection, retry before the first content event)
// and nothing drains RawStreamCh, so blocking on it would stall the pre-read as soon as
// the channel buffer fills. Raw count and bytes are bounded before pipeline delivery,
// including events filtered by the transformer and retained by persistence.
type rawStreamBacklog struct {
	mu       sync.Mutex
	attached bool
	events   []*httpclient.StreamEvent
	bytes    int
	count    int
	waitCh   chan struct{}
}

const (
	maxRawEventBytes      = 32 * 1024 * 1024
	maxRawPreAttachEvents = 1024
	maxRawPreAttachBytes  = 64 * 1024 * 1024
	maxRawQueueBytes      = 128 * 1024 * 1024
)

var errRawPreAttachBudgetExceeded = fmt.Errorf("pass-through pre-attachment raw event budget exceeded: %w", pipeline.ErrPreCommitBufferExceeded)

// hold keeps event for a consumer that has not attached yet. It reports false once the
// consumer is attached, in which case the caller delivers the event via RawStreamCh.
func (b *rawStreamBacklog) hold(event *httpclient.StreamEvent) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.attached {
		return false, nil
	}

	size := rawStreamEventSize(event)
	maxBytes := maxRawPreAttachBytes
	if b.attached {
		maxBytes = maxRawQueueBytes
	}
	if b.count >= maxRawPreAttachEvents || size > maxBytes-b.bytes {
		return false, errRawPreAttachBudgetExceeded
	}
	b.bytes += size
	b.count++
	b.events = append(b.events, event)

	return true, nil
}

// attach hands the held events over to the consumer; every later event is delivered
// via RawStreamCh, so the consumer reads the held events first to preserve order.
func (b *rawStreamBacklog) attach() []*httpclient.StreamEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.attached = true
	events := b.events
	b.events = nil

	return events
}

func (b *rawStreamBacklog) reserve(ctx context.Context, size int) error {
	if size > maxRawEventBytes {
		return errRawPreAttachBudgetExceeded
	}

	for {
		b.mu.Lock()
		maxBytes := maxRawPreAttachBytes
		if b.attached {
			maxBytes = maxRawQueueBytes
		}
		if b.count < maxRawPreAttachEvents && size <= maxBytes-b.bytes {
			b.bytes += size
			b.count++
			b.mu.Unlock()
			return nil
		}
		if !b.attached {
			b.mu.Unlock()
			return errRawPreAttachBudgetExceeded
		}
		if b.waitCh == nil {
			b.waitCh = make(chan struct{})
		}
		waitCh := b.waitCh
		b.mu.Unlock()

		select {
		case <-waitCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (b *rawStreamBacklog) release(size int) {
	b.mu.Lock()
	b.bytes -= size
	b.count--
	if b.waitCh != nil {
		close(b.waitCh)
		b.waitCh = nil
	}
	b.mu.Unlock()
}

// passThroughChannelStream wraps a channel as a Stream.
//
//nolint:containedctx // Required so Next() can observe request cancellation.
type passThroughChannelStream struct {
	ctx       context.Context
	ch        <-chan *httpclient.StreamEvent
	queue     *rawStreamQueue
	current   *httpclient.StreamEvent
	errRef    *error
	errMu     *sync.Mutex
	cancel    context.CancelFunc
	owner     bool
	detach    context.CancelFunc
	once      sync.Once
	currentMu sync.RWMutex
	ctxDone   bool
}

func (s *passThroughChannelStream) Next() bool {
	if s.queue != nil {
		if s.ctxDone || (s.ctx != nil && s.ctx.Err() != nil) {
			s.ctxDone = true
			return s.nextQueuedBuffered()
		}

		event, ok := s.queue.next(s.ctx)
		if !ok {
			_ = s.Close()
			return false
		}
		s.current = event
		return true
	}

	if s.ctx == nil {
		ev, ok := <-s.ch
		if !ok {
			return false
		}

		s.currentMu.Lock()
		s.current = ev
		s.currentMu.Unlock()

		return true
	}

	if s.ctxDone || s.ctx.Err() != nil {
		s.ctxDone = true

		return s.nextBuffered()
	}

	select {
	case ev, ok := <-s.ch:
		if !ok {
			return false
		}

		s.currentMu.Lock()
		s.current = ev
		s.currentMu.Unlock()

		return true
	case <-s.ctx.Done():
		// Client disconnect often races with the terminal event still sitting in
		// the channel (especially the pipeline drain path under pass-through).
		// Prefer draining already-buffered events over aborting, matching the
		// inbound/outbound Close() rule: cancel after a complete stream is still
		// completed.
		s.ctxDone = true

		return s.nextBuffered()
	}
}

func (s *passThroughChannelStream) nextQueuedBuffered() bool {
	event, ok := s.queue.nextBuffered()
	if !ok {
		_ = s.Close()
		return false
	}
	s.current = event
	return true
}

// nextBuffered consumes buffered events after cancellation has been observed.
// It never blocks on the producer: the stream ends (and cancels upstream via
// Close) at the first moment the buffer is empty or the channel is closed.
func (s *passThroughChannelStream) nextBuffered() bool {
	select {
	case ev, ok := <-s.ch:
		if !ok {
			_ = s.Close()

			return false
		}

		s.currentMu.Lock()
		s.current = ev
		s.currentMu.Unlock()

		return true
	default:
		_ = s.Close()

		return false
	}
}

func (s *passThroughChannelStream) Current() *httpclient.StreamEvent {
	s.currentMu.RLock()
	defer s.currentMu.RUnlock()

	return s.current
}

func (s *passThroughChannelStream) Err() error {
	if s.errMu != nil {
		s.errMu.Lock()
		defer s.errMu.Unlock()
	}
	if s.errRef != nil && *s.errRef != nil {
		return *s.errRef
	}
	if s.ctx != nil {
		return s.ctx.Err()
	}

	return nil
}

func (s *passThroughChannelStream) Close() error {
	s.once.Do(func() {
		if s.owner && s.cancel != nil {
			s.cancel()
		}
		if s.detach != nil {
			s.detach()
		}
	})

	return nil
}
