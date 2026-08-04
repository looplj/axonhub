package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRequestExtensions_ReplaysNamespaceWithFutureClientSubtool(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[{
			"type":"namespace",
			"name":"workspace",
			"tools":[
				{"type":"function","name":"read","parameters":{"type":"object"},"defer_loading":true},
				{
					"type":"future_client_tool",
					"name":"later",
					"execution":"client",
					"parameters":{"type":"object"},
					"defer_loading":true,
					"future_option":{"mode":"lossless"}
				}
			]
		}]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	require.True(t, llmRequest.Tools[0].Function.DeferLoading)
	require.True(t, llmRequest.Tools[1].Function.DeferLoading)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Tools []struct {
				Type         string         `json:"type"`
				Name         string         `json:"name"`
				DeferLoading bool           `json:"defer_loading"`
				FutureOption map[string]any `json:"future_option"`
			} `json:"tools"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "namespace", payload.Tools[0].Type)
	require.Equal(t, "workspace", payload.Tools[0].Name)
	require.Len(t, payload.Tools[0].Tools, 2)
	require.Equal(t, "function", payload.Tools[0].Tools[0].Type)
	require.Equal(t, "read", payload.Tools[0].Tools[0].Name)
	require.True(t, payload.Tools[0].Tools[0].DeferLoading)
	require.Equal(t, "future_client_tool", payload.Tools[0].Tools[1].Type)
	require.Equal(t, "later", payload.Tools[0].Tools[1].Name)
	require.True(t, payload.Tools[0].Tools[1].DeferLoading)
	require.Equal(t, "lossless", payload.Tools[0].Tools[1].FutureOption["mode"])
}

func TestRequestExtensions_NamespaceOpaqueIdentityDoesNotReviveRemovedTopLevelTwin(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"namespace","name":"workspace","tools":[
				{"type":"future_server_tool","name":"same","slot":"namespace"}
			]},
			{"type":"future_server_tool","name":"same","slot":"top-level"}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	require.Equal(t, "workspace", llmRequest.Tools[0].ResponseOpaqueTool.Namespace)
	require.Empty(t, llmRequest.Tools[1].ResponseOpaqueTool.Namespace)

	llmRequest.Tools = llmRequest.Tools[:1]
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []map[string]any `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "namespace", payload.Tools[0]["type"])
	require.Equal(t, "workspace", payload.Tools[0]["name"])
	require.NotContains(t, string(httpRequest.Body), "top-level")
}

func TestRequestExtensions_PartialNamespaceRemovalKeepsRemainingOrder(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"namespace","name":"workspace","tools":[
				{"type":"function","name":"keep","parameters":{"type":"object"}},
				{"type":"future_server_tool","name":"removed","slot":"namespace"}
			]},
			{"type":"function","name":"mid","parameters":{"type":"object"}},
			{"type":"future_server_tool","name":"tail","slot":"tail"}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 4)
	llmRequest.Tools = append(llmRequest.Tools[:1], llmRequest.Tools[2:]...)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []map[string]any `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 3)
	require.Equal(t, "workspace__keep", payload.Tools[0]["name"])
	require.Equal(t, "mid", payload.Tools[1]["name"])
	require.Equal(t, "tail", payload.Tools[2]["name"])
	require.NotContains(t, string(httpRequest.Body), "removed")
}

func TestRequestExtensions_ModifiedRawFunctionLikeToolsUseCurrentSchema(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		toolIndex      int
		expectedName   string
		expectedTopLen int
	}{
		{
			name: "namespace future client",
			body: `{"model":"gpt-5.5","input":"use tools","tools":[{
				"type":"namespace","name":"workspace","tools":[{
					"type":"future_client_tool","name":"later","execution":"client",
					"parameters":{"type":"object","properties":{"old":{"type":"string"}}},
					"future_option":{"mode":"raw"}
				}]}]}`,
			expectedName:   "workspace__later",
			expectedTopLen: 1,
		},
		{
			name: "top-level future client",
			body: `{"model":"gpt-5.5","input":"use tools","tools":[{
				"type":"future_client_tool","name":"later","execution":"client",
				"parameters":{"type":"object","properties":{"old":{"type":"string"}}},
				"future_option":{"mode":"raw"}
			}]}`,
			expectedName:   "later",
			expectedTopLen: 1,
		},
		{
			name: "developer additional tool",
			body: `{"model":"gpt-5.5","input":[
				{"type":"additional_tools","role":"developer","tools":[{
					"type":"future_client_tool","name":"later","execution":"client",
					"parameters":{"type":"object","properties":{"old":{"type":"string"}}},
					"future_option":{"mode":"raw"}
				}]},
				{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
			]}`,
			expectedName:   "later",
			expectedTopLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound := NewInboundTransformer()
			llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(tt.body)})
			require.NoError(t, err)
			require.Greater(t, len(llmRequest.Tools), tt.toolIndex)
			llmRequest.Tools[tt.toolIndex].Function.Parameters = json.RawMessage(`{
				"type":"object","properties":{"current":{"type":"integer"}}
			}`)

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)
			httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
			require.NoError(t, err)

			var payload struct {
				Tools []Tool          `json:"tools"`
				Input json.RawMessage `json:"input"`
			}
			require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
			require.Len(t, payload.Tools, tt.expectedTopLen)
			require.Equal(t, "function", payload.Tools[0].Type)
			require.Equal(t, tt.expectedName, payload.Tools[0].Name)
			require.Contains(t, payload.Tools[0].Parameters["properties"], "current")
			require.NotContains(t, payload.Tools[0].Parameters["properties"], "old")
			require.NotContains(t, string(httpRequest.Body), "future_option")
			var inputItems []map[string]any
			if json.Unmarshal(payload.Input, &inputItems) == nil {
				for _, item := range inputItems {
					require.NotEqual(t, "additional_tools", item["type"])
				}
			}
		})
	}
}

func TestRequestExtensions_RemovedToolSearchLifecycleDoesNotReplayRawItemsOrConsumeUser(t *testing.T) {
	const body = `{
		"model":"gpt-5.5",
		"input":[
			{"type":"tool_search_call","call_id":"search_1","execution":"client","arguments":{"query":"agents"},"raw_marker":"call"},
			{"type":"tool_search_output","call_id":"search_1","execution":"client","tools":[
				{"type":"function","name":"spawn","parameters":{"type":"object"}}
			],"raw_marker":"output"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"keep me"}]}
		]
	}`

	tests := []struct {
		name       string
		keepRole   string
		absentType string
	}{
		{name: "remove call", keepRole: "tool", absentType: "tool_search_call"},
		{name: "remove output", keepRole: "assistant", absentType: "tool_search_output"},
		{name: "remove both", keepRole: "", absentType: "tool_search_call"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound := NewInboundTransformer()
			llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
			require.NoError(t, err)

			kept := llmRequest.Messages[:0]
			for _, message := range llmRequest.Messages {
				if message.Role == "user" || (tt.keepRole != "" && message.Role == tt.keepRole) {
					kept = append(kept, message)
				}
			}
			llmRequest.Messages = kept
			if tt.keepRole == "" {
				llmRequest.Tools = nil
			}

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)
			httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
			require.NoError(t, err)

			var payload struct {
				Input []map[string]any `json:"input"`
			}
			require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
			require.NotContains(t, string(httpRequest.Body), "raw_marker")
			for _, item := range payload.Input {
				require.NotEqual(t, tt.absentType, item["type"])
			}
			require.Contains(t, string(httpRequest.Body), "keep me")
		})
	}
}

func TestRequestExtensions_FunctionDeferLoadingRoundTripsStructurally(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"discover later",
		"tools":[{
			"type":"function",
			"name":"deferred_lookup",
			"description":"Loaded through tool search",
			"parameters":{"type":"object"},
			"defer_loading":true
		}]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 1)
	require.True(t, llmRequest.Tools[0].Function.DeferLoading)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []Tool `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "function", payload.Tools[0].Type)
	require.Equal(t, "deferred_lookup", payload.Tools[0].Name)
	require.True(t, payload.Tools[0].DeferLoading)
}

func TestRequestExtensions_DoesNotReplayRemovedRawOnlyTool(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"function","name":"lookup","parameters":{"type":"object"}},
			{"type":"future_server_tool","name":"hosted","future_option":{"mode":"lossless"}}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	llmRequest.Tools = llmRequest.Tools[:1]

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []Tool `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "function", payload.Tools[0].Type)
	require.Equal(t, "lookup", payload.Tools[0].Name)
}

func TestRequestExtensions_ReplaysOnlyRemainingDuplicateRawTool(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"future_server_tool","name":"hosted","slot":1},
			{"type":"future_server_tool","name":"hosted","slot":2}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	llmRequest.Tools = llmRequest.Tools[:1]

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []map[string]any `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "future_server_tool", payload.Tools[0]["type"])
	require.Equal(t, float64(1), payload.Tools[0]["slot"])
}

func TestRequestExtensions_ReplaysTheRetainedDuplicateRawToolInstance(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"future_server_tool","name":"hosted","slot":1},
			{"type":"future_server_tool","name":"hosted","slot":2}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	llmRequest.Tools = llmRequest.Tools[1:]

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []map[string]any `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "future_server_tool", payload.Tools[0]["type"])
	require.Equal(t, float64(2), payload.Tools[0]["slot"])
}

func TestRequestExtensions_DoesNotMatchOneNamespaceAcrossLaterRawGroups(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"namespace","name":"n","group":"first","tools":[
				{"type":"future_server_tool","name":"a"},
				{"type":"future_server_tool","name":"b"}
			]},
			{"type":"namespace","name":"n","group":"second","tools":[
				{"type":"future_server_tool","name":"a"}
			]},
			{"type":"namespace","name":"n","group":"third","tools":[
				{"type":"future_server_tool","name":"b"}
			]}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 4)
	llmRequest.Tools = llmRequest.Tools[2:]

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []map[string]any `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 2)
	require.Equal(t, "second", payload.Tools[0]["group"])
	require.Equal(t, "third", payload.Tools[1]["group"])
	require.NotContains(t, string(httpRequest.Body), `"group":"first"`)
}

func TestRequestExtensions_ToolSearchOutputUsesCurrentPromotedDefinitions(t *testing.T) {
	const body = `{
		"model":"gpt-5.5",
		"input":[
			{"type":"tool_search_call","call_id":"search_1","execution":"client","arguments":{"query":"agents"}},
			{"type":"tool_search_output","call_id":"search_1","execution":"client","tools":[
				{"type":"function","name":"spawn","parameters":{"type":"object","properties":{"old":{"type":"string"}}}}
			]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
		]
	}`

	tests := []struct {
		name          string
		mutate        func([]llm.Tool) []llm.Tool
		expectedTools int
	}{
		{
			name: "deleted definition",
			mutate: func([]llm.Tool) []llm.Tool {
				return nil
			},
		},
		{
			name: "modified definition",
			mutate: func(tools []llm.Tool) []llm.Tool {
				tools[0].Function.Parameters = json.RawMessage(`{
					"type":"object","properties":{"current":{"type":"integer"}}
				}`)
				return tools
			},
			expectedTools: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound := NewInboundTransformer()
			llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
			require.NoError(t, err)
			require.Len(t, llmRequest.Tools, 1)
			llmRequest.Tools = tt.mutate(llmRequest.Tools)

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)
			httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
			require.NoError(t, err)

			var payload struct {
				Tools []Tool `json:"tools"`
				Input []Item `json:"input"`
			}
			require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
			require.Empty(t, payload.Tools)
			var output *Item
			for i := range payload.Input {
				if payload.Input[i].Type == "tool_search_output" {
					output = &payload.Input[i]
					break
				}
			}
			require.NotNil(t, output)
			require.Len(t, output.Tools, tt.expectedTools)
			require.NotContains(t, string(httpRequest.Body), `"old"`)
			if tt.expectedTools > 0 {
				require.Contains(t, output.Tools[0].Parameters["properties"], "current")
			}
		})
	}
}

func TestRequestExtensions_ReusedToolSearchCallIDKeepsOutputOccurrencesSeparate(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"tool_search_call","call_id":"reused","execution":"client","arguments":{"query":"first"}},
			{"type":"tool_search_output","call_id":"reused","execution":"client","tools":[
				{"type":"function","name":"first","parameters":{"type":"object","properties":{"first":{"type":"string"}}}}
			]},
			{"type":"tool_search_call","call_id":"reused","execution":"client","arguments":{"query":"second"}},
			{"type":"tool_search_output","call_id":"reused","execution":"client","tools":[
				{"type":"function","name":"second","parameters":{"type":"object","properties":{"old":{"type":"string"}}}}
			]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	llmRequest.Tools[1].Function.Parameters = json.RawMessage(`{
		"type":"object","properties":{"current":{"type":"integer"}}
	}`)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Input []Item `json:"input"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	outputs := make([]Item, 0, 2)
	for _, item := range payload.Input {
		if item.Type == "tool_search_output" {
			outputs = append(outputs, item)
		}
	}
	require.Len(t, outputs, 2)
	require.Len(t, outputs[0].Tools, 1)
	require.Equal(t, "first", outputs[0].Tools[0].Name)
	require.Contains(t, outputs[0].Tools[0].Parameters["properties"], "first")
	require.Len(t, outputs[1].Tools, 1)
	require.Equal(t, "second", outputs[1].Tools[0].Name)
	require.Contains(t, outputs[1].Tools[0].Parameters["properties"], "current")
	require.NotContains(t, string(httpRequest.Body), `"old"`)
}

func TestRequestExtensions_DoesNotReplayRemovedAdditionalToolsItem(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"additional_tools","role":"developer","tools":[
				{"type":"function","name":"deferred_lookup","parameters":{"type":"object"}},
				{"type":"future_client_tool","name":"later","execution":"client","parameters":{"type":"object"}}
			]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	llmRequest.Tools = nil

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Input []Item `json:"input"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Input, 1)
	require.Equal(t, "message", payload.Input[0].Type)
	require.Equal(t, "user", payload.Input[0].Role)
}

func TestRawAllowedToolChoiceRequiresExactAllowlist(t *testing.T) {
	raw := json.RawMessage(`{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"first"}]}`)
	allowedType := "allowed_tools"
	auto := "auto"
	require.True(t, rawToolChoiceMatchesCurrentTools(raw, &ToolChoice{
		Type: &allowedType, Mode: &auto,
		Tools: []ToolOption{{Type: "function", Name: "first"}},
	}))
	require.False(t, rawToolChoiceMatchesCurrentTools(raw, &ToolChoice{
		Type: &allowedType, Mode: &auto,
		Tools: []ToolOption{{Type: "function", Name: "second"}},
	}))
}

func TestRawToolChoiceRequiresCurrentSemanticMatch(t *testing.T) {
	raw := json.RawMessage(`{"type":"web_search","future_option":"keep"}`)

	require.False(t, rawToolChoiceMatchesCurrentTools(raw, nil))
	require.True(t, rawToolChoiceMatchesCurrentTools(raw, &ToolChoice{}))

	none := "none"
	require.False(t, rawToolChoiceMatchesCurrentTools(raw, &ToolChoice{Mode: &none}))
}

func TestRequestExtensions_ReplaysEmptyAllowedToolChoice(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"run",
		"tool_choice":{
			"type":"allowed_tools",
			"mode":"auto",
			"tools":[],
			"future_option":{"mode":"keep"}
		}
	}`)})
	require.NoError(t, err)
	require.NotNil(t, llmRequest.ProviderExtensions)
	require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses.Request)
	require.JSONEq(t, `{
		"type":"allowed_tools",
		"mode":"auto",
		"tools":[],
		"future_option":{"mode":"keep"}
	}`, string(llmRequest.ProviderExtensions.OpenAIResponses.Request.RawToolChoice))

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.JSONEq(t, `{
		"type":"allowed_tools",
		"mode":"auto",
		"tools":[],
		"future_option":{"mode":"keep"}
	}`, string(payload["tool_choice"]))
}

func TestRawUnsupportedToolChoicePreservesOnlyUnrepresentedSelectors(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		preserve bool
	}{
		{name: "string mode", raw: `"auto"`},
		{name: "named function", raw: `{"type":"function","name":"lookup"}`},
		{name: "clean allowed tools", raw: `{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"lookup"}]}`},
		{name: "type only hosted tool", raw: `{"type":"web_search"}`, preserve: true},
		{name: "future selector fields", raw: `{"type":"future_selector","policy":"strict"}`, preserve: true},
		{name: "mcp identity", raw: `{"type":"mcp","server_label":"docs","name":"search"}`, preserve: true},
		{name: "allowed tools future field", raw: `{"type":"allowed_tools","mode":"auto","tools":[],"future_option":{"mode":"keep"}}`, preserve: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var choice ToolChoice
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &choice))
			preserved := rawUnsupportedToolChoice(&choice, json.RawMessage(tc.raw))
			if !tc.preserve {
				require.Empty(t, preserved)
				return
			}
			require.JSONEq(t, tc.raw, string(preserved))
		})
	}
}

func TestRequestExtensions_ReplaysUnrepresentedToolChoices(t *testing.T) {
	tests := []struct {
		name       string
		tools      string
		toolChoice string
	}{
		{
			name:       "type only web search",
			tools:      `[{"type":"web_search"}]`,
			toolChoice: `{"type":"web_search"}`,
		},
		{
			name:       "future selector policy",
			tools:      `[]`,
			toolChoice: `{"type":"future_selector","policy":"strict"}`,
		},
		{
			name:       "mcp selector identity",
			tools:      `[{"type":"mcp","server_label":"docs","server_url":"https://example.com/mcp"}]`,
			toolChoice: `{"type":"mcp","server_label":"docs","name":"search"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.5","input":"use tools","tools":` + tc.tools + `,"tool_choice":` + tc.toolChoice + `}`)
			inbound := NewInboundTransformer()
			llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: body})
			require.NoError(t, err)
			require.NotNil(t, llmRequest.ProviderExtensions)
			require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses)
			require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses.Request)
			require.JSONEq(t, tc.toolChoice, string(llmRequest.ProviderExtensions.OpenAIResponses.Request.RawToolChoice))

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)
			httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
			require.NoError(t, err)

			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
			require.JSONEq(t, tc.toolChoice, string(payload["tool_choice"]))
		})
	}
}

func TestRequestExtensions_ToolChoicePrimitiveMatrixRoundTrips(t *testing.T) {
	tests := []struct {
		name       string
		toolChoice string
		expectsRaw bool
	}{
		{name: "mode", toolChoice: `"auto"`},
		{name: "named function", toolChoice: `{"type":"function","name":"lookup"}`},
		{name: "named custom", toolChoice: `{"type":"custom","name":"apply_patch"}`},
		{name: "named namespace", toolChoice: `{"type":"namespace","name":"workspace"}`},
		{name: "named tool search", toolChoice: `{"type":"tool_search","name":"discover"}`},
		{name: "named future client", toolChoice: `{"type":"future_client_tool","name":"later"}`},
		{
			name: "allowed mixed primitives",
			toolChoice: `{"type":"allowed_tools","mode":"required","tools":[
				{"type":"function","name":"same"},
				{"type":"custom","name":"same"},
				{"type":"namespace","name":"workspace"},
				{"type":"tool_search","name":"discover"},
				{"type":"future_client_tool","name":"later"},
				{"type":"future_server_tool","name":"hosted"}
			]}`,
		},
		{name: "type only hosted", toolChoice: `{"type":"web_search"}`, expectsRaw: true},
		{name: "future selector fields", toolChoice: `{"type":"future_selector","policy":"strict"}`, expectsRaw: true},
		{name: "mcp identity", toolChoice: `{"type":"mcp","server_label":"docs","name":"search"}`, expectsRaw: true},
		{
			name:       "allowed future fields",
			toolChoice: `{"type":"allowed_tools","mode":"auto","tools":[],"future_option":{"mode":"keep"}}`,
			expectsRaw: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.5","input":"use tools","tools":[],"tool_choice":` + tt.toolChoice + `}`)
			inbound := NewInboundTransformer()
			llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: body})
			require.NoError(t, err)

			if tt.expectsRaw {
				require.NotNil(t, llmRequest.ProviderExtensions)
				require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses)
				require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses.Request)
				require.JSONEq(t, tt.toolChoice, string(llmRequest.ProviderExtensions.OpenAIResponses.Request.RawToolChoice))
			} else if llmRequest.ProviderExtensions != nil &&
				llmRequest.ProviderExtensions.OpenAIResponses != nil &&
				llmRequest.ProviderExtensions.OpenAIResponses.Request != nil {
				require.Empty(t, llmRequest.ProviderExtensions.OpenAIResponses.Request.RawToolChoice)
			}

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)
			httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
			require.NoError(t, err)

			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
			require.JSONEq(t, tt.toolChoice, string(payload["tool_choice"]))
		})
	}
}

func TestRequestExtensions_DoesNotReplayClearedRawToolChoice(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"search",
		"tools":[{"type":"web_search"}],
		"tool_choice":{"type":"web_search"}
	}`)})
	require.NoError(t, err)
	require.NotNil(t, llmRequest.ProviderExtensions)
	require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses.Request)
	require.NotEmpty(t, llmRequest.ProviderExtensions.OpenAIResponses.Request.RawToolChoice)

	llmRequest.ToolChoice = nil
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	_, hasToolChoice := payload["tool_choice"]
	require.False(t, hasToolChoice)
}
