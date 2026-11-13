package responses

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestResponsesToolOutputConversionPreservesItemIdentity(t *testing.T) {
	for _, kind := range []string{"function_call_output", "custom_tool_call_output"} {
		t.Run(kind, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{
				"model": "synthetic-model", "input": []map[string]any{{
					"type": strings.TrimSuffix(kind, "_output"), "id": "call-item", "call_id": "paired-call",
					"name": "payload", "arguments": "{}", "input": "synthetic-input",
				}, {
					"type": kind, "id": "output-item", "call_id": "paired-call",
					"output": "synthetic-output",
				}},
			})
			require.NoError(t, err)
			request, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: body})
			require.NoError(t, err)
			items := convertInputFromMessages(request.Messages, request.TransformOptions)
			encoded, err := json.Marshal(items)
			require.NoError(t, err)
			require.Equal(t, "output-item", gjson.GetBytes(encoded, "1.id").String())
			require.Equal(t, kind, gjson.GetBytes(encoded, "1.type").String())
			require.Equal(t, "paired-call", gjson.GetBytes(encoded, "1.call_id").String())
			require.Equal(t, "synthetic-output", gjson.GetBytes(encoded, "1.output").String())
		})
	}
}

func TestResponsesToolOutputWithoutCallIDPreservesName(t *testing.T) {
	body := []byte(`{"model":"synthetic-model","input":[{"type":"function_call_output","name":"codex_apps_open_page","output":"{\"page_id\":null}"}]}`)
	request, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: body})
	require.NoError(t, err)
	encoded, err := json.Marshal(convertInputFromMessages(request.Messages, request.TransformOptions))
	require.NoError(t, err)
	require.Equal(t, "codex_apps_open_page", gjson.GetBytes(encoded, "0.name").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(encoded, "0.type").String())
	require.False(t, gjson.GetBytes(encoded, "0.call_id").Exists())
	require.Equal(t, "{\"page_id\":null}", gjson.GetBytes(encoded, "0.output").String())
}

func TestResponsesToolOutputWithoutCallIDBecomesChatUserMessage(t *testing.T) {
	body := []byte(`{"model":"synthetic-model","input":[
		{"type":"function_call_output","name":"automation_update","output":"schedule"},
		{"type":"function_call","call_id":"call_1","name":"exec","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_1","output":"ok"}
	]}`)
	request, err := NewWebSocketInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: body})
	require.NoError(t, err)

	chatRequest := openai.RequestFromLLM(t.Context(), request, openai.ReasoningFieldNone)
	require.Len(t, chatRequest.Messages, 3)
	require.Equal(t, "user", chatRequest.Messages[0].Role)
	require.Nil(t, chatRequest.Messages[0].ToolCallID)
	require.Equal(t, "schedule", lo.FromPtr(chatRequest.Messages[0].Content.Content))
	require.Equal(t, "assistant", chatRequest.Messages[1].Role)
	require.Equal(t, "tool", chatRequest.Messages[2].Role)
	require.Equal(t, "call_1", lo.FromPtr(chatRequest.Messages[2].ToolCallID))
	require.Equal(t, "ok", lo.FromPtr(chatRequest.Messages[2].Content.Content))
}
