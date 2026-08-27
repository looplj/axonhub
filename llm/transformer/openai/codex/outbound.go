package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

const (
	codexBaseURL = "https://chatgpt.com/backend-api/codex#"
	codexAPIURL  = "https://chatgpt.com/backend-api/codex/responses"
)

// OutboundTransformer implements transformer.Outbound for Codex proxy.
// It normally talks to the Codex Responses upstream over SSE and adapts requests accordingly.
// The official backend always streams SSE; compatible relays that return a
// completed JSON response are also supported for non-stream callers.
//
//nolint:containedctx // It is used as a transformer.
type OutboundTransformer struct {
	tokens          oauth.TokenGetter
	transport       string
	baseURL         string
	alphaSearchPath string

	// official reports whether the configured upstream is the official Codex
	// backend (chatgpt.com). Official endpoints always stream SSE, so they keep
	// the DoStream path; only compatible relays get the JSON passthrough path.
	official bool

	// reuse existing Responses outbound for payload building.
	responsesOutbound *responses.OutboundTransformer

	executorMu         sync.Mutex
	webSocketExecutors map[pipeline.Executor]*responses.WebSocketExecutor
}

var (
	_ transformer.Outbound               = (*OutboundTransformer)(nil)
	_ transformer.PassThroughBodyPolicy  = (*OutboundTransformer)(nil)
	_ pipeline.ChannelCustomizedExecutor = (*OutboundTransformer)(nil)
)

var responsesBlockedPassThroughFields = []string{
	"max_output_tokens",
	"max_completion_tokens",
	"max_tokens",
}

type Params struct {
	TokenProvider   oauth.TokenGetter
	BaseURL         string
	Transport       string
	AlphaSearchPath string
}

// isOfficialCodexBaseURL reports whether baseURL points at the official Codex
// backend. Everything else is treated as a compatible relay that may return a
// completed JSON response instead of SSE.
func isOfficialCodexBaseURL(baseURL string) bool {
	return strings.Contains(strings.ToLower(baseURL), "chatgpt.com")
}

// isOfficialCodex reports whether the transformer targets the official Codex backend.
func (t *OutboundTransformer) isOfficialCodex() bool {
	return t != nil && t.official
}

func NewOutboundTransformer(params Params) (*OutboundTransformer, error) {
	if params.TokenProvider == nil {
		return nil, errors.New("token provider is required")
	}

	baseURL := params.BaseURL
	// Compatibility with old codex channel base url.
	if baseURL == "" || baseURL == "https://api.openai.com/v1" {
		baseURL = codexBaseURL
	}
	alphaSearchPath := params.AlphaSearchPath
	if alphaSearchPath == "" {
		alphaSearchPath = "/alpha/search"
	}

	// The underlying responses outbound requires baseURL/apiKey. We only need its request body logic.
	// Use a dummy config and then override URL/auth.
	ro, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider("dummy"),
		Transport:      params.Transport,
	})
	if err != nil {
		return nil, err
	}

	return &OutboundTransformer{
		tokens:            params.TokenProvider,
		transport:         params.Transport,
		baseURL:           strings.TrimSuffix(baseURL, "##"),
		alphaSearchPath:   alphaSearchPath,
		official:          isOfficialCodexBaseURL(baseURL),
		responsesOutbound: ro,
	}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIResponse
}

func (t *OutboundTransformer) TokenProvider() oauth.TokenGetter {
	if t == nil {
		return nil
	}

	return t.tokens
}

func (t *OutboundTransformer) AllowPassThroughBody(_ context.Context, llmReq *llm.Request, _ *httpclient.Request) bool {
	if llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIResponse || llmReq.RawRequest == nil {
		return true
	}

	for _, field := range responsesBlockedPassThroughFields {
		if gjson.GetBytes(llmReq.RawRequest.Body, field).Exists() {
			return false
		}
	}

	return true
}

func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return t.responsesOutbound.TransformError(ctx, rawErr)
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, errors.New("request is nil")
	}
	if llmReq.RequestType == llm.RequestTypeAlphaSearch {
		return t.transformAlphaSearchRequest(ctx, llmReq)
	}

	rawSessionID := ""
	rawOriginator := ""
	rawUserAgent := ""
	rawTurnMetadata := ""

	var rawHeaders http.Header

	if llmReq.RawRequest != nil && llmReq.RawRequest.Headers != nil {
		rawHeaders = llmReq.RawRequest.Headers
		rawSessionID = llmReq.RawRequest.Headers.Get(SessionHeader)
		if rawSessionID == "" {
			rawSessionID = llmReq.RawRequest.Headers.Get(SessionHeaderHyphen)
		}
		// Remove underscore variant to prevent it from leaking upstream via MergeInboundRequest.
		llmReq.RawRequest.Headers.Del(SessionHeader)
		rawOriginator = llmReq.RawRequest.Headers.Get("Originator")
		rawUserAgent = llmReq.RawRequest.Headers.Get("User-Agent")
		rawTurnMetadata = llmReq.RawRequest.Headers.Get(TurnMetadataHeader)

		// Responses Lite selects a private Codex protocol mode. It is not
		// identity metadata: never fabricate it for OpenAI-compatible clients.
		// A client-selected Lite request is retained only for the official Codex
		// backend; relays are not assumed to implement the same private protocol.
		if !t.isOfficialCodex() || !strings.EqualFold(strings.TrimSpace(rawHeaders.Get(ResponsesLiteHeader)), "true") {
			rawHeaders.Del(ResponsesLiteHeader)
		}
	}

	creds, err := t.tokens.Get(ctx)
	if err != nil {
		return nil, err
	}

	// Parse account ID from access token JWT.
	accountID := ExtractChatGPTAccountIDFromJWT(creds.AccessToken)

	// Clone request so we do not mutate upstream pipeline state.
	reqCopy := *llmReq
	originalRequestType := reqCopy.RequestType
	originalAPIFormat := reqCopy.APIFormat
	isImageRequest := originalRequestType == llm.RequestTypeImage

	// Codex expects Responses API payload with some strict rules.
	// Always enable stream except for compact requests and disable store.
	//nolint: exhaustive // We only care about compact requests.
	switch reqCopy.RequestType {
	case llm.RequestTypeCompact:
		reqCopy.Stream = lo.ToPtr(false)
	default:
		reqCopy.Stream = lo.ToPtr(true)
	}

	reqCopy.Store = lo.ToPtr(false)

	// Codex recommends parallel tool calls.
	reqCopy.ParallelToolCalls = lo.ToPtr(true)

	if reqCopy.TransformerMetadata == nil {
		reqCopy.TransformerMetadata = map[string]any{}
	}

	if isImageRequest {
		reqCopy.Model = defaultImageMainModel
		reqCopy.TransformerMetadata[responses.ImageGenerationToolModelMetadataKey] = llmReq.Model
	}

	// Ask for encrypted reasoning content so the downstream can surface reasoning blocks.
	if !isImageRequest {
		if _, ok := reqCopy.TransformerMetadata["include"]; !ok {
			reqCopy.TransformerMetadata["include"] = []string{"reasoning.encrypted_content"}
		}

		if reqCopy.ReasoningSummary == nil || *reqCopy.ReasoningSummary == "" {
			// Enable reasoning summary for Codex CLI requests.
			reqCopy.ReasoningSummary = lo.ToPtr("auto")
		}

	}

	// Codex Responses rejects token limit fields, so strip them out.
	reqCopy.MaxCompletionTokens = nil
	reqCopy.MaxTokens = nil

	reqCopy.Metadata = nil

	reqCopy.TransformOptions.ArrayInputs = lo.ToPtr(true)

	hreq, err := t.responsesOutbound.TransformRequest(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}

	if isImageRequest {
		hreq.RequestType = originalRequestType.String()
		hreq.APIFormat = originalAPIFormat.String()
	}

	// Overwrite auth.
	hreq.Auth = &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: creds.AccessToken}
	// Compact requests expect JSON response, others expect SSE stream.
	if llmReq.RequestType == llm.RequestTypeCompact {
		hreq.Headers.Set("Accept", "application/json")
	} else {
		hreq.Headers.Set("Accept", "text/event-stream")
	}

	hreq.Headers.Del("User-Agent")

	if rawOriginator != "" {
		hreq.Headers.Set("Originator", rawOriginator)
	} else {
		hreq.Headers.Set("Originator", AxonHubOriginator)
	}

	if rawUserAgent != "" {
		hreq.Headers.Set("User-Agent", rawUserAgent)
	}

	for _, header := range PassthroughHeaders {
		if value := rawHeaders.Get(header); value != "" {
			hreq.Headers.Set(header, value)
		}
	}

	if rawSessionID != "" {
		hreq.Headers.Set(SessionHeaderHyphen, rawSessionID)
	} else if sessionID := ExtractSessionIDFromTurnMetadata(rawTurnMetadata); sessionID != "" {
		hreq.Headers.Set(SessionHeaderHyphen, sessionID)
	} else if hreq.Headers.Get(SessionHeaderHyphen) == "" {
		if sessionID, ok := shared.GetSessionID(ctx); ok {
			hreq.Headers.Set(SessionHeaderHyphen, sessionID)
		} else {
			hreq.Headers.Set(SessionHeaderHyphen, uuid.NewString())
		}
	}

	// Fabricate the remaining Codex identity headers for non-Codex inbound
	// clients so the upstream always sees a complete Codex session shape.
	sessionID := hreq.Headers.Get(SessionHeaderHyphen)
	windowID := sessionID + ":0"
	if hreq.Headers.Get(ThreadIDHeader) == "" {
		// Codex clients send Thread-Id equal to Session-Id (both identify the
		// conversation/thread); keep thread-scoped upstream behavior (e.g.
		// prompt caching) consistent for non-Codex clients too.
		hreq.Headers.Set(ThreadIDHeader, sessionID)
	}
	if hreq.Headers.Get(WindowIDHeader) == "" {
		hreq.Headers.Set(WindowIDHeader, windowID)
	}
	if hreq.Headers.Get(TurnMetadataHeader) == "" {
		installationID := ""
		if accountID != "" {
			// Deterministic per-account installation id derived from the
			// ChatGPT account id.
			installationID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(accountID)).String()
		}
		turnMetadata, _ := json.Marshal(TurnMetadata{
			InstallationID:      installationID,
			SessionID:           sessionID,
			ThreadID:            sessionID,
			TurnID:              uuid.NewString(),
			WindowID:            windowID,
			RequestKind:         "turn",
			ThreadSource:        "user",
			Sandbox:             "none",
			TurnStartedAtUnixMS: turnStartedAtUnixMS(sessionID),
		})
		hreq.Headers.Set(TurnMetadataHeader, string(turnMetadata))
	}
	if hreq.Headers.Get(ClientRequestIDHeader) == "" {
		hreq.Headers.Set(ClientRequestIDHeader, uuid.NewString())
	}
	if hreq.Headers.Get(BetaFeaturesHeader) == "" {
		hreq.Headers.Set(BetaFeaturesHeader, fabricatedBetaFeatures)
	}

	if accountID != "" {
		hreq.Headers.Set("Chatgpt-Account-Id", accountID)
	}

	if hreq.Headers.Get("Conversation_id") == "" {
		if sessionID := hreq.Headers.Get(SessionHeaderHyphen); sessionID != "" {
			hreq.Headers.Set("Conversation_id", sessionID)
		}
	}

	if hreq.Headers.Get("Version") == "" {
		hreq.Headers.Set("Version", codexDefaultVersion)
	}

	return hreq, nil
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp != nil && httpResp.Request != nil && httpResp.Request.RequestType == llm.RequestTypeAlphaSearch.String() {
		return t.transformAlphaSearchResponse(ctx, httpResp)
	}
	if httpResp != nil && httpResp.Request != nil && httpResp.Request.RequestType == llm.RequestTypeImage.String() {
		if httpResp.StatusCode >= 400 {
			return nil, fmt.Errorf("codex image HTTP error %d: %s", httpResp.StatusCode, httpResp.Body)
		}

		var upstream responses.Response
		if err := json.Unmarshal(httpResp.Body, &upstream); err != nil {
			return nil, err
		}

		metadata := map[string]any{}
		if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
			metadata = httpResp.Request.TransformerMetadata
		}

		return responses.BuildImageResponse(&upstream, metadata)
	}

	return t.responsesOutbound.TransformResponse(ctx, httpResp)
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, req *httpclient.Request, streamIn streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return t.responsesOutbound.TransformStream(ctx, req, streamIn)
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, req *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return t.responsesOutbound.AggregateStreamChunks(ctx, req, chunks)
}

func (t *OutboundTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	inner := executor
	if t != nil && t.transport == responses.TransportWebSocket {
		inner = t.customizeWebSocketExecutor(inner)
	}

	return &codexExecutor{
		inner:        inner,
		httpExecutor: executor,
		transformer:  t,
	}
}

func (t *OutboundTransformer) customizeWebSocketExecutor(executor pipeline.Executor) pipeline.Executor {
	if !responses.ExecutorComparable(executor) {
		return responses.NewWebSocketExecutor(executor)
	}

	t.executorMu.Lock()
	defer t.executorMu.Unlock()

	if t.webSocketExecutors == nil {
		t.webSocketExecutors = make(map[pipeline.Executor]*responses.WebSocketExecutor)
	}
	if cached, ok := t.webSocketExecutors[executor]; ok {
		return cached
	}

	webSocketExecutor := responses.NewWebSocketExecutor(executor)
	t.webSocketExecutors[executor] = webSocketExecutor

	return webSocketExecutor
}

func (t *OutboundTransformer) Stop() {
	if t == nil {
		return
	}

	t.executorMu.Lock()
	executors := make([]*responses.WebSocketExecutor, 0, len(t.webSocketExecutors))
	for _, executor := range t.webSocketExecutors {
		executors = append(executors, executor)
	}
	t.webSocketExecutors = nil
	t.executorMu.Unlock()

	for _, executor := range executors {
		_ = executor.Close()
	}
}

type codexExecutor struct {
	inner        pipeline.Executor
	httpExecutor pipeline.Executor
	transformer  *OutboundTransformer
}

func (e *codexExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	// Compact and alpha search are non-streaming endpoints; proxy them
	// through the real HTTP client instead of the SSE stream path.
	if request.RequestType == string(llm.RequestTypeCompact) ||
		request.RequestType == llm.RequestTypeAlphaSearch.String() {
		return e.httpExecutor.Do(ctx, request)
	}

	// The official Codex backend always streams SSE, so keep the historical
	// DoStream path for it. Compatible relays may instead return a completed
	// Responses JSON document; for those, execute once and dispatch on the
	// response Content-Type. The request is never reissued, which could
	// duplicate model execution and billing.
	if e.transformer != nil && e.transformer.isOfficialCodex() {
		return e.doStreamAndAggregate(ctx, request)
	}

	return e.doOnceAndDispatch(ctx, request)
}

// doStreamAndAggregate preserves the original Codex behavior: consume the
// upstream SSE stream and aggregate the events into a completed Responses
// JSON body.
func (e *codexExecutor) doStreamAndAggregate(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	stream, err := e.inner.DoStream(ctx, request)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = stream.Close()
	}()

	var chunks []*httpclient.StreamEvent
	for stream.Next() {
		ev := stream.Current()
		if ev == nil {
			continue
		}

		chunks = append(chunks, &httpclient.StreamEvent{
			Type:        ev.Type,
			LastEventID: ev.LastEventID,
			Data:        append([]byte(nil), ev.Data...),
		})
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}
	if err := responses.TopLevelWebSocketError(chunks); err != nil {
		return nil, err
	}

	body, _, err := e.transformer.AggregateStreamChunks(ctx, request, chunks)
	if err != nil {
		return nil, err
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    body,
		Request: request,
	}, nil
}

// doOnceAndDispatch sends a single upstream request and inspects the completed
// body. Codex normally responds with SSE even for downstream non-stream
// requests, but compatible relays can return a completed Responses JSON
// document instead. JSON responses pass through unchanged; everything else is
// decoded as SSE and aggregated into a completed JSON response.
func (e *codexExecutor) doOnceAndDispatch(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	response, err := e.inner.Do(ctx, request)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("empty response")
	}
	if isJSONResponse(response) {
		return response, nil
	}

	chunks, err := decodeSSEChunks(ctx, response.Body)
	if err != nil {
		return nil, err
	}
	if err := responses.TopLevelWebSocketError(chunks); err != nil {
		return nil, err
	}

	body, _, err := e.transformer.AggregateStreamChunks(ctx, request, chunks)
	if err != nil {
		return nil, err
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    body,
		Request: request,
	}, nil
}

func isJSONResponse(response *httpclient.Response) bool {
	if response == nil {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(response.Headers.Get("Content-Type"))
	if err != nil {
		return false
	}

	mediaType = strings.ToLower(mediaType)

	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func decodeSSEChunks(ctx context.Context, body []byte) ([]*httpclient.StreamEvent, error) {
	stream := httpclient.NewDefaultSSEDecoder(ctx, io.NopCloser(bytes.NewReader(body)))
	defer func() {
		_ = stream.Close()
	}()

	chunks := make([]*httpclient.StreamEvent, 0)
	for stream.Next() {
		ev := stream.Current()
		if ev == nil {
			continue
		}

		chunks = append(chunks, &httpclient.StreamEvent{
			Type:        ev.Type,
			LastEventID: ev.LastEventID,
			Data:        append([]byte(nil), ev.Data...),
		})
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}

func (e *codexExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return e.inner.DoStream(ctx, request)
}
