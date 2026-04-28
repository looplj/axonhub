package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestCodexMCPRequestRoundTrip_NonPassThroughPreservesResponsesExtensions(t *testing.T) {
	rawRequest := []byte(`{
		"model": "gpt-5.1-codex-mini",
		"instructions": "You are Codex.",
		"input": [
			{
				"type": "message",
				"role": "user",
				"content": [{"type": "input_text", "text": "Use MCP"}]
			},
			{
				"type": "function_call",
				"call_id": "call_mcp_1",
				"name": "read_file",
				"namespace": "filesystem",
				"arguments": "{\"path\":\"README.md\"}"
			},
			{
				"type": "mcp_tool_call_output",
				"call_id": "call_mcp_1",
				"result": {
					"content": [{"type": "text", "text": "ok"}],
					"isError": false
				}
			}
		],
		"tools": [
			{
				"type": "namespace",
				"name": "filesystem",
				"description": "File tools",
				"tools": [
					{
						"type": "function",
						"name": "read_file",
						"description": "Read file",
						"parameters": {"type": "object"},
						"defer_loading": true
					}
				]
			},
			{
				"type": "tool_search",
				"execution": "client",
				"description": "Search tools",
				"parameters": {"type": "object"}
			},
			{"type": "local_shell"}
		],
		"client_metadata": {"x-codex-installation-id": "install_123"},
		"stream": true,
		"store": false
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: rawRequest})
	require.NoError(t, err)
	require.NotNil(t, llmReq.ProtocolExtensions)
	require.NotNil(t, llmReq.ProtocolExtensions.OpenAIResponses)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(httpReq.Body, &body))

	require.Equal(t, map[string]any{"x-codex-installation-id": "install_123"}, body["client_metadata"])

	tools := body["tools"].([]any)
	require.Len(t, tools, 3)
	require.Equal(t, "namespace", tools[0].(map[string]any)["type"])
	namespaceTools := tools[0].(map[string]any)["tools"].([]any)
	require.Equal(t, true, namespaceTools[0].(map[string]any)["defer_loading"])
	require.Equal(t, "tool_search", tools[1].(map[string]any)["type"])
	require.Equal(t, "local_shell", tools[2].(map[string]any)["type"])

	input := body["input"].([]any)
	require.Len(t, input, 3)
	functionCall := input[1].(map[string]any)
	require.Equal(t, "function_call", functionCall["type"])
	require.Equal(t, "filesystem", functionCall["namespace"])

	mcpOutput := input[2].(map[string]any)
	require.Equal(t, "mcp_tool_call_output", mcpOutput["type"])
	require.Contains(t, mcpOutput, "result")
}
