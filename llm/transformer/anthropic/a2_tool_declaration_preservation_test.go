package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// A2: same-protocol public seam must preserve Anthropic-only function-tool children
// and non-web-search / non-mcp native tool declarations, without widening llm.Tool.
func TestA2_FunctionToolAnthropicOnlyChildren_SameProtocolRoundTrip(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 64,
		"messages": [{"role": "user", "content": "tools"}],
		"tools": [
			{
				"name": "lookup",
				"description": "Lookup things",
				"input_schema": {"type": "object", "properties": {"q": {"type": "string"}}, "required": ["q"]},
				"strict": true,
				"allowed_callers": ["direct", "code_execution_20250825"],
				"defer_loading": true,
				"input_examples": [{"q": "weather"}],
				"future_tool_field": {"keep": true}
			},
			{
				"name": "plain",
				"description": "plain",
				"input_schema": {"type": "object", "properties": {}}
			}
		]
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	// Common function tools still convert; Anthropic-only children must not require llm.Tool expansion.
	require.Len(t, llmReq.Tools, 2)
	require.Equal(t, llm.ToolTypeFunction, llmReq.Tools[0].Type)
	require.Equal(t, "lookup", llmReq.Tools[0].Function.Name)
	require.Equal(t, "plain", llmReq.Tools[1].Function.Name)

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstream, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var source, out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(upstream.Body, &out))

	var sourceTools, outTools []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["tools"], &sourceTools))
	require.NoError(t, json.Unmarshal(out["tools"], &outTools))
	require.Len(t, outTools, 2)

	require.JSONEq(t, string(sourceTools[0]["name"]), string(outTools[0]["name"]))
	require.JSONEq(t, string(sourceTools[0]["strict"]), string(outTools[0]["strict"]))
	require.JSONEq(t, string(sourceTools[0]["allowed_callers"]), string(outTools[0]["allowed_callers"]))
	require.JSONEq(t, string(sourceTools[0]["defer_loading"]), string(outTools[0]["defer_loading"]))
	require.JSONEq(t, string(sourceTools[0]["input_examples"]), string(outTools[0]["input_examples"]))
	require.JSONEq(t, string(sourceTools[0]["future_tool_field"]), string(outTools[0]["future_tool_field"]))
	require.JSONEq(t, string(sourceTools[0]["input_schema"]), string(outTools[0]["input_schema"]))

	require.JSONEq(t, string(sourceTools[1]["name"]), string(outTools[1]["name"]))
	require.JSONEq(t, string(sourceTools[1]["input_schema"]), string(outTools[1]["input_schema"]))
}

func TestA2_NonWebSearchNativeToolDeclaration_SameProtocolRoundTrip(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 64,
		"messages": [{"role": "user", "content": "native tools"}],
		"tools": [
			{
				"type": "code_execution_20250825",
				"name": "code_execution",
				"future_native": {"enabled": true}
			},
			{
				"name": "lookup",
				"description": "Lookup",
				"input_schema": {"type": "object", "properties": {}}
			},
			{
				"type": "web_search_20250305",
				"name": "web_search",
				"max_uses": 3
			}
		]
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	// Function + web_search convert; code_execution is Anthropic-native raw only.
	require.Len(t, llmReq.Tools, 2)
	require.Equal(t, "lookup", llmReq.Tools[0].Function.Name)
	require.Equal(t, llm.ToolTypeWebSearch, llmReq.Tools[1].Type)

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstream, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstream.Body, &out))
	var outTools []map[string]any
	require.NoError(t, json.Unmarshal(out["tools"], &outTools))
	require.Len(t, outTools, 3, "native non-web-search tool must keep order with function/web_search")

	require.Equal(t, "code_execution_20250825", outTools[0]["type"])
	require.Equal(t, "code_execution", outTools[0]["name"])
	require.Equal(t, map[string]any{"enabled": true}, outTools[0]["future_native"])

	require.Equal(t, "lookup", outTools[1]["name"])
	require.Equal(t, "web_search_20250305", outTools[2]["type"])
	require.Equal(t, float64(3), outTools[2]["max_uses"])
}
