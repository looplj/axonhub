package orchestrator

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/log"
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
// to the global setting in the request's settings snapshot.
func (p *PersistentOutboundTransformer) isPassThroughEnabled() bool {
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

	if channel.Settings != nil && channel.Settings.PassThroughBody != nil {
		return *channel.Settings.PassThroughBody
	}

	return p.state.settings().PassThrough
}

func passThroughStreamAligned(originalStream, effectiveStream *bool) bool {
	originalEnabled := originalStream != nil && *originalStream
	effectiveEnabled := effectiveStream != nil && *effectiveStream

	return originalEnabled == effectiveEnabled
}

func responsesTransportAdaptationEnabled(outbound *PersistentOutboundTransformer) bool {
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

func applyResponsesTransportRequestBody(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("responses-transport-request-body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		if !responsesTransportAdaptationEnabled(outbound) {
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
func applyPassThroughRequestBody(outbound *PersistentOutboundTransformer, hooks RequestHookProvider) pipeline.Middleware {
	return pipeline.OnRawRequest("pass-through-request-body", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		outbound.state.RawProviderRequest = request

		if !outbound.isPassThroughEnabled() && !responsesTransportAdaptationEnabled(outbound) {
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
		if hooks != nil && channel != nil {
			hooks.PassThroughApplied(ctx, channel.ID)
		}

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
func applyUserAgentPassThrough(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawRequest("user-agent-pass-through", func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
		channel := outbound.GetCurrentChannel()
		if channel == nil {
			return request, nil
		}

		passThroughEnabled := outbound.state.settings().UserAgentPassThrough
		if channel.Settings != nil && channel.Settings.PassThroughUserAgent != nil {
			passThroughEnabled = *channel.Settings.PassThroughUserAgent
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
func captureRawProviderResponse(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawResponse("capture-raw-provider-response", func(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
		if outbound.isPassThroughEnabled() || responsesTransportAdaptationEnabled(outbound) || responsesIdentityPreservationEnabled(outbound) {
			outbound.state.RawProviderResponse = response
		}

		return response, nil
	})
}

// applyPassThroughResponse replaces the transformed response with the raw provider response
// when PassThroughBody is enabled and the inbound/outbound API formats match, unless the
// outbound transformer vetoes it via transformer.PassThroughResponsePolicy.
func applyPassThroughResponse(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnInboundRawResponse("pass-through-response", func(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
		if !outbound.isPassThroughEnabled() && !responsesTransportAdaptationEnabled(outbound) && !responsesIdentityPreservationEnabled(outbound) {
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

		if outbound.isPassThroughEnabled() || responsesIdentityPreservationEnabled(outbound) {
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

// captureRawProviderStream records the current attempt's raw provider events as the
// outbound transformer reads them. applyPassThroughStream forwards the recorded events
// to the client on the same goroutine that drives the transform chain, so pass-through
// needs no fan-out goroutine or channel.
func captureRawProviderStream(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnRawStream("capture-raw-provider-stream", func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
		if !outbound.isPassThroughEnabled() && !responsesTransportAdaptationEnabled(outbound) && !responsesIdentityPreservationEnabled(outbound) {
			return stream, nil
		}

		recorder := &rawEventRecorder{source: stream}
		outbound.state.PassThroughRecorder = recorder

		return recorder, nil
	})
}

// applyPassThroughStream returns the recorded raw provider events instead of the
// re-encoded inbound events. The transformed stream is still consumed event by event,
// so LLM middlewares and persistence observe the attempt as they do without pass-through.
func applyPassThroughStream(outbound *PersistentOutboundTransformer) pipeline.Middleware {
	return pipeline.OnInboundRawStream("pass-through-response-stream", func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
		adaptResponses := responsesTransportAdaptationEnabled(outbound)
		preserveResponses := responsesIdentityPreservationEnabled(outbound)
		if !outbound.isPassThroughEnabled() && !adaptResponses && !preserveResponses {
			return stream, nil
		}

		recorder := outbound.state.PassThroughRecorder
		if recorder == nil {
			return stream, nil
		}

		log.Debug(ctx, "applying pass-through stream",
			log.String("channel", outbound.GetCurrentChannel().Name),
		)

		rawStream := streams.Stream[*httpclient.StreamEvent](&passThroughStream{transformed: stream, recorder: recorder})
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

		return withPassThroughDelivery(ctx, stream, rawStream), nil
	})
}

// passThroughStream pulls the transformed stream and emits the raw events recorded
// while producing each transformed event. Raw events the transformer drops (for
// example Anthropic ping) are emitted together with the next transformed event.
type passThroughStream struct {
	transformed streams.Stream[*httpclient.StreamEvent]
	recorder    *rawEventRecorder
	pending     []*httpclient.StreamEvent
	current     *httpclient.StreamEvent
	done        bool
}

func (s *passThroughStream) Next() bool {
	for len(s.pending) == 0 {
		if s.done {
			return false
		}

		if s.transformed.Next() {
			// Current drives persistence and terminal tracking of the transformed event.
			_ = s.transformed.Current()
		} else {
			s.done = true
		}

		s.pending = s.recorder.take()
	}

	s.current = s.pending[0]
	s.pending[0] = nil
	s.pending = s.pending[1:]

	return true
}

func (s *passThroughStream) Current() *httpclient.StreamEvent { return s.current }

func (s *passThroughStream) Err() error {
	if err := s.recorder.Err(); err != nil {
		return err
	}

	return s.transformed.Err()
}

// Close closes only the transformed chain: the outbound persistent stream releases the
// raw source after collecting its tail.
func (s *passThroughStream) Close() error { return s.transformed.Close() }

// passThroughDeliveryStream keeps delivery acknowledgement and finalization connected
// to the transformed stream, which owns persistence of the attempt.
type passThroughDeliveryStream struct {
	streams.Stream[*httpclient.StreamEvent]

	observer streams.DeliveryAcknowledger
}

func withPassThroughDelivery(
	ctx context.Context,
	transformed streams.Stream[*httpclient.StreamEvent],
	raw streams.Stream[*httpclient.StreamEvent],
) streams.Stream[*httpclient.StreamEvent] {
	observer, ok := transformed.(streams.DeliveryAcknowledger)
	if !ok {
		if lifecycleOwner := streamLifecycleFromContext(ctx); lifecycleOwner != nil {
			observer = lifecycleOwner
		}
	}
	if observer == nil {
		return raw
	}

	return &passThroughDeliveryStream{Stream: raw, observer: observer}
}

func (s *passThroughDeliveryStream) AcknowledgeDelivery() {
	s.observer.AcknowledgeDelivery()
}

func (s *passThroughDeliveryStream) DeliveryAcknowledged() bool {
	return s.observer.DeliveryAcknowledged()
}

func (s *passThroughDeliveryStream) Deliver(ctx context.Context, write func() (bool, error)) error {
	if boundary, ok := s.observer.(streams.DeliveryBoundary); ok {
		return boundary.Deliver(ctx, write)
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

// rawEventRecorder wraps the raw provider stream of one attempt and retains every
// event read by the outbound transformer until passThroughStream takes it.
//
// Before the first take, the pipeline may still be pre-reading the attempt
// (first-event timeout, empty-response detection, retry before the first content
// event), so the retained count and bytes are bounded and exceeding them fails the
// attempt with a retryable pre-commit error. Afterwards only bytes are bounded:
// events stay retained only while the inbound transformer holds back its output.
type rawEventRecorder struct {
	source  streams.Stream[*httpclient.StreamEvent]
	current *httpclient.StreamEvent

	mu        sync.Mutex
	pending   []*httpclient.StreamEvent
	bytes     int
	attached  bool
	err       error
	closeOnce sync.Once
	closeErr  error
}

const (
	maxRawEventBytes      = 32 * 1024 * 1024
	maxRawPreAttachEvents = 1024
	maxRawPreAttachBytes  = 64 * 1024 * 1024
	maxRawQueueBytes      = 128 * 1024 * 1024
)

var errRawPreAttachBudgetExceeded = fmt.Errorf("pass-through pre-attachment raw event budget exceeded: %w", pipeline.ErrPreCommitBufferExceeded)

func (r *rawEventRecorder) Next() bool {
	r.mu.Lock()
	failed := r.err != nil
	r.mu.Unlock()
	if failed || !r.source.Next() {
		return false
	}

	event := r.source.Current()

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.retain(event); err != nil {
		r.err = err
		return false
	}
	r.current = event

	return true
}

func (r *rawEventRecorder) retain(event *httpclient.StreamEvent) error {
	size := rawStreamEventSize(event)
	if r.attached {
		if size > maxRawEventBytes || size > maxRawQueueBytes-r.bytes {
			return errRawPreAttachBudgetExceeded
		}
	} else if len(r.pending) >= maxRawPreAttachEvents || size > maxRawPreAttachBytes-r.bytes {
		return errRawPreAttachBudgetExceeded
	}

	r.bytes += size
	r.pending = append(r.pending, event)

	return nil
}

// take hands every retained event over to the pass-through consumer in read order.
func (r *rawEventRecorder) take() []*httpclient.StreamEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attached = true
	events := r.pending
	r.pending = nil
	r.bytes = 0

	return events
}

func (r *rawEventRecorder) Current() *httpclient.StreamEvent { return r.current }

func (r *rawEventRecorder) Err() error {
	r.mu.Lock()
	err := r.err
	r.mu.Unlock()
	if err != nil {
		return err
	}

	return r.source.Err()
}

func (r *rawEventRecorder) Close() error {
	r.closeOnce.Do(func() {
		r.closeErr = r.source.Close()
	})

	return r.closeErr
}

func rawStreamEventSize(event *httpclient.StreamEvent) int {
	if event == nil {
		return 0
	}

	return len(event.Data) + len(event.Type) + len(event.LastEventID)
}
