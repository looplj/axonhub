package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/server/safeguards"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestPrepareSafeguardsRequest(t *testing.T) {
	original := &httpclient.Request{Headers: http.Header{"Content-Length": {"123"}}, Body: []byte(`{"model":"main","safeguards":[{"type":"dangerous_tool_use"},{"type":"other","setting":1}],"extra":true}`)}
	evaluator, prepared, err := prepareSafeguardsRequest(original, safeguards.Config{}, nil)
	require.NoError(t, err)
	require.Nil(t, evaluator)
	require.Same(t, original, prepared)
	evaluator, prepared, err = prepareSafeguardsRequest(original, safeguards.Config{ClassifierModel: "reviewer"}, nil)
	require.NoError(t, err)
	require.NotNil(t, evaluator)
	require.NotSame(t, original, prepared)
	require.JSONEq(t, `{"model":"main","safeguards":[{"type":"other","setting":1}],"extra":true}`, string(prepared.Body))
	require.Empty(t, prepared.Headers.Get("Content-Length"))
	require.Equal(t, "123", original.Headers.Get("Content-Length"))
	require.Contains(t, string(original.Body), "dangerous_tool_use")
	_, _, err = prepareSafeguardsRequest(&httpclient.Request{Body: []byte(`{"safeguards":"wrong"}`)}, safeguards.Config{ClassifierModel: "reviewer"}, nil)
	require.Error(t, err)
}

func TestSafeguardsHTTP(t *testing.T) {
	anthropicNormal := `{"id":"msg","type":"message","role":"assistant","model":"gpt-test","content":[{"type":"tool_use","id":"tool1","name":"Bash","input":{"command":"pwd"}}],"stop_reason":"tool_use","usage":{"input_tokens":12,"output_tokens":34}}`
	anthropicStream := ""
	for _, event := range []string{
		`{"type":"message_start","message":{"id":"msg","type":"message","role":"assistant","model":"gpt-test","content":[],"usage":{"input_tokens":12,"output_tokens":0}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool1","name":"Bash","input":{}}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"pwd\"}"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":12,"output_tokens":34}}`,
		`{"type":"message_stop"}`,
	} {
		var value struct {
			Type string `json:"type"`
		}
		require.NoError(t, json.Unmarshal([]byte(event), &value))
		anthropicStream += "event: " + value.Type + "\ndata: " + event + "\n\n"
	}
	chatStream := "data: " + `{"id":"chat","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"tool1","type":"function","function":{"name":"Bash","arguments":"{\"command\":\"pwd\"}"}}]}}]}` + "\n\ndata: " + `{"id":"chat","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":12,"completion_tokens":34,"total_tokens":46}}` + "\n\ndata: [DONE]\n\n"
	for _, tc := range []struct {
		name                                   string
		format                                 llm.APIFormat
		payload                                string
		stream, passthrough, timeout, fallback bool
	}{
		{"normal", llm.APIFormatAnthropicMessage, anthropicNormal, false, false, false, false},
		{"passthrough", llm.APIFormatAnthropicMessage, anthropicNormal, false, true, false, false},
		{"stream", llm.APIFormatAnthropicMessage, anthropicStream, true, false, false, false},
		{"stream_passthrough", llm.APIFormatAnthropicMessage, anthropicStream, true, true, false, false},
		{"converted_stream", llm.APIFormatOpenAIChatCompletion, chatStream, true, false, false, false},
		{"timeout", llm.APIFormatAnthropicMessage, anthropicNormal, false, false, true, false},
		{"stream_timeout", llm.APIFormatAnthropicMessage, anthropicStream, true, false, true, false},
		{"fallback_timeout", llm.APIFormatAnthropicMessage, anthropicNormal, false, false, true, true},
		{"stream_fallback_timeout", llm.APIFormatAnthropicMessage, anthropicStream, true, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStreamDeliveryE2EFixture(t, tc.format, tc.payload, false)
			f.passthrough = tc.passthrough
			h := f.handler(t)
			h.ChatCompletionOrchestrator.Inbound = anthropic.NewInboundTransformer()
			h.safeguardsConfig = safeguards.Config{ClassifierModel: "reviewer", Timeout: time.Millisecond * 10}
			if tc.fallback {
				h.safeguardsConfig.FallbackModels = []string{"backup"}
				h.safeguardsConfig.AttemptTimeout = 10 * time.Millisecond
				h.safeguardsConfig.Timeout = time.Second
			}
			calls := 0
			h.reviewer = func(ctx context.Context, req safeguards.ClassifierRequest) (string, error) {
				calls++
				if calls == 1 {
					require.Equal(t, "reviewer", req.Model)
				} else {
					require.Equal(t, "backup", req.Model)
				}
				require.Contains(t, req.User, `"command":"pwd"`)
				if tc.timeout && calls == 1 {
					<-ctx.Done()
					return "", ctx.Err()
				}
				return `{"verdicts":{"tool1":{"flagged":false}}}`, nil
			}
			engine := gin.New()
			engine.Use(f.testContext)
			engine.POST("/v1/messages", h.ChatCompletion)
			body := `{"model":"gpt-test","max_tokens":128,"messages":[{"role":"user","content":"show cwd"}],"safeguards":[{"type":"dangerous_tool_use","classifier_context":{"trusted_directories":["/tmp"]}}],"stream":false}`
			if tc.stream {
				body = strings.Replace(body, `"stream":false`, `"stream":true`, 1)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)).WithContext(f.ctx)
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			require.Equal(t, 200, recorder.Code, recorder.Body.String())
			expectedCalls := 1
			if tc.fallback {
				expectedCalls = 2
			}
			require.Equal(t, expectedCalls, calls, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), `"safeguard_results"`)
			if tc.timeout && !tc.fallback {
				require.Contains(t, recorder.Body.String(), `"reason":"timeout"`)
			} else {
				require.Contains(t, recorder.Body.String(), `"outcome":"not_flagged"`)
			}
			if tc.stream {
				require.Less(t, strings.Index(recorder.Body.String(), `"safeguard_results"`), strings.Index(recorder.Body.String(), "event:message_stop"))
			}
			f.assertDatabase(t, request.StatusCompleted, true)
		})
	}
}

func TestSafeguardsTerminalTailRejectsIncompleteStream(t *testing.T) {
	for _, tail := range [][]*httpclient.StreamEvent{
		nil,
		{{Data: []byte(`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"late","name":"Bash","input":{}}}`)}},
	} {
		calls := 0
		evaluator, err := safeguards.New([]byte(`{"safeguards":[{"type":"dangerous_tool_use"}]}`), safeguards.Config{ClassifierModel: "reviewer"}, func(context.Context, safeguards.ClassifierRequest) (string, error) {
			calls++
			return `{"verdicts":{"tool1":{"flagged":false}}}`, nil
		})
		require.NoError(t, err)
		events := []*httpclient.StreamEvent{
			{Data: []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool1","name":"Bash","input":{}}}`)},
			{Data: []byte(`{"type":"content_block_stop","index":0}`)},
			{Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`)},
		}
		source := streams.SliceStream(append(events, tail...))
		reviewed := wrapSafeguardsStream(t.Context(), source, evaluator, nil)
		for reviewed.Next() {
			require.NotContains(t, string(reviewed.Current().Data), "not_flagged")
		}
		require.Error(t, reviewed.Err())
		require.Zero(t, calls)
	}
}

// The same handler must resolve settings anew for each request without sharing
// mutable configuration with in-flight reviews.
func TestClaudeCodeSafeguardsRuntimeSettings(t *testing.T) {
	payload := `{"id":"msg","type":"message","role":"assistant","model":"gpt-test","content":[{"type":"tool_use","id":"tool1","name":"Bash","input":{"command":"pwd"}}],"stop_reason":"tool_use","usage":{"input_tokens":12,"output_tokens":34}}`
	f := newStreamDeliveryE2EFixture(t, llm.APIFormatAnthropicMessage, payload, false)
	h := f.handler(t)
	h.ChatCompletionOrchestrator.Inbound = anthropic.NewInboundTransformer()
	config := safeguards.Config{ClassifierModel: "first", Policy: "First policy", Timeout: time.Second}
	var loadErr error
	lookups, reviews := 0, 0
	h.safeguardsConfigProvider = func(context.Context) (safeguards.Config, error) {
		lookups++
		return config, loadErr
	}
	h.reviewer = func(_ context.Context, req safeguards.ClassifierRequest) (string, error) {
		reviews++
		require.Equal(t, config.ClassifierModel, req.Model)
		require.Contains(t, req.System, config.Policy)
		return `{"verdicts":{"tool1":{"flagged":false}}}`, nil
	}
	engine := gin.New()
	engine.Use(f.testContext)
	engine.POST("/v1/messages", h.ChatCompletion)
	send := func(withSafeguard bool) *httptest.ResponseRecorder {
		body := `{"model":"gpt-test","max_tokens":128,"messages":[{"role":"user","content":"show cwd"}]}`
		if withSafeguard {
			body = strings.TrimSuffix(body, "}") + `,"safeguards":[{"type":"dangerous_tool_use"}]}`
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)).WithContext(f.ctx)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		return recorder
	}
	require.Equal(t, 200, send(true).Code)
	config.ClassifierModel = "second"
	config.Policy = "Second policy"
	require.Equal(t, 200, send(true).Code)
	require.Equal(t, 2, reviews)
	config.ClassifierModel = ""
	require.Equal(t, 200, send(true).Code)
	require.Equal(t, 2, reviews)
	loadErr = errors.New("database unavailable")
	require.Equal(t, 500, send(true).Code)
	require.Equal(t, 2, reviews)
	require.Equal(t, 4, lookups)
	require.Equal(t, 200, send(false).Code)
	require.Equal(t, 4, lookups, "ordinary requests must not read safeguards settings")
}
