package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// TestStreamDeliveryE2E_ResponsesDecorators verifies that Responses decorators
// preserve delivery finalization and persisted usage across route and live-preview settings.
func TestStreamDeliveryE2E_ResponsesDecorators(t *testing.T) {
	for _, realRoute := range []bool{false, true} {
		for _, livePreview := range []bool{false, true} {
			t.Run(fmt.Sprintf("real_route=%v/live_preview=%v", realRoute, livePreview), func(t *testing.T) {
				// Usage accompanies the finishing choice, so accounting needs a clean
				// source end. This is distinct from a separate usage-only final chunk.
				payload := "data: {\"id\":\"chat\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\n" +
					"data: {\"id\":\"chat\",\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":34,\"total_tokens\":46}}\n\n" +
					"data: [DONE]\n\n"
				fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion, payload, false)
				h := fixture.handler(t)
				h.ChatCompletionOrchestrator.Inbound = responses.NewInboundTransformer()
				h.streamAdapterFactory = newResponsesStreamAdapter
				policy := *h.ChatCompletionOrchestrator.SystemService.StoragePolicyOrDefault(fixture.ctx)
				policy.LivePreview = livePreview
				require.NoError(t, h.ChatCompletionOrchestrator.SystemService.SetStoragePolicy(fixture.ctx, &policy))
				engine := gin.New()
				engine.Use(fixture.testContext)
				done := make(chan struct{})
				engine.POST("/v1/responses", func(c *gin.Context) {
					defer close(done)
					if realRoute {
						(&OpenAIHandlers{ResponseCompletionHandlers: h}).CreateResponse(c)
					} else {
						h.ChatCompletion(c)
					}
				})
				server := httptest.NewServer(engine)
				t.Cleanup(server.Close)
				client := server.Client()
				client.Timeout = 5 * time.Second
				resp, err := client.Post(server.URL+"/v1/responses", "application/json", strings.NewReader(`{"model":"gpt-test","input":"hello","stream":true}`))
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, http.StatusOK, resp.StatusCode)
				require.Contains(t, string(body), "event:response.completed")
				require.Contains(t, string(body), `"input_tokens":12`)
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("handler did not finalize")
				}
				req, err := fixture.client.Request.Query().Only(fixture.ctx)
				require.NoError(t, err)
				logs, err := fixture.client.UsageLog.Query().All(fixture.ctx)
				require.NoError(t, err)
				require.Equal(t, request.StatusCompleted, req.Status)
				require.Len(t, logs, 1)
				require.Equal(t, req.ID, logs[0].RequestID)
				require.EqualValues(t, 12, logs[0].PromptTokens)
				require.EqualValues(t, 34, logs[0].CompletionTokens)
				require.EqualValues(t, 46, logs[0].TotalTokens)
			})
		}
	}
}
