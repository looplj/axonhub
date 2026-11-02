package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestAllowedCallersInbound(t *testing.T) {
	// Given an Anthropic request whose function tool declares allowed_callers.
	httpReq := &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "claude-sonnet-4-20250514",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": "hi"}],
			"tools": [{
				"name": "get_weather",
				"description": "Get the weather",
				"input_schema": {"type": "object", "properties": {}},
				"allowed_callers": ["direct"]
			}]
		}`),
	}

	// When the request crosses the inbound transformer.
	chatReq, err := NewInboundTransformer().TransformRequest(t.Context(), httpReq)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)

	// Then the unified function keeps the caller kinds.
	require.Equal(t, []string{"direct"}, chatReq.Tools[0].Function.AllowedCallers)
}

func TestAllowedCallersOutbound(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	// Given a unified request whose function tool declares allowed_callers.
	chatReq := &llm.Request{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: lo.ToPtr(int64(1024)),
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
		},
		Tools: []llm.Tool{
			{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name:           "get_weather",
					Description:    "Get the weather",
					Parameters:     json.RawMessage(`{"type":"object","properties":{}}`),
					AllowedCallers: []string{"direct"},
				},
			},
		},
	}

	// When the request is forwarded to Anthropic.
	result, err := transformer.TransformRequest(t.Context(), chatReq)
	require.NoError(t, err)

	// Then the wire JSON carries allowed_callers compactly.
	require.Contains(t, string(result.Body), `"allowed_callers":["direct"]`)

	var anthropicReq MessageRequest

	err = json.Unmarshal(result.Body, &anthropicReq)
	require.NoError(t, err)
	require.Len(t, anthropicReq.Tools, 1)
	require.Equal(t, []string{"direct"}, anthropicReq.Tools[0].AllowedCallers)
}

func TestAllowedCallersAbsentStaysAbsent(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		value []string
	}{
		{name: "nil slice", value: nil},
		{name: "empty slice", value: []string{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// The wire tool does not emit allowed_callers as null or [].
			body, err := json.Marshal(Tool{
				Name:           "get_weather",
				InputSchema:    json.RawMessage(`{"type":"object","properties":{}}`),
				AllowedCallers: tt.value,
			})
			require.NoError(t, err)
			require.NotContains(t, string(body), "allowed_callers")

			// The unified function omits it too.
			fnBody, err := json.Marshal(llm.Function{Name: "get_weather", AllowedCallers: tt.value})
			require.NoError(t, err)
			require.NotContains(t, string(fnBody), "allowed_callers")

			// A full outbound pass keeps it absent from the forwarded JSON.
			result, err := outbound.TransformRequest(t.Context(), &llm.Request{
				Model:     "claude-sonnet-4-20250514",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
				},
				Tools: []llm.Tool{
					{
						Type: llm.ToolTypeFunction,
						Function: llm.Function{
							Name:           "get_weather",
							Parameters:     json.RawMessage(`{"type":"object","properties":{}}`),
							AllowedCallers: tt.value,
						},
					},
				},
			})
			require.NoError(t, err)
			require.NotContains(t, string(result.Body), "allowed_callers")

			// An absent inbound field stays nil rather than becoming []string{}.
			unified, ok := convertToolToLLM(Tool{Name: "get_weather"})
			require.True(t, ok)
			require.Nil(t, unified.Function.AllowedCallers)
		})
	}
}
