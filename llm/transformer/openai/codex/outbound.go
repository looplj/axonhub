package codex

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
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

var codexHeaders = [][]string{
	{"Accept", "text/event-stream"},
	{"Connection", "Keep-Alive"},
	{"Openai-Beta", "responses=experimental"},
	{"Originator", "codex_cli_rs"},
}

// OutboundTransformer implements transformer.Outbound for Codex proxy.
// It always talks to the Codex Responses upstream (SSE only) and adapts requests accordingly.
//
// It also implements pipeline.ChannelCustomizedExecutor to support non-streaming callers:
// the executor will transparently perform an SSE request and aggregate chunks.
//
//nolint:containedctx // It is used as a transformer.
type OutboundTransformer struct {
	tokens oauth.TokenGetter

	// reuse existing Responses outbound for payload building.
	responsesOutbound *responses.OutboundTransformer
}

var (
	_ transformer.Outbound               = (*OutboundTransformer)(nil)
	_ pipeline.ChannelCustomizedExecutor = (*OutboundTransformer)(nil)
)

type Params struct {
	TokenProvider oauth.TokenGetter
	BaseURL       string
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

	// The underlying responses outbound requires baseURL/apiKey. We only need its request body logic.
	// Use a dummy config and then override URL/auth.
	ro, err := responses.NewOutboundTransformer(baseURL, "dummy")
	if err != nil {
		return nil, err
	}

	return &OutboundTransformer{
		tokens:            params.TokenProvider,
		responsesOutbound: ro,
	}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIResponse
}

func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return t.responsesOutbound.TransformError(ctx, rawErr)
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, errors.New("request is nil")
	}

	rawUA := ""
	keepClientUA := false
	rawVersion := ""
	keepClientVersion := false
	rawSessionID := ""

	if llmReq.RawRequest != nil && llmReq.RawRequest.Headers != nil {
		rawUA = llmReq.RawRequest.Headers.Get("User-Agent")
		keepClientUA = isCodexCLIUserAgent(rawUA)
		rawVersion = llmReq.RawRequest.Headers.Get("Version")
		keepClientVersion = keepClientUA && isCodexCLIVersion(rawVersion)
		rawSessionID = llmReq.RawRequest.Headers.Get("Session_id")

		for _, header := range codexHeaders {
			llmReq.RawRequest.Headers.Del(header[0])
		}

		llmReq.RawRequest.Headers.Del("Conversation_id")
		llmReq.RawRequest.Headers.Del("Chatgpt-Account-Id")

		if !keepClientVersion {
			llmReq.RawRequest.Headers.Del("Version")
		}

		if !keepClientUA {
			llmReq.RawRequest.Headers.Del("User-Agent")
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

	// Codex expects Responses API payload with some strict rules.
	// Always enable stream and disable store.
	reqCopy.Stream = lo.ToPtr(true)
	reqCopy.Store = lo.ToPtr(false)

	// Codex recommends parallel tool calls.
	reqCopy.ParallelToolCalls = lo.ToPtr(true)

	// Ask for encrypted reasoning content so the downstream can surface reasoning blocks.
	if reqCopy.TransformerMetadata == nil {
		reqCopy.TransformerMetadata = map[string]any{}
	}

	if _, ok := reqCopy.TransformerMetadata["include"]; !ok {
		reqCopy.TransformerMetadata["include"] = []string{"reasoning.encrypted_content"}
	}

	// Codex Responses rejects token limit fields, so strip them out.
	reqCopy.MaxCompletionTokens = nil
	reqCopy.MaxTokens = nil

	// Strip sampling params and tier.
	reqCopy.ServiceTier = nil
	reqCopy.Temperature = nil
	reqCopy.TopP = nil
	reqCopy.Metadata = nil

	// Codex upstream validates the raw `instructions` string more strictly.
	// If incoming request is not already a Codex CLI prompt, force the Codex CLI instructions.
	if !isCodexRequest(reqCopy.Messages) {
		reqCopy.Messages = appendCodexSystemInstruction(reqCopy.Messages)
	}

	hreq, err := t.responsesOutbound.TransformRequest(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}

	// Codex upstream expects SSE.
	for _, header := range codexHeaders {
		hreq.Headers.Set(header[0], header[1])
	}

	// Overwrite auth.
	hreq.Auth = &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: creds.AccessToken}

	// Keep Codex-specific headers.
	if keepClientUA && rawUA != "" {
		hreq.Headers.Set("User-Agent", rawUA)
	} else {
		hreq.Headers.Set("User-Agent", UserAgent)
	}

	if keepClientVersion && rawVersion != "" {
		hreq.Headers.Set("Version", rawVersion)
	} else {
		hreq.Headers.Set("Version", codexDefaultVersion)
	}

	if rawSessionID != "" {
		hreq.Headers.Set("Session_id", rawSessionID)
	} else if hreq.Headers.Get("Session_id") == "" {
		if sessionID, ok := shared.GetSessionID(ctx); ok {
			hreq.Headers.Set("Session_id", sessionID)
		} else {
			hreq.Headers.Set("Session_id", uuid.NewString())
		}
	}

	if accountID != "" {
		hreq.Headers.Set("Chatgpt-Account-Id", accountID)
	}

	return hreq, nil
}

func appendCodexSystemInstruction(msgs []llm.Message) []llm.Message {
	systemMsg := llm.Message{
		Role: "system",
		Content: llm.MessageContent{
			Content: lo.ToPtr(CodexInstructions),
		},
	}

	return append([]llm.Message{systemMsg}, msgs...)
}

func isCodexRequest(msgs []llm.Message) bool {
	for _, msg := range msgs {
		if msg.Role != "system" && msg.Role != "developer" {
			continue
		}

		if msg.Content.Content != nil {
			content := *msg.Content.Content
			if strings.HasPrefix(content, CodexInstructionPrefix) || strings.HasPrefix(content, "You are Codex") {
				return true
			}
		} else if len(msg.Content.MultipleContent) > 0 {
			for _, item := range msg.Content.MultipleContent {
				if item.Text != nil && (strings.HasPrefix(*item.Text, CodexInstructionPrefix) || strings.HasPrefix(*item.Text, "You are Codex")) {
					return true
				}
			}
		}
	}

	return false
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	// Codex upstream returns Responses API response.
	return t.responsesOutbound.TransformResponse(ctx, httpResp)
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, streamIn streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return t.responsesOutbound.TransformStream(ctx, streamIn)
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return t.responsesOutbound.AggregateStreamChunks(ctx, chunks)
}

func (t *OutboundTransformer) CustomizeExecutor(executor pipeline.Executor) pipeline.Executor {
	return &codexExecutor{
		inner:       executor,
		transformer: t,
	}
}

type codexExecutor struct {
	inner       pipeline.Executor
	transformer *OutboundTransformer
}

func (e *codexExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	// Ensure Codex-required headers are not overridden by inbound headers.
	for _, header := range codexHeaders {
		request.Headers.Set(header[0], header[1])
	}

	if !isCodexCLIUserAgent(request.Headers.Get("User-Agent")) {
		request.Headers.Set("User-Agent", UserAgent)
	}

	if request.Headers.Get("Conversation_id") == "" {
		request.Headers.Set("Conversation_id", request.Headers.Get("Session_id"))
	}

	if !isCodexCLIUserAgent(request.Headers.Get("User-Agent")) || !isCodexCLIVersion(request.Headers.Get("Version")) {
		request.Headers.Set("Version", codexDefaultVersion)
	}

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
		// Copy data because decoder may reuse buffers.
		copied := &httpclient.StreamEvent{Type: ev.Type, LastEventID: ev.LastEventID, Data: append([]byte(nil), ev.Data...)}
		chunks = append(chunks, copied)
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	body, _, err := e.transformer.AggregateStreamChunks(ctx, chunks)
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

func (e *codexExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	// Ensure Codex-required headers are not overridden by inbound headers.
	for _, header := range codexHeaders {
		request.Headers.Set(header[0], header[1])
	}

	if !isCodexCLIUserAgent(request.Headers.Get("User-Agent")) {
		request.Headers.Set("User-Agent", UserAgent)
	}

	if request.Headers.Get("Conversation_id") == "" {
		request.Headers.Set("Conversation_id", request.Headers.Get("Session_id"))
	}

	if !isCodexCLIUserAgent(request.Headers.Get("User-Agent")) || !isCodexCLIVersion(request.Headers.Get("Version")) {
		request.Headers.Set("Version", codexDefaultVersion)
	}

	return e.inner.DoStream(ctx, request)
}

func isCodexCLIUserAgent(value string) bool {
	return strings.HasPrefix(value, "codex_cli_rs/")
}
