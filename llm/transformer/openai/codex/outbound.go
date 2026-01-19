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
)

const codexAPIURL = "https://chatgpt.com/backend-api/codex/responses"

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
}

func NewOutboundTransformer(params Params) (*OutboundTransformer, error) {
	if params.TokenProvider == nil {
		return nil, errors.New("token provider is required")
	}

	// The underlying responses outbound requires baseURL/apiKey. We only need its request body logic.
	// Use a dummy config and then override URL/auth.
	ro, err := responses.NewOutboundTransformer("https://api.openai.com/v1", "dummy")
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

	// Codex upstream validates the raw `instructions` string more strictly.
	// If incoming request is not already a Codex CLI prompt, force the Codex CLI instructions.
	instructions := responsesInstructionFromMessages(reqCopy.Messages)

	isCodex := strings.HasPrefix(instructions, "You are a coding agent running in the Codex CLI") || strings.HasPrefix(instructions, "You are Codex")
	if !isCodex {
		reqCopy.Messages = setCodexSystemInstruction(reqCopy.Messages)
	}

	hreq, err := t.responsesOutbound.TransformRequest(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}

	// Force Codex upstream (this is a ChatGPT backend endpoint, not api.openai.com).
	hreq.URL = codexAPIURL

	// Codex upstream expects SSE.
	hreq.Headers.Set("Accept", "text/event-stream")
	hreq.Headers.Set("Connection", "Keep-Alive")
	hreq.Headers.Set("Openai-Beta", "responses=experimental")
	hreq.Headers.Set("Originator", "codex_cli_rs")
	hreq.Headers.Set("Session_id", uuid.NewString())
	hreq.Headers.Set("Version", "0.21.0")

	// Overwrite auth.
	hreq.Auth = &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: creds.AccessToken}

	// Keep Codex-specific headers.
	hreq.Headers.Set("User-Agent", UserAgent)

	if accountID != "" {
		hreq.Headers.Set("Chatgpt-Account-Id", accountID)
	}

	return hreq, nil
}

func setCodexSystemInstruction(msgs []llm.Message) []llm.Message {
	systemMsg := llm.Message{
		Role: "system",
		Content: llm.MessageContent{
			Content: lo.ToPtr(CodexInstructions),
		},
	}

	// Drop existing system/developer instructions to keep Codex `instructions` clean.
	var filtered []llm.Message

	for _, msg := range msgs {
		if msg.Role == "system" || msg.Role == "developer" {
			continue
		}

		filtered = append(filtered, msg)
	}

	return append([]llm.Message{systemMsg}, filtered...)
}

func responsesInstructionFromMessages(msgs []llm.Message) string {
	var parts []string

	for _, msg := range msgs {
		if msg.Role != "system" && msg.Role != "developer" {
			continue
		}

		if msg.Content.Content != nil {
			parts = append(parts, *msg.Content.Content)
		}
	}

	return strings.Join(parts, "\n")
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
	request.Headers.Set("Accept", "text/event-stream")
	request.Headers.Set("User-Agent", UserAgent)
	request.Headers.Set("Connection", "Keep-Alive")
	request.Headers.Set("Openai-Beta", "responses=experimental")
	request.Headers.Set("Originator", "codex_cli_rs")

	if request.Headers.Get("Session_id") == "" {
		request.Headers.Set("Session_id", uuid.NewString())
	}

	if request.Headers.Get("Conversation_id") == "" {
		request.Headers.Set("Conversation_id", request.Headers.Get("Session_id"))
	}

	request.Headers.Set("Version", "0.21.0")

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
	request.Headers.Set("Accept", "text/event-stream")
	request.Headers.Set("User-Agent", UserAgent)
	request.Headers.Set("Connection", "Keep-Alive")
	request.Headers.Set("Openai-Beta", "responses=experimental")
	request.Headers.Set("Originator", "codex_cli_rs")

	if request.Headers.Get("Session_id") == "" {
		request.Headers.Set("Session_id", uuid.NewString())
	}

	if request.Headers.Get("Conversation_id") == "" {
		request.Headers.Set("Conversation_id", request.Headers.Get("Session_id"))
	}

	request.Headers.Set("Version", "0.21.0")

	return e.inner.DoStream(ctx, request)
}
