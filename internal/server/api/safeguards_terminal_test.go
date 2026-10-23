package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/safeguards"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

// TestSafeguardsCompletedUpstreamStaysOpen verifies semantic completion does not
// cancel downstream delivery when an Anthropic provider delays transport EOF.
func TestSafeguardsCompletedUpstreamStaysOpen(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "converted"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			var reviewCalls atomic.Int32
			upstreamClosed := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Stream bool `json:"stream"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if !body.Stream {
					reviewCalls.Add(1)
					_, _ = io.WriteString(w, `{"id":"review","type":"message","role":"assistant","model":"gpt-test","content":[{"type":"text","text":"{\"verdicts\":{\"tool1\":{\"flagged\":false}}}"}],"stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":34}}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				for _, event := range []string{
					`{"type":"message_start","message":{"id":"msg","type":"message","role":"assistant","model":"gpt-test","content":[],"usage":{"input_tokens":12,"output_tokens":0}}}`,
					`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool1","name":"Bash","input":{}}}`,
					`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"pwd\"}"}}`,
					`{"type":"content_block_stop","index":0}`,
					`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":12,"output_tokens":34}}`,
					`{"type":"message_stop"}`,
				} {
					var envelope struct {
						Type string `json:"type"`
					}
					_ = json.Unmarshal([]byte(event), &envelope)
					_, _ = io.WriteString(w, "event: "+envelope.Type+"\ndata: "+event+"\n\n")
				}
				w.(http.Flusher).Flush()
				// Do not send HTTP EOF. The proxy must close only the completed upstream,
				// keeping its downstream lifecycle alive until reviewed delivery succeeds.
				select {
				case <-r.Context().Done():
				case <-time.After(5 * time.Second):
				}
				close(upstreamClosed)
			}))
			defer upstream.Close()
			f := newStreamDeliveryE2EFixture(t, llm.APIFormatAnthropicMessage, "", false)
			f.passthrough = passthrough
			var err error
			f.channel, err = f.channel.Update().SetBaseURL(upstream.URL).SetEndpoints([]objects.ChannelEndpoint{{APIFormat: string(llm.APIFormatAnthropicMessage), BaseURL: upstream.URL, Transport: objects.ChannelEndpointTransportHTTP}}).SetSettings(&objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{MaxConcurrent: lo.ToPtr(int64(1)), QueueSize: lo.ToPtr(int64(1))}}).Save(f.ctx)
			require.NoError(t, err)
			h := f.handler(t)
			h.ChatCompletionOrchestrator.Inbound = anthropic.NewInboundTransformer()
			h.safeguardsConfig = safeguards.Config{ClassifierModel: "gpt-test", Timeout: 500 * time.Millisecond}
			h.reviewer = newSafeguardsClassifier(h.ChatCompletionOrchestrator)
			router := gin.New()
			router.Use(f.testContext)
			router.POST("/v1/messages", h.ChatCompletion)
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-test","max_tokens":128,"messages":[{"role":"user","content":"show cwd"}],"safeguards":[{"type":"dangerous_tool_use"}],"stream":true}`)).WithContext(f.ctx)
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			require.Equal(t, 200, response.Code)
			require.Equal(t, int32(1), reviewCalls.Load(), response.Body.String())
			require.Contains(t, response.Body.String(), `"outcome":"not_flagged"`)
			require.Contains(t, response.Body.String(), "event:message_stop")
			require.NotContains(t, response.Body.String(), "event:error")
			select {
			case <-upstreamClosed:
			case <-time.After(time.Second):
				t.Fatal("completed upstream was not closed")
			}
			rows, err := f.client.Request.Query().All(f.ctx)
			require.NoError(t, err)
			require.Len(t, rows, 2)
			for _, row := range rows {
				require.Equal(t, request.StatusCompleted, row.Status)
			}
			count, err := f.client.UsageLog.Query().Count(f.ctx)
			require.NoError(t, err)
			require.Equal(t, 2, count)
		})
	}
}
