package openai

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// C1/C2 public Chat same-protocol fixtures. These intentionally exercise HTTP
// transformers rather than internal converter helpers: the client-visible
// OpenAI Chat wire form must survive Chat -> canonical -> Chat.
func TestOpenAIChatRequest_CustomToolsChoiceAndContentPartsRoundTrip(t *testing.T) {
	source := []byte(`{
		"model": "gpt-5",
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "Read this file"},
					{"type": "file", "file": {"file_id": "file_123", "filename": "notes.txt"}}
				]
			},
			{
				"role": "assistant",
				"content": [{"type": "refusal", "refusal": "I cannot process that document."}]
			}
		],
		"tools": [{
			"type": "custom",
			"custom": {
				"name": "run_sql",
				"description": "Execute a constrained query",
				"format": {
					"type": "grammar",
					"grammar": {"syntax": "lark", "definition": "start: SELECT"}
				}
			}
		}],
		"tool_choice": {"type": "custom", "custom": {"name": "run_sql"}}
	}`)

	outboundBody := roundTripOpenAIChatRequest(t, source)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(outboundBody, &got))
	require.JSONEq(t, `[{"type":"custom","custom":{"name":"run_sql","description":"Execute a constrained query","format":{"type":"grammar","grammar":{"syntax":"lark","definition":"start: SELECT"}}}}]`, string(got["tools"]))
	require.JSONEq(t, `{"type":"custom","custom":{"name":"run_sql"}}`, string(got["tool_choice"]))

	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got["messages"], &messages))
	require.JSONEq(t, `[{"type":"text","text":"Read this file"},{"type":"file","file":{"file_id":"file_123","filename":"notes.txt"}}]`, string(messages[0]["content"]))
	require.JSONEq(t, `[{"type":"refusal","refusal":"I cannot process that document."}]`, string(messages[1]["content"]))
}

func TestOpenAIChatRequest_AllowedToolsChoiceRoundTrip(t *testing.T) {
	source := []byte(`{
		"model":"gpt-5",
		"messages":[{"role":"user","content":"choose"}],
		"tool_choice":{"type":"allowed_tools","allowed_tools":{"mode":"required","tools":[{"type":"function","function":{"name":"get_weather"}},{"type":"custom","custom":{"name":"run_sql"}}]}}
	}`)

	outboundBody := roundTripOpenAIChatRequest(t, source)
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(outboundBody, &got))
	require.JSONEq(t, `{"type":"allowed_tools","allowed_tools":{"mode":"required","tools":[{"type":"function","function":{"name":"get_weather"}},{"type":"custom","custom":{"name":"run_sql"}}]}}`, string(got["tool_choice"]))
}

func TestOpenAIChatResponse_CustomToolCallRoundTrip(t *testing.T) {
	upstreamBody := []byte(`{
		"id": "chatcmpl-custom",
		"object": "chat.completion",
		"created": 1,
		"model": "gpt-5",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [{
					"id": "call_custom_1",
					"type": "custom",
					"custom": {"name": "run_sql", "input": "SELECT 1"}
				}]
			},
			"finish_reason": "tool_calls"
		}]
	}`)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{Body: upstreamBody})
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	clientResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(clientResp.Body, &got))
	var choices []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got["choices"], &choices))
	var message map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(choices[0]["message"], &message))
	require.JSONEq(t, `[{"id":"call_custom_1","type":"custom","custom":{"name":"run_sql","input":"SELECT 1"}}]`, string(message["tool_calls"]))
}

func TestOpenAIChatStream_CustomToolCallRoundTrip(t *testing.T) {
	upstreamChunk := []byte(`{
		"id": "chatcmpl-custom-stream",
		"object": "chat.completion.chunk",
		"created": 1,
		"model": "gpt-5",
		"choices": [{
			"index": 0,
			"delta": {
				"role": "assistant",
				"tool_calls": [{
					"index": 0,
					"id": "call_custom_stream",
					"type": "custom",
					"custom": {"name": "run_sql", "input": "SELECT "}
				}]
			},
			"finish_reason": null
		}]
	}`)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{Body: upstreamChunk})
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	clientEvent, err := inbound.TransformStreamChunk(t.Context(), llmResp)
	require.NoError(t, err)
	require.NotNil(t, clientEvent)

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(clientEvent.Data, &got))
	var choices []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got["choices"], &choices))
	var delta map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(choices[0]["delta"], &delta))
	require.JSONEq(t, `[{"index":0,"id":"call_custom_stream","type":"custom","custom":{"name":"run_sql","input":"SELECT "}}]`, string(delta["tool_calls"]))
}

func roundTripOpenAIChatRequest(t *testing.T, source []byte) []byte {
	t.Helper()

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    source,
	})
	require.NoError(t, err)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)
	return httpReq.Body
}
