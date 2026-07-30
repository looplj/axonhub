package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	responsesapi "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestResponsesToChatTools_NonStreamingRoundTrip(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"additional_tools","role":"developer","tools":[
				{"type":"namespace","name":"collaboration","tools":[
					{"type":"function","name":"spawn_agent","parameters":{"type":"object","properties":{}}}
				]}
			]},
			{"role":"user","type":"message","content":[{"type":"input_text","text":"continue"}]}
		],
		"tools":[
			{"type":"custom","name":"apply_patch","description":"Apply patch"},
			{"type":"tool_search","execution":"client","description":"Find tools","parameters":{"type":"object","properties":{"query":{"type":"string"}}}},
			{"type":"future_client_tool","name":"future_lookup","execution":"client","parameters":{"type":"object","properties":{"id":{"type":"string"}}}}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Tools, 4)
	for _, tool := range converted.Tools {
		require.Equal(t, "function", tool.Type)
	}

	chatResponse := &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    chatRequest,
		Body: []byte(`{
			"id":"chatcmpl_1","object":"chat.completion","created":1,"model":"glm-5.2",
			"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[
				{"id":"call_custom","type":"function","function":{"name":"apply_patch","arguments":"{\"input\":\"patch\"}"}},
				{"id":"call_search","type":"function","function":{"name":"tool_search","arguments":"{\"query\":\"agents\"}"}},
				{"id":"call_future","type":"function","function":{"name":"future_lookup","arguments":"{\"id\":\"42\"}"}},
				{"id":"call_namespace","type":"function","function":{"name":"collaboration__spawn_agent","arguments":"{}"}}
			]}}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`),
	}
	llmResponse, err := chatOutbound.TransformResponse(ctx, chatResponse)
	require.NoError(t, err)
	responsesResponse, err := responsesInbound.TransformResponse(ctx, llmResponse)
	require.NoError(t, err)

	var result responsesapi.Response
	require.NoError(t, json.Unmarshal(responsesResponse.Body, &result))
	require.Len(t, result.Output, 4)
	require.Equal(t, "custom_tool_call", result.Output[0].Type)
	require.Equal(t, "tool_search_call", result.Output[1].Type)
	require.Equal(t, "function_call", result.Output[2].Type)
	require.Equal(t, "future_lookup", result.Output[2].Name)
	require.Equal(t, "function_call", result.Output[3].Type)
	require.Equal(t, "spawn_agent", result.Output[3].Name)
	require.Equal(t, "collaboration", result.Output[3].Namespace)
}

func TestResponsesToChatTools_DropsUnsupportedServerToolWithWarning(t *testing.T) {
	ctx := context.Background()
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"run","tools":[
			{"type":"function","name":"local_lookup","parameters":{"type":"object"}},
			{"type":"future_server_tool","name":"hosted","execution":"server"}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Tools, 1)
	require.Equal(t, "local_lookup", converted.Tools[0].Function.Name)
	warnings, ok := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
	require.True(t, ok)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "unsupported_tool_type")
	require.Contains(t, logs.String(), "Responses tools degraded during Chat Completions conversion")
	require.Contains(t, logs.String(), `"model":"gpt-5.5"`)
	require.Contains(t, logs.String(), "unsupported_tool_type")
}
