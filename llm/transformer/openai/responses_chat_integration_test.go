package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
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

func TestResponsesToChatTools_NamespaceCustomToolRoundTrip(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"additional_tools","role":"developer","tools":[
				{"type":"namespace","name":"functions","tools":[
					{"type":"custom","name":"exec","description":"Run code"},
					{"type":"function","name":"wait","parameters":{"type":"object","properties":{}}}
				]}
			]},
			{"type":"custom_tool_call","call_id":"call_history_exec","namespace":"functions","name":"exec","input":"pwd"},
			{"type":"custom_tool_call_output","call_id":"call_history_exec","output":"/workspace"},
			{"role":"user","type":"message","content":[{"type":"input_text","text":"continue"}]}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))

	// The namespace custom tool must survive conversion into a declared chat
	// function instead of being dropped as an opaque Responses-only tool.
	names := make([]string, 0, len(converted.Tools))
	for _, tool := range converted.Tools {
		require.Equal(t, "function", tool.Type)
		names = append(names, tool.Function.Name)
	}
	require.ElementsMatch(t, []string{"functions__exec", "functions__wait"}, names)
	var historyAssistant *Message
	for i := range converted.Messages {
		if converted.Messages[i].Role == "assistant" {
			historyAssistant = &converted.Messages[i]
			break
		}
	}
	require.NotNil(t, historyAssistant)
	require.Len(t, historyAssistant.ToolCalls, 1)
	require.Equal(t, "functions__exec", historyAssistant.ToolCalls[0].Function.Name)

	chatResponse := &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    chatRequest,
		Body: []byte(`{
			"id":"chatcmpl_1","object":"chat.completion","created":1,"model":"gpt-5.5",
			"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[
				{"id":"call_exec","type":"function","function":{"name":"functions__exec","arguments":"{\"input\":\"ls -la\"}"}},
				{"id":"call_wait","type":"function","function":{"name":"functions__wait","arguments":"{}"}}
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
	require.Len(t, result.Output, 2)
	require.Equal(t, "custom_tool_call", result.Output[0].Type)
	require.Equal(t, "exec", result.Output[0].Name)
	require.Equal(t, "functions", result.Output[0].Namespace)
	require.Equal(t, "function_call", result.Output[1].Type)
	require.Equal(t, "wait", result.Output[1].Name)
	require.Equal(t, "functions", result.Output[1].Namespace)
}

func TestResponsesToChatTools_NamespaceCustomNamesAreStrictlyFlattened(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"namespace","name":"first","tools":[{"type":"custom","name":"exec","description":"First"}]},
			{"type":"namespace","name":"second","tools":[{"type":"custom","name":"exec","description":"Second"}]}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Tools, 2)
	require.Equal(t, "first__exec", converted.Tools[0].Function.Name)
	require.Contains(t, converted.Tools[0].Function.Description, "First")
	require.Equal(t, "second__exec", converted.Tools[1].Function.Name)
	require.Contains(t, converted.Tools[1].Function.Description, "Second")

	warnings, _ := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
	require.Empty(t, warnings)
}

func TestResponsesToChatTools_NamespaceWireSelectors(t *testing.T) {
	const tools = `{"type":"namespace","name":"functions","tools":[
		{"type":"custom","name":"exec"},
		{"type":"function","name":"wait","parameters":{"type":"object"}}
	]}`

	t.Run("allowed tools keeps custom and function", func(t *testing.T) {
		ctx := context.Background()
		responsesInbound := responsesapi.NewInboundTransformer()
		llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
			"model":"gpt-5.5","input":"run","tools":[` + tools + `],
			"tool_choice":{"type":"allowed_tools","mode":"auto","tools":[{"type":"namespace","name":"functions"}]}
		}`)})
		require.NoError(t, err)
		chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
		require.NoError(t, err)
		chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
		require.NoError(t, err)

		var converted Request
		require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
		require.Equal(t, []string{"functions__exec", "functions__wait"}, lo.Map(
			converted.Tools, func(tool Tool, _ int) string { return tool.Function.Name },
		))
		require.NotNil(t, converted.ToolChoice.ToolChoice)
		require.Equal(t, "auto", *converted.ToolChoice.ToolChoice)
	})

	t.Run("named custom uses flattened name", func(t *testing.T) {
		ctx := context.Background()
		responsesInbound := responsesapi.NewInboundTransformer()
		llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
			"model":"gpt-5.5","input":"run","tools":[` + tools + `],
			"tool_choice":{"type":"custom","name":"functions__exec"}
		}`)})
		require.NoError(t, err)
		chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
		require.NoError(t, err)
		chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
		require.NoError(t, err)

		var converted Request
		require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
		require.Equal(t, "function", converted.ToolChoice.NamedToolChoice.Type)
		require.Equal(t, "functions__exec", converted.ToolChoice.NamedToolChoice.Function.Name)
	})

	t.Run("named namespace rejects multiple members", func(t *testing.T) {
		ctx := context.Background()
		responsesInbound := responsesapi.NewInboundTransformer()
		llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
			"model":"gpt-5.5","input":"run","tools":[` + tools + `],
			"tool_choice":{"type":"namespace","name":"functions"}
		}`)})
		require.NoError(t, err)
		chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
		require.NoError(t, err)
		chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
		require.Nil(t, chatRequest)
		require.ErrorContains(t, err, `namespace "functions" has multiple callable tools`)
	})
}

func TestResponsesToChatTools_NamespaceSelectorRejectsOpaqueMember(t *testing.T) {
	const requestBody = `{
		"model":"gpt-5.5","input":"run","tools":[{
			"type":"namespace","name":"functions","tools":[
				{"type":"future_server_tool","name":"hosted"},
				{"type":"custom","name":"exec"}
			]}],
		"tool_choice":{"type":"allowed_tools","mode":"auto","tools":[{"type":"namespace","name":"functions"}]}
	}`
	t.Run("allowed tools", func(t *testing.T) {
		responsesInbound := responsesapi.NewInboundTransformer()
		llmRequest, err := responsesInbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(requestBody)})
		require.NoError(t, err)
		chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
		require.NoError(t, err)
		chatRequest, err := chatOutbound.TransformRequest(t.Context(), llmRequest)
		require.Nil(t, chatRequest)
		require.ErrorContains(t, err, `namespace "functions" contains future_server_tool tool(s) without a Chat lifecycle codec`)
	})

	t.Run("named namespace", func(t *testing.T) {
		body := strings.Replace(requestBody,
			`{"type":"allowed_tools","mode":"auto","tools":[{"type":"namespace","name":"functions"}]}`,
			`{"type":"namespace","name":"functions"}`, 1)
		responsesInbound := responsesapi.NewInboundTransformer()
		llmRequest, err := responsesInbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(body)})
		require.NoError(t, err)
		chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
		require.NoError(t, err)
		chatRequest, err := chatOutbound.TransformRequest(t.Context(), llmRequest)
		require.Nil(t, chatRequest)
		require.ErrorContains(t, err, `namespace "functions" contains future_server_tool tool(s) without a Chat lifecycle codec`)
	})
}

func TestResponsesToChatTools_TopCustomAndNamespaceCustomBareHistoryStayStrict(t *testing.T) {
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"custom_tool_call","call_id":"call_exec","name":"exec","input":"top-level"},
			{"type":"custom_tool_call_output","call_id":"call_exec","output":"done"},
			{"role":"user","type":"message","content":[{"type":"input_text","text":"continue"}]}
		],
		"tools":[{
			"type":"namespace","name":"functions","tools":[{"type":"custom","name":"exec"}]
		},{"type":"custom","name":"exec"}]
	}`)})
	require.NoError(t, err)
	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(t.Context(), llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Equal(t, []string{"functions__exec", "exec"}, lo.Map(converted.Tools, func(tool Tool, _ int) string {
		return tool.Function.Name
	}))
	var assistant *Message
	for i := range converted.Messages {
		if converted.Messages[i].Role == "assistant" {
			assistant = &converted.Messages[i]
			break
		}
	}
	require.NotNil(t, assistant)
	require.Len(t, assistant.ToolCalls, 1)
	require.Equal(t, "exec", assistant.ToolCalls[0].Function.Name)
}

func TestResponsesToChatTools_NamespaceCustomFlatteningCollisionsFailInBothOrders(t *testing.T) {
	tests := []struct {
		name     string
		request  string
		expected string
	}{
		{
			name: "member first",
			request: `{"model":"gpt-5.5","input":"run","tools":[
				{"type":"namespace","name":"a","tools":[{"type":"custom","name":"b__c"}]},
				{"type":"namespace","name":"a__b","tools":[{"type":"custom","name":"c"}]}
			]}`,
			expected: `custom a.b__c and custom a__b.c`,
		},
		{
			name: "namespace first",
			request: `{"model":"gpt-5.5","input":"run","tools":[
				{"type":"namespace","name":"a__b","tools":[{"type":"custom","name":"c"}]},
				{"type":"namespace","name":"a","tools":[{"type":"custom","name":"b__c"}]}
			]}`,
			expected: `custom a__b.c and custom a.b__c`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responsesInbound := responsesapi.NewInboundTransformer()
			llmRequest, err := responsesInbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(tt.request)})
			require.NoError(t, err)
			chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
			require.NoError(t, err)
			chatRequest, err := chatOutbound.TransformRequest(t.Context(), llmRequest)
			require.Nil(t, chatRequest)
			require.ErrorContains(t, err, `tool_name_conflict: Chat name "a__b__c" is required by `+tt.expected)
		})
	}
}

func TestResponsesToChatTools_BareCustomHistoryDoesNotMapToNamespaceDefinition(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"additional_tools","role":"developer","tools":[{
				"type":"namespace",
				"name":"functions",
				"tools":[{
					"type":"custom",
					"name":"exec",
					"description":"Current definition"
				}]
			}]},
			{"type":"custom_tool_call","call_id":"call_old_exec","name":"exec","input":"old input"},
			{"type":"custom_tool_call_output","call_id":"call_old_exec","output":"old output"},
			{"role":"user","type":"message","content":[{"type":"input_text","text":"continue"}]}
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
	require.Equal(t, "functions__exec", converted.Tools[0].Function.Name)
	require.Contains(t, converted.Tools[0].Function.Description, "Current definition")

	var assistant *Message
	for i := range converted.Messages {
		if converted.Messages[i].Role == "assistant" {
			assistant = &converted.Messages[i]
			break
		}
	}
	require.NotNil(t, assistant)
	require.Len(t, assistant.ToolCalls, 1)
	require.Equal(t, "exec", assistant.ToolCalls[0].Function.Name)
}

func TestResponsesToChatTools_NamespaceFlattenedNameCollisionFailsStrictly(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"namespace","name":"mcp","tools":[{"type":"function","name":"server__list","parameters":{"type":"object"}}]},
			{"type":"namespace","name":"mcp__server","tools":[{"type":"function","name":"list","parameters":{"type":"object"}}]}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.ErrorContains(t, err, `tool_name_conflict: Chat name "mcp__server__list" is required by namespace mcp.server__list and namespace mcp__server.list`)
	require.Nil(t, chatRequest)
}

func TestResponsesToChatTools_NormalizesMissingObjectParameterTypes(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"run tools",
		"tools":[
			{"type":"function","name":"plain"},
			{"type":"namespace","name":"agents","tools":[
				{"type":"function","name":"spawn","parameters":{"properties":{"task":{"type":"string"}}}}
			]},
			{"type":"tool_search","execution":"client","parameters":{}}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Tools, 3)
	for _, tool := range converted.Tools {
		var schema map[string]any
		require.NoError(t, json.Unmarshal(tool.Function.Parameters, &schema))
		require.Equal(t, "object", schema["type"])
	}
	require.Equal(t, "plain", converted.Tools[0].Function.Name)
	require.Equal(t, "agents__spawn", converted.Tools[1].Function.Name)
	require.Equal(t, "tool_search", converted.Tools[2].Function.Name)
}

func TestResponsesToChatHistory_MergesConsecutiveNamespaceCallsBeforeOutputs(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"additional_tools","role":"developer","tools":[
				{"type":"namespace","name":"multi_agent_v1","tools":[
					{"type":"function","name":"spawn_agent","parameters":{"type":"object","properties":{}}},
					{"type":"function","name":"send_message","parameters":{"type":"object","properties":{}}}
				]}
			]},
			{"type":"function_call","call_id":"multi_agent_v1__spawn_agent:6","name":"spawn_agent","namespace":"multi_agent_v1","arguments":{}},
			{"type":"function_call","call_id":"multi_agent_v1__send_message:7","name":"send_message","namespace":"multi_agent_v1","arguments":{}},
			{"type":"function_call_output","call_id":"multi_agent_v1__spawn_agent:6","output":"spawned"},
			{"type":"function_call_output","call_id":"multi_agent_v1__send_message:7","output":"sent"},
			{"role":"user","type":"message","content":[{"type":"input_text","text":"continue"}]}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Messages, 4)
	require.Equal(t, "assistant", converted.Messages[0].Role)
	require.Len(t, converted.Messages[0].ToolCalls, 2)
	require.Equal(t, "multi_agent_v1__spawn_agent:6", converted.Messages[0].ToolCalls[0].ID)
	require.Equal(t, "multi_agent_v1__spawn_agent", converted.Messages[0].ToolCalls[0].Function.Name)
	require.Equal(t, "multi_agent_v1__send_message:7", converted.Messages[0].ToolCalls[1].ID)
	require.Equal(t, "multi_agent_v1__send_message", converted.Messages[0].ToolCalls[1].Function.Name)
	require.Equal(t, []int{0, 1}, []int{
		converted.Messages[0].ToolCalls[0].Index,
		converted.Messages[0].ToolCalls[1].Index,
	})
	require.Equal(t, "tool", converted.Messages[1].Role)
	require.Equal(t, "multi_agent_v1__spawn_agent:6", *converted.Messages[1].ToolCallID)
	require.Equal(t, "tool", converted.Messages[2].Role)
	require.Equal(t, "multi_agent_v1__send_message:7", *converted.Messages[2].ToolCallID)
	require.Equal(t, "user", converted.Messages[3].Role)
}

func TestResponsesToChatHistory_DropsResponsesOnlyEmptyAssistants(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"role":"user","type":"message","content":[{"type":"input_text","text":"before"}]},
			{"id":"rs_empty","type":"reasoning","summary":[],"encrypted_content":"opaque"},
			{"id":"msg_empty","role":"assistant","type":"message","content":[]},
			{"role":"user","type":"message","content":[{"type":"input_text","text":"after"}]}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Messages, 3)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Messages, 2)
	require.Equal(t, []string{"user", "user"}, []string{converted.Messages[0].Role, converted.Messages[1].Role})
	warnings, ok := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
	require.True(t, ok)
	require.Contains(t, warnings, "empty_assistant_message: dropped 1 history message(s) with no Chat-compatible payload")
}

func TestResponsesToChatCustomTool_ParateraNonStreamingSimulation(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"apply the patch","tools":[
			{"type":"function","name":"apply_patch_json","description":"JSON function","parameters":{"type":"object"}},
			{"type":"custom","name":"apply_patch","description":"Apply unified patch. This is a FREEFORM tool, so do not wrap the patch in JSON."}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://paratera.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Tools, 2)
	customChatTool := converted.Tools[1]
	require.Equal(t, "function", customChatTool.Type)
	require.Equal(t, "apply_patch", customChatTool.Function.Name)
	require.Contains(t, customChatTool.Function.Description, "represented as a Chat Completions function")
	require.Contains(t, customChatTool.Function.Description, "The outer function arguments must be valid JSON")
	require.Contains(t, customChatTool.Function.Description, "Original custom-tool instructions (apply to the `input` string):")
	require.Contains(t, customChatTool.Function.Description, "This is a FREEFORM tool, so do not wrap the patch in JSON.")
	var customSchema struct {
		Type       string `json:"type"`
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required             []string `json:"required"`
		AdditionalProperties bool     `json:"additionalProperties"`
	}
	require.NoError(t, json.Unmarshal(customChatTool.Function.Parameters, &customSchema))
	require.Equal(t, "object", customSchema.Type)
	require.Equal(t, []string{"input"}, customSchema.Required)
	require.False(t, customSchema.AdditionalProperties)
	require.Equal(t, "string", customSchema.Properties["input"].Type)
	require.Contains(t, customSchema.Properties["input"].Description, "Exact raw custom-tool input")
	require.Contains(t, customSchema.Properties["input"].Description, "Escape quotes, backslashes")
	require.Contains(t, customSchema.Properties["input"].Description, "outer JSON string")

	mappings := responsesChatToolMappings(chatRequest)
	mapping, ok := mappings[customChatTool.Function.Name]
	require.True(t, ok)
	require.Equal(t, responsesChatToolCustom, mapping.Kind)
	require.Equal(t, "apply_patch", mapping.Name)
	require.False(t, mapping.HistoryOnly)

	responseBody := marshalResponsesChatTestJSON(t, map[string]any{
		"id": "chatcmpl_custom", "object": "chat.completion", "created": 1, "model": "glm-5.2",
		"choices": []any{map[string]any{
			"index": 0, "finish_reason": "tool_calls",
			"message": map[string]any{
				"role": "assistant", "reasoning_content": "Need to apply the requested patch.",
				"tool_calls": []any{map[string]any{
					"id": "call_patch_42", "type": "function",
					"function": map[string]any{"name": customChatTool.Function.Name, "arguments": `{"input":"*** Begin Patch"}`},
				}},
			},
		}},
	})
	llmResponse, err := chatOutbound.TransformResponse(ctx, &httpclient.Response{
		StatusCode: http.StatusOK, Request: chatRequest, Body: responseBody,
	})
	require.NoError(t, err)
	require.Equal(t, "tool_calls", *llmResponse.Choices[0].FinishReason)
	require.Equal(t, "Need to apply the requested patch.", *llmResponse.Choices[0].Message.ReasoningContent)
	require.Len(t, llmResponse.Choices[0].Message.ToolCalls, 1)
	customCall := llmResponse.Choices[0].Message.ToolCalls[0]
	require.Equal(t, "call_patch_42", customCall.ID)
	require.NotNil(t, customCall.ResponseCustomToolCall)
	require.Equal(t, "call_patch_42", customCall.ResponseCustomToolCall.CallID)
	require.Equal(t, "apply_patch", customCall.ResponseCustomToolCall.Name)
	require.Equal(t, "*** Begin Patch", customCall.ResponseCustomToolCall.Input)

	responsesResponse, err := responsesInbound.TransformResponse(ctx, llmResponse)
	require.NoError(t, err)
	var result responsesapi.Response
	require.NoError(t, json.Unmarshal(responsesResponse.Body, &result))
	var reasoningItem, customItem *responsesapi.Item
	for i := range result.Output {
		switch result.Output[i].Type {
		case "reasoning":
			reasoningItem = &result.Output[i]
		case "custom_tool_call":
			customItem = &result.Output[i]
		}
	}
	require.NotNil(t, reasoningItem)
	require.NotNil(t, customItem)
	require.Equal(t, "call_patch_42", customItem.CallID)
	require.Equal(t, "apply_patch", customItem.Name)
	require.Equal(t, "*** Begin Patch", *customItem.Input)
}

func TestResponsesToChatCustomTool_ParateraStreamingSimulation(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","stream":true,"input":"apply the patch","tools":[
			{"type":"function","name":"apply_patch_json","parameters":{"type":"object"}},
			{"type":"custom","name":"apply_patch","description":"Apply unified patch"}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://paratera.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)
	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Tools, 2)
	customChatName := converted.Tools[1].Function.Name
	require.Equal(t, "apply_patch", customChatName)
	require.Contains(t, responsesChatToolMappings(chatRequest), customChatName)

	chatChunk := func(choice map[string]any) *httpclient.StreamEvent {
		return &httpclient.StreamEvent{Data: marshalResponsesChatTestJSON(t, map[string]any{
			"id": "chatcmpl_stream", "object": "chat.completion.chunk", "created": 1, "model": "glm-5.2",
			"choices": []any{choice},
		})}
	}
	providerEvents := []*httpclient.StreamEvent{
		chatChunk(map[string]any{
			"index": 0, "delta": map[string]any{"role": "assistant", "reasoning_content": "Need a patch."},
		}),
		chatChunk(map[string]any{
			"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{
				"index": 0, "id": "call_patch_stream", "type": "function",
				"function": map[string]any{"name": customChatName, "arguments": `{"input":"*** Begin`},
			}}},
		}),
		chatChunk(map[string]any{
			"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{
				"index": 0, "function": map[string]any{"arguments": ` Patch"}`},
			}}},
		}),
		chatChunk(map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"}),
		{Data: []byte("[DONE]")},
	}
	llmStream, err := chatOutbound.TransformStream(ctx, chatRequest, streams.SliceStream(providerEvents))
	require.NoError(t, err)
	responsesStream, err := responsesInbound.TransformStream(ctx, llmStream)
	require.NoError(t, err)

	var events []responsesapi.StreamEvent
	for responsesStream.Next() {
		var event responsesapi.StreamEvent
		require.NoError(t, json.Unmarshal(responsesStream.Current().Data, &event))
		events = append(events, event)
	}
	require.NoError(t, responsesStream.Err())

	customLifecycle := make([]responsesapi.StreamEventType, 0, 4)
	var customDone *responsesapi.Item
	var reasoningDone bool
	var completed *responsesapi.Response
	for i := range events {
		event := &events[i]
		switch event.Type {
		case responsesapi.StreamEventTypeOutputItemAdded:
			if event.Item != nil && event.Item.Type == "custom_tool_call" {
				customLifecycle = append(customLifecycle, event.Type)
				require.Equal(t, "call_patch_stream", event.Item.CallID)
				require.Equal(t, "apply_patch", event.Item.Name)
			}
		case responsesapi.StreamEventTypeCustomToolCallInputDelta:
			customLifecycle = append(customLifecycle, event.Type)
			require.Equal(t, "*** Begin Patch", event.Delta)
			require.NotContains(t, event.Delta, `{"input"`)
		case responsesapi.StreamEventTypeCustomToolCallInputDone:
			customLifecycle = append(customLifecycle, event.Type)
			require.Equal(t, "*** Begin Patch", event.Input)
		case responsesapi.StreamEventTypeOutputItemDone:
			if event.Item != nil && event.Item.Type == "reasoning" {
				reasoningDone = true
			}
			if event.Item != nil && event.Item.Type == "custom_tool_call" {
				customLifecycle = append(customLifecycle, event.Type)
				customDone = event.Item
			}
		case responsesapi.StreamEventTypeResponseCompleted:
			completed = event.Response
		}
	}
	require.True(t, reasoningDone)
	require.Equal(t, []responsesapi.StreamEventType{
		responsesapi.StreamEventTypeOutputItemAdded,
		responsesapi.StreamEventTypeCustomToolCallInputDelta,
		responsesapi.StreamEventTypeCustomToolCallInputDone,
		responsesapi.StreamEventTypeOutputItemDone,
	}, customLifecycle)
	require.NotNil(t, customDone)
	require.Equal(t, "call_patch_stream", customDone.CallID)
	require.Equal(t, "apply_patch", customDone.Name)
	require.Equal(t, "*** Begin Patch", *customDone.Input)
	require.NotNil(t, completed)
	require.Equal(t, "completed", *completed.Status)
	var completedCustom *responsesapi.Item
	for i := range completed.Output {
		if completed.Output[i].Type == "custom_tool_call" {
			completedCustom = &completed.Output[i]
			break
		}
	}
	require.NotNil(t, completedCustom)
	require.Equal(t, customDone.CallID, completedCustom.CallID)
	require.Equal(t, customDone.Name, completedCustom.Name)
	require.Equal(t, "*** Begin Patch", lo.FromPtr(completedCustom.Input))
}

func TestResponsesToChatCustomTool_StreamingSingleChunkWrapper(t *testing.T) {
	wrapper := string(marshalResponsesChatTestJSON(t, map[string]string{"input": "*** Begin Patch"}))
	events, streamErr := simulateResponsesChatCustomStream(t, []string{wrapper})
	require.NoError(t, streamErr)
	require.Equal(t, "*** Begin Patch", completedCustomInput(t, events))
}

func TestResponsesToChatCustomTool_StreamingRawArgumentsWithoutWrapper(t *testing.T) {
	events, streamErr := simulateResponsesChatCustomStream(t, []string{"*** Begin Patch"})
	require.NoError(t, streamErr)
	require.Equal(t, "*** Begin Patch", completedCustomInput(t, events))
}

func TestResponsesToChatStream_MapsAbnormalFinishReasons(t *testing.T) {
	tests := []struct {
		finishReason     string
		terminalType     responsesapi.StreamEventType
		status           string
		incompleteReason string
	}{
		{finishReason: "length", terminalType: responsesapi.StreamEventTypeResponseIncomplete, status: "incomplete", incompleteReason: "max_output_tokens"},
		{finishReason: "content_filter", terminalType: responsesapi.StreamEventTypeResponseIncomplete, status: "incomplete", incompleteReason: "content_filter"},
		{finishReason: "error", terminalType: responsesapi.StreamEventTypeResponseFailed, status: "failed"},
		{finishReason: "cancelled", terminalType: responsesapi.StreamEventTypeResponseCancelled, status: "cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.finishReason, func(t *testing.T) {
			events, streamErr := simulateResponsesChatStream(t, `{"model":"gpt-5.5","stream":true,"input":"hello"}`, []map[string]any{
				{"index": 0, "delta": map[string]any{"role": "assistant", "content": "partial"}},
				{"index": 0, "delta": map[string]any{}, "finish_reason": tt.finishReason},
			})
			require.NoError(t, streamErr)
			require.NotEmpty(t, events)
			terminal := events[len(events)-1]
			require.Equal(t, tt.terminalType, terminal.Type)
			require.NotNil(t, terminal.Response)
			require.Equal(t, tt.status, *terminal.Response.Status)
			if tt.incompleteReason != "" {
				require.NotNil(t, terminal.Response.IncompleteDetails)
				require.Equal(t, tt.incompleteReason, terminal.Response.IncompleteDetails.Reason)
			}
		})
	}
}

func TestResponsesToChatStream_AbnormalFinishDoesNotCompletePartialToolCalls(t *testing.T) {
	terminals := []struct {
		finishReason string
		terminalType responsesapi.StreamEventType
		status       string
	}{
		{finishReason: "length", terminalType: responsesapi.StreamEventTypeResponseIncomplete, status: "incomplete"},
		{finishReason: "content_filter", terminalType: responsesapi.StreamEventTypeResponseIncomplete, status: "incomplete"},
		{finishReason: "error", terminalType: responsesapi.StreamEventTypeResponseFailed, status: "failed"},
		{finishReason: "cancelled", terminalType: responsesapi.StreamEventTypeResponseCancelled, status: "cancelled"},
	}
	states := []struct {
		name     string
		toolName string
		started  bool
	}{
		{name: "pending identity", toolName: "apply_"},
		{name: "started call", toolName: "apply_patch", started: true},
	}

	for _, terminal := range terminals {
		for _, state := range states {
			t.Run(terminal.finishReason+"/"+state.name, func(t *testing.T) {
				events, streamErr := simulateResponsesChatStream(t, `{
					"model":"gpt-5.5","stream":true,"input":"apply","tools":[
						{"type":"custom","name":"apply_patch","description":"Apply patch"}
					]
				}`, []map[string]any{
					responsesChatToolDelta(map[string]any{
						"index": 0, "id": "call_partial", "type": "function",
						"function": map[string]any{"name": state.toolName, "arguments": `{"input":"partial`},
					}),
					{"index": 0, "delta": map[string]any{}, "finish_reason": terminal.finishReason},
				})
				require.NoError(t, streamErr)
				require.NotEmpty(t, events)

				added := false
				for _, event := range events {
					require.NotEqual(t, responsesapi.StreamEventTypeCustomToolCallInputDone, event.Type)
					if event.Type == responsesapi.StreamEventTypeOutputItemDone && event.Item != nil {
						require.NotEqual(t, "custom_tool_call", event.Item.Type)
					}
					require.NotEqual(t, responsesapi.StreamEventTypeResponseCompleted, event.Type)
					if event.Type == responsesapi.StreamEventTypeOutputItemAdded && event.Item != nil && event.Item.Type == "custom_tool_call" {
						added = true
						require.Equal(t, "in_progress", lo.FromPtr(event.Item.Status))
					}
				}
				require.Equal(t, state.started, added)

				last := events[len(events)-1]
				require.Equal(t, terminal.terminalType, last.Type)
				require.NotNil(t, last.Response)
				require.Equal(t, terminal.status, lo.FromPtr(last.Response.Status))
				partialCalls := 0
				for _, item := range last.Response.Output {
					if item.Type == "custom_tool_call" {
						partialCalls++
						require.Equal(t, "in_progress", lo.FromPtr(item.Status))
					}
				}
				if state.started {
					require.Equal(t, 1, partialCalls)
				} else {
					require.Zero(t, partialCalls)
				}
			})
		}
	}
}

func TestResponsesToChatCustomTool_StreamAcceptsLateOrFragmentedName(t *testing.T) {
	wrapper := string(marshalResponsesChatTestJSON(t, map[string]string{"input": "*** Begin Patch"}))
	tests := []struct {
		name  string
		build func(string) []map[string]any
	}{
		{
			name: "id before name",
			build: func(customName string) []map[string]any {
				return []map[string]any{
					responsesChatToolDelta(map[string]any{
						"index": 0, "id": "call_patch_late_name", "type": "function", "function": map[string]any{},
					}),
					responsesChatToolDelta(map[string]any{
						"index": 0, "function": map[string]any{"name": customName, "arguments": wrapper},
					}),
				}
			},
		},
		{
			name: "fragmented name",
			build: func(customName string) []map[string]any {
				split := len(customName) / 2
				return []map[string]any{
					responsesChatToolDelta(map[string]any{
						"index": 0, "id": "call_patch_fragmented_name", "type": "function",
						"function": map[string]any{"name": customName[:split]},
					}),
					responsesChatToolDelta(map[string]any{
						"index": 0, "function": map[string]any{"name": customName[split:], "arguments": wrapper},
					}),
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []responsesapi.StreamEvent
			var streamErr error
			require.NotPanics(t, func() {
				events, streamErr = simulateResponsesChatCustomChoices(t, tt.build)
			})
			require.NoError(t, streamErr)
			require.Equal(t, "*** Begin Patch", completedCustomInput(t, events))
		})
	}
}

func TestResponsesToChatCustomTool_StreamKeepsItemIDWhenCallIDArrivesLate(t *testing.T) {
	wrapper := string(marshalResponsesChatTestJSON(t, map[string]string{"input": "*** Begin Patch"}))
	events, streamErr := simulateResponsesChatCustomChoices(t, func(customName string) []map[string]any {
		return []map[string]any{
			responsesChatToolDelta(map[string]any{
				"index": 0, "type": "function", "function": map[string]any{"name": customName},
			}),
			responsesChatToolDelta(map[string]any{
				"index": 0, "id": "call_patch_late_id", "function": map[string]any{"arguments": wrapper},
			}),
		}
	})
	require.NoError(t, streamErr)

	var added, done *responsesapi.Item
	for i := range events {
		event := events[i]
		if event.Item == nil || event.Item.Type != "custom_tool_call" {
			continue
		}
		switch event.Type {
		case responsesapi.StreamEventTypeOutputItemAdded:
			added = event.Item
		case responsesapi.StreamEventTypeOutputItemDone:
			done = event.Item
		}
	}
	require.NotNil(t, added)
	require.NotNil(t, done)
	require.Equal(t, added.ID, done.ID)
	require.Equal(t, "call_patch_late_id", added.CallID)
	require.Equal(t, "call_patch_late_id", done.CallID)
	require.Equal(t, added.CallID, done.CallID)
	require.Equal(t, "*** Begin Patch", *done.Input)
}

func TestResponsesToChatStream_ReassemblesFragmentedPlainFunctionName(t *testing.T) {
	events, streamErr := simulateResponsesChatStream(t, `{
		"model":"gpt-5.5","stream":true,"input":"look up the city","tools":[
			{"type":"function","name":"lookup","parameters":{"type":"object"}},
			{"type":"custom","name":"apply_patch"}
		]
	}`, []map[string]any{
		responsesChatToolDelta(map[string]any{
			"index": 0, "id": "call_lookup", "type": "function",
			"function": map[string]any{"name": "look", "arguments": `{"city":"Par`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 0, "function": map[string]any{"name": "up", "arguments": `is"}`},
		}),
		{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"},
	})
	require.NoError(t, streamErr)

	var done *responsesapi.Item
	for i := range events {
		event := events[i]
		if event.Type == responsesapi.StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "function_call" {
			done = event.Item
		}
	}
	require.NotNil(t, done)
	require.Equal(t, "call_lookup", done.CallID)
	require.Equal(t, "lookup", done.Name)
	require.JSONEq(t, `{"city":"Paris"}`, done.Arguments)
}

func TestResponsesToChatStream_ResolvesCatalogPrefixAmbiguityAtFinish(t *testing.T) {
	tests := []struct {
		name       string
		toolDeltas []map[string]any
		wantName   string
		wantArgs   string
	}{
		{
			name: "fragmented long name",
			toolDeltas: []map[string]any{
				responsesChatToolDelta(map[string]any{
					"index": 0, "id": "call_get_weather", "type": "function",
					"function": map[string]any{"name": "get", "arguments": `{"city":"Par`},
				}),
				responsesChatToolDelta(map[string]any{
					"index": 0, "function": map[string]any{"name": "_weather", "arguments": `is"}`},
				}),
			},
			wantName: "get_weather",
			wantArgs: `{"city":"Paris"}`,
		},
		{
			name: "complete short name",
			toolDeltas: []map[string]any{
				responsesChatToolDelta(map[string]any{
					"index": 0, "id": "call_get", "type": "function",
					"function": map[string]any{"name": "get", "arguments": `{"id":"42"}`},
				}),
			},
			wantName: "get",
			wantArgs: `{"id":"42"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			choices := append([]map[string]any{}, tt.toolDeltas...)
			choices = append(choices, map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"})
			events, streamErr := simulateResponsesChatStream(t, `{
				"model":"gpt-5.5","stream":true,"input":"fetch data","tools":[
					{"type":"function","name":"get","parameters":{"type":"object"}},
					{"type":"function","name":"get_weather","parameters":{"type":"object"}}
				]
			}`, choices)
			require.NoError(t, streamErr)

			var doneItems []*responsesapi.Item
			for i := range events {
				event := events[i]
				if event.Type == responsesapi.StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "function_call" {
					doneItems = append(doneItems, event.Item)
				}
			}
			require.Len(t, doneItems, 1)
			require.Equal(t, tt.wantName, doneItems[0].Name)
			require.JSONEq(t, tt.wantArgs, doneItems[0].Arguments)
		})
	}
}

func TestResponsesToChatCustomTool_StreamDoesNotMatchNameSuffix(t *testing.T) {
	events, streamErr := simulateResponsesChatStream(t, `{
		"model":"gpt-5.5","stream":true,"input":"apply the patch","tools":[
			{"type":"custom","name":"apply_patch"},
			{"type":"custom","name":"patch"}
		]
	}`, []map[string]any{
		responsesChatToolDelta(map[string]any{
			"index": 0, "id": "call_apply_patch", "type": "function",
			"function": map[string]any{"name": "apply_", "arguments": `{"input":"*** Begin`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 0, "function": map[string]any{"name": "patch", "arguments": ` Patch"}`},
		}),
		{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"},
	})
	require.NoError(t, streamErr)

	var done *responsesapi.Item
	for i := range events {
		event := events[i]
		if event.Type == responsesapi.StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "custom_tool_call" {
			done = event.Item
		}
	}
	require.NotNil(t, done)
	require.Equal(t, "call_apply_patch", done.CallID)
	require.Equal(t, "apply_patch", done.Name)
	require.Equal(t, "*** Begin Patch", *done.Input)
}

func TestResponsesToChatStream_ParallelPlainAndCustomCallsCloseAfterAllDeltas(t *testing.T) {
	events, streamErr := simulateResponsesChatStream(t, `{
		"model":"gpt-5.5","stream":true,"input":"look up and patch","tools":[
			{"type":"function","name":"lookup","parameters":{"type":"object"}},
			{"type":"custom","name":"apply_patch"}
		]
	}`, []map[string]any{
		responsesChatToolDelta(map[string]any{
			"index": 0, "id": "call_lookup_parallel", "type": "function",
			"function": map[string]any{"name": "lookup", "arguments": `{"query":"hel`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 1, "id": "call_patch_parallel", "type": "function",
			"function": map[string]any{"name": "apply_patch", "arguments": `{"input":"pat`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 0, "function": map[string]any{"arguments": `lo"}`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 1, "function": map[string]any{"arguments": `ch"}`},
		}),
		{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"},
	})
	require.NoError(t, streamErr)

	lastLookupDeltaIndex := -1
	lookupArgumentsDoneIndex := -1
	lookupOutputDoneIndex := -1
	lookupArgumentsDoneCount := 0
	lookupOutputDoneCount := 0
	customOutputDoneCount := 0
	for i := range events {
		event := events[i]
		if event.Type == responsesapi.StreamEventTypeFunctionCallArgumentsDelta && event.ItemID != nil && *event.ItemID == "call_lookup_parallel" {
			lastLookupDeltaIndex = i
		}
		if event.Type == responsesapi.StreamEventTypeFunctionCallArgumentsDone && event.ItemID != nil && *event.ItemID == "call_lookup_parallel" {
			lookupArgumentsDoneCount++
			lookupArgumentsDoneIndex = i
			require.JSONEq(t, `{"query":"hello"}`, event.Arguments)
		}
		if event.Type != responsesapi.StreamEventTypeOutputItemDone || event.Item == nil {
			continue
		}
		switch event.Item.CallID {
		case "call_lookup_parallel":
			lookupOutputDoneCount++
			lookupOutputDoneIndex = i
			require.Equal(t, "function_call", event.Item.Type)
			require.JSONEq(t, `{"query":"hello"}`, event.Item.Arguments)
		case "call_patch_parallel":
			customOutputDoneCount++
			require.Equal(t, "custom_tool_call", event.Item.Type)
			require.Equal(t, "patch", *event.Item.Input)
		}
	}
	require.NotEqual(t, -1, lastLookupDeltaIndex)
	require.Greater(t, lookupArgumentsDoneIndex, lastLookupDeltaIndex)
	require.Greater(t, lookupOutputDoneIndex, lookupArgumentsDoneIndex)
	require.Equal(t, 1, lookupArgumentsDoneCount)
	require.Equal(t, 1, lookupOutputDoneCount)
	require.Equal(t, 1, customOutputDoneCount)
}

func TestResponsesToChatStream_ParallelOutputOrderMatchesNonStreaming(t *testing.T) {
	const nonStreamingRequest = `{
		"model":"gpt-5.5","input":"look up and patch","tools":[
			{"type":"custom","name":"apply_patch"},
			{"type":"function","name":"lookup","parameters":{"type":"object"}}
		]
	}`
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(nonStreamingRequest)})
	require.NoError(t, err)
	chatOutbound, err := NewOutboundTransformer("https://paratera.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	chatResponse := &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    chatRequest,
		Body: []byte(`{
			"id":"chatcmpl_order","object":"chat.completion","created":1,"model":"glm-5.2",
			"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[
				{"id":"call_patch_order","type":"function","function":{"name":"apply_patch","arguments":"{\"input\":\"patch\"}"}},
				{"id":"call_lookup_order","type":"function","function":{"name":"lookup","arguments":"{\"query\":\"hello\"}"}}
			]}}]
		}`),
	}
	llmResponse, err := chatOutbound.TransformResponse(ctx, chatResponse)
	require.NoError(t, err)
	responsesResponse, err := responsesInbound.TransformResponse(ctx, llmResponse)
	require.NoError(t, err)
	var nonStreaming responsesapi.Response
	require.NoError(t, json.Unmarshal(responsesResponse.Body, &nonStreaming))

	type orderedCall struct {
		OutputIndex int
		Type        string
		CallID      string
		Name        string
		Payload     string
	}
	orderedItem := func(outputIndex int, item responsesapi.Item, includePayload bool) orderedCall {
		payload := ""
		if includePayload {
			payload = item.Arguments
			if item.Input != nil {
				payload = *item.Input
			}
		}
		return orderedCall{
			OutputIndex: outputIndex, Type: item.Type, CallID: item.CallID, Name: item.Name, Payload: payload,
		}
	}
	nonStreamingOrder := make([]orderedCall, 0, len(nonStreaming.Output))
	nonStreamingAddedOrder := make([]orderedCall, 0, len(nonStreaming.Output))
	for index := range nonStreaming.Output {
		nonStreamingOrder = append(nonStreamingOrder, orderedItem(index, nonStreaming.Output[index], true))
		nonStreamingAddedOrder = append(nonStreamingAddedOrder, orderedItem(index, nonStreaming.Output[index], false))
	}

	streamEvents, streamErr := simulateResponsesChatStream(t, `{
		"model":"gpt-5.5","stream":true,"input":"look up and patch","tools":[
			{"type":"custom","name":"apply_patch"},
			{"type":"function","name":"lookup","parameters":{"type":"object"}}
		]
	}`, []map[string]any{
		responsesChatToolDelta(map[string]any{
			"index": 0, "id": "call_patch_order", "type": "function",
			"function": map[string]any{"name": "apply_", "arguments": `{"input":"pa`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 1, "id": "call_lookup_order", "type": "function",
			"function": map[string]any{"name": "lookup", "arguments": `{"query":"hello"}`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 0, "function": map[string]any{"name": "apply_", "arguments": `t`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 0, "function": map[string]any{"name": "apply_patch", "arguments": `ch"}`},
		}),
		{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"},
	})
	require.NoError(t, streamErr)

	streamingOrder := make([]orderedCall, 0, len(nonStreamingOrder))
	streamingDoneOrder := make([]orderedCall, 0, len(nonStreamingOrder))
	deltaCounts := make(map[int]int)
	inputDoneCounts := make(map[int]int)
	inputDonePayloads := make(map[int]string)
	inputDoneTypes := make(map[int]responsesapi.StreamEventType)
	inputDoneItemIDs := make(map[int]string)
	var completed *responsesapi.Response
	for i := range streamEvents {
		event := streamEvents[i]
		switch event.Type {
		case responsesapi.StreamEventTypeOutputItemAdded:
			if event.Item != nil {
				switch event.Item.Type {
				case "custom_tool_call":
					require.NotNil(t, event.Item.Input)
					require.Empty(t, *event.Item.Input)
					require.Empty(t, event.Item.Arguments)
				case "function_call":
					require.Nil(t, event.Item.Input)
					require.Empty(t, event.Item.Arguments)
				}
				streamingOrder = append(streamingOrder, orderedItem(event.OutputIndex, *event.Item, false))
			}
		case responsesapi.StreamEventTypeFunctionCallArgumentsDelta,
			responsesapi.StreamEventTypeCustomToolCallInputDelta:
			deltaCounts[event.OutputIndex]++
		case responsesapi.StreamEventTypeFunctionCallArgumentsDone,
			responsesapi.StreamEventTypeCustomToolCallInputDone:
			inputDoneCounts[event.OutputIndex]++
			inputDoneTypes[event.OutputIndex] = event.Type
			inputDoneItemIDs[event.OutputIndex] = lo.FromPtr(event.ItemID)
			inputDonePayloads[event.OutputIndex] = event.Arguments
			if event.Type == responsesapi.StreamEventTypeCustomToolCallInputDone {
				inputDonePayloads[event.OutputIndex] = event.Input
			}
		case responsesapi.StreamEventTypeOutputItemDone:
			if event.Item != nil {
				streamingDoneOrder = append(streamingDoneOrder, orderedItem(event.OutputIndex, *event.Item, true))
			}
		case responsesapi.StreamEventTypeResponseCompleted:
			completed = event.Response
		}
	}
	require.Equal(t, nonStreamingAddedOrder, streamingOrder)
	require.Equal(t, nonStreamingOrder, streamingDoneOrder)
	for outputIndex := range nonStreamingOrder {
		require.Positive(t, deltaCounts[outputIndex], "output index %d must emit at least one delta", outputIndex)
		require.Equal(t, 1, inputDoneCounts[outputIndex], "output index %d must emit exactly one input/arguments done", outputIndex)
		require.Equal(t, nonStreamingOrder[outputIndex].Payload, inputDonePayloads[outputIndex], "output index %d done payload must match non-streaming output", outputIndex)
		require.Equal(t, nonStreaming.Output[outputIndex].ID, inputDoneItemIDs[outputIndex], "output index %d done item ID must match", outputIndex)
		expectedDoneType := responsesapi.StreamEventTypeFunctionCallArgumentsDone
		if nonStreamingOrder[outputIndex].Type == "custom_tool_call" {
			expectedDoneType = responsesapi.StreamEventTypeCustomToolCallInputDone
		}
		require.Equal(t, expectedDoneType, inputDoneTypes[outputIndex], "output index %d must use type-specific done event", outputIndex)
	}
	require.NotNil(t, completed)
	require.Len(t, completed.Output, len(nonStreaming.Output))
	completedOrder := make([]orderedCall, 0, len(completed.Output))
	for outputIndex := range completed.Output {
		completedOrder = append(completedOrder, orderedItem(outputIndex, completed.Output[outputIndex], true))
	}
	require.Equal(t, nonStreamingOrder, completedOrder)
}

func TestResponsesToChatStream_ToolSearchAndNamespaceFinishWithFinalFragments(t *testing.T) {
	events, streamErr := simulateResponsesChatStream(t, `{
		"model":"gpt-5.5","stream":true,"input":"discover and spawn","tools":[
			{"type":"tool_search","execution":"client","parameters":{"type":"object"}},
			{"type":"namespace","name":"collaboration","tools":[
				{"type":"function","name":"spawn_agent","parameters":{"type":"object"}}
			]}
		]
	}`, []map[string]any{
		responsesChatToolDelta(map[string]any{
			"index": 0, "id": "call_search_stream", "type": "function",
			"function": map[string]any{"name": "tool_", "arguments": `{"query":"ag`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 1, "id": "call_spawn_stream", "type": "function",
			"function": map[string]any{"name": "collaboration__spawn_", "arguments": `{"task":"re`},
		}),
		{
			"index": 0,
			"delta": map[string]any{"tool_calls": []any{
				map[string]any{"index": 0, "function": map[string]any{"name": "tool_search", "arguments": `ents"}`}},
				map[string]any{"index": 1, "function": map[string]any{"name": "collaboration__spawn_agent", "arguments": `view"}`}},
			}},
			"finish_reason": "tool_calls",
		},
	})
	require.NoError(t, streamErr)

	added := make(map[int]responsesapi.Item)
	deltas := make(map[int]string)
	donePayloads := make(map[int]string)
	doneItems := make(map[int]responsesapi.Item)
	var completed *responsesapi.Response
	for i := range events {
		event := events[i]
		switch event.Type {
		case responsesapi.StreamEventTypeOutputItemAdded:
			require.NotNil(t, event.Item)
			added[event.OutputIndex] = *event.Item
		case responsesapi.StreamEventTypeFunctionCallArgumentsDelta:
			deltas[event.OutputIndex] += event.Delta
		case responsesapi.StreamEventTypeFunctionCallArgumentsDone:
			donePayloads[event.OutputIndex] = event.Arguments
		case responsesapi.StreamEventTypeOutputItemDone:
			require.NotNil(t, event.Item)
			doneItems[event.OutputIndex] = *event.Item
		case responsesapi.StreamEventTypeResponseCompleted:
			completed = event.Response
		}
	}

	require.Len(t, added, 2)
	require.Equal(t, "tool_search_call", added[0].Type)
	require.Equal(t, "call_search_stream", added[0].ID)
	require.Equal(t, "call_search_stream", added[0].CallID)
	require.Equal(t, "client", added[0].Execution)
	require.JSONEq(t, `{}`, added[0].Arguments)
	require.Equal(t, "function_call", added[1].Type)
	require.Equal(t, "call_spawn_stream", added[1].ID)
	require.Equal(t, "call_spawn_stream", added[1].CallID)
	require.Equal(t, "spawn_agent", added[1].Name)
	require.Equal(t, "collaboration", added[1].Namespace)
	require.Empty(t, added[1].Arguments)

	require.JSONEq(t, `{"query":"agents"}`, deltas[0])
	require.JSONEq(t, `{"task":"review"}`, deltas[1])
	require.JSONEq(t, deltas[0], donePayloads[0])
	require.JSONEq(t, deltas[1], donePayloads[1])
	require.Len(t, doneItems, 2)
	require.Equal(t, "tool_search_call", doneItems[0].Type)
	require.Equal(t, "client", doneItems[0].Execution)
	require.JSONEq(t, deltas[0], doneItems[0].Arguments)
	require.Equal(t, "function_call", doneItems[1].Type)
	require.Equal(t, "spawn_agent", doneItems[1].Name)
	require.Equal(t, "collaboration", doneItems[1].Namespace)
	require.JSONEq(t, deltas[1], doneItems[1].Arguments)

	require.NotNil(t, completed)
	require.Len(t, completed.Output, 2)
	require.Equal(t, doneItems[0], completed.Output[0])
	require.Equal(t, doneItems[1], completed.Output[1])
}

func TestResponsesToChatStream_NamespaceCustomAndFunctionRestoreSeparately(t *testing.T) {
	events, streamErr := simulateResponsesChatStream(t, `{
		"model":"gpt-5.5","stream":true,"input":"run and wait","tools":[{
			"type":"namespace","name":"functions","tools":[
				{"type":"custom","name":"exec"},
				{"type":"function","name":"wait","parameters":{"type":"object"}}
			]
		}]
	}`, []map[string]any{
		responsesChatToolDelta(map[string]any{
			"index": 0, "id": "call_exec", "type": "function",
			"function": map[string]any{"name": "functions__ex", "arguments": `{"input":"ls`},
		}),
		responsesChatToolDelta(map[string]any{
			"index": 1, "id": "call_wait", "type": "function",
			"function": map[string]any{"name": "functions__wait", "arguments": `{}`},
		}),
		{
			"index": 0,
			"delta": map[string]any{"tool_calls": []any{
				map[string]any{"index": 0, "function": map[string]any{"name": "ec", "arguments": ` -la"}`}},
				map[string]any{"index": 1, "function": map[string]any{"name": "functions__wait"}},
			}},
			"finish_reason": "tool_calls",
		},
	})
	require.NoError(t, streamErr)

	items := make(map[int]responsesapi.Item)
	for i := range events {
		event := events[i]
		if event.Type == responsesapi.StreamEventTypeOutputItemDone && event.Item != nil {
			items[event.OutputIndex] = *event.Item
		}
	}
	require.Len(t, items, 2)
	require.Equal(t, "custom_tool_call", items[0].Type)
	require.Equal(t, "exec", items[0].Name)
	require.Equal(t, "ls -la", *items[0].Input)
	require.Equal(t, "function_call", items[1].Type)
	require.Equal(t, "wait", items[1].Name)
	require.Equal(t, "functions", items[1].Namespace)
	require.JSONEq(t, `{}`, items[1].Arguments)
}

func TestResponsesToChatStream_NamespaceCustomAbnormalFinishDoesNotFlush(t *testing.T) {
	const requestBody = `{
		"model":"gpt-5.5","stream":true,"input":"run","tools":[{
			"type":"namespace","name":"functions","tools":[{"type":"custom","name":"exec"}]
		}]
	}`
	tests := []struct {
		name         string
		terminal     map[string]any
		terminalType responsesapi.StreamEventType
	}{
		{
			name:         "length finish",
			terminal:     map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "length"},
			terminalType: responsesapi.StreamEventTypeResponseIncomplete,
		},
		{
			name:         "missing finish",
			terminal:     map[string]any{"index": 0, "delta": map[string]any{}},
			terminalType: responsesapi.StreamEventTypeResponseFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, streamErr := simulateResponsesChatStream(t, requestBody, []map[string]any{
				responsesChatToolDelta(map[string]any{
					"index": 0, "id": "call_exec_abnormal", "type": "function",
					"function": map[string]any{"name": "functions__exec", "arguments": `{"input":"ls`},
				}),
				tt.terminal,
			})
			require.NoError(t, streamErr)

			outputDone := 0
			customDone := 0
			completed := 0
			terminalEvents := 0
			for i := range events {
				event := events[i]
				switch event.Type {
				case responsesapi.StreamEventTypeOutputItemDone:
					outputDone++
					if event.Item != nil && event.Item.Type == "custom_tool_call" {
						customDone++
					}
				case responsesapi.StreamEventTypeResponseCompleted:
					completed++
				case responsesapi.StreamEventTypeResponseIncomplete, responsesapi.StreamEventTypeResponseFailed:
					terminalEvents++
				}
			}
			require.Equal(t, 0, outputDone, "abnormal stream must not flush output items as done")
			require.Equal(t, 0, customDone, "partial namespace custom call must not be restored")
			require.Equal(t, 0, completed, "abnormal stream must not emit response.completed")
			require.Equal(t, 1, terminalEvents, "abnormal stream must emit exactly one terminal failure event")
		})
	}
}

func TestResponsesToChatStream_StrictFinishAppliesWithoutTools(t *testing.T) {
	tests := []struct {
		name         string
		finish       any
		terminalType responsesapi.StreamEventType
	}{
		{name: "normal finish", finish: "stop", terminalType: responsesapi.StreamEventTypeResponseCompleted},
		{name: "missing finish", finish: nil, terminalType: responsesapi.StreamEventTypeResponseFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terminal := map[string]any{"index": 0, "delta": map[string]any{"content": "done"}}
			if tt.finish != nil {
				terminal["finish_reason"] = tt.finish
			}
			requestBody := "{\"model\":\"gpt-5.5\",\"stream\":true,\"input\":\"hello\"}"
			events, streamErr := simulateResponsesChatStream(t, requestBody, []map[string]any{terminal})
			require.NoError(t, streamErr)

			terminals := 0
			for i := range events {
				switch events[i].Type {
				case responsesapi.StreamEventTypeResponseCompleted,
					responsesapi.StreamEventTypeResponseIncomplete,
					responsesapi.StreamEventTypeResponseFailed:
					terminals++
					require.Equal(t, tt.terminalType, events[i].Type)
				}
			}
			require.Equal(t, 1, terminals)
		})
	}
}

func TestResponsesToChatStream_FilteredNamespaceCustomDoesNotRestoreSpecialCall(t *testing.T) {
	events, streamErr := simulateResponsesChatStream(t, `{
		"model":"gpt-5.5","stream":true,"input":"run and wait","tools":[{
			"type":"namespace","name":"functions","tools":[
				{"type":"custom","name":"exec"},
				{"type":"function","name":"wait","parameters":{"type":"object"}}
			]}],
		"tool_choice":{"type":"allowed_tools","mode":"auto","tools":[
			{"type":"function","name":"functions__wait"}
		]}
	}`, []map[string]any{
		responsesChatToolDelta(map[string]any{
			"index": 0, "id": "call_exec", "type": "function",
			"function": map[string]any{"name": "functions__exec", "arguments": `{"input":"ls -la"}`},
		}),
		{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"},
	})
	require.NoError(t, streamErr)

	var item *responsesapi.Item
	for i := range events {
		event := events[i]
		if event.Type == responsesapi.StreamEventTypeOutputItemDone && event.Item != nil {
			require.Nil(t, item, "expected one completed output item")
			completed := *event.Item
			item = &completed
		}
	}
	require.NotNil(t, item)
	require.Equal(t, "function_call", item.Type)
	require.Equal(t, "functions__exec", item.Name)
	require.Empty(t, item.Namespace)
	require.Nil(t, item.Input)
}

func TestResponsesToChatCustomTool_PreservesJSONLikeRawInput(t *testing.T) {
	rawInput := `{"path":"C:\\tmp","quote":"say \"hi\""}`
	wrapper := string(marshalResponsesChatTestJSON(t, map[string]string{"input": rawInput}))

	t.Run("non-streaming", func(t *testing.T) {
		ctx := context.Background()
		responsesInbound := responsesapi.NewInboundTransformer()
		llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
			"model":"gpt-5.5","input":"apply the patch","tools":[
				{"type":"custom","name":"apply_patch","description":"Apply unified patch"}
			]
		}`)})
		require.NoError(t, err)
		chatOutbound, err := NewOutboundTransformer("https://paratera.example.com", "test-key")
		require.NoError(t, err)
		chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
		require.NoError(t, err)

		var converted Request
		require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
		require.Len(t, converted.Tools, 1)
		responseBody := marshalResponsesChatTestJSON(t, map[string]any{
			"id": "chatcmpl_json_input", "object": "chat.completion", "created": 1, "model": "glm-5.2",
			"choices": []any{map[string]any{
				"index": 0, "finish_reason": "tool_calls",
				"message": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
					"id": "call_json_input", "type": "function",
					"function": map[string]any{"name": converted.Tools[0].Function.Name, "arguments": wrapper},
				}}},
			}},
		})
		llmResponse, err := chatOutbound.TransformResponse(ctx, &httpclient.Response{
			StatusCode: http.StatusOK, Request: chatRequest, Body: responseBody,
		})
		require.NoError(t, err)
		require.Equal(t, rawInput, llmResponse.Choices[0].Message.ToolCalls[0].ResponseCustomToolCall.Input)

		responsesResponse, err := responsesInbound.TransformResponse(ctx, llmResponse)
		require.NoError(t, err)
		var result responsesapi.Response
		require.NoError(t, json.Unmarshal(responsesResponse.Body, &result))
		require.Len(t, result.Output, 1)
		require.Equal(t, "custom_tool_call", result.Output[0].Type)
		require.Equal(t, rawInput, *result.Output[0].Input)
	})

	t.Run("streaming", func(t *testing.T) {
		split := len(wrapper) / 2
		events, streamErr := simulateResponsesChatCustomStream(t, []string{wrapper[:split], wrapper[split:]})
		require.NoError(t, streamErr)
		require.Equal(t, rawInput, completedCustomInput(t, events))
	})
}

func TestResponsesToChatCustomTool_UnwrapFallbackPreservesRawInput(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		want      string
	}{
		{name: "wrapped input", arguments: `{"input":"patch"}`, want: "patch"},
		{name: "wrapped empty input", arguments: `{"input":""}`, want: ""},
		{name: "missing input", arguments: `{}`, want: `{}`},
		{name: "other field", arguments: `{"patch":"value"}`, want: `{"patch":"value"}`},
		{name: "null input", arguments: `{"input":null}`, want: `{"input":null}`},
		{name: "malformed wrapper", arguments: `{"input":"patch"`, want: `{"input":"patch"`},
	}

	mappings := map[string]responsesChatToolMapping{
		"apply_patch": {Kind: responsesChatToolCustom, Name: "apply_patch"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" non-streaming", func(t *testing.T) {
			message := &llm.Message{ToolCalls: []llm.ToolCall{{
				ID: "call_patch", Function: llm.FunctionCall{Name: "apply_patch", Arguments: tt.arguments},
			}}}
			restoreResponsesChatMessage(message, mappings, true)
			require.Len(t, message.ToolCalls, 1)
			require.NotNil(t, message.ToolCalls[0].ResponseCustomToolCall)
			require.Equal(t, tt.want, message.ToolCalls[0].ResponseCustomToolCall.Input)
		})

		t.Run(tt.name+" streaming", func(t *testing.T) {
			events, streamErr := simulateResponsesChatCustomStream(t, []string{tt.arguments})
			require.NoError(t, streamErr)
			require.Equal(t, tt.want, completedCustomInput(t, events))
		})
	}
}

func TestResponsesToChatCustomTool_DegradesGrammarConstraint(t *testing.T) {
	responsesInbound := responsesapi.NewInboundTransformer()
	body := []byte(`{"model":"gpt-5.5","input":"apply the patch","tools":[
		{"type":"custom","name":"apply_patch","format":{"type":"grammar","syntax":"lark","definition":"start: /.+/"}}
	]}`)
	llmRequest, err := responsesInbound.TransformRequest(t.Context(), &httpclient.Request{Body: body})
	require.NoError(t, err)
	chatOutbound, err := NewOutboundTransformer("https://paratera.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(t.Context(), llmRequest)
	require.NoError(t, err)
	require.NotNil(t, chatRequest)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Tools, 1)
	require.Equal(t, "apply_patch", converted.Tools[0].Function.Name)
	require.Contains(t, converted.Tools[0].Function.Description, "lark grammar:")
	require.Contains(t, converted.Tools[0].Function.Description, "start: /.+/")

	warnings, ok := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
	require.True(t, ok)
	require.Contains(t, warnings, `grammar_constraint_degraded: custom tool "apply_patch" grammar is advisory after conversion to Chat Completions`)
}

func TestResponsesToChatTools_RejectNamespaceDeferLoading(t *testing.T) {
	responsesInbound := responsesapi.NewInboundTransformer()
	body := []byte(`{"model":"gpt-5.5","input":"run","tools":[{
		"type":"namespace","name":"functions","tools":[
			{"type":"function","name":"wait","parameters":{"type":"object"},"defer_loading":true}
		]}]}`)
	llmRequest, err := responsesInbound.TransformRequest(t.Context(), &httpclient.Request{Body: body})
	require.NoError(t, err)
	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(t.Context(), llmRequest)
	require.Nil(t, chatRequest)
	require.ErrorContains(t, err, `function tool "functions__wait" uses defer_loading`)
}

func TestResponsesToChatTools_DegradesNamespaceCustomFormat(t *testing.T) {
	responsesInbound := responsesapi.NewInboundTransformer()
	body := []byte(`{"model":"gpt-5.5","input":"run","tools":[{
		"type":"namespace","name":"functions","tools":[
			{"type":"custom","name":"exec","format":{"type":"grammar","syntax":"lark","definition":"start: /.+/"}}
		]}]}`)
	llmRequest, err := responsesInbound.TransformRequest(t.Context(), &httpclient.Request{Body: body})
	require.NoError(t, err)
	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(t.Context(), llmRequest)
	require.NoError(t, err)
	require.NotNil(t, chatRequest)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Len(t, converted.Tools, 1)
	require.Equal(t, "functions__exec", converted.Tools[0].Function.Name)
	require.Contains(t, converted.Tools[0].Function.Description, "start: /.+/")

	warnings, ok := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
	require.True(t, ok)
	require.Contains(t, warnings, `grammar_constraint_degraded: custom tool "exec" grammar is advisory after conversion to Chat Completions`)
}

func responsesChatToolDelta(toolCall map[string]any) map[string]any {
	return map[string]any{
		"index": 0,
		"delta": map[string]any{"tool_calls": []any{toolCall}},
	}
}

// runResponsesChatStream drives the full Responses-to-Chat streaming round trip:
// it transforms the Responses request, lets build construct chat chunk choices
// from the converted Chat Completions request, wraps each choice in a
// chat.completion.chunk envelope terminated by [DONE], transforms the stream
// back through both transformers, and drains the resulting Responses events.
func runResponsesChatStream(
	t *testing.T,
	requestBody string,
	build func(converted *Request) []map[string]any,
) ([]responsesapi.StreamEvent, error) {
	t.Helper()
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(requestBody)})
	require.NoError(t, err)
	chatOutbound, err := NewOutboundTransformer("https://paratera.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)
	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))

	choices := build(&converted)
	providerEvents := make([]*httpclient.StreamEvent, 0, len(choices)+1)
	for _, choice := range choices {
		providerEvents = append(providerEvents, &httpclient.StreamEvent{Data: marshalResponsesChatTestJSON(t, map[string]any{
			"id": "chatcmpl_stream_test", "object": "chat.completion.chunk", "created": 1, "model": "glm-5.2",
			"choices": []any{choice},
		})})
	}
	providerEvents = append(providerEvents, &httpclient.StreamEvent{Data: []byte("[DONE]")})
	llmStream, err := chatOutbound.TransformStream(ctx, chatRequest, streams.SliceStream(providerEvents))
	require.NoError(t, err)
	responsesStream, err := responsesInbound.TransformStream(ctx, llmStream)
	require.NoError(t, err)

	var events []responsesapi.StreamEvent
	for responsesStream.Next() {
		var event responsesapi.StreamEvent
		require.NoError(t, json.Unmarshal(responsesStream.Current().Data, &event))
		events = append(events, event)
	}
	return events, responsesStream.Err()
}

const responsesChatCustomToolRequestBody = `{
	"model":"gpt-5.5","stream":true,"input":"apply the patch","tools":[
		{"type":"custom","name":"apply_patch","description":"Apply unified patch"}
	]
}`

func simulateResponsesChatStream(t *testing.T, requestBody string, choices []map[string]any) ([]responsesapi.StreamEvent, error) {
	t.Helper()
	return runResponsesChatStream(t, requestBody, func(*Request) []map[string]any { return choices })
}

func simulateResponsesChatCustomChoices(
	t *testing.T,
	build func(customName string) []map[string]any,
) ([]responsesapi.StreamEvent, error) {
	t.Helper()
	return runResponsesChatStream(t, responsesChatCustomToolRequestBody, func(converted *Request) []map[string]any {
		require.Len(t, converted.Tools, 1)
		choices := build(converted.Tools[0].Function.Name)
		return append(choices, map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"})
	})
}

func simulateResponsesChatCustomStream(t *testing.T, argumentFragments []string) ([]responsesapi.StreamEvent, error) {
	t.Helper()
	return runResponsesChatStream(t, responsesChatCustomToolRequestBody, func(converted *Request) []map[string]any {
		require.Len(t, converted.Tools, 1)
		choices := make([]map[string]any, 0, len(argumentFragments)+1)
		for i, fragment := range argumentFragments {
			toolCall := map[string]any{"index": 0, "function": map[string]any{"arguments": fragment}}
			if i == 0 {
				toolCall["id"] = "call_patch_edge"
				toolCall["type"] = "function"
				toolCall["function"].(map[string]any)["name"] = converted.Tools[0].Function.Name
			}
			choices = append(choices, responsesChatToolDelta(toolCall))
		}
		return append(choices, map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"})
	})
}

func completedCustomInput(t *testing.T, events []responsesapi.StreamEvent) string {
	t.Helper()
	for i := range events {
		event := events[i]
		if event.Type == responsesapi.StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "custom_tool_call" {
			require.NotNil(t, event.Item.Input)
			return *event.Item.Input
		}
	}
	t.Fatal("custom_tool_call output_item.done not found")
	return ""
}

func marshalResponsesChatTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
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
	require.Contains(t, logs.String(), "Responses request degraded during Chat Completions conversion")
	require.Contains(t, logs.String(), `"model":"gpt-5.5"`)
	require.Contains(t, logs.String(), "unsupported_tool_type")
}

func TestResponsesToChatTools_UnsupportedNamedChoiceReturnsBadRequest(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"run hosted search",
		"tools":[{"type":"tool_search","execution":"server"}],
		"tool_choice":{"type":"tool_search","name":"tool_search"}
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	_, err = chatOutbound.TransformRequest(ctx, llmRequest)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.ErrorContains(t, err, "unsupported_tool_choice")

	httpErr := responsesInbound.TransformError(ctx, err)
	require.Equal(t, http.StatusBadRequest, httpErr.StatusCode)
	var responseErr responsesapi.ResponseError
	require.NoError(t, json.Unmarshal(httpErr.Body, &responseErr))
	require.Equal(t, "invalid_request_error", responseErr.Error.Type)
	require.Contains(t, responseErr.Error.Message, "unsupported_tool_choice")
}

func TestResponsesToChatTools_WarnsWhenRawToolSelectorDegradesToAuto(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"search",
		"tools":[{
			"type":"tool_search",
			"execution":"client",
			"parameters":{"type":"object"}
		}],
		"tool_choice":{
			"type":"tool_search",
			"tools":[{"type":"tool_search","name":"search_docs"}]
		}
	}`)})
	require.NoError(t, err)
	require.NotNil(t, llmRequest.ProviderExtensions)
	require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses.Request)
	require.JSONEq(t, `{
		"type":"tool_search",
		"tools":[{"type":"tool_search","name":"search_docs"}]
	}`, string(llmRequest.ProviderExtensions.OpenAIResponses.Request.RawToolChoice))

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Nil(t, converted.ToolChoice)
	require.Len(t, converted.Tools, 1)
	require.Equal(t, "tool_search", converted.Tools[0].Function.Name)
	warnings, ok := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
	require.True(t, ok)
	require.Contains(t, warnings,
		"unsupported_tool_choice_degraded: tool_search selector cannot be represented in Chat Completions; using auto")
}

func TestResponsesToChatTools_DegradesEveryUnrepresentedRawSelector(t *testing.T) {
	tests := []struct {
		name            string
		tools           string
		toolChoice      string
		selectorType    string
		expectedToolLen int
	}{
		{
			name: "type only web search", tools: `[{"type":"web_search"}]`,
			toolChoice: `{"type":"web_search"}`, selectorType: "web_search",
		},
		{
			name: "type only image generation", tools: `[]`,
			toolChoice: `{"type":"image_generation"}`, selectorType: "image_generation",
		},
		{
			name: "future selector policy", tools: `[]`,
			toolChoice: `{"type":"future_selector","policy":"strict"}`, selectorType: "future_selector",
		},
		{
			name:       "mcp selector identity",
			tools:      `[{"type":"mcp","server_label":"docs","server_url":"https://example.com/mcp"}]`,
			toolChoice: `{"type":"mcp","server_label":"docs","name":"search"}`, selectorType: "mcp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			responsesInbound := responsesapi.NewInboundTransformer()
			body := []byte(`{"model":"gpt-5.5","input":"use tools","tools":` + tc.tools + `,"tool_choice":` + tc.toolChoice + `}`)
			llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: body})
			require.NoError(t, err)

			chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
			require.NoError(t, err)
			chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
			require.NoError(t, err)

			var converted Request
			require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
			require.Nil(t, converted.ToolChoice)
			require.Len(t, converted.Tools, tc.expectedToolLen)
			warnings, ok := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
			require.True(t, ok)
			require.Contains(t, warnings,
				"unsupported_tool_choice_degraded: "+tc.selectorType+" selector cannot be represented in Chat Completions; using auto")
		})
	}
}

func TestResponsesToChatTools_DoesNotDegradeRepresentedNamedSelector(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"lookup",
		"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],
		"tool_choice":{"type":"function","name":"lookup"}
	}`)})
	require.NoError(t, err)
	if llmRequest.ProviderExtensions != nil && llmRequest.ProviderExtensions.OpenAIResponses != nil &&
		llmRequest.ProviderExtensions.OpenAIResponses.Request != nil {
		require.Empty(t, llmRequest.ProviderExtensions.OpenAIResponses.Request.RawToolChoice)
	}

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.NotNil(t, converted.ToolChoice)
	require.NotNil(t, converted.ToolChoice.NamedToolChoice)
	require.Equal(t, "lookup", converted.ToolChoice.NamedToolChoice.Function.Name)
	warnings, _ := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
	for _, warning := range warnings {
		require.NotContains(t, warning, "unsupported_tool_choice_degraded")
	}
}

func TestResponsesToChatTools_StaleRawSelectorDoesNotOverrideCurrentChoice(t *testing.T) {
	tests := []struct {
		name         string
		currentMode  *string
		clear        bool
		expectedMode *string
	}{
		{name: "cleared selector", clear: true},
		{name: "replaced with none", currentMode: lo.ToPtr("none"), expectedMode: lo.ToPtr("none")},
		{name: "named replaced with mode", currentMode: lo.ToPtr("auto"), expectedMode: lo.ToPtr("auto")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			responsesInbound := responsesapi.NewInboundTransformer()
			llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
				"model":"gpt-5.5",
				"input":"lookup",
				"tools":[
					{"type":"function","name":"lookup","parameters":{"type":"object"}},
					{"type":"mcp","server_label":"docs","server_url":"https://example.com/mcp"}
				],
				"tool_choice":{"type":"mcp","server_label":"docs","name":"search"}
			}`)})
			require.NoError(t, err)
			require.NotNil(t, llmRequest.ProviderExtensions)
			require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses)
			require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses.Request)
			require.NotEmpty(t, llmRequest.ProviderExtensions.OpenAIResponses.Request.RawToolChoice)

			if tc.clear {
				llmRequest.ToolChoice = nil
			} else {
				llmRequest.ToolChoice = &llm.ToolChoice{ToolChoice: tc.currentMode}
			}
			chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
			require.NoError(t, err)
			chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
			require.NoError(t, err)

			var converted Request
			require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
			if tc.expectedMode == nil {
				require.Nil(t, converted.ToolChoice)
			} else {
				require.NotNil(t, converted.ToolChoice)
				require.Equal(t, *tc.expectedMode, lo.FromPtr(converted.ToolChoice.ToolChoice))
			}
			warnings, _ := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey].([]string)
			for _, warning := range warnings {
				require.NotContains(t, warning, "unsupported_tool_choice_degraded")
			}
		})
	}
}

func TestChatTools_FiltersEstablishedNonChatToolsWithoutCompatibilityWarning(t *testing.T) {
	ctx := context.Background()
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	prompt := "hello"
	chatRequest, err := chatOutbound.TransformRequest(ctx, &llm.Request{
		Model: "chat-model", Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: &prompt}}},
		Tools: []llm.Tool{
			{Type: llm.ToolTypeImageGeneration},
			{Type: llm.ToolTypeWebSearch},
			{Type: llm.ToolTypeGoogleSearch},
			{Type: llm.ToolTypeGoogleCodeExecution},
			{Type: llm.ToolTypeGoogleUrlContext},
		},
	})
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))
	require.Empty(t, converted.Tools)
	_, warned := chatRequest.TransformerMetadata[responsesChatToolWarningsMetadataKey]
	require.False(t, warned)
	require.NotContains(t, logs.String(), "Responses request degraded during Chat Completions conversion")
}

// TestResponsesToChatHistory_FlattensMultipartToolOutputToString guards against
// a 400 from Chat Completions providers: the tool role requires string content,
// but a Responses function_call_output with several text items produced an array.
func TestResponsesToChatHistory_FlattensMultipartToolOutputToString(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"role":"user","type":"message","content":[{"type":"input_text","text":"run it"}]},
			{"type":"function_call","call_id":"call_1","name":"exec","arguments":"{\"input\":\"x\"}"},
			{"type":"function_call_output","call_id":"call_1","output":[
				{"type":"output_text","text":"Script failed\nWall time 0.0 seconds\nOutput:\n"},
				{"type":"output_text","text":"Script error:\nSyntaxError: Unexpected token ':'"}
			]}
		],
		"tools":[
			{"type":"function","name":"exec","parameters":{"type":"object","properties":{"input":{"type":"string"}}}}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))

	toolMsg, ok := lo.Find(converted.Messages, func(m Message) bool { return m.Role == "tool" })
	require.True(t, ok)
	require.Empty(t, toolMsg.Content.MultipleContent, "tool content must not be an array")
	require.Equal(t,
		"Script failed\nWall time 0.0 seconds\nOutput:\nScript error:\nSyntaxError: Unexpected token ':'",
		lo.FromPtr(toolMsg.Content.Content))

	// The serialized body must carry the tool content as a JSON string.
	var raw struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(chatRequest.Body, &raw))
	rawTool, ok := lo.Find(raw.Messages, func(m struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	},
	) bool {
		return m.Role == "tool"
	})
	require.True(t, ok)
	require.True(t, strings.HasPrefix(strings.TrimSpace(string(rawTool.Content)), `"`),
		"tool content should serialize as a string, got: %s", string(rawTool.Content))
}

// TestResponsesToChatHistory_FlattensCustomToolOutput mirrors the Codex exec
// custom tool, whose outputs arrive as custom_tool_call_output with several
// text items and must land in the Chat tool message as a single string.
func TestResponsesToChatHistory_FlattensCustomToolOutput(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"role":"user","type":"message","content":[{"type":"input_text","text":"run"}]},
			{"type":"custom_tool_call","call_id":"call_exec","name":"exec","input":"await tools.x()"},
			{"type":"custom_tool_call_output","call_id":"call_exec","name":"exec","output":[
				{"type":"output_text","text":"Script completed\nWall time 1.5 seconds\nOutput:\n"},
				{"type":"output_text","text":"/repo\nfile.txt\n"}
			]}
		],
		"tools":[
			{"type":"custom","name":"exec","description":"Run JS"}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))

	toolMsg, ok := lo.Find(converted.Messages, func(m Message) bool { return m.Role == "tool" })
	require.True(t, ok)
	require.Empty(t, toolMsg.Content.MultipleContent, "custom tool output must not be an array")
	require.Equal(t,
		"Script completed\nWall time 1.5 seconds\nOutput:\n/repo\nfile.txt\n",
		lo.FromPtr(toolMsg.Content.Content))
}

func TestResponsesToChatHistory_SanitizesEmptyToolSearchArguments(t *testing.T) {
	ctx := context.Background()
	responsesInbound := responsesapi.NewInboundTransformer()
	llmRequest, err := responsesInbound.TransformRequest(ctx, &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"tool_search_call","call_id":"call_search","arguments":{}},
			{"type":"tool_search_call","call_id":"call_search_bad"},
			{"type":"function_call_output","call_id":"call_search","output":"done"},
			{"type":"function_call_output","call_id":"call_search_bad","output":"done"},
			{"role":"user","type":"message","content":[{"type":"input_text","text":"go"}]}
		],
		"tools":[
			{"type":"tool_search","execution":"client","parameters":{"type":"object"}}
		]
	}`)})
	require.NoError(t, err)

	chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
	require.NoError(t, err)
	chatRequest, err := chatOutbound.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)

	var converted Request
	require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))

	var assistantMsg *Message
	for i := range converted.Messages {
		if converted.Messages[i].Role == "assistant" && len(converted.Messages[i].ToolCalls) > 0 {
			assistantMsg = &converted.Messages[i]
			break
		}
	}
	require.NotNil(t, assistantMsg, "expected an assistant message with tool calls")
	require.Len(t, assistantMsg.ToolCalls, 2)
	for _, call := range assistantMsg.ToolCalls {
		require.Equal(t, "{}", call.Function.Arguments, "empty tool_search arguments must be normalized to {}")
	}
}

// When a same-named plain tool wins the name reservation, the client tool
// definition is renamed. Historical calls must resolve through the same
// mapping as the definitions: if the original name no longer matches any
// emitted Chat tool, the call must follow the renamed definition; if the
// plain twin is still emitted, the original name already matches it.
func TestResponsesToChatHistory_ClientToolCallFollowsRenamedDefinition(t *testing.T) {
	const input = `[
		{"role":"user","type":"message","content":[{"type":"input_text","text":"run"}]},
		{"type":"function_call","name":"lookup","call_id":"call_lookup","arguments":"{\"id\":\"42\"}"},
		{"type":"function_call_output","call_id":"call_lookup","output":"ok"},
		{"role":"user","type":"message","content":[{"type":"input_text","text":"again"}]}
	]`

	tests := []struct {
		name         string
		toolChoice   string
		expectedTool string
		expectedCall string
	}{
		{
			name:         "filtered plain twin forces renamed definition",
			toolChoice:   `{"type":"allowed_tools","mode":"auto","tools":[{"type":"future_client_tool","name":"lookup"}]}`,
			expectedTool: "axonhub_client_tool_1",
			expectedCall: "axonhub_client_tool_1",
		},
		{
			name:         "coexisting plain twin keeps original call name",
			toolChoice:   "",
			expectedTool: "lookup",
			expectedCall: "lookup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolChoice := ""
			if tt.toolChoice != "" {
				toolChoice = `,"tool_choice":` + tt.toolChoice
			}
			body := `{
		"model":"gpt-5.5",
		"input":` + input + `,
		"tools":[
			{"type":"function","name":"lookup","parameters":{"type":"object","properties":{"id":{"type":"string"}}}},
			{"type":"future_client_tool","name":"lookup","execution":"client","parameters":{"type":"object","properties":{"id":{"type":"string"}}}}
		]` + toolChoice + `
	}`

			responsesInbound := responsesapi.NewInboundTransformer()
			llmRequest, err := responsesInbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
			require.NoError(t, err)

			chatOutbound, err := NewOutboundTransformer("https://chat.example.com", "test-key")
			require.NoError(t, err)
			chatRequest, err := chatOutbound.TransformRequest(context.Background(), llmRequest)
			require.NoError(t, err)

			var converted Request
			require.NoError(t, json.Unmarshal(chatRequest.Body, &converted))

			definitionNames := make(map[string]struct{}, len(converted.Tools))
			for _, tool := range converted.Tools {
				definitionNames[tool.Function.Name] = struct{}{}
			}
			require.Contains(t, definitionNames, tt.expectedTool)

			var assistant *Message
			for i := range converted.Messages {
				if converted.Messages[i].Role == "assistant" && len(converted.Messages[i].ToolCalls) > 0 {
					assistant = &converted.Messages[i]
					break
				}
			}
			require.NotNil(t, assistant)
			require.Len(t, assistant.ToolCalls, 1)
			require.Equal(t, tt.expectedCall, assistant.ToolCalls[0].Function.Name)
			_, defined := definitionNames[assistant.ToolCalls[0].Function.Name]
			require.True(t, defined, "history call name must match a converted definition")
		})
	}
}

func TestResponsesToChatTools_NamespaceDedupDeferLoadingAndInvalid(t *testing.T) {
	tools := []llm.Tool{
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "ws__read", Namespace: "ws", Parameters: json.RawMessage(`{"type":"object"}`)}},
		// Identical duplicate: deduplicated silently.
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "ws__read", Namespace: "ws", Parameters: json.RawMessage(`{"type":"object"}`)}},
		// Conflicting duplicate: strict identity mapping must fail.
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "ws__read", Namespace: "ws", Description: "other", Parameters: json.RawMessage(`{"type":"object"}`)}},
		// Invalid schema: dropped with a warning.
		{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "ws__bad", Namespace: "ws", Parameters: json.RawMessage(`{"type":"string"}`)}},
	}

	adapter := newResponsesChatToolAdapter(tools)
	result := adapter.convertTools(tools)

	names := make([]string, 0, len(result))
	for _, tool := range result {
		names = append(names, tool.Function.Name)
	}
	require.Equal(t, []string{"ws__read"}, names)
	require.ErrorContains(t, adapter.err, "tool_definition_conflict: namespace ws.read has multiple different definitions")

	warnings := strings.Join(adapter.warnings, "\n")
	require.Contains(t, warnings, "invalid namespace tool")
}

func TestResponsesChatAllowedToolChoiceSignaturesAndDegradation(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		allowedTools  []llm.ToolOption
		unsupported   bool
		wantSignature string
	}{
		{
			name:          "mode only",
			raw:           `{"type":"allowed_tools","mode":"auto"}`,
			unsupported:   true,
			wantSignature: `allowed:{"mode":"auto","tools":[]}`,
		},
		{
			name:          "full tools",
			raw:           `{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"lookup"}]}`,
			allowedTools:  []llm.ToolOption{{Type: "function", Name: "lookup"}},
			unsupported:   false,
			wantSignature: `allowed:{"mode":"auto","tools":[{"type":"function","name":"lookup"}]}`,
		},
		{
			name:          "unknown field",
			raw:           `{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"lookup"}],"future_option":{"mode":"lossless"}}`,
			allowedTools:  []llm.ToolOption{{Type: "function", Name: "lookup"}},
			unsupported:   true,
			wantSignature: `allowed:{"mode":"auto","tools":[{"type":"function","name":"lookup"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selectorType, unsupported := unsupportedRawChatToolSelector(json.RawMessage(tt.raw))
			require.Equal(t, "allowed_tools", selectorType)
			require.Equal(t, tt.unsupported, unsupported)

			signature, ok := rawChatToolChoiceSemanticSignature(json.RawMessage(tt.raw))
			require.True(t, ok)
			require.Equal(t, tt.wantSignature, signature)

			// The current llm.ToolChoice decoded from the same selector shares
			// the signature, so the degradation decision rests on unsupported.
			currentSignature, ok := llmToolChoiceSignature(&llm.ToolChoice{
				ToolChoice: lo.ToPtr("auto"), AllowedTools: tt.allowedTools, AllowedToolsSet: true,
			})
			require.True(t, ok)
			require.Equal(t, tt.wantSignature, currentSignature)
		})
	}
}
