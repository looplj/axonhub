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

func TestSafeguardsSharedChannelSingleConcurrency(t *testing.T) {
	var reviewCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !body.Stream {
			reviewCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"review","object":"chat.completion","model":"gpt-test","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"verdicts\":{\"tool1\":{\"flagged\":false}}}"}}],"usage":{"prompt_tokens":12,"completion_tokens":34,"total_tokens":46}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: "+`{"id":"chat","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"tool1","type":"function","function":{"name":"Bash","arguments":"{\"command\":\"pwd\"}"}}]}}]}`+"\n\ndata: "+`{"id":"chat","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":12,"completion_tokens":34,"total_tokens":46}}`+"\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	f := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion, "", false)
	var err error
	f.channel, err = f.channel.Update().SetBaseURL(upstream.URL).SetEndpoints([]objects.ChannelEndpoint{{APIFormat: string(llm.APIFormatOpenAIChatCompletion), BaseURL: upstream.URL, Transport: objects.ChannelEndpointTransportHTTP}}).SetSettings(&objects.ChannelSettings{RateLimit: &objects.ChannelRateLimit{MaxConcurrent: lo.ToPtr(int64(1)), QueueSize: lo.ToPtr(int64(1))}}).Save(f.ctx)
	require.NoError(t, err)
	h := f.handler(t)
	h.ChatCompletionOrchestrator.Inbound = anthropic.NewInboundTransformer()
	h.safeguardsConfig = safeguards.Config{ClassifierModel: "gpt-test", Timeout: 500 * time.Millisecond}
	h.reviewer = newSafeguardsClassifier(h.ChatCompletionOrchestrator)
	engine := gin.New()
	engine.Use(f.testContext)
	engine.POST("/v1/messages", h.ChatCompletion)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-test","max_tokens":128,"messages":[{"role":"user","content":"show cwd"}],"safeguards":[{"type":"dangerous_tool_use"}],"stream":true}`)).WithContext(f.ctx)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	require.Equal(t, 200, recorder.Code)
	require.Equal(t, int32(1), reviewCalls.Load(), recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"outcome":"not_flagged"`)
	rows, err := f.client.Request.Query().All(f.ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, request.StatusCompleted, row.Status)
	}
	count, err := f.client.UsageLog.Query().Count(f.ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
