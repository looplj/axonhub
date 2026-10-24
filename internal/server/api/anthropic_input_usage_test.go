package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestEstimateAnthropicInput(t *testing.T) {
	baseline := `{"system":"Policy","messages":[{"role":"user","content":[{"type":"text","text":"Hello"}]}]}`
	count, err := estimateAnthropicInput([]byte(baseline))
	require.NoError(t, err)
	require.Positive(t, count)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(baseline), &body))
	body["metadata"] = map[string]any{"user_id": strings.Repeat("secret", 1000)}
	body["max_tokens"] = 100000
	messages := body["messages"].([]any)
	m := messages[0].(map[string]any)
	m["content"] = append(m["content"].([]any), map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": strings.Repeat("aaaa", 10000)}})
	encoded, _ := json.Marshal(body)
	same, err := estimateAnthropicInput(encoded)
	require.NoError(t, err)
	require.Equal(t, count, same)
	body["tools"] = []any{map[string]any{"name": "Read", "description": strings.Repeat("Read a file. ", 20), "input_schema": map[string]any{"type": "object"}}}
	encoded, _ = json.Marshal(body)
	larger, err := estimateAnthropicInput(encoded)
	require.NoError(t, err)
	require.Greater(t, larger, count)
	nested, err := estimateAnthropicInput([]byte(`{"messages":[{"role":"user","content":[{"type":"tool_result","content":[{"type":"text","text":"a detailed tool result with several words"}]}]}]}`))
	require.NoError(t, err)
	require.Greater(t, nested, count)
	_, err = estimateAnthropicInput([]byte(`{`))
	require.Error(t, err)
}

func TestAnthropicInputUsageStream(t *testing.T) {
	t.Parallel()

	for _, initial := range []string{`{"input_tokens":0,"output_tokens":0}`, `{"input_tokens":42,"output_tokens":0}`, `{"input_tokens":0,"cache_read_input_tokens":100,"output_tokens":0}`, `{"cache_creation_input_tokens":100}`} {
		t.Run(initial, func(t *testing.T) {
			t.Parallel()
			start := &httpclient.StreamEvent{Type: "message_start", Data: []byte(`{"type":"message_start","message":{"usage":` + initial + `}}`)}
			original := string(start.Data)
			final := &httpclient.StreamEvent{Type: "message_delta", Data: []byte(`{"type":"message_delta","usage":{"input_tokens":12,"output_tokens":34}}`)}
			source := streams.SliceStream([]*httpclient.StreamEvent{start, final})
			wrapped := wrapAnthropicInputUsage(source, &httpclient.Request{Path: "/v1/messages", Body: []byte(`{"messages":[{"role":"user","content":"Hello test request"}]}`)}, nil)
			require.True(t, wrapped.Next())
			first := wrapped.Current()
			require.Same(t, first, wrapped.Current())
			if initial == `{"input_tokens":0,"output_tokens":0}` {
				require.Positive(t, gjson.GetBytes(first.Data, "message.usage.input_tokens").Int())
			} else {
				require.Equal(t, original, string(first.Data))
			}
			require.Equal(t, original, string(start.Data), "must not mutate persistence events")
			require.True(t, wrapped.Next())
			require.Same(t, final, wrapped.Current())
			require.False(t, wrapped.Next())
			require.NoError(t, wrapped.Err())
			require.NoError(t, wrapped.Close())
		})
	}
}

func TestAnthropicEstimatedUsageDoesNotChangeAccounting(t *testing.T) {
	f := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion, streamDeliveryChat(12, 34, true), false)
	h := f.handler(t)
	h.ChatCompletionOrchestrator.Inbound = anthropic.NewInboundTransformer()
	engine := gin.New()
	engine.Use(f.testContext)
	engine.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithAPIKey(c.Request.Context(), &ent.APIKey{ID: 41}))
		c.Next()
	})
	engine.POST("/v1/messages", h.ChatCompletion)
	body := `{"model":"gpt-test","max_tokens":128,"stream":true,"messages":[{"role":"user","content":"` + strings.Repeat("token estimate fixture ", 100) + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)).WithContext(f.ctx)
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	engine.ServeHTTP(out, req)
	require.Equal(t, 200, out.Code)
	var initial, final int64
	for line := range strings.SplitSeq(out.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		e := gjson.Parse(strings.TrimPrefix(line, "data:"))
		if e.Get("type").String() == "message_start" {
			initial = e.Get("message.usage.input_tokens").Int()
		}
		if e.Get("type").String() == "message_delta" {
			final = e.Get("usage.input_tokens").Int()
		}
	}
	require.Greater(t, initial, int64(12))
	require.Equal(t, int64(12), final)
	f.assertDatabase(t, request.StatusCompleted, true)
	usage, err := f.client.UsageLog.Query().Only(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(12), usage.PromptTokens)
	require.Equal(t, int64(34), usage.CompletionTokens)
	require.Len(t, h.inputUsageCache.samples, 1, "committed route and authenticated scope must reach calibration")
	require.Equal(t, int64(12), h.inputUsageCache.samples[0].actual)
}

func TestAnthropicInputUsageSkipsOtherProtocols(t *testing.T) {
	source := streams.SliceStream([]*httpclient.StreamEvent{})
	require.Same(t, source, wrapAnthropicInputUsage(source, &httpclient.Request{Path: "/v1/responses"}, nil))
}

func TestEstimateAnthropicInputConcurrent(t *testing.T) {
	t.Parallel()
	body, err := json.Marshal(map[string]any{"messages": []map[string]string{{"role": "user", "content": strings.Repeat("Concurrent counting: Debian 日本語 1234567. ", 200)}}})
	require.NoError(t, err)
	want, err := estimateAnthropicInput(body)
	require.NoError(t, err)
	require.Positive(t, want)
	for range 16 {
		t.Run("count", func(t *testing.T) {
			t.Parallel()
			for range 5 {
				got, err := estimateAnthropicInput(body)
				require.NoError(t, err)
				require.Equal(t, want, got)
			}
		})
	}
}
