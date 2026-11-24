// 职责: 验证 Codex 审查只关联到同一订阅凭据下上游响应及其 Lite 请求格式。
// 关联文件:
//
//	internal/server/orchestrator/guardian_linkage.go

package orchestrator

import (
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

func TestGuardianLinkage(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		name := "response"
		if streaming {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			ctx := WithSafeguardsParent(t.Context())
			link := ctx.Value(guardianLinkageKey{}).(guardianLinkage)
			reviewCtx := WithSafeguardsReview(ctx)
			reviewLink := reviewCtx.Value(guardianLinkageKey{}).(guardianLinkage)
			parentChannel := quotaRoutingCandidate(2, "")
			outbound := &PersistentOutboundTransformer{
				wrapped: &codex.OutboundTransformer{},
				state:   &PersistenceState{CurrentCandidate: parentChannel},
			}
			parent := guardianLinkageMiddleware{link: link, outbound: outbound}
			review := guardianLinkageMiddleware{link: reviewLink, outbound: outbound}

			token := guardianTestToken(t, "parent-user")
			parentReq := guardianTestRequest(token, "gpt-6-luna")
			parentReq.Body = []byte(`{"model":"gpt-6-luna","client_metadata":{"existing":"keep"}}`)
			_, err := parent.OnOutboundRawRequest(ctx, parentReq)
			require.NoError(t, err)
			require.Equal(t, "true", gjson.GetBytes(parentReq.Body, "client_metadata.guardian_credits_requested").String())
			require.Equal(t, "keep", gjson.GetBytes(parentReq.Body, "client_metadata.existing").String())

			if streaming {
				stream := streams.SliceStream([]*httpclient.StreamEvent{
					{Data: []byte(`{"type":"response.created","response":{"id":"resp_parent","status":"in_progress"}}`)},
					{Data: []byte(`{"type":"response.completed","response":{"id":"resp_parent","status":"completed"}}`)},
				})
				observed, err := parent.OnOutboundRawStream(ctx, stream)
				require.NoError(t, err)
				require.True(t, observed.Next())
				require.NotNil(t, observed.Current())
				linked, err := review.OnOutboundRawRequest(reviewCtx, guardianTestRequest(token, codex.AutoReviewModel))
				require.NoError(t, err)
				require.Equal(t, "resp_parent", gjson.GetBytes(linked.Body, "client_metadata.parent_response_id").String())
				require.True(t, observed.Next())
				require.NotNil(t, observed.Current())
			} else {
				_, err := parent.OnOutboundRawResponse(ctx, &httpclient.Response{Body: []byte(`{"id":"resp_parent","status":"completed"}`)})
				require.NoError(t, err)
			}

			require.Equal(t, parentChannel.Channel.ID, guardianReviewParentChannelID(reviewCtx))
			require.Equal(t, llm.APIFormatOpenAIResponse.String(), guardianReviewParentAPIFormat(reviewCtx))
			for _, tc := range []struct {
				name string
				req  *httpclient.Request
				want string
			}{
				{name: "same credential", req: guardianTestRequest(token, codex.AutoReviewModel), want: "resp_parent"},
				{name: "different credential in the same account", req: guardianTestRequest(guardianTestToken(t, "other-user"), codex.AutoReviewModel)},
				{name: "fallback model", req: guardianTestRequest(token, "other-reviewer")},
			} {
				t.Run(tc.name, func(t *testing.T) {
					req, err := review.OnOutboundRawRequest(reviewCtx, tc.req)
					require.NoError(t, err)
					require.Equal(t, tc.want, gjson.GetBytes(req.Body, "client_metadata.parent_response_id").String())
					require.False(t, gjson.GetBytes(req.Body, "client_metadata.guardian_credits_requested").Exists())
				})
			}

			relay := guardianTestRequest(token, codex.AutoReviewModel)
			relay.URL = "https://relay.example/backend-api/codex/responses"
			_, err = review.OnOutboundRawRequest(reviewCtx, relay)
			require.NoError(t, err)
			require.False(t, gjson.GetBytes(relay.Body, "client_metadata.parent_response_id").Exists())

			outbound.wrapped = nil
			next := guardianTestRequest(token, "gpt-6-luna")
			next.APIFormat = llm.APIFormatAnthropicMessage.String()
			_, err = parent.OnOutboundRawRequest(ctx, next)
			require.NoError(t, err)
			require.False(t, gjson.GetBytes(next.Body, "client_metadata.guardian_credits_requested").Exists())
			require.Zero(t, guardianReviewParentChannelID(reviewCtx))
			require.Equal(t, llm.APIFormatAnthropicMessage.String(), guardianReviewParentAPIFormat(reviewCtx))
			outbound.wrapped = &codex.OutboundTransformer{}
			unlinked, err := review.OnOutboundRawRequest(reviewCtx, guardianTestRequest(token, codex.AutoReviewModel))
			require.NoError(t, err)
			require.False(t, gjson.GetBytes(unlinked.Body, "client_metadata.parent_response_id").Exists())
		})
	}
}

func TestGuardianResponsesLite(t *testing.T) {
	for _, transport := range []string{"http", "websocket"} {
		t.Run(transport, func(t *testing.T) {
			ctx := WithSafeguardsReview(WithSafeguardsParent(t.Context()))
			link := ctx.Value(guardianLinkageKey{}).(guardianLinkage)
			token := guardianTestToken(t, "reviewer")
			outbound, err := codex.NewOutboundTransformer(codex.Params{
				Transport:     transport,
				TokenProvider: oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{AccessToken: token}),
			})
			require.NoError(t, err)
			request, err := outbound.TransformRequest(ctx, &llm.Request{
				RequestType: llm.RequestTypeChat,
				Model:       codex.AutoReviewModel,
				Messages: []llm.Message{
					{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("review policy")}},
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("proposed action")}},
				},
				ServiceTier: lo.ToPtr("priority"),
			})
			require.NoError(t, err)
			request, err = httpclient.FinalizeAuthHeaders(request)
			require.NoError(t, err)
			request.Body, err = sjson.SetBytes(request.Body, "client_metadata", map[string]string{
				"custom":                     "keep",
				"parent_response_id":         "untrusted",
				"guardian_credits_requested": "true",
			})
			require.NoError(t, err)
			request.Headers.Set("X-Codex-Routing-Hint", "model=codex-auto-review;tier=priority")

			review := guardianLinkageMiddleware{
				link:     link,
				outbound: &PersistentOutboundTransformer{wrapped: outbound},
			}
			request, err = review.OnOutboundRawRequest(ctx, outbound.FinalizeTransportRequest(request))
			require.NoError(t, err)
			body := gjson.ParseBytes(request.Body)
			require.Equal(t, "codex-auto-review", body.Get("model").String())
			require.False(t, body.Get("instructions").Exists())
			require.False(t, body.Get("service_tier").Exists())
			require.False(t, body.Get("tools").Exists())
			require.False(t, body.Get("tool_choice").Exists())
			require.Equal(t, gjson.False, body.Get("parallel_tool_calls").Type)
			require.Equal(t, "developer", body.Get("input.0.role").String())
			require.Equal(t, "review policy", body.Get("input.0.content.0.text").String())
			require.Equal(t, "user", body.Get("input.1.role").String())
			require.Equal(t, "proposed action", body.Get("input.1.content.0.text").String())
			require.Equal(t, "all_turns", body.Get("reasoning.context").String())
			require.Equal(t, "low", body.Get("reasoning.effort").String())

			metadata := body.Get("client_metadata")
			require.Equal(t, "guardian", metadata.Get("x-openai-subagent").String())
			require.Equal(t, "keep", metadata.Get("custom").String())
			require.False(t, metadata.Get("parent_response_id").Exists())
			require.False(t, metadata.Get("guardian_credits_requested").Exists())
			turnMetadata := request.Headers.Get(codex.TurnMetadataHeader)
			require.Equal(t, "guardian_review", gjson.Get(turnMetadata, "thread_source").String())
			require.NotEmpty(t, gjson.Get(turnMetadata, "session_id").String())
			require.Equal(t, turnMetadata, metadata.Get("x-codex-turn-metadata").String())
			require.Empty(t, request.Headers.Get("X-Codex-Routing-Hint"))
			liteFrameSignal := metadata.Get("ws_request_header_x_openai_internal_codex_responses_lite")
			if transport == "websocket" {
				require.Equal(t, llm.APIFormatOpenAIResponseWebSocket.String(), request.APIFormat)
				require.Equal(t, "true", liteFrameSignal.String())
				require.Empty(t, request.Headers.Get(codex.ResponsesLiteHeader))
			} else {
				require.False(t, liteFrameSignal.Exists())
				require.Equal(t, "true", request.Headers.Get(codex.ResponsesLiteHeader))
			}
		})
	}
}

func TestGuardianParentChannelPreference(t *testing.T) {
	const parentID = 2
	req := &llm.Request{Model: codex.AutoReviewModel}
	fallback := quotaRoutingCandidate(1, "")
	parent := quotaRoutingCandidate(parentID, "")
	parent.Priority = 1 // Default routing would pick fallback even without retries.
	for _, tc := range []struct {
		name       string
		isReview   bool
		hasParent  bool
		restricted bool
		exhausted  bool
		want       int
	}{
		{name: "eligible Codex parent", isReview: true, hasParent: true, want: parentID},
		{name: "non-Codex parent", isReview: true, want: 1},
		{name: "ordinary request", hasParent: true, want: 1},
		{name: "parent is ineligible for reviewer", isReview: true, hasParent: true, restricted: true, want: 1},
		{name: "parent quota is exhausted", isReview: true, hasParent: true, exhausted: true, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithSafeguardsParent(t.Context())
			link := ctx.Value(guardianLinkageKey{}).(guardianLinkage)
			if tc.hasParent {
				link.parent.attempt = &guardianParentAttempt{channelID: parentID, responseID: "resp_parent"}
			}
			if tc.isReview {
				ctx = WithSafeguardsReview(ctx)
			}
			candidates := []*ChannelModelsCandidate{fallback}
			if !tc.restricted {
				candidates = append(candidates, parent)
			}
			var statuses map[int]*biz.QuotaChannelStatus
			if tc.exhausted {
				statuses = map[int]*biz.QuotaChannelStatus{parentID: quotaExhaustedStatus()}
			}
			balanced, gate := newQuotaRoutingSelector(candidates, statuses, biz.QuotaRoutingSettings{})
			balanced.retryPolicy = &biz.RetryPolicy{}
			got, err := balanced.Select(ctx, req)
			require.NoError(t, err)
			require.NotEmpty(t, got)
			require.Equal(t, tc.want, got[0].Channel.ID)
			require.False(t, got[0].TraceSticky)

			direct := WithQuotaRoutingSelector(&staticChannelSelector{candidates: candidates}, gate)
			got, err = direct.Select(ctx, req)
			require.NoError(t, err)
			require.NotEmpty(t, got)
			require.Equal(t, tc.want, got[0].Channel.ID)
		})
	}
}

func TestSafeguardsReviewerFollowsParentProtocol(t *testing.T) {
	httpFormat := llm.APIFormatOpenAIResponse.String()
	wsFormat := llm.APIFormatOpenAIResponseWebSocket.String()
	anthropicFormat := llm.APIFormatAnthropicMessage.String()
	for _, tc := range []struct {
		name         string
		parentFormat string
		formats      []string
		review       bool
		forced       bool
		preferWS     bool
		want         string
	}{
		{name: "HTTP parent", parentFormat: httpFormat, formats: []string{wsFormat, httpFormat}, review: true, preferWS: true, want: httpFormat},
		{name: "WebSocket parent", parentFormat: wsFormat, formats: []string{httpFormat, wsFormat}, review: true, want: wsFormat},
		{name: "Anthropic parent", parentFormat: anthropicFormat, formats: []string{httpFormat, anthropicFormat}, review: true, want: anthropicFormat},
		{name: "no parent protocol uses normal endpoint order", formats: []string{httpFormat, wsFormat}, review: true, want: httpFormat},
		{name: "no parent protocol uses opted-in WebSocket preference", formats: []string{httpFormat, wsFormat}, review: true, preferWS: true, want: wsFormat},
		{name: "missing parent endpoint uses normal endpoint order", parentFormat: wsFormat, formats: []string{httpFormat, anthropicFormat}, review: true, want: httpFormat},
		{name: "HTTP-only model override", parentFormat: wsFormat, formats: []string{httpFormat}, review: true, forced: true, want: httpFormat},
		{name: "ordinary request uses normal endpoint order", parentFormat: wsFormat, formats: []string{httpFormat, wsFormat}, want: httpFormat},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithSafeguardsParent(t.Context())
			link := ctx.Value(guardianLinkageKey{}).(guardianLinkage)
			parent := guardianLinkageMiddleware{link: link, outbound: &PersistentOutboundTransformer{}}
			if tc.parentFormat != "" {
				_, err := parent.OnOutboundRawRequest(ctx, &httpclient.Request{APIFormat: tc.parentFormat})
				require.NoError(t, err)
			}
			if tc.review {
				ctx = WithSafeguardsReview(ctx)
			}
			endpoints := make([]objects.ChannelEndpoint, 0, len(tc.formats))
			for _, format := range tc.formats {
				endpoints = append(endpoints, objects.ChannelEndpoint{APIFormat: format})
			}
			request := &llm.Request{
				Model:       codex.AutoReviewModel,
				APIFormat:   llm.APIFormatOpenAIChatCompletion,
				RequestType: llm.RequestTypeChat,
			}
			ch := &biz.Channel{Channel: &ent.Channel{}}
			if tc.preferWS {
				ch.Settings = &objects.ChannelSettings{PreferResponsesWebSocket: true}
			}
			require.Equal(t, tc.want, selectEndpointAPIFormat(ctx, ch, endpoints, request, tc.forced))
		})
	}
}

func guardianTestRequest(token, model string) *httpclient.Request {
	return &httpclient.Request{
		URL:       "https://chatgpt.com/backend-api/codex/responses",
		APIFormat: string(llm.APIFormatOpenAIResponse),
		Headers: http.Header{
			"Authorization":      {"Bearer " + token},
			"Chatgpt-Account-Id": {"test-account"},
			"X-Codex-Guardian":   {"reviewer"},
		},
		Body: []byte(`{"model":"` + model + `"}`),
	}
}

func guardianTestToken(t *testing.T, subject string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": subject,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "test-account",
		},
	}).SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return token
}
