package anthropic

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestAnthropicMCPConnectorSameProtocolRoundTrip(t *testing.T) {
	body, err := os.ReadFile("testdata/anthropic-mcp-connector.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	require.NotNil(t, llmReq.ProviderExtensions)
	require.NotNil(t, llmReq.ProviderExtensions.Anthropic)
	require.NotNil(t, llmReq.ProviderExtensions.Anthropic.Request)
	mcpServers := llmReq.ProviderExtensions.Anthropic.Request.MCPServers
	require.NotEmpty(t, mcpServers)
	require.JSONEq(t, `[
		{
			"type": "url",
			"url": "https://example-server.modelcontextprotocol.io/sse",
			"name": "example-mcp",
			"authorization_token": "YOUR_TOKEN",
			"future_nested": {"enabled": true, "note": "unknown-server-field"}
		}
	]`, string(mcpServers))

	rawTools, ok := llmReq.TransformerMetadata[TransformerMetadataKeyRawTools].([]anthropicRawToolFragment)
	require.True(t, ok)
	require.Len(t, rawTools, 1)
	require.Equal(t, 1, rawTools[0].OriginalIndex)
	require.Contains(t, string(rawTools[0].Raw), `"type": "mcp_toolset"`)
	require.Contains(t, string(rawTools[0].Raw), `"future_nested"`)

	// Function tool still converted to common abstraction; mcp_toolset is not.
	require.Len(t, llmReq.Tools, 1)
	require.Equal(t, llm.ToolTypeFunction, llmReq.Tools[0].Type)
	require.Equal(t, "lookup", llmReq.Tools[0].Function.Name)

	// Do not widen llm.Request with connector fields.
	_, hasMCPServersField := any(llmReq).(interface{ GetMCPServers() })
	_ = hasMCPServersField

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var source, outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	require.JSONEq(t, string(source["mcp_servers"]), string(outboundBody["mcp_servers"]))

	var tools []map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["tools"], &tools))
	require.Len(t, tools, 2)

	// Function tool first (common path), mcp_toolset second (raw fragment).
	require.Equal(t, "lookup", tools[0]["name"])
	require.Equal(t, "mcp_toolset", tools[1]["type"])
	require.Equal(t, "example-mcp", tools[1]["mcp_server_name"])
	require.Contains(t, tools[1], "future_nested")
	require.Contains(t, tools[1], "configs")
}

func TestAnthropicMCPToolsetOnlySameProtocolRoundTrip(t *testing.T) {
	body, err := os.ReadFile("testdata/anthropic-mcp-toolset-only.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	require.Empty(t, llmReq.Tools)
	rawTools, ok := llmReq.TransformerMetadata[TransformerMetadataKeyRawTools].([]anthropicRawToolFragment)
	require.True(t, ok)
	require.Len(t, rawTools, 1)
	require.Equal(t, 0, rawTools[0].OriginalIndex)

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	require.Contains(t, outboundBody, "mcp_servers")
	require.Contains(t, outboundBody, "tools")

	var tools []map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["tools"], &tools))
	require.Len(t, tools, 1)
	require.Equal(t, "mcp_toolset", tools[0]["type"])
	require.Equal(t, "example-mcp", tools[0]["mcp_server_name"])
}

func TestAnthropicMCPConnectorNotSynthesizedForResponsesOrChat(t *testing.T) {
	body, err := os.ReadFile("testdata/anthropic-mcp-connector.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	// Responses outbound must not invent Anthropic connector fields or mcp_toolset.
	respOutbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	respReq, err := respOutbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)
	var respBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(respReq.Body, &respBody))
	require.NotContains(t, respBody, "mcp_servers")
	if rawTools, ok := respBody["tools"]; ok {
		require.NotContains(t, string(rawTools), "mcp_toolset")
		require.NotContains(t, string(rawTools), `"type":"mcp"`)
	}

	// Chat outbound must not invent Anthropic connector fields.
	chatOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	chatReq, err := chatOutbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)
	var chatBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chatReq.Body, &chatBody))
	require.NotContains(t, chatBody, "mcp_servers")
	if rawTools, ok := chatBody["tools"]; ok {
		require.NotContains(t, string(rawTools), "mcp_toolset")
	}
}

func TestAnthropicMCPToolsetOrderPreservedWhenFirst(t *testing.T) {
	body := []byte(`{
		"model": "claude-opus-4-8",
		"max_tokens": 512,
		"messages": [{"role": "user", "content": "order"}],
		"mcp_servers": [{"type":"url","url":"https://example-server.modelcontextprotocol.io/sse","name":"example-mcp"}],
		"tools": [
			{"type":"mcp_toolset","mcp_server_name":"example-mcp","default_config":{"enabled":true}},
			{"name":"lookup","description":"lookup","input_schema":{"type":"object","properties":{}}}
		]
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	rawTools, ok := llmReq.TransformerMetadata[TransformerMetadataKeyRawTools].([]anthropicRawToolFragment)
	require.True(t, ok)
	require.Equal(t, 0, rawTools[0].OriginalIndex)
	require.Len(t, llmReq.Tools, 1)
	require.Equal(t, "lookup", llmReq.Tools[0].Function.Name)

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstreamReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamReq.Body, &outboundBody))
	var tools []map[string]any
	require.NoError(t, json.Unmarshal(outboundBody["tools"], &tools))
	require.Len(t, tools, 2)
	require.Equal(t, "mcp_toolset", tools[0]["type"])
	require.Equal(t, "lookup", tools[1]["name"])
}

func TestAnthropicMCPConnectorDiagnosesLossyDowngradeToChatAndResponses(t *testing.T) {
	body, err := os.ReadFile("testdata/anthropic-mcp-connector.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()

	requireHasLossy := func(t *testing.T, req *llm.Request, field string, target llm.APIFormat) {
		t.Helper()
		found := false
		for _, d := range llm.LossyDowngrades(req) {
			if d.SourceProtocol == llm.APIFormatAnthropicMessage &&
				d.SourceField == field &&
				d.TargetProtocol == target &&
				d.Reason == llm.LossyDowngradeReasonNoEquivalentSemantics {
				found = true
				break
			}
		}
		require.Truef(t, found, "missing LossyDowngrade for %s -> %s: %#v", field, target, llm.LossyDowngrades(req))
	}

	// Responses outbound: no fabricated MCP bridge, but explicit diagnostics.
	respLLMReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	respOutbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	respReq, err := respOutbound.TransformRequest(t.Context(), respLLMReq)
	require.NoError(t, err)
	var respBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(respReq.Body, &respBody))
	require.NotContains(t, respBody, "mcp_servers")
	if rawTools, ok := respBody["tools"]; ok {
		require.NotContains(t, string(rawTools), "mcp_toolset")
		require.NotContains(t, string(rawTools), `"type":"mcp"`)
	}
	requireHasLossy(t, respLLMReq, "mcp_servers", llm.APIFormatOpenAIResponse)
	requireHasLossy(t, respLLMReq, "tools[].type=mcp_toolset", llm.APIFormatOpenAIResponse)

	// Chat outbound: same explicit loss, still no fabrication.
	chatLLMReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	chatOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	chatReq, err := chatOutbound.TransformRequest(t.Context(), chatLLMReq)
	require.NoError(t, err)
	var chatBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chatReq.Body, &chatBody))
	require.NotContains(t, chatBody, "mcp_servers")
	if rawTools, ok := chatBody["tools"]; ok {
		require.NotContains(t, string(rawTools), "mcp_toolset")
	}
	requireHasLossy(t, chatLLMReq, "mcp_servers", llm.APIFormatOpenAIChatCompletion)
	requireHasLossy(t, chatLLMReq, "tools[].type=mcp_toolset", llm.APIFormatOpenAIChatCompletion)
}
