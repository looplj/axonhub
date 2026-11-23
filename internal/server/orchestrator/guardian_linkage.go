// 职责: 为 Codex 订阅渠道构建 Lite 工具审查请求，并关联同一凭据下的上游响应。
// 关联文件:
//
//	internal/server/orchestrator/orchestrator.go
//	internal/server/api/safeguards_classifier.go
//
// 注意事项: 仅使用真实的上游响应 ID，不能跨订阅凭据传递。

package orchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

type guardianLinkageKey struct{}

type guardianLinkage struct {
	parent *guardianParent
	review bool
}

type guardianParent struct {
	mu        sync.Mutex
	apiFormat string
	attempt   *guardianParentAttempt
}

type guardianParentAttempt struct {
	identity   guardianIdentity
	channelID  int
	responseID string
}

type guardianIdentity struct {
	host string
	auth [sha256.Size]byte
}

// WithSafeguardsParent tracks the upstream response for a request with local safeguards.
func WithSafeguardsParent(ctx context.Context) context.Context {
	return context.WithValue(ctx, guardianLinkageKey{}, guardianLinkage{parent: &guardianParent{}})
}

// WithSafeguardsReview identifies an internal classifier call for the parent request.
func WithSafeguardsReview(ctx context.Context) context.Context {
	link, ok := ctx.Value(guardianLinkageKey{}).(guardianLinkage)
	if !ok {
		return ctx
	}
	link.review = true
	return context.WithValue(ctx, guardianLinkageKey{}, link)
}

func guardianReviewLink(ctx context.Context) (guardianLinkage, bool) {
	link, ok := ctx.Value(guardianLinkageKey{}).(guardianLinkage)
	return link, ok && link.review
}

func guardianReviewParentChannelID(ctx context.Context) int {
	link, ok := guardianReviewLink(ctx)
	if !ok {
		return 0
	}
	link.parent.mu.Lock()
	defer link.parent.mu.Unlock()
	if attempt := link.parent.attempt; attempt != nil && attempt.responseID != "" {
		return attempt.channelID
	}
	return 0
}

func guardianReviewParentAPIFormat(ctx context.Context) string {
	link, ok := guardianReviewLink(ctx)
	if !ok {
		return ""
	}
	link.parent.mu.Lock()
	defer link.parent.mu.Unlock()
	return link.parent.apiFormat
}

type guardianLinkageMiddleware struct {
	pipeline.DummyMiddleware

	link     guardianLinkage
	outbound *PersistentOutboundTransformer
}

func (m *guardianLinkageMiddleware) Name() string { return "guardian_linkage" }

func (m *guardianLinkageMiddleware) OnOutboundRawRequest(_ context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	parent := m.link.parent
	parent.mu.Lock()
	defer parent.mu.Unlock()

	if !m.link.review {
		parent.apiFormat = request.APIFormat
		parent.attempt = nil
	}
	if _, ok := m.outbound.wrapped.(*codex.OutboundTransformer); !ok {
		return request, nil
	}
	identity, ok := subscriptionGuardianIdentity(request)
	if !ok {
		return request, nil
	}

	isReviewer := gjson.GetBytes(request.Body, "model").String() == codex.AutoReviewModel
	if m.link.review {
		if !isReviewer || request.Headers.Get("X-Codex-Guardian") != "reviewer" {
			return request, nil
		}
		var parentID string
		if parent.attempt != nil && parent.attempt.identity == identity {
			parentID = parent.attempt.responseID
		}
		return prepareGuardianLiteReview(request, parentID)
	}
	if isReviewer {
		return request, nil
	}
	if _, err := setGuardianMetadata(request, "guardian_credits_requested", "true"); err != nil {
		return nil, err
	}
	parent.attempt = &guardianParentAttempt{
		identity:  identity,
		channelID: m.outbound.GetCurrentChannel().ID,
	}
	return request, nil
}

func subscriptionGuardianIdentity(request *httpclient.Request) (guardianIdentity, bool) {
	format := llm.APIFormat(request.APIFormat)
	if format != llm.APIFormatOpenAIResponse && format != llm.APIFormatOpenAIResponseWebSocket {
		return guardianIdentity{}, false
	}
	endpoint, err := url.Parse(request.URL)
	if err != nil || (endpoint.Scheme != "https" && endpoint.Scheme != "wss") || endpoint.Path != "/backend-api/codex/responses" {
		return guardianIdentity{}, false
	}
	host := strings.ToLower(endpoint.Hostname())
	if host != "chatgpt.com" && !strings.HasSuffix(host, ".chatgpt.com") {
		return guardianIdentity{}, false
	}
	token, ok := strings.CutPrefix(request.Headers.Get("Authorization"), "Bearer ")
	if !ok {
		return guardianIdentity{}, false
	}
	account := codex.ExtractChatGPTAccountIDFromJWT(token)
	if account == "" || account != request.Headers.Get("Chatgpt-Account-Id") {
		return guardianIdentity{}, false
	}
	return guardianIdentity{host: host, auth: sha256.Sum256([]byte(token))}, true
}

func setGuardianMetadata(request *httpclient.Request, key, value string) (*httpclient.Request, error) {
	body, err := sjson.SetBytes(request.Body, "client_metadata."+key, value)
	if err != nil {
		return nil, fmt.Errorf("set Codex Guardian metadata: %w", err)
	}
	request.Body = body
	request.Headers.Del("Content-Length")
	return request, nil
}

// prepareGuardianLiteReview converts the internal tool-free classifier to the
// Codex Responses Lite request shape after channel overrides have run.
func prepareGuardianLiteReview(request *httpclient.Request, parentID string) (*httpclient.Request, error) {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(request.Body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Codex Guardian request: %w", err)
	}

	if instructions, _ := payload["instructions"].(string); instructions != "" {
		input := []any{guardianInputMessage("developer", instructions)}
		switch original := payload["input"].(type) {
		case []any:
			input = append(input, original...)
		case string:
			input = append(input, guardianInputMessage("user", original))
		case nil:
		default:
			return nil, fmt.Errorf("invalid Codex Guardian input type %T", original)
		}
		payload["input"] = input
	}
	delete(payload, "instructions")
	delete(payload, "service_tier")
	delete(payload, "tools")
	delete(payload, "tool_choice")
	payload["parallel_tool_calls"] = false

	reasoning, _ := payload["reasoning"].(map[string]any)
	if reasoning == nil {
		reasoning = make(map[string]any)
		payload["reasoning"] = reasoning
	}
	reasoning["context"] = "all_turns"
	if _, ok := reasoning["effort"]; !ok {
		reasoning["effort"] = "low"
	}

	metadata, _ := payload["client_metadata"].(map[string]any)
	if metadata == nil {
		metadata = make(map[string]any)
		payload["client_metadata"] = metadata
	}
	metadata["x-openai-subagent"] = "guardian"
	delete(metadata, "guardian_credits_requested")
	delete(metadata, "parent_response_id")
	if parentID != "" {
		metadata["parent_response_id"] = parentID
	}

	turnMetadata := request.Headers.Get(codex.TurnMetadataHeader)
	turnMetadata, err := sjson.Set(turnMetadata, "thread_source", "guardian_review")
	if err != nil {
		return nil, fmt.Errorf("set Codex Guardian thread source: %w", err)
	}
	metadata["x-codex-turn-metadata"] = turnMetadata

	const liteWebSocketMetadata = "ws_request_header_x_openai_internal_codex_responses_lite"
	delete(metadata, liteWebSocketMetadata)
	webSocket := request.APIFormat == llm.APIFormatOpenAIResponseWebSocket.String()
	if webSocket {
		metadata[liteWebSocketMetadata] = "true"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Codex Guardian request: %w", err)
	}

	request.Body = body
	request.Headers.Del("Content-Length")
	request.Headers.Del("X-Codex-Routing-Hint")
	request.Headers.Set(codex.TurnMetadataHeader, turnMetadata)
	if webSocket {
		request.Headers.Del(codex.ResponsesLiteHeader)
	} else {
		request.Headers.Set(codex.ResponsesLiteHeader, "true")
	}
	return request, nil
}

func guardianInputMessage(role, text string) map[string]any {
	return map[string]any{
		"type": "message",
		"role": role,
		"content": []map[string]string{
			{"type": "input_text", "text": text},
		},
	}
}

func (m *guardianLinkageMiddleware) OnOutboundRawResponse(_ context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	if !m.link.review {
		parent := m.link.parent
		parent.mu.Lock()
		defer parent.mu.Unlock()
		if attempt := parent.attempt; attempt != nil {
			recordGuardianResponse(attempt, gjson.ParseBytes(response.Body))
		}
	}
	return response, nil
}

func (m *guardianLinkageMiddleware) OnOutboundRawStream(_ context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	if m.link.review {
		return stream, nil
	}
	parent := m.link.parent
	parent.mu.Lock()
	attempt := parent.attempt
	parent.mu.Unlock()
	if attempt == nil {
		return stream, nil
	}

	observed := streams.Map(stream, func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
		if event == nil {
			return event
		}
		eventType := gjson.GetBytes(event.Data, "type").String()
		if eventType == "response.created" || eventType == "response.completed" {
			// Safeguards still drains the parent stream before classifying.
			id := gjson.GetBytes(event.Data, "response.id").String()
			parent.mu.Lock()
			if parent.attempt == attempt && id != "" && (eventType == "response.created" || attempt.responseID == "") {
				attempt.responseID = id
			}
			parent.mu.Unlock()
		}
		return event
	})
	return httpclient.WithResponseHeaders(preserveDeliveryCapabilities(stream, observed), httpclient.GetResponseHeaders(stream)), nil
}

func recordGuardianResponse(attempt *guardianParentAttempt, response gjson.Result) {
	if id := response.Get("id").String(); id != "" && response.Get("status").String() == "completed" {
		attempt.responseID = id
	}
}
