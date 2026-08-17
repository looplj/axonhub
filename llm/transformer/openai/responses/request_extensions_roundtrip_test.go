package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestRequestExtensions_ReplaysNamespaceWithCustomSubtool(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[{
			"type":"namespace",
			"name":"functions",
			"description":"Developer tools",
			"tools":[
				{"type":"function","name":"wait","parameters":{"type":"object"}},
				{
					"type":"custom",
					"name":"exec",
					"description":"Run code",
					"format":{"type":"grammar","syntax":"lark","definition":"start: SOURCE"}
				}
			]
		}]
	}`)})
	require.NoError(t, err)
	// The custom subtool promotes to the custom-tool IR instead of degrading
	// to an opaque tool, so the namespace converts to two structured tools.
	require.Len(t, llmRequest.Tools, 2)
	require.Equal(t, "function", llmRequest.Tools[0].Type)
	require.Equal(t, "functions__wait", llmRequest.Tools[0].Function.Name)
	require.Equal(t, llm.ToolTypeResponsesCustomTool, llmRequest.Tools[1].Type)
	require.Equal(t, "exec", llmRequest.Tools[1].ResponseCustomTool.Name)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	// Same-protocol replay must restore the raw namespace declaration verbatim,
	// including the grammar custom subtool.
	var payload struct {
		Tools []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Tools []struct {
				Type        string `json:"type"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Format      struct {
					Type       string `json:"type"`
					Syntax     string `json:"syntax"`
					Definition string `json:"definition"`
				} `json:"format"`
			} `json:"tools"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "namespace", payload.Tools[0].Type)
	require.Equal(t, "functions", payload.Tools[0].Name)
	var document map[string]any
	require.NoError(t, json.Unmarshal(httpRequest.Body, &document))
	require.Equal(t, "Developer tools", document["tools"].([]any)[0].(map[string]any)["description"])
	require.Len(t, payload.Tools[0].Tools, 2)
	require.Equal(t, "function", payload.Tools[0].Tools[0].Type)
	require.Equal(t, "wait", payload.Tools[0].Tools[0].Name)
	require.Equal(t, "custom", payload.Tools[0].Tools[1].Type)
	require.Equal(t, "exec", payload.Tools[0].Tools[1].Name)
	require.Equal(t, "Run code", payload.Tools[0].Tools[1].Description)
	require.Equal(t, "grammar", payload.Tools[0].Tools[1].Format.Type)
	require.Equal(t, "lark", payload.Tools[0].Tools[1].Format.Syntax)
	require.Equal(t, "start: SOURCE", payload.Tools[0].Tools[1].Format.Definition)
}

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

func TestRequestExtensions_ReplaysNestedNamespaceAsOpaqueSubtool(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[{
			"type":"namespace",
			"name":"outer",
			"tools":[{
				"type":"namespace",
				"name":"inner",
				"tools":[{"type":"function","name":"run","parameters":{"type":"object"}}]
			}]
		}]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 1)
	require.Equal(t, llm.ToolTypeResponsesOpaqueTool, llmRequest.Tools[0].Type)
	require.Equal(t, "namespace", llmRequest.Tools[0].ResponseOpaqueTool.SourceType)
	require.Equal(t, "inner", llmRequest.Tools[0].ResponseOpaqueTool.Name)
	require.Equal(t, "outer", llmRequest.Tools[0].ResponseOpaqueTool.Namespace)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Tools []struct {
				Type  string `json:"type"`
				Name  string `json:"name"`
				Tools []struct {
					Type string `json:"type"`
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"tools"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "namespace", payload.Tools[0].Type)
	require.Equal(t, "outer", payload.Tools[0].Name)
	require.Len(t, payload.Tools[0].Tools, 1)
	require.Equal(t, "namespace", payload.Tools[0].Tools[0].Type)
	require.Equal(t, "inner", payload.Tools[0].Tools[0].Name)
	require.Len(t, payload.Tools[0].Tools[0].Tools, 1)
	require.Equal(t, "function", payload.Tools[0].Tools[0].Tools[0].Type)
	require.Equal(t, "run", payload.Tools[0].Tools[0].Tools[0].Name)
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
	require.Equal(t, "namespace", payload.Tools[0]["type"])
	require.Equal(t, "workspace", payload.Tools[0]["name"])
	require.Equal(t, "mid", payload.Tools[1]["name"])
	require.Equal(t, "tail", payload.Tools[2]["name"])
	require.NotContains(t, string(httpRequest.Body), "removed")
}

func TestRequestExtensions_ModifiedNamespaceCustomKeepsNamespaceIdentity(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[{
			"type":"namespace",
			"name":"functions",
			"description":"Developer tools",
			"tools":[
				{"type":"custom","name":"exec","description":"old"},
				{"type":"function","name":"wait","parameters":{"type":"object"}}
			]
		}]
	}`)})
	require.NoError(t, err)
	llmRequest.Tools[0].ResponseCustomTool.Description = "current"

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Tools []struct {
				Type        string `json:"type"`
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "namespace", payload.Tools[0].Type)
	require.Equal(t, "functions", payload.Tools[0].Name)
	document := map[string]any(nil)
	require.NoError(t, json.Unmarshal(httpRequest.Body, &document))
	require.Equal(t, "Developer tools", document["tools"].([]any)[0].(map[string]any)["description"])
	require.Len(t, payload.Tools[0].Tools, 2)
	require.Equal(t, "custom", payload.Tools[0].Tools[0].Type)
	require.Equal(t, "exec", payload.Tools[0].Tools[0].Name)
	require.Equal(t, "current", payload.Tools[0].Tools[0].Description)
	require.Equal(t, "function", payload.Tools[0].Tools[1].Type)
	require.Equal(t, "wait", payload.Tools[0].Tools[1].Name)
}

func TestRequestExtensions_ModifiedNamespaceStillReplaysUnrelatedRawTool(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[
			{"type":"namespace","name":"workspace","tools":[
				{"type":"function","name":"first","parameters":{"type":"object","properties":{"old":{"type":"string"}}}},
				{"type":"function","name":"second","parameters":{"type":"object"}}
			]},
			{"type":"future_server_tool","name":"hosted","future_option":{"mode":"lossless"}}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 3)
	llmRequest.Tools[0].Function.Parameters = json.RawMessage(`{"type":"object","properties":{"current":{"type":"string"}}}`)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Tools []struct {
				Name       string         `json:"name"`
				Parameters map[string]any `json:"parameters"`
			} `json:"tools"`
			FutureOption map[string]any `json:"future_option"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 2)
	require.Equal(t, "namespace", payload.Tools[0].Type)
	require.Equal(t, "workspace", payload.Tools[0].Name)
	require.Len(t, payload.Tools[0].Tools, 2)
	require.Equal(t, "first", payload.Tools[0].Tools[0].Name)
	require.Contains(t, payload.Tools[0].Tools[0].Parameters["properties"], "current")
	require.Equal(t, "future_server_tool", payload.Tools[1].Type)
	require.Equal(t, "lossless", payload.Tools[1].FutureOption["mode"])
}

func TestRequestExtensions_MixedNamespaceReplaysWithoutDuplicateError(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"use tools","tools":[{
			"type":"namespace","name":"functions","tools":[
				{"type":"function","name":"wait","parameters":{"type":"object"}},
				{"type":"web_search"},
				{"type":"custom","name":"exec"}
			]}]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 3)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Tools []struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "namespace", payload.Tools[0].Type)
	require.Equal(t, "functions", payload.Tools[0].Name)
	require.Len(t, payload.Tools[0].Tools, 3)
	require.Equal(t, "function", payload.Tools[0].Tools[0].Type)
	require.Equal(t, "web_search", payload.Tools[0].Tools[1].Type)
	require.Equal(t, "custom", payload.Tools[0].Tools[2].Type)
}

func TestRequestExtensions_ModifiedNamespaceWithOpaqueMemberFailsLosslessly(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"use tools","tools":[{
			"type":"namespace","name":"functions","tools":[
				{"type":"future_server_tool","name":"hosted","future_option":{"mode":"keep"}},
				{"type":"function","name":"wait","parameters":{"type":"object","properties":{"old":{"type":"string"}}}}
			]}]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	llmRequest.Tools[1].Function.Parameters = json.RawMessage(
		`{"type":"object","properties":{"current":{"type":"string"}}}`,
	)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.Nil(t, httpRequest)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.ErrorContains(t, err, `unsupported_namespace_replay: namespace "functions" was modified and contains member type(s) without a structural Responses codec`)
}

func TestRequestExtensions_AppendedNamespaceMemberUsesCurrentWrapper(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"use tools","tools":[{
			"type":"namespace","name":"functions","tools":[
				{"type":"function","name":"wait","parameters":{"type":"object"}}
			]}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 1)
	llmRequest.Tools = append(llmRequest.Tools, llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.Function{
			Name: "functions__extra", Namespace: "functions",
			Parameters: json.RawMessage(`{"type":"object"}`),
		},
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Name  string `json:"name"`
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "functions", payload.Tools[0].Name)
	require.Equal(t, []string{"wait", "extra"}, []string{
		payload.Tools[0].Tools[0].Name, payload.Tools[0].Tools[1].Name,
	})
}

func TestRequestExtensions_AppendedOpaqueNamespaceMemberFailsLosslessly(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"use tools","tools":[{
			"type":"namespace","name":"functions","tools":[
				{"type":"function","name":"wait","parameters":{"type":"object"}}
			]}
		]
	}`)})
	require.NoError(t, err)
	llmRequest.Tools = append(llmRequest.Tools, llm.Tool{
		Type: llm.ToolTypeResponsesOpaqueTool,
		ResponseOpaqueTool: &llm.ResponseOpaqueTool{
			SourceType: "future_server_tool", Name: "hosted", Namespace: "functions",
		},
		ResponsesOrigin: "raw_tool",
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.Nil(t, httpRequest)
	require.ErrorContains(t, err, `unsupported_namespace_replay: namespace "functions" was modified`)
}

func TestOutboundTransformer_RejectsDuplicateNamespaceRawFragments(t *testing.T) {
	raw := json.RawMessage(`{"type":"namespace","name":"functions","tools":[{"type":"function","name":"one","parameters":{"type":"object"}}]}`)
	llmRequest := &llm.Request{Tools: []llm.Tool{{
		Type:     llm.ToolTypeFunction,
		Function: llm.Function{Name: "functions__one", Namespace: "functions", Parameters: []byte(`{"type":"object"}`)},
	}}, ProviderExtensions: &llm.ProviderExtensions{OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
		Request: &llm.OpenAIResponsesRequestExtensions{RawTools: []llm.OpenAIResponsesRawFragment{
			{Type: "namespace", Name: "functions", OriginalIndex: 0, Raw: raw},
			{Type: "namespace", Name: "functions", OriginalIndex: 1, Raw: raw},
		}},
	}}}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.Nil(t, httpRequest)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.ErrorContains(t, err, `duplicate_namespace: namespace "functions" appears in multiple tool declarations`)
}

func TestRequestExtensions_RemovedNamespaceMemberUsesCurrentWrapper(t *testing.T) {
	const body = `{"model":"gpt-5.5","input":"use tools","tools":[{
		"type":"namespace","name":"functions","tools":[
			{"type":"custom","name":"exec"},
			{"type":"function","name":"wait","parameters":{"type":"object"}}
		]}]}`
	tests := []struct {
		name           string
		remove         func(tools []llm.Tool) []llm.Tool
		expectedType   string
		expectedMember string
	}{
		{
			name:           "custom removed",
			remove:         func(tools []llm.Tool) []llm.Tool { return tools[1:] },
			expectedType:   "function",
			expectedMember: "wait",
		},
		{
			name:           "function removed",
			remove:         func(tools []llm.Tool) []llm.Tool { return tools[:1] },
			expectedType:   "custom",
			expectedMember: "exec",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound := NewInboundTransformer()
			llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
			require.NoError(t, err)
			require.Len(t, llmRequest.Tools, 2)
			llmRequest.Tools = tt.remove(llmRequest.Tools)

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)
			httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
			require.NoError(t, err)

			var payload struct {
				Tools []struct {
					Type  string `json:"type"`
					Name  string `json:"name"`
					Tools []struct {
						Type string `json:"type"`
						Name string `json:"name"`
					} `json:"tools"`
				} `json:"tools"`
			}
			require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
			require.Len(t, payload.Tools, 1)
			require.Equal(t, "namespace", payload.Tools[0].Type)
			require.Equal(t, "functions", payload.Tools[0].Name)
			require.Len(t, payload.Tools[0].Tools, 1)
			require.Equal(t, tt.expectedType, payload.Tools[0].Tools[0].Type)
			require.Equal(t, tt.expectedMember, payload.Tools[0].Tools[0].Name)
		})
	}
}

func TestRequestExtensions_NamespaceMemberOrderChangeUsesCurrentWrapper(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"use tools","tools":[{
			"type":"namespace","name":"functions","tools":[
				{"type":"custom","name":"exec"},
				{"type":"function","name":"wait","parameters":{"type":"object"}}
			]}]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)
	llmRequest.Tools[0], llmRequest.Tools[1] = llmRequest.Tools[1], llmRequest.Tools[0]

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Tools []struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "functions", payload.Tools[0].Name)
	require.Equal(t, "function", payload.Tools[0].Tools[0].Type)
	require.Equal(t, "wait", payload.Tools[0].Tools[0].Name)
	require.Equal(t, "custom", payload.Tools[0].Tools[1].Type)
	require.Equal(t, "exec", payload.Tools[0].Tools[1].Name)
}

func TestMergeRawOnlyTools_RejectsMismatchedNamespaceWrapper(t *testing.T) {
	requestExt := &llm.OpenAIResponsesRequestExtensions{RawTools: []llm.OpenAIResponsesRawFragment{{
		Type: "namespace", Name: "workspace", OriginalIndex: 0,
		Raw: json.RawMessage(`{"type":"namespace","name":"workspace","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`),
	}}}
	currentTools := []llm.Tool{{
		Type:     llm.ToolTypeFunction,
		Function: llm.Function{Name: "workspace__lookup", Namespace: "workspace", Parameters: json.RawMessage(`{"type":"object"}`)},
	}}

	tools, ok, err := mergeRawOnlyTools(
		json.RawMessage(`[{"type":"namespace","name":"other","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}]`),
		requestExt, currentTools, false,
	)
	require.False(t, ok)
	require.NoError(t, err)
	require.Nil(t, tools)
}

func TestMergeRawOnlyTools_PropagatesMalformedStructuredTools(t *testing.T) {
	requestExt := &llm.OpenAIResponsesRequestExtensions{RawTools: []llm.OpenAIResponsesRawFragment{{
		Type: "future_server_tool", Name: "hosted", OriginalIndex: 0,
		Raw: json.RawMessage(`{"type":"future_server_tool","name":"hosted"}`),
	}}}

	tools, replayed, err := mergeRawOnlyTools(json.RawMessage(`{}`), requestExt, nil, false)
	require.Error(t, err)
	require.False(t, replayed)
	require.Nil(t, tools)
}

func TestBuildReplayPlan(t *testing.T) {
	currentFromRaw := func(t *testing.T, raw string) []llm.Tool {
		t.Helper()
		var rawTool Tool
		require.NoError(t, json.Unmarshal([]byte(raw), &rawTool))
		tools, err := convertToolsToLLM([]Tool{rawTool})
		require.NoError(t, err)
		require.NotEmpty(t, tools)
		for i := range tools {
			tools[i].ResponsesRawID = fmt.Sprintf("tools:%d", i)
		}
		return tools
	}

	futureClientRaw := `{"type":"future_client_tool","name":"lookup","execution":"client","parameters":{"type":"object"}}`
	namespaceRaw := `{
		"type":"namespace","name":"workspace",
		"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]
	}`
	lossyNamespaceRaw := `{
		"type":"namespace","name":"workspace",
		"tools":[{"type":"future_server_tool","name":"hosted"}]
	}`

	tests := []struct {
		name            string
		raw             string
		current         func(t *testing.T) []llm.Tool
		structured      []json.RawMessage
		replayRawInput  bool
		expectedError   string
		expectedPlan    bool
		expectedToolIDs []string
	}{
		{
			name:            "exact raw group consumes its structured index",
			raw:             futureClientRaw,
			current:         func(t *testing.T) []llm.Tool { return currentFromRaw(t, futureClientRaw) },
			structured:      []json.RawMessage{json.RawMessage(`{"type":"function","name":"consumed"}`)},
			expectedPlan:    true,
			expectedToolIDs: []string{"future_client_tool/lookup"},
		},
		{
			name: "structured index advances after a replayed raw group",
			raw:  futureClientRaw,
			current: func(t *testing.T) []llm.Tool {
				return append(currentFromRaw(t, futureClientRaw), llm.Tool{
					Type: llm.ToolTypeFunction,
					Function: llm.Function{
						Name: "plain", Parameters: json.RawMessage(`{"type":"object"}`),
					},
				})
			},
			structured: []json.RawMessage{
				json.RawMessage(`{"type":"function","name":"consumed"}`),
				json.RawMessage(`{"type":"function","name":"plain","parameters":{"type":"object"}}`),
			},
			expectedPlan:    true,
			expectedToolIDs: []string{"future_client_tool/lookup", "function/plain"},
		},
		{
			name:            "exact namespace consumes one structured wrapper",
			raw:             namespaceRaw,
			current:         func(t *testing.T) []llm.Tool { return currentFromRaw(t, namespaceRaw) },
			structured:      []json.RawMessage{json.RawMessage(`{"type":"namespace","name":"workspace","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`)},
			expectedPlan:    true,
			expectedToolIDs: []string{"namespace/workspace"},
		},
		{
			name:         "namespace wrapper mismatch falls back",
			raw:          namespaceRaw,
			current:      func(t *testing.T) []llm.Tool { return currentFromRaw(t, namespaceRaw) },
			structured:   []json.RawMessage{json.RawMessage(`{"type":"namespace","name":"other","tools":[]}`)},
			expectedPlan: false,
		},
		{
			name:         "missing structured index falls back",
			raw:          futureClientRaw,
			current:      func(t *testing.T) []llm.Tool { return currentFromRaw(t, futureClientRaw) },
			expectedPlan: false,
		},
		{
			name:            "lossy namespace replays when unchanged",
			raw:             lossyNamespaceRaw,
			current:         func(t *testing.T) []llm.Tool { return currentFromRaw(t, lossyNamespaceRaw) },
			expectedPlan:    true,
			expectedToolIDs: []string{"namespace/workspace"},
		},
		{
			name: "modified unsupported namespace fails closed",
			raw:  lossyNamespaceRaw,
			current: func(t *testing.T) []llm.Tool {
				return append(currentFromRaw(t, lossyNamespaceRaw), llm.Tool{
					Type: llm.ToolTypeResponsesOpaqueTool,
					ResponseOpaqueTool: &llm.ResponseOpaqueTool{
						SourceType: "future_server_tool", Name: "added", Namespace: "workspace",
					},
					ResponsesOrigin: "raw_tool",
				})
			},
			expectedError: `unsupported_namespace_replay: namespace "workspace"`,
		},
		{
			name: "additional tools depend on raw input replay mode",
			raw:  futureClientRaw,
			current: func(t *testing.T) []llm.Tool {
				return []llm.Tool{{
					Type: llm.ToolTypeFunction,
					Function: llm.Function{
						Name: "lookup", Parameters: json.RawMessage(`{"type":"object"}`),
					},
					ResponsesOrigin: "additional_tools",
				}}
			},
			structured:     []json.RawMessage{json.RawMessage(`{"type":"function","name":"lookup","parameters":{"type":"object"}}`)},
			replayRawInput: true,
			expectedPlan:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := buildRawToolGroups([]llm.OpenAIResponsesRawFragment{{
				Type: "test", OriginalIndex: 0, Raw: json.RawMessage(tt.raw),
			}})
			require.Len(t, groups, 1)

			plan, err := buildReplayPlan(tt.current(t), groups, tt.structured, tt.replayRawInput)
			if tt.expectedError != "" {
				require.Nil(t, plan)
				require.ErrorContains(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			if !tt.expectedPlan {
				require.Nil(t, plan)
				return
			}
			require.NotNil(t, plan)
			require.True(t, plan.replayed)
			require.Len(t, plan.steps, len(tt.expectedToolIDs))
			for i, step := range plan.steps {
				var tool struct {
					Type string `json:"type"`
					Name string `json:"name"`
				}
				require.NoError(t, json.Unmarshal(step, &tool))
				require.Equal(t, tt.expectedToolIDs[i], tool.Type+"/"+tool.Name)
			}
		})
	}
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
			if tt.name == "namespace future client" {
				require.Equal(t, "namespace", payload.Tools[0].Type)
				require.Equal(t, "workspace", payload.Tools[0].Name)
				require.Len(t, payload.Tools[0].Tools, 1)
				member := payload.Tools[0].Tools[0]
				require.Equal(t, "future_client_tool", member.Type)
				require.Equal(t, "later", member.Name)
				require.Contains(t, member.Parameters["properties"], "current")
				require.NotContains(t, member.Parameters["properties"], "old")
			} else {
				require.Equal(t, "function", payload.Tools[0].Type)
				require.Equal(t, tt.expectedName, payload.Tools[0].Name)
				require.Contains(t, payload.Tools[0].Parameters["properties"], "current")
				require.NotContains(t, payload.Tools[0].Parameters["properties"], "old")
			}
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

func TestRawRepresentedToolChoiceRequiresCurrentSemanticMatch(t *testing.T) {
	mode := "auto"
	current := ToolChoice{Mode: &mode}
	preserved := rawUnsupportedToolChoice(&current, json.RawMessage(`{"mode":"required"}`))
	require.JSONEq(t, `{"mode":"required"}`, string(preserved))
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

// agent_message is structurally converted into a user message so Chat
// providers receive the dispatched task, but replaying to a native Responses
// upstream must restore the original agent_message item exactly once instead
// of emitting both the raw item and an equivalent user message.
func TestRequestExtensions_AgentMessageReplaysRawItemWithoutDuplicateUserMessage(t *testing.T) {
	const body = `{
		"model":"gpt-5.5",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"context"}]},
			{"id":"am_1","type":"agent_message","content":[
				{"type":"input_text","text":"do the task"},
				{"type":"encrypted_content","encrypted_content":"plain agent payload"}
			],"raw_marker":"agent"}
		]
	}`

	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
	require.NoError(t, err)

	// Chat direction: agent_message becomes a plain user message.
	require.Len(t, llmRequest.Messages, 2)
	require.Equal(t, "user", llmRequest.Messages[0].Role)
	require.Equal(t, "user", llmRequest.Messages[1].Role)
	parts := llmRequest.Messages[1].Content.MultipleContent
	require.Len(t, parts, 2)
	require.NotNil(t, parts[0].Text)
	require.Equal(t, "do the task", *parts[0].Text)
	require.NotNil(t, parts[1].Text)
	require.Equal(t, "plain agent payload", *parts[1].Text)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Input []map[string]any `json:"input"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Input, 2)

	agentMessages := 0
	userMessages := 0
	for _, item := range payload.Input {
		switch item["type"] {
		case "agent_message":
			agentMessages++
			require.Equal(t, "agent", item["raw_marker"])
		case "message":
			require.Equal(t, "user", item["role"])
			userMessages++
		}
	}
	require.Equal(t, 1, agentMessages)
	require.Equal(t, 1, userMessages)
}

// Removing the user message derived from an agent_message must abort replay
// instead of reviving the raw agent_message item.
func TestRequestExtensions_RemovedAgentMessageUserDoesNotReviveRawItem(t *testing.T) {
	const body = `{
		"model":"gpt-5.5",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"context"}]},
			{"id":"am_1","type":"agent_message","content":[{"type":"input_text","text":"do the task"}],"raw_marker":"agent"}
		]
	}`

	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
	require.NoError(t, err)

	kept := llmRequest.Messages[:0]
	for _, message := range llmRequest.Messages {
		if message.Content.Content == nil || *message.Content.Content != "do the task" {
			kept = append(kept, message)
		}
	}
	llmRequest.Messages = kept

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	require.NotContains(t, string(httpRequest.Body), "agent_message")
	require.NotContains(t, string(httpRequest.Body), "raw_marker")
	require.Contains(t, string(httpRequest.Body), "context")
}
