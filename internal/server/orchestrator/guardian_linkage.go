// 职责: 将 Codex 订阅渠道中的工具审查关联到同一凭据下的上游响应。
// 关联文件:
//
//	internal/server/orchestrator/orchestrator.go
//	internal/server/api/safeguards_classifier.go
//
// 注意事项: 仅使用已完成的上游响应 ID，不能跨订阅凭据传递。
package orchestrator

import (
	"context"
	"crypto/sha256"
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
	mu      sync.Mutex
	attempt *guardianParentAttempt
}

type guardianParentAttempt struct {
	identity   guardianIdentity
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
		if !isReviewer || parent.attempt == nil || parent.attempt.identity != identity || parent.attempt.responseID == "" ||
			request.Headers.Get("X-Codex-Guardian") != "reviewer" {
			return request, nil
		}
		return setGuardianMetadata(request, "parent_response_id", parent.attempt.responseID)
	}
	if isReviewer {
		return request, nil
	}
	if _, err := setGuardianMetadata(request, "guardian_credits_requested", "true"); err != nil {
		return nil, err
	}
	parent.attempt = &guardianParentAttempt{identity: identity}
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
		if event != nil && gjson.GetBytes(event.Data, "type").String() == "response.completed" {
			parent.mu.Lock()
			if parent.attempt == attempt {
				recordGuardianResponse(attempt, gjson.GetBytes(event.Data, "response"))
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
