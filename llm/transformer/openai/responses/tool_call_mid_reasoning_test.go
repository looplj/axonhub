package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	chatopenai "github.com/looplj/axonhub/llm/transformer/openai"
)

// Public seam: Responses inbound history with
// function_call -> reasoning -> function_call_output must convert to Chat as
// assistant(tool_calls[+reasoning]) immediately followed by role=tool.
// Strict Chat providers reject an intervening assistant reasoning message with
// "No tool output found for function call ...".
func TestCrossProtocol_ResponsesMidReasoningKeepsChatToolCallAdjacentToToolResult(t *testing.T) {
	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gpt-5.6-sol",
			"input": [
				{"type":"message","role":"user","content":"run"},
				{
					"type":"function_call",
					"id":"fc_1",
					"call_id":"call_1dfb1152-5e5c-4aa1-ad30-8aeb7d62d670",
					"name":"exec_command",
					"arguments":"{\"cmd\":\"echo hi\"}"
				},
				{
					"type":"reasoning",
					"id":"rs_mid",
					"summary":[{"type":"summary_text","text":"...\n"}]
				},
				{
					"type":"function_call_output",
					"call_id":"call_1dfb1152-5e5c-4aa1-ad30-8aeb7d62d670",
					"output":"tool result"
				}
			]
		}`),
	})
	require.NoError(t, err)

	chatOutbound, err := chatopenai.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	out, err := chatOutbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Body, &body))
	var messages []map[string]any
	require.NoError(t, json.Unmarshal(body["messages"], &messages))
	require.GreaterOrEqual(t, len(messages), 3)

	callIdx := -1
	for i, m := range messages {
		if m["role"] != "assistant" {
			continue
		}
		rawCalls, ok := m["tool_calls"]
		if !ok || rawCalls == nil {
			continue
		}
		b, err := json.Marshal(rawCalls)
		require.NoError(t, err)
		if strings.Contains(string(b), "call_1dfb1152-5e5c-4aa1-ad30-8aeb7d62d670") {
			callIdx = i
			// reasoning may be attached on same assistant
			if rc, ok := m["reasoning_content"].(string); ok {
				require.Contains(t, rc, "...")
			}
			break
		}
	}
	require.NotEqual(t, -1, callIdx, "assistant tool_calls for target call not found: %#v", messages)
	require.Less(t, callIdx+1, len(messages))
	next := messages[callIdx+1]
	require.Equal(t, "tool", next["role"], "expected tool result immediately after tool_calls, got intervening %#v", next)
	require.Equal(t, "call_1dfb1152-5e5c-4aa1-ad30-8aeb7d62d670", next["tool_call_id"])
}
