// 职责: 验证 Codex 审查只关联到同一订阅凭据下已完成的上游响应。
// 关联文件:
//
//	internal/server/orchestrator/guardian_linkage.go
package orchestrator

import (
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
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
			outbound := &PersistentOutboundTransformer{wrapped: &codex.OutboundTransformer{}}
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
					{Data: []byte(`{"type":"response.created","response":{"id":"resp_started","status":"in_progress"}}`)},
					{Data: []byte(`{"type":"response.completed","response":{"id":"resp_parent","status":"completed"}}`)},
				})
				observed, err := parent.OnOutboundRawStream(ctx, stream)
				require.NoError(t, err)
				require.True(t, observed.Next())
				require.NotNil(t, observed.Current())
				unlinked, err := review.OnOutboundRawRequest(reviewCtx, guardianTestRequest(token, codex.AutoReviewModel))
				require.NoError(t, err)
				require.False(t, gjson.GetBytes(unlinked.Body, "client_metadata.parent_response_id").Exists())
				require.True(t, observed.Next())
				require.NotNil(t, observed.Current())
			} else {
				_, err := parent.OnOutboundRawResponse(ctx, &httpclient.Response{Body: []byte(`{"id":"resp_parent","status":"completed"}`)})
				require.NoError(t, err)
			}

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
			_, err = parent.OnOutboundRawRequest(ctx, next)
			require.NoError(t, err)
			require.False(t, gjson.GetBytes(next.Body, "client_metadata.guardian_credits_requested").Exists())
			outbound.wrapped = &codex.OutboundTransformer{}
			unlinked, err := review.OnOutboundRawRequest(reviewCtx, guardianTestRequest(token, codex.AutoReviewModel))
			require.NoError(t, err)
			require.False(t, gjson.GetBytes(unlinked.Body, "client_metadata.parent_response_id").Exists())
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
