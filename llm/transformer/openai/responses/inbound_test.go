package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestNewInboundTransformer(t *testing.T) {
	transformer := NewInboundTransformer()
	require.NotNil(t, transformer)
}

func TestInboundTransformer_PromotesAdditionalAndToolSearchTools(t *testing.T) {
	transformer := NewInboundTransformer()
	request, err := transformer.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"additional_tools","role":"developer","tools":[
				{"type":"custom","name":"exec","description":"Run code"},
				{"type":"namespace","name":"collaboration","tools":[
					{"type":"function","name":"spawn_agent","parameters":{"type":"object","properties":{}}}
				]}
			]},
			{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"query":"agents"}},
			{"type":"tool_search_output","execution":"client","call_id":"call_search","tools":[
				{"type":"function","name":"send_message","parameters":{"type":"object","properties":{}}}
			]},
			{"role":"user","type":"message","content":[{"type":"input_text","text":"continue"}]}
		],
		"tools":[{"type":"tool_search","execution":"client","description":"Find tools","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}]
	}`)})
	require.NoError(t, err)
	require.Len(t, request.Tools, 4)
	require.Equal(t, llm.ToolTypeResponsesToolSearch, request.Tools[0].Type)
	require.Equal(t, llm.ToolTypeResponsesCustomTool, request.Tools[1].Type)
	require.Equal(t, "collaboration", request.Tools[2].Function.Namespace)
	require.Equal(t, "collaboration__spawn_agent", request.Tools[2].Function.Name)
	require.Equal(t, "additional_tools", request.Tools[2].ResponsesOrigin)
	require.Equal(t, "send_message", request.Tools[3].Function.Name)
	require.Equal(t, "tool_search_output", request.Tools[3].ResponsesOrigin)

	require.Len(t, request.Messages, 3)
	require.NotNil(t, request.Messages[0].ToolCalls[0].ResponseToolSearchCall)
	require.Equal(t, "call_search", lo.FromPtr(request.Messages[1].ToolCallID))
	require.JSONEq(t, `[{"type":"function","name":"send_message","parameters":{"type":"object","properties":{}}}]`, *request.Messages[1].Content.Content)
}

func TestInboundTransformer_EncodesEmptyToolSearchOutputAsArray(t *testing.T) {
	transformer := NewInboundTransformer()
	request, err := transformer.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"tool_search_call","execution":"client","call_id":"call_search","arguments":{"query":"agents"}},
			{"type":"tool_search_output","execution":"client","call_id":"call_search"}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, request.Messages, 2)
	require.Equal(t, "[]", lo.FromPtr(request.Messages[1].Content.Content))
}

func TestInboundTransformer_PromotesFutureClientFunctionLikeTools(t *testing.T) {
	transformer := NewInboundTransformer()
	request, err := transformer.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":[{"type":"additional_tools","role":"developer","tools":[{
			"type":"future_client_tool","name":"later_lookup","description":"Lookup later",
			"execution":"client","parameters":{"type":"object","properties":{"id":{"type":"string"}}}
		}]}],
		"tools":[{
			"type":"future_client_tool","name":"lookup","description":"Lookup",
			"execution":"client","parameters":{"type":"object","properties":{"query":{"type":"string"}}}
		}]
	}`)})
	require.NoError(t, err)
	require.Len(t, request.Tools, 2)
	require.Equal(t, llm.ToolTypeFunction, request.Tools[0].Type)
	require.Equal(t, "lookup", request.Tools[0].Function.Name)
	require.Equal(t, "raw_tool", request.Tools[0].ResponsesOrigin)
	require.Equal(t, "future_client_tool", request.Tools[0].ResponsesSourceType)
	require.JSONEq(t, `{"type":"object","properties":{"query":{"type":"string"}}}`, string(request.Tools[0].Function.Parameters))
	require.Equal(t, llm.ToolTypeFunction, request.Tools[1].Type)
	require.Equal(t, "later_lookup", request.Tools[1].Function.Name)
	require.Equal(t, "additional_tools", request.Tools[1].ResponsesOrigin)
	require.Equal(t, "future_client_tool", request.Tools[1].ResponsesSourceType)
}

func TestInboundTransformer_PreservesAllowedToolsChoice(t *testing.T) {
	transformer := NewInboundTransformer()
	request, err := transformer.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"run","tools":[
			{"type":"function","name":"lookup","parameters":{"type":"object"}},
			{"type":"custom","name":"apply_patch"}
		],
		"tool_choice":{"type":"allowed_tools","mode":"auto","tools":[
			{"type":"function","name":"lookup"},
			{"type":"custom","name":"apply_patch"}
		]}
	}`)})
	require.NoError(t, err)
	require.NotNil(t, request.ToolChoice)
	require.True(t, request.ToolChoice.AllowedToolsSet)
	require.Equal(t, "auto", lo.FromPtr(request.ToolChoice.ToolChoice))
	require.Equal(t, []llm.ToolOption{
		{Type: "function", Name: "lookup"},
		{Type: "custom", Name: "apply_patch"},
	}, request.ToolChoice.AllowedTools)
}

func TestInboundTransformer_PreservesEmptyAllowedToolsChoice(t *testing.T) {
	transformer := NewInboundTransformer()
	request, err := transformer.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"run",
		"tool_choice":{"type":"allowed_tools","mode":"auto","tools":[]}
	}`)})
	require.NoError(t, err)
	require.NotNil(t, request.ToolChoice)
	require.True(t, request.ToolChoice.AllowedToolsSet)
	require.Empty(t, request.ToolChoice.AllowedTools)
}

func TestInboundTransformer_PreservesUnsupportedFutureToolForExplicitRejection(t *testing.T) {
	transformer := NewInboundTransformer()
	request, err := transformer.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"hello","tools":[
			{"type":"future_server_tool","name":"hosted","execution":"server"},
			{"type":"future_unknown_tool","name":"unknown","parameters":{"type":"object"}}
		]
	}`)})
	require.NoError(t, err)
	require.Len(t, request.Tools, 2)
	require.Equal(t, llm.ToolTypeResponsesOpaqueTool, request.Tools[0].Type)
	require.Equal(t, "future_server_tool", request.Tools[0].ResponseOpaqueTool.SourceType)
	require.Equal(t, llm.ToolTypeResponsesOpaqueTool, request.Tools[1].Type)
	require.Equal(t, "future_unknown_tool", request.Tools[1].ResponseOpaqueTool.SourceType)
}

func TestInboundTransformer_DoesNotInferUnknownNamespaceExecutionOwner(t *testing.T) {
	transformer := NewInboundTransformer()
	request, err := transformer.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5","input":"hello","tools":[{
			"type":"namespace","name":"hosted","tools":[
				{"type":"future_hosted_tool","name":"implicit","parameters":{"type":"object"}},
				{"type":"future_client_tool","name":"explicit","execution":"client","parameters":{"type":"object"}}
			]
		}]
	}`)})
	require.NoError(t, err)
	require.Len(t, request.Tools, 2)
	require.Equal(t, llm.ToolTypeResponsesOpaqueTool, request.Tools[0].Type)
	require.Equal(t, "future_hosted_tool", request.Tools[0].ResponseOpaqueTool.SourceType)
	require.Equal(t, llm.ToolTypeFunction, request.Tools[1].Type)
	require.Equal(t, "hosted", request.Tools[1].Function.Namespace)
	require.Equal(t, "hosted__explicit", request.Tools[1].Function.Name)
}

func TestInboundTransformer_EmitsResponsesSpecialToolCalls(t *testing.T) {
	transformer := NewInboundTransformer()
	result, err := transformer.TransformResponse(context.Background(), &llm.Response{
		ID: "resp_1", Model: "glm-5.2",
		Choices: []llm.Choice{{Message: &llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{
			{ID: "call_custom", ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_custom", Name: "apply_patch", Input: "*** Begin Patch"}},
			{ID: "call_search", ResponseToolSearchCall: &llm.ResponseToolSearchCall{CallID: "call_search", Execution: "client", Arguments: `{"query":"agents"}`}},
		}}}},
	})
	require.NoError(t, err)
	var response Response
	require.NoError(t, json.Unmarshal(result.Body, &response))
	require.Len(t, response.Output, 2)
	require.Equal(t, "custom_tool_call", response.Output[0].Type)
	require.Equal(t, "*** Begin Patch", *response.Output[0].Input)
	require.Equal(t, "tool_search_call", response.Output[1].Type)
	require.Equal(t, "client", response.Output[1].Execution)
	require.JSONEq(t, `{"query":"agents"}`, response.Output[1].Arguments)
}

func TestInboundTransformer_TransformRequest(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		httpReq     *httpclient.Request
		expectError bool
		validate    func(t *testing.T, result *llm.Request)
	}{
		{
			name:        "nil request",
			httpReq:     nil,
			expectError: true,
		},
		{
			name: "empty body",
			httpReq: &httpclient.Request{
				Body: []byte{},
			},
			expectError: true,
		},
		{
			name: "invalid JSON",
			httpReq: &httpclient.Request{
				Body: []byte(`{invalid json}`),
			},
			expectError: true,
		},
		{
			name: "missing model",
			httpReq: &httpclient.Request{
				Body: []byte(`{"input": "Hello"}`),
			},
			expectError: true,
		},
		{
			name: "simple text input",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello, world!"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Messages, 1)
				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(t, "Hello, world!", *result.Messages[0].Content.Content)
			},
		},
		{
			name: "request with instructions",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"instructions": "You are a helpful assistant.",
					"input": "Hello!"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Messages, 2)
				require.Equal(t, "system", result.Messages[0].Role)
				require.Equal(t, "You are a helpful assistant.", *result.Messages[0].Content.Content)
				require.Equal(t, "user", result.Messages[1].Role)
				require.Equal(t, "Hello!", *result.Messages[1].Content.Content)
			},
		},
		{
			name: "request with temperature and top_p",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"temperature": 0.7,
					"top_p": 0.9
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "gpt-4o", result.Model)
				require.NotNil(t, result.Temperature)
				require.Equal(t, 0.7, *result.Temperature)
				require.NotNil(t, result.TopP)
				require.Equal(t, 0.9, *result.TopP)
			},
		},
		{
			name: "request with max_output_tokens",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"max_output_tokens": 1000
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.MaxCompletionTokens)
				require.Equal(t, int64(1000), *result.MaxCompletionTokens)
			},
		},
		{
			name: "request with function tools",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "What's the weather?",
					"tools": [
						{
							"type": "function",
							"name": "get_weather",
							"description": "Get weather information",
							"parameters": {
								"type": "object",
								"properties": {
									"location": {"type": "string"}
								}
							}
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Len(t, result.Tools, 1)
				require.Equal(t, "function", result.Tools[0].Type)
				require.Equal(t, "get_weather", result.Tools[0].Function.Name)
				require.Equal(t, "Get weather information", result.Tools[0].Function.Description)
			},
		},
		{
			name: "request with namespace tools",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "List the projects.",
					"tools": [
						{
							"type": "namespace",
							"name": "mcp__codebase_memory_mcp",
							"tools": [
								{
									"type": "function",
									"name": "list_projects",
									"description": "List stored projects",
									"parameters": {
										"type": "object",
										"properties": {}
									},
									"strict": true
								},
								{
									"type": "function",
									"name": "get_project",
									"description": "Get a stored project",
									"parameters": {
										"type": "object",
										"properties": {"id": {"type": "string"}},
										"required": ["id"]
									}
								},
								{
									"type": "web_search",
									"name": "unsupported_nested_tool"
								}
							]
						},
						{
							"type": "function",
							"name": "get_weather",
							"parameters": {"type": "object", "properties": {}}
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Len(t, result.Tools, 4)

				namespaceTool := result.Tools[0]
				require.Equal(t, "function", namespaceTool.Type)
				require.Equal(t, "mcp__codebase_memory_mcp__list_projects", namespaceTool.Function.Name)
				require.Equal(t, "List stored projects", namespaceTool.Function.Description)
				require.JSONEq(t, `{"type":"object","properties":{}}`, string(namespaceTool.Function.Parameters))
				require.NotNil(t, namespaceTool.Function.Strict)
				require.True(t, *namespaceTool.Function.Strict)

				require.Equal(t, "mcp__codebase_memory_mcp__get_project", result.Tools[1].Function.Name)
				require.Equal(t, llm.ToolTypeResponsesOpaqueTool, result.Tools[2].Type)
				require.Equal(t, "web_search", result.Tools[2].ResponseOpaqueTool.SourceType)
				require.Equal(t, "get_weather", result.Tools[3].Function.Name)
			},
		},
		{
			name: "captures responses provider raw tools and tool choice",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Search and run shell.",
					"tools": [
						{
							"type": "tool_search",
							"name": "search_docs",
							"namespace": "docs"
						},
						{
							"type": "function",
							"name": "get_weather",
							"parameters": {"type": "object", "properties": {}}
						}
					],
					"tool_choice": {
						"type": "tool_search",
						"tools": [
							{"type": "tool_search", "name": "search_docs"}
						]
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Len(t, result.Tools, 2)
				require.Equal(t, llm.ToolTypeResponsesToolSearch, result.Tools[0].Type)
				require.Equal(t, llm.ToolTypeFunction, result.Tools[1].Type)
				require.NotNil(t, result.ProviderExtensions)
				require.NotNil(t, result.ProviderExtensions.OpenAIResponses)
				require.NotNil(t, result.ProviderExtensions.OpenAIResponses.Request)
				require.Len(t, result.ProviderExtensions.OpenAIResponses.Request.RawTools, 1)
				require.JSONEq(t, `{"type":"tool_search","name":"search_docs","namespace":"docs"}`, string(result.ProviderExtensions.OpenAIResponses.Request.RawTools[0].Raw))
				require.JSONEq(t, `{"type":"tool_search","tools":[{"type":"tool_search","name":"search_docs"}]}`, string(result.ProviderExtensions.OpenAIResponses.Request.RawToolChoice))
			},
		},
		{
			name: "request with image generation tool",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Generate an image of a cat",
					"tools": [
						{
							"type": "image_generation",
							"quality": "high",
							"size": "1024x1024"
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Len(t, result.Tools, 1)
				require.Equal(t, llm.ToolTypeImageGeneration, result.Tools[0].Type)
				require.NotNil(t, result.Tools[0].ImageGeneration)
				require.Equal(t, "high", result.Tools[0].ImageGeneration.Quality)
				require.Equal(t, "1024x1024", result.Tools[0].ImageGeneration.Size)
			},
		},
		{
			name: "request with reasoning",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": "Solve this problem",
					"reasoning": {
						"effort": "high",
						"max_tokens": 5000
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "high", result.ReasoningEffort)
				require.NotNil(t, result.ReasoningBudget)
				require.Equal(t, int64(5000), *result.ReasoningBudget)
			},
		},
		{
			name: "request with reasoning and summary",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": "Solve this problem",
					"reasoning": {
						"effort": "medium",
						"summary": "detailed"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "medium", result.ReasoningEffort)
				require.NotNil(t, result.ReasoningSummary)
				require.Equal(t, "detailed", *result.ReasoningSummary)
			},
		},
		{
			name: "request with reasoning and generate_summary (merged to summary)",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": "Solve this problem",
					"reasoning": {
						"effort": "low",
						"generate_summary": "concise"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "low", result.ReasoningEffort)
				// generate_summary is merged into ReasoningSummary at inbound level
				require.NotNil(t, result.ReasoningSummary)
				require.Equal(t, "concise", *result.ReasoningSummary)
			},
		},
		{
			name: "request with reasoning both summary and generate_summary - summary takes priority",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": "Solve this problem",
					"reasoning": {
						"effort": "high",
						"summary": "auto",
						"generate_summary": "detailed"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "high", result.ReasoningEffort)
				// summary takes priority over generate_summary
				require.NotNil(t, result.ReasoningSummary)
				require.Equal(t, "auto", *result.ReasoningSummary)
			},
		},
		{
			name: "request with auto tool choice mode",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"tool_choice": "auto"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.ToolChoice)
				require.NotNil(t, result.ToolChoice.ToolChoice)
				require.Equal(t, "auto", *result.ToolChoice.ToolChoice)
			},
		},
		{
			name: "request with tool choice mode",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"tool_choice": {
						"mode": "auto"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.ToolChoice)
				require.NotNil(t, result.ToolChoice.ToolChoice)
				require.Equal(t, "auto", *result.ToolChoice.ToolChoice)
			},
		},
		{
			name: "request with specific tool choice",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"tool_choice": {
						"type": "function",
						"name": "get_weather"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.ToolChoice)
				require.NotNil(t, result.ToolChoice.NamedToolChoice)
				require.Equal(t, "function", result.ToolChoice.NamedToolChoice.Type)
				require.Equal(t, "get_weather", result.ToolChoice.NamedToolChoice.Function.Name)
			},
		},
		{
			name: "request with metadata",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"metadata": {
						"user_id": "123",
						"session_id": "abc"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.Metadata)
				require.Equal(t, "123", result.Metadata["user_id"])
				require.Equal(t, "abc", result.Metadata["session_id"])
			},
		},
		{
			name: "request with store and service_tier",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"store": true,
					"service_tier": "default"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.Store)
				require.True(t, *result.Store)
				require.NotNil(t, result.ServiceTier)
				require.Equal(t, "default", *result.ServiceTier)
			},
		},
		{
			name: "request with text format",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Return JSON",
					"text": {
						"format": {
							"type": "json_object"
						}
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.ResponseFormat)
				require.Equal(t, "json_object", result.ResponseFormat.Type)
			},
		},
		{
			name: "request with stream options",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"stream": true,
					"stream_options": {
						"include_obfuscation": true
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.Stream)
				require.True(t, *result.Stream)
				require.NotNil(t, result.StreamOptions)
			},
		},
		{
			name: "request with top_logprobs",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"top_logprobs": 5
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.TopLogprobs)
				require.Equal(t, int64(5), *result.TopLogprobs)
			},
		},
		{
			name: "request with include field",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"include": ["file_search_call.results", "reasoning.encrypted_content"]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.TransformerMetadata)
				v, ok := result.TransformerMetadata["include"]
				require.True(t, ok)
				require.Equal(t, []string{"file_search_call.results", "reasoning.encrypted_content"}, v.([]string))
			},
		},
		{
			name: "request with previous_response_id",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-5.4",
					"previous_response_id": "resp_prev_123",
					"input": "Continue"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.PreviousResponseID)
				require.Equal(t, "resp_prev_123", *result.PreviousResponseID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformRequest(context.Background(), tt.httpReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, llm.RequestTypeChat, result.RequestType)
				require.Equal(t, llm.APIFormatOpenAIResponse, result.APIFormat)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformRequest_PreservesWebSearchTools(t *testing.T) {
	trans := NewInboundTransformer()

	result, err := trans.TransformRequest(context.Background(), &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-5.4",
			"input": "Use web search.",
			"tool_choice": "required",
			"tools": [
				{
					"type": "web_search",
					"filters": {
						"allowed_domains": ["example.com"]
					},
					"user_location": {
						"city": "San Francisco",
						"country": "US"
					}
				}
			]
		}`),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Tools, 1)
	tool := result.Tools[0]
	require.Equal(t, llm.ToolTypeWebSearch, tool.Type)
	require.NotNil(t, tool.WebSearch)
	require.Equal(t, []string{"example.com"}, tool.WebSearch.AllowedDomains)
	require.Equal(t, "San Francisco", tool.WebSearch.UserLocation.City)
	require.Equal(t, "US", tool.WebSearch.UserLocation.Country)
	require.Equal(t, "approximate", tool.WebSearch.UserLocation.Type)
}

func TestInboundTransformer_TransformStream_AttachesAnnotationsToFirstTextItem(t *testing.T) {
	trans := NewInboundTransformer()
	stream, err := trans.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID:      "resp_stream_annotations",
			Object:  "chat.completion.chunk",
			Created: 1677652288,
			Model:   "gpt-4o",
			Choices: []llm.Choice{{
				Delta: &llm.Message{
					Role: "assistant",
					Annotations: []llm.Annotation{{
						Type:       "url_citation",
						StartIndex: lo.ToPtr(int64(0)),
						EndIndex:   lo.ToPtr(int64(5)),
						URLCitation: &llm.URLCitation{
							URL:   "https://example.com/stream",
							Title: "Stream Example",
						},
					}},
					Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
				},
			}},
		},
		{
			ID:      "resp_stream_annotations",
			Object:  "chat.completion.chunk",
			Created: 1677652288,
			Model:   "gpt-4o",
			Choices: []llm.Choice{{FinishReason: lo.ToPtr("stop")}},
			Usage:   &llm.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		},
	}))
	require.NoError(t, err)

	events, err := streams.All(stream)
	require.NoError(t, err)

	var contentAdded *StreamEvent
	var itemDone *StreamEvent
	for _, raw := range events {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(raw.Data, &ev))
		switch ev.Type {
		case StreamEventTypeContentPartAdded:
			contentAdded = &ev
		case StreamEventTypeOutputItemDone:
			if ev.Item != nil && ev.Item.Type == "message" {
				itemDone = &ev
			}
		}
	}

	require.NotNil(t, contentAdded)
	require.NotNil(t, contentAdded.Part)
	require.Len(t, contentAdded.Part.Annotations, 1)
	require.Equal(t, "url_citation", contentAdded.Part.Annotations[0].Type)
	require.NotNil(t, itemDone)
	require.NotNil(t, itemDone.Item)
	require.NotNil(t, itemDone.Item.Content)
	require.Len(t, itemDone.Item.Content.Items, 1)
	require.Len(t, itemDone.Item.Content.Items[0].Annotations, 1)
	require.Equal(t, "https://example.com/stream", itemDone.Item.Content.Items[0].Annotations[0].URLCitation.URL)
}

func TestInboundTransformer_TransformStream_AttachesAnnotationsFromChoiceMessageToFirstTextItem(t *testing.T) {
	trans := NewInboundTransformer()
	stream, err := trans.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID:      "resp_stream_message_annotations",
			Object:  "chat.completion.chunk",
			Created: 1677652288,
			Model:   "gpt-4o",
			Choices: []llm.Choice{{
				Message: &llm.Message{
					Annotations: []llm.Annotation{{
						Type:       "url_citation",
						StartIndex: lo.ToPtr(int64(0)),
						EndIndex:   lo.ToPtr(int64(5)),
						URLCitation: &llm.URLCitation{
							URL:   "https://example.com/message-stream",
							Title: "Message Stream Example",
						},
					}},
				},
				Delta: &llm.Message{
					Role:    "assistant",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
				},
			}},
		},
		{
			ID:      "resp_stream_message_annotations",
			Object:  "chat.completion.chunk",
			Created: 1677652288,
			Model:   "gpt-4o",
			Choices: []llm.Choice{{FinishReason: lo.ToPtr("stop")}},
			Usage:   &llm.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		},
	}))
	require.NoError(t, err)

	events, err := streams.All(stream)
	require.NoError(t, err)

	var contentAdded *StreamEvent
	var itemDone *StreamEvent
	for _, raw := range events {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(raw.Data, &ev))
		switch ev.Type {
		case StreamEventTypeContentPartAdded:
			contentAdded = &ev
		case StreamEventTypeOutputItemDone:
			if ev.Item != nil && ev.Item.Type == "message" {
				itemDone = &ev
			}
		}
	}

	require.NotNil(t, contentAdded)
	require.NotNil(t, contentAdded.Part)
	require.Len(t, contentAdded.Part.Annotations, 1)
	require.Equal(t, "url_citation", contentAdded.Part.Annotations[0].Type)
	require.Equal(t, "https://example.com/message-stream", contentAdded.Part.Annotations[0].URLCitation.URL)
	require.NotNil(t, itemDone)
	require.NotNil(t, itemDone.Item)
	require.NotNil(t, itemDone.Item.Content)
	require.Len(t, itemDone.Item.Content.Items, 1)
	require.Len(t, itemDone.Item.Content.Items[0].Annotations, 1)
	require.Equal(t, "https://example.com/message-stream", itemDone.Item.Content.Items[0].Annotations[0].URLCitation.URL)
}

func TestInboundTransformer_TransformRequest_GroupsConsecutiveFunctionCalls(t *testing.T) {
	trans := NewInboundTransformer()

	result, err := trans.TransformRequest(context.Background(), &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": [
				{"role": "user", "content": "Run both tools."},
				{"type": "function_call", "call_id": "call_a", "name": "first_tool", "arguments": "{}"},
				{"type": "function_call", "call_id": "call_b", "name": "second_tool", "arguments": "{}"},
				{"type": "function_call_output", "call_id": "call_a", "output": "first result"},
				{"type": "function_call_output", "call_id": "call_b", "output": "second result"}
			]
		}`),
	})

	require.NoError(t, err)
	require.Len(t, result.Messages, 4)
	require.Equal(t, "user", result.Messages[0].Role)
	require.Equal(t, "assistant", result.Messages[1].Role)
	require.Len(t, result.Messages[1].ToolCalls, 2)
	require.Equal(t, "call_a", result.Messages[1].ToolCalls[0].ID)
	require.Equal(t, "call_b", result.Messages[1].ToolCalls[1].ID)
	require.Equal(t, "tool", result.Messages[2].Role)
	require.Equal(t, "call_a", lo.FromPtr(result.Messages[2].ToolCallID))
	require.Equal(t, "tool", result.Messages[3].Role)
	require.Equal(t, "call_b", lo.FromPtr(result.Messages[3].ToolCallID))
}

func TestInboundTransformer_TransformRequest_MergesRepeatedToolOutputs(t *testing.T) {
	trans := NewInboundTransformer()

	result, err := trans.TransformRequest(context.Background(), &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": [
				{"role": "user", "content": "run exec"},
				{
					"type": "custom_tool_call",
					"call_id": "call_1612831776e44d5ca84206c0",
					"name": "exec",
					"input": "const r = await tools.exec_command({cmd: 'echo hi'}); text(r.output);"
				},
				{
					"type": "custom_tool_call_output",
					"call_id": "call_1612831776e44d5ca84206c0",
					"output": [{"type": "input_text", "text": "Script completed\nWall time 5.2 seconds\nOutput:\n"}]
				},
				{
					"type": "custom_tool_call_output",
					"call_id": "call_1612831776e44d5ca84206c0",
					"name": "exec",
					"output": "notify 工具测试成功"
				}
			]
		}`),
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	// user, assistant(tool_call), tool(merged) = 3 messages, not 4.
	require.Len(t, result.Messages, 3)
	require.Equal(t, "assistant", result.Messages[1].Role)
	require.Len(t, result.Messages[1].ToolCalls, 1)
	require.Equal(t, "tool", result.Messages[2].Role)
	require.Equal(t, "call_1612831776e44d5ca84206c0", lo.FromPtr(result.Messages[2].ToolCallID))
	// Both outputs are concatenated into the single tool message.
	merged := flattenToolContent(result.Messages[2].Content)
	require.Contains(t, merged, "Script completed")
	require.Contains(t, merged, "notify 工具测试成功")
}

func TestInboundTransformer_TransformRequest_MergesRepeatedFunctionOutputs(t *testing.T) {
	trans := NewInboundTransformer()

	result, err := trans.TransformRequest(context.Background(), &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": [
				{"role": "user", "content": "run fn"},
				{"type": "function_call", "call_id": "call_x", "name": "fn", "arguments": "{}"},
				{"type": "function_call_output", "call_id": "call_x", "output": "first"},
				{"type": "function_call_output", "call_id": "call_x", "output": "second"}
			]
		}`),
	})

	require.NoError(t, err)
	require.Len(t, result.Messages, 3)
	require.Equal(t, "tool", result.Messages[2].Role)
	require.Equal(t, "call_x", lo.FromPtr(result.Messages[2].ToolCallID))
	merged := flattenToolContent(result.Messages[2].Content)
	require.Contains(t, merged, "first")
	require.Contains(t, merged, "second")
}

func flattenToolContent(c llm.MessageContent) string {
	if len(c.MultipleContent) == 0 && c.Content != nil {
		return *c.Content
	}
	var b strings.Builder
	for _, p := range c.MultipleContent {
		if p.Type == "text" && p.Text != nil {
			b.WriteString(*p.Text)
		}
	}
	return b.String()
}

func TestInboundTransformer_TransformResponse(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.Response
		expectError bool
		validate    func(t *testing.T, result *httpclient.Response)
	}{
		{
			name:        "nil response",
			chatResp:    nil,
			expectError: true,
		},
		{
			name: "simple text response",
			chatResp: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello! How can I help you?"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				require.Equal(t, http.StatusOK, result.StatusCode)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))

				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.Equal(t, "response", resp.Object)
				require.Equal(t, "chatcmpl-123", resp.ID)
				require.Equal(t, "gpt-4o", resp.Model)
				require.NotNil(t, resp.Status)
				require.Equal(t, "completed", *resp.Status)
				require.Len(t, resp.Output, 1)
				output := resp.Output[0]
				require.Equal(t, "message", output.Type)
				require.Equal(t, "assistant", output.Role)
			},
		},
		{
			name: "response with tool calls",
			chatResp: &llm.Response{
				ID:      "chatcmpl-456",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "get_weather",
										Arguments: `{"location": "San Francisco"}`,
									},
								},
							},
						},
						FinishReason: lo.ToPtr("tool_calls"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				require.Equal(t, http.StatusOK, result.StatusCode)

				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Status)
				require.Equal(t, "completed", *resp.Status)
			},
		},
		{
			name: "response with usage details",
			chatResp: &llm.Response{
				ID:      "chatcmpl-789",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Response with usage"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
					PromptTokensDetails: &llm.PromptTokensDetails{
						CachedTokens: 20,
					},
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						ReasoningTokens: 10,
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(100), resp.Usage.InputTokens)
				require.Equal(t, int64(50), resp.Usage.OutputTokens)
				require.Equal(t, int64(150), resp.Usage.TotalTokens)
				require.Equal(t, int64(20), resp.Usage.InputTokenDetails.CachedTokens)
				require.Equal(t, int64(10), resp.Usage.OutputTokenDetails.ReasoningTokens)
			},
		},
		{
			name: "response with length finish reason",
			chatResp: &llm.Response{
				ID:      "chatcmpl-length",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Truncated response..."),
							},
						},
						FinishReason: lo.ToPtr("length"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Status)
				require.Equal(t, "incomplete", *resp.Status)
				require.NotNil(t, resp.IncompleteDetails)
				require.Equal(t, "max_output_tokens", resp.IncompleteDetails.Reason)
			},
		},
		{
			name: "response with content filter finish reason",
			chatResp: &llm.Response{
				ID:      "chatcmpl-content-filter",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Filtered response..."),
							},
						},
						FinishReason: lo.ToPtr("content_filter"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Status)
				require.Equal(t, "incomplete", *resp.Status)
				require.NotNil(t, resp.IncompleteDetails)
				require.Equal(t, "content_filter", resp.IncompleteDetails.Reason)
			},
		},
		{
			name: "response with previous_response_id",
			chatResp: &llm.Response{
				ID:                 "chatcmpl-prev",
				Object:             "chat.completion",
				Created:            1677652288,
				Model:              "gpt-5.4",
				PreviousResponseID: lo.ToPtr("resp_prev_123"),
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("continued"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.PreviousResponseID)
				require.Equal(t, "resp_prev_123", *resp.PreviousResponseID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformResponse(context.Background(), tt.chatResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestConvertToResponsesAPIResponse_AbnormalFinishKeepsToolCallInProgress(t *testing.T) {
	tests := []struct {
		finishReason string
		status       string
	}{
		{finishReason: "length", status: "incomplete"},
		{finishReason: "content_filter", status: "incomplete"},
		{finishReason: "error", status: "failed"},
		{finishReason: "cancelled", status: "cancelled"},
		{finishReason: "canceled", status: "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.finishReason, func(t *testing.T) {
			response := convertToResponsesAPIResponse(&llm.Response{
				ID: "chatcmpl_abnormal", Model: "glm", Choices: []llm.Choice{{
					FinishReason: lo.ToPtr(tt.finishReason),
					Message: &llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{
						ID: "call_partial", Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{Name: "lookup", Arguments: `{"query":"partial`},
					}}},
				}},
			})

			require.Equal(t, tt.status, lo.FromPtr(response.Status))
			require.Len(t, response.Output, 1)
			require.Equal(t, "function_call", response.Output[0].Type)
			require.Equal(t, "in_progress", lo.FromPtr(response.Output[0].Status))
		})
	}
}

func TestConvertItemToMessage_Compaction(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		validate func(t *testing.T, result *llm.Message, err error)
	}{
		{
			name: "compaction item with all fields",
			item: &Item{
				ID:               "compaction_123",
				Type:             "compaction",
				EncryptedContent: lo.ToPtr("encrypted_data_here"),
				CreatedBy:        lo.ToPtr("assistant"),
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "assistant", result.Role)
				require.Len(t, result.Content.MultipleContent, 1)
				require.Equal(t, "compaction", result.Content.MultipleContent[0].Type)
				require.NotNil(t, result.Content.MultipleContent[0].Compact)
				require.Equal(t, "compaction_123", result.Content.MultipleContent[0].Compact.ID)
				require.Equal(t, "encrypted_data_here", result.Content.MultipleContent[0].Compact.EncryptedContent)
				require.NotNil(t, result.Content.MultipleContent[0].Compact.CreatedBy)
				require.Equal(t, "assistant", *result.Content.MultipleContent[0].Compact.CreatedBy)
			},
		},
		{
			name: "compaction item without created_by",
			item: &Item{
				ID:               "compaction_456",
				Type:             "compaction",
				EncryptedContent: lo.ToPtr("encrypted_only"),
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "assistant", result.Role)
				require.Len(t, result.Content.MultipleContent, 1)
				require.Equal(t, "compaction", result.Content.MultipleContent[0].Type)
				require.NotNil(t, result.Content.MultipleContent[0].Compact)
				require.Equal(t, "compaction_456", result.Content.MultipleContent[0].Compact.ID)
				require.Equal(t, "encrypted_only", result.Content.MultipleContent[0].Compact.EncryptedContent)
				require.Nil(t, result.Content.MultipleContent[0].Compact.CreatedBy)
			},
		},
		{
			name: "compaction item with empty encrypted_content",
			item: &Item{
				ID:               "compaction_789",
				Type:             "compaction",
				EncryptedContent: lo.ToPtr(""),
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "assistant", result.Role)
				require.Len(t, result.Content.MultipleContent, 1)
				require.Equal(t, "compaction", result.Content.MultipleContent[0].Type)
				require.NotNil(t, result.Content.MultipleContent[0].Compact)
				require.Equal(t, "", result.Content.MultipleContent[0].Compact.EncryptedContent)
			},
		},
		{
			name: "compaction item with nil encrypted_content",
			item: &Item{
				ID:               "compaction_nil",
				Type:             "compaction",
				EncryptedContent: nil,
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "assistant", result.Role)
				require.Len(t, result.Content.MultipleContent, 1)
				require.Equal(t, "compaction", result.Content.MultipleContent[0].Type)
				require.NotNil(t, result.Content.MultipleContent[0].Compact)
				require.Equal(t, "", result.Content.MultipleContent[0].Compact.EncryptedContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertItemToMessage(tt.item)
			tt.validate(t, result, err)
		})
	}
}

func TestConvertContentItemToPart_Compaction(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		validate func(t *testing.T, result *llm.MessageContentPart, err error)
	}{
		{
			name: "compaction item to compaction part",
			item: &Item{
				ID:               "compaction_part_123",
				Type:             "compaction",
				EncryptedContent: lo.ToPtr("encrypted_content"),
				CreatedBy:        lo.ToPtr("user"),
			},
			validate: func(t *testing.T, result *llm.MessageContentPart, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "compaction", result.Type)
				require.NotNil(t, result.Compact)
				require.Equal(t, "compaction_part_123", result.Compact.ID)
				require.Equal(t, "encrypted_content", result.Compact.EncryptedContent)
				require.NotNil(t, result.Compact.CreatedBy)
				require.Equal(t, "user", *result.Compact.CreatedBy)
			},
		},
		{
			name: "compaction item without created_by to compaction part",
			item: &Item{
				ID:               "compaction_part_456",
				Type:             "compaction",
				EncryptedContent: lo.ToPtr("data"),
			},
			validate: func(t *testing.T, result *llm.MessageContentPart, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "compaction", result.Type)
				require.NotNil(t, result.Compact)
				require.Equal(t, "compaction_part_456", result.Compact.ID)
				require.Equal(t, "data", result.Compact.EncryptedContent)
				require.Nil(t, result.Compact.CreatedBy)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertContentItemToPart(tt.item, "message")
			tt.validate(t, result, err)
		})
	}
}

func TestConvertContentItemToPart_EncryptedContent(t *testing.T) {
	tests := []struct {
		name      string
		item      *Item
		ownerType string
		validate  func(t *testing.T, result *llm.MessageContentPart, err error)
	}{
		{
			name:      "agent_message owner converts encrypted_content to text",
			item:      &Item{ID: "enc_1", Type: "encrypted_content", EncryptedContent: lo.ToPtr("dispatched task")},
			ownerType: "agent_message",
			validate: func(t *testing.T, result *llm.MessageContentPart, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "text", result.Type)
				require.Equal(t, "dispatched task", *result.Text)
			},
		},
		{
			name:      "agent_message owner drops empty encrypted_content",
			item:      &Item{ID: "enc_2", Type: "encrypted_content", EncryptedContent: lo.ToPtr("")},
			ownerType: "agent_message",
			validate: func(t *testing.T, result *llm.MessageContentPart, err error) {
				require.NoError(t, err)
				require.Nil(t, result)
			},
		},
		{
			name:      "message owner does not surface encrypted_content as text",
			item:      &Item{ID: "enc_3", Type: "encrypted_content", EncryptedContent: lo.ToPtr("opaque")},
			ownerType: "message",
			validate: func(t *testing.T, result *llm.MessageContentPart, err error) {
				require.NoError(t, err)
				require.Nil(t, result)
			},
		},
		{
			name:      "function_call_output owner does not surface encrypted_content as text",
			item:      &Item{ID: "enc_4", Type: "encrypted_content", EncryptedContent: lo.ToPtr("opaque")},
			ownerType: "function_call_output",
			validate: func(t *testing.T, result *llm.MessageContentPart, err error) {
				require.NoError(t, err)
				require.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertContentItemToPart(tt.item, tt.ownerType)
			tt.validate(t, result, err)
		})
	}
}

func TestConvertItemToMessage_AgentMessage(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		validate func(t *testing.T, result *llm.Message, err error)
	}{
		{
			name: "agent_message with input_text and encrypted_content parts",
			item: &Item{
				ID:   "amsg_1",
				Type: "agent_message",
				Content: &Input{Items: []Item{
					{Type: "input_text", Text: lo.ToPtr("Message Type: NEW_TASK\nTask name: /root/say_hello\nSender: /root\nPayload:\n")},
					{Type: "encrypted_content", EncryptedContent: lo.ToPtr("请用中文向用户打个招呼,做一个简短的自我介绍(说明你是一个子 agent),然后就结束,不要做其他工作。")},
				}},
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "user", result.Role)
				require.Len(t, result.Content.MultipleContent, 2)
				require.Equal(t, "text", result.Content.MultipleContent[0].Type)
				require.Contains(t, *result.Content.MultipleContent[0].Text, "NEW_TASK")
				require.Equal(t, "text", result.Content.MultipleContent[1].Type)
				require.Contains(t, *result.Content.MultipleContent[1].Text, "打个招呼")
			},
		},
		{
			name: "agent_message with only encrypted_content part",
			item: &Item{
				ID:   "amsg_2",
				Type: "agent_message",
				Content: &Input{Items: []Item{
					{Type: "encrypted_content", EncryptedContent: lo.ToPtr("仅一句问候语作为最终答案")},
				}},
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "user", result.Role)
				require.NotNil(t, result.Content.Content)
				require.Contains(t, *result.Content.Content, "问候语")
			},
		},
		{
			name: "agent_message with empty encrypted_content part is dropped",
			item: &Item{
				ID:   "amsg_3",
				Type: "agent_message",
				Content: &Input{Items: []Item{
					{Type: "input_text", Text: lo.ToPtr("header")},
					{Type: "encrypted_content", EncryptedContent: lo.ToPtr("")},
				}},
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "user", result.Role)
				require.NotNil(t, result.Content.Content)
				require.Contains(t, *result.Content.Content, "header")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertItemToMessage(tt.item)
			tt.validate(t, result, err)
		})
	}
}

func TestInboundTransformer_TransformRequest_WithCompactionInput(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		httpReq     *httpclient.Request
		expectError bool
		validate    func(t *testing.T, result *llm.Request)
	}{
		{
			name: "request with compaction input item",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": [
						{
							"type": "message",
							"role": "user",
							"content": "Hello"
						},
						{
							"type": "compaction",
							"id": "compaction_abc",
							"encrypted_content": "base64encoded",
							"created_by": "assistant"
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Messages, 2)

				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(t, "Hello", *result.Messages[0].Content.Content)

				require.Equal(t, "assistant", result.Messages[1].Role)
				require.Len(t, result.Messages[1].Content.MultipleContent, 1)
				require.Equal(t, "compaction", result.Messages[1].Content.MultipleContent[0].Type)
				require.NotNil(t, result.Messages[1].Content.MultipleContent[0].Compact)
				require.Equal(t, "compaction_abc", result.Messages[1].Content.MultipleContent[0].Compact.ID)
				require.Equal(t, "base64encoded", result.Messages[1].Content.MultipleContent[0].Compact.EncryptedContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformRequest(context.Background(), tt.httpReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformRequest_WithAgentMessage(t *testing.T) {
	trans := NewInboundTransformer()

	httpReq := &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": [
				{
					"type": "message",
					"role": "user",
					"content": "请帮我起一个子agent打招呼"
				},
				{
					"type": "agent_message",
					"id": "amsg_1",
					"author": "/root",
					"recipient": "/root/say_hello",
					"content": [
						{"type": "input_text", "text": "Message Type: NEW_TASK\nTask name: /root/say_hello\nSender: /root\nPayload:\n"},
						{"type": "encrypted_content", "encrypted_content": "请用中文向用户打个招呼,做一个简短的自我介绍(说明你是一个子 agent),然后就结束,不要做其他工作。"}
					]
				}
			]
		}`),
	}

	result, err := trans.TransformRequest(context.Background(), httpReq)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Messages, 2)

	require.Equal(t, "user", result.Messages[0].Role)
	require.Equal(t, "请帮我起一个子agent打招呼", *result.Messages[0].Content.Content)

	require.Equal(t, "user", result.Messages[1].Role)
	require.Len(t, result.Messages[1].Content.MultipleContent, 2)
	require.Equal(t, "text", result.Messages[1].Content.MultipleContent[0].Type)
	require.Contains(t, *result.Messages[1].Content.MultipleContent[0].Text, "NEW_TASK")
	require.Equal(t, "text", result.Messages[1].Content.MultipleContent[1].Type)
	require.Contains(t, *result.Messages[1].Content.MultipleContent[1].Text, "打个招呼")
}

func TestInboundTransformer_TransformResponse_WithCompactionContent(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.Response
		expectError bool
		validate    func(t *testing.T, result *httpclient.Response)
	}{
		{
			name: "response with compaction content part",
			chatResp: &llm.Response{
				ID:      "chatcmpl-compact",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "compaction",
										Compact: &llm.CompactContent{
											ID:               "compaction_xyz",
											EncryptedContent: "encrypted_response_data",
											CreatedBy:        lo.ToPtr("model"),
										},
									},
								},
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				require.Equal(t, http.StatusOK, result.StatusCode)

				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.Equal(t, "response", resp.Object)

				require.Len(t, resp.Output, 1)
				compactionOutput := resp.Output[0]
				require.Equal(t, "compaction", compactionOutput.Type)
				require.Equal(t, "compaction_xyz", compactionOutput.ID)
				require.NotNil(t, compactionOutput.EncryptedContent)
				require.Equal(t, "encrypted_response_data", *compactionOutput.EncryptedContent)
				require.NotNil(t, compactionOutput.CreatedBy)
				require.Equal(t, "model", *compactionOutput.CreatedBy)
			},
		},
		{
			name: "response with mixed text and compaction content",
			chatResp: &llm.Response{
				ID:      "chatcmpl-mixed",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "text",
										Text: lo.ToPtr("Here is the response"),
									},
									{
										Type: "compaction",
										Compact: &llm.CompactContent{
											ID:               "compaction_mixed",
											EncryptedContent: "mixed_encrypted",
										},
									},
								},
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				require.Equal(t, http.StatusOK, result.StatusCode)

				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)

				require.Len(t, resp.Output, 2)

				require.Equal(t, "compaction", resp.Output[0].Type)
				require.Equal(t, "compaction_mixed", resp.Output[0].ID)
				require.NotNil(t, resp.Output[0].EncryptedContent)
				require.Equal(t, "mixed_encrypted", *resp.Output[0].EncryptedContent)

				require.Equal(t, "message", resp.Output[1].Type)
				require.Equal(t, "assistant", resp.Output[1].Role)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformResponse(context.Background(), tt.chatResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformError(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name     string
		err      error
		validate func(t *testing.T, result *httpclient.Error)
	}{
		{
			name: "nil error",
			err:  nil,
			validate: func(t *testing.T, result *httpclient.Error) {
				require.Equal(t, http.StatusInternalServerError, result.StatusCode)
			},
		},
		{
			name: "invalid request error",
			err:  transformer.ErrInvalidRequest,
			validate: func(t *testing.T, result *httpclient.Error) {
				require.Equal(t, http.StatusBadRequest, result.StatusCode)
				require.Contains(t, string(result.Body), "invalid_request_error")
			},
		},
		{
			name: "invalid model error",
			err:  transformer.ErrInvalidModel,
			validate: func(t *testing.T, result *httpclient.Error) {
				require.Equal(t, http.StatusUnprocessableEntity, result.StatusCode)
				require.Contains(t, string(result.Body), "invalid_model_error")
			},
		},
		{
			name: "llm response error",
			err: &llm.ResponseError{
				StatusCode: http.StatusTooManyRequests,
				Detail: llm.ErrorDetail{
					Message: "Rate limit exceeded",
					Type:    "rate_limit_error",
					Code:    "rate_limit",
				},
			},
			validate: func(t *testing.T, result *httpclient.Error) {
				require.Equal(t, http.StatusTooManyRequests, result.StatusCode)
				require.Contains(t, string(result.Body), "Rate limit exceeded")
				require.Contains(t, string(result.Body), "rate_limit_error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trans.TransformError(context.Background(), tt.err)
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestConvertToolChoiceToLLM(t *testing.T) {
	tests := []struct {
		name     string
		input    *ToolChoice
		validate func(t *testing.T, result *llm.ToolChoice)
	}{
		{
			name:  "nil input",
			input: nil,
			validate: func(t *testing.T, result *llm.ToolChoice) {
				require.Nil(t, result)
			},
		},
		{
			name: "mode only",
			input: &ToolChoice{
				Mode: lo.ToPtr("auto"),
			},
			validate: func(t *testing.T, result *llm.ToolChoice) {
				require.NotNil(t, result)
				require.NotNil(t, result.ToolChoice)
				require.Equal(t, "auto", *result.ToolChoice)
				require.Nil(t, result.NamedToolChoice)
			},
		},
		{
			name: "specific function",
			input: &ToolChoice{
				Type: lo.ToPtr("function"),
				Name: lo.ToPtr("get_weather"),
			},
			validate: func(t *testing.T, result *llm.ToolChoice) {
				require.NotNil(t, result)
				require.Nil(t, result.ToolChoice)
				require.NotNil(t, result.NamedToolChoice)
				require.Equal(t, "function", result.NamedToolChoice.Type)
				require.Equal(t, "get_weather", result.NamedToolChoice.Function.Name)
			},
		},
		{
			name: "specific non-function tool without name",
			input: &ToolChoice{
				Type: lo.ToPtr("image_generation"),
			},
			validate: func(t *testing.T, result *llm.ToolChoice) {
				require.NotNil(t, result)
				require.Nil(t, result.ToolChoice)
				require.NotNil(t, result.NamedToolChoice)
				require.Equal(t, "image_generation", result.NamedToolChoice.Type)
				require.Empty(t, result.NamedToolChoice.Function.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToolChoiceToLLM(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertToolChoiceToLLMPrimitiveMatrix(t *testing.T) {
	t.Run("named selectors retain primitive identity", func(t *testing.T) {
		tests := []struct {
			toolType string
			name     string
		}{
			{toolType: "function", name: "lookup"},
			{toolType: "custom", name: "apply_patch"},
			{toolType: "namespace", name: "workspace"},
			{toolType: "tool_search", name: "discover"},
			{toolType: "future_client_tool", name: "later"},
			{toolType: "future_server_tool", name: "hosted"},
		}
		for _, tt := range tests {
			t.Run(tt.toolType, func(t *testing.T) {
				result := convertToolChoiceToLLM(&ToolChoice{
					Type: lo.ToPtr(tt.toolType),
					Name: lo.ToPtr(tt.name),
				})
				require.NotNil(t, result)
				require.Nil(t, result.ToolChoice)
				require.NotNil(t, result.NamedToolChoice)
				require.Equal(t, tt.toolType, result.NamedToolChoice.Type)
				require.Equal(t, tt.name, result.NamedToolChoice.Function.Name)
				require.False(t, result.AllowedToolsSet)
				require.Nil(t, result.AllowedTools)
			})
		}
	})

	t.Run("allowed selectors retain same name across primitive types", func(t *testing.T) {
		for _, mode := range []string{"auto", "required"} {
			t.Run(mode, func(t *testing.T) {
				result := convertToolChoiceToLLM(&ToolChoice{
					Type: lo.ToPtr("allowed_tools"),
					Mode: lo.ToPtr(mode),
					Tools: []ToolOption{
						{Type: "function", Name: "same"},
						{Type: "custom", Name: "same"},
						{Type: "namespace", Name: "workspace"},
						{Type: "tool_search", Name: "discover"},
						{Type: "future_client_tool", Name: "later"},
						{Type: "future_server_tool", Name: "hosted"},
					},
				})
				require.NotNil(t, result)
				require.Equal(t, mode, lo.FromPtr(result.ToolChoice))
				require.Nil(t, result.NamedToolChoice)
				require.True(t, result.AllowedToolsSet)
				require.Equal(t, []llm.ToolOption{
					{Type: "function", Name: "same"},
					{Type: "custom", Name: "same"},
					{Type: "namespace", Name: "workspace"},
					{Type: "tool_search", Name: "discover"},
					{Type: "future_client_tool", Name: "later"},
					{Type: "future_server_tool", Name: "hosted"},
				}, result.AllowedTools)
			})
		}
	})

	t.Run("type only selectors retain type without name", func(t *testing.T) {
		for _, toolType := range []string{"web_search", "future_server_tool", "mcp"} {
			t.Run(toolType, func(t *testing.T) {
				result := convertToolChoiceToLLM(&ToolChoice{Type: lo.ToPtr(toolType)})
				require.NotNil(t, result)
				require.Nil(t, result.ToolChoice)
				require.NotNil(t, result.NamedToolChoice)
				require.Equal(t, toolType, result.NamedToolChoice.Type)
				require.Empty(t, result.NamedToolChoice.Function.Name)
				require.False(t, result.AllowedToolsSet)
				require.Nil(t, result.AllowedTools)
			})
		}
	})
}

func TestConvertToMessageContentParts(t *testing.T) {
	tests := []struct {
		name     string
		input    Input
		validate func(t *testing.T, result []llm.MessageContentPart)
	}{
		{
			name:  "text input returns one part",
			input: Input{Text: lo.ToPtr("Hello world")},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "input_text", result[0].Type)
				require.Equal(t, "Hello world", *result[0].Text)
			},
		},
		{
			name:  "single input_text item returns one part",
			input: Input{Items: []Item{{Type: "input_text", Text: lo.ToPtr("Hello world")}}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "Hello world", *result[0].Text)
			},
		},
		{
			name:  "single text item returns one part",
			input: Input{Items: []Item{{Type: "text", Text: lo.ToPtr("Hello world")}}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "Hello world", *result[0].Text)
			},
		},
		{
			name: "multiple items returns multiple parts",
			input: Input{Items: []Item{
				{Type: "input_text", Text: lo.ToPtr("First")},
				{Type: "input_text", Text: lo.ToPtr("Second")},
			}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 2)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "First", *result[0].Text)
				require.Equal(t, "text", result[1].Type)
				require.Equal(t, "Second", *result[1].Text)
			},
		},
		{
			name: "single input_image returns one part",
			input: Input{Items: []Item{
				{Type: "input_image", ImageURL: lo.ToPtr("https://example.com/image.png")},
			}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "image_url", result[0].Type)
				require.NotNil(t, result[0].ImageURL)
				require.Equal(t, "https://example.com/image.png", result[0].ImageURL.URL)
			},
		},
		{
			name: "mixed text and image returns multiple parts",
			input: Input{Items: []Item{
				{Type: "input_text", Text: lo.ToPtr("Look at this image:")},
				{Type: "input_image", ImageURL: lo.ToPtr("https://example.com/image.png")},
			}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 2)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "Look at this image:", *result[0].Text)
				require.Equal(t, "image_url", result[1].Type)
				require.NotNil(t, result[1].ImageURL)
				require.Equal(t, "https://example.com/image.png", result[1].ImageURL.URL)
			},
		},
		{
			name:  "empty items returns empty slice",
			input: Input{Items: []Item{}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Empty(t, result)
			},
		},
		{
			name: "output_text item returns one part",
			input: Input{Items: []Item{
				{Type: "output_text", Text: lo.ToPtr("Generated text")},
			}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "Generated text", *result[0].Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToMessageContentParts(tt.input, "message")
			tt.validate(t, result)
		})
	}
}

func TestConvertToMessageContent(t *testing.T) {
	tests := []struct {
		name     string
		input    Input
		validate func(t *testing.T, result llm.MessageContent)
	}{
		{
			name:  "text input returns simple Content",
			input: Input{Text: lo.ToPtr("Hello world")},
			validate: func(t *testing.T, result llm.MessageContent) {
				require.NotNil(t, result.Content)
				require.Equal(t, "Hello world", *result.Content)
				require.Nil(t, result.MultipleContent)
			},
		},
		{
			name:  "single input_text item returns simple Content",
			input: Input{Items: []Item{{Type: "input_text", Text: lo.ToPtr("Hello world")}}},
			validate: func(t *testing.T, result llm.MessageContent) {
				require.NotNil(t, result.Content)
				require.Equal(t, "Hello world", *result.Content)
				require.Nil(t, result.MultipleContent)
			},
		},
		{
			name: "multiple items returns MultipleContent",
			input: Input{Items: []Item{
				{Type: "input_text", Text: lo.ToPtr("First")},
				{Type: "input_text", Text: lo.ToPtr("Second")},
			}},
			validate: func(t *testing.T, result llm.MessageContent) {
				require.Nil(t, result.Content)
				require.Len(t, result.MultipleContent, 2)
				require.Equal(t, "text", result.MultipleContent[0].Type)
				require.Equal(t, "First", *result.MultipleContent[0].Text)
			},
		},
		{
			name:  "single input_image returns MultipleContent",
			input: Input{Items: []Item{{Type: "input_image", ImageURL: lo.ToPtr("https://example.com/image.png")}}},
			validate: func(t *testing.T, result llm.MessageContent) {
				require.Nil(t, result.Content)
				require.Len(t, result.MultipleContent, 1)
				require.Equal(t, "image_url", result.MultipleContent[0].Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToMessageContent(tt.input, "message")
			tt.validate(t, result)
		})
	}
}

func TestConvertItemToMessage_Reasoning(t *testing.T) {
	// convertItemToMessage returns nil for reasoning items since they are
	// handled by convertReasoningWithFollowing in convertInputToMessages
	item := &Item{
		ID:   "reasoning_123",
		Type: "reasoning",
		Summary: []ReasoningSummary{
			{Type: "summary_text", Text: "First, I need to analyze the problem."},
		},
	}

	result, err := convertItemToMessage(item)
	require.NoError(t, err)
	require.Nil(t, result, "reasoning items should return nil from convertItemToMessage")
}

func TestConvertInputToMessages_GroupsConsecutiveMixedToolCalls(t *testing.T) {
	input := &Input{Items: []Item{
		{Type: "function_call", CallID: "function:1", Name: "lookup", Arguments: `{"id":"42"}`},
		{Type: "custom_tool_call", CallID: "custom:2", Name: "apply_patch", Input: lo.ToPtr("patch")},
		{Type: "tool_search_call", CallID: "search:3", Execution: "client", Arguments: `{"query":"agents"}`},
	}}

	messages, err := convertInputToMessages(input)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "assistant", messages[0].Role)
	require.Len(t, messages[0].ToolCalls, 3)
	require.Equal(t, []string{"function:1", "custom:2", "search:3"}, []string{
		messages[0].ToolCalls[0].ID,
		messages[0].ToolCalls[1].ID,
		messages[0].ToolCalls[2].ID,
	})
	require.Equal(t, []int{0, 1, 2}, []int{
		messages[0].ToolCalls[0].Index,
		messages[0].ToolCalls[1].Index,
		messages[0].ToolCalls[2].Index,
	})
	require.Equal(t, "lookup", messages[0].ToolCalls[0].Function.Name)
	require.NotNil(t, messages[0].ToolCalls[1].ResponseCustomToolCall)
	require.Equal(t, "apply_patch", messages[0].ToolCalls[1].ResponseCustomToolCall.Name)
	require.NotNil(t, messages[0].ToolCalls[2].ResponseToolSearchCall)
	require.Equal(t, "client", messages[0].ToolCalls[2].ResponseToolSearchCall.Execution)
}

func TestConvertInputToMessages_StopsToolCallGroupsAtBoundaries(t *testing.T) {
	functionCall := func(callID string) Item {
		return Item{Type: "function_call", CallID: callID, Name: "lookup", Arguments: `{}`}
	}
	customCall := func(callID string) Item {
		return Item{Type: "custom_tool_call", CallID: callID, Name: "apply_patch", Input: lo.ToPtr("patch")}
	}
	toolSearchCall := func(callID string) Item {
		return Item{Type: "tool_search_call", CallID: callID, Execution: "client", Arguments: `{}`}
	}

	tests := []struct {
		name      string
		items     []Item
		wantRoles []string
	}{
		{
			name: "function output",
			items: []Item{
				functionCall("call:1"),
				{Type: "function_call_output", CallID: "call:1", Output: &Input{Text: lo.ToPtr("result")}},
				functionCall("call:2"),
			},
			wantRoles: []string{"assistant", "tool", "assistant"},
		},
		{
			name: "custom output",
			items: []Item{
				customCall("call:1"),
				{Type: "custom_tool_call_output", CallID: "call:1", Output: &Input{Text: lo.ToPtr("result")}},
				functionCall("call:2"),
			},
			wantRoles: []string{"assistant", "tool", "assistant"},
		},
		{
			name: "tool search output",
			items: []Item{
				toolSearchCall("call:1"),
				{Type: "tool_search_output", CallID: "call:1", Tools: []Tool{}},
				functionCall("call:2"),
			},
			wantRoles: []string{"assistant", "tool", "assistant"},
		},
		{
			name: "user message",
			items: []Item{
				functionCall("call:1"),
				{Type: "message", Role: "user", Content: &Input{Text: lo.ToPtr("next turn")}},
				customCall("call:2"),
			},
			wantRoles: []string{"assistant", "user", "assistant"},
		},
		{
			name: "reasoning",
			items: []Item{
				functionCall("call:1"),
				{Type: "reasoning", Summary: []ReasoningSummary{{Type: "summary_text", Text: "next turn"}}},
				customCall("call:2"),
			},
			wantRoles: []string{"assistant", "assistant"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := convertInputToMessages(&Input{Items: tt.items})
			require.NoError(t, err)
			require.Len(t, messages, len(tt.wantRoles))
			roles := make([]string, len(messages))
			for i := range messages {
				roles[i] = messages[i].Role
			}
			require.Equal(t, tt.wantRoles, roles)
			require.Equal(t, "call:1", messages[0].ToolCalls[0].ID)
			require.Equal(t, "call:2", messages[len(messages)-1].ToolCalls[0].ID)
		})
	}
}

func TestConvertReasoningWithFollowing(t *testing.T) {
	tests := []struct {
		name     string
		items    []Item
		startIdx int
		validate func(t *testing.T, result *llm.Message, consumed int, err error)
	}{
		{
			name: "reasoning item with summary only",
			items: []Item{
				{
					ID:   "reasoning_123",
					Type: "reasoning",
					Summary: []ReasoningSummary{
						{Type: "summary_text", Text: "First, I need to analyze the problem."},
						{Type: "summary_text", Text: " Then, I will solve it step by step."},
					},
				},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 1, consumed)
				require.Equal(t, "assistant", result.Role)
				require.NotNil(t, result.ReasoningContent)
				require.Equal(t, "First, I need to analyze the problem. Then, I will solve it step by step.", *result.ReasoningContent)
			},
		},
		{
			name: "reasoning item with encrypted content",
			items: []Item{
				{
					ID:   "reasoning_456",
					Type: "reasoning",
					Summary: []ReasoningSummary{
						{Type: "summary_text", Text: "Reasoning summary"},
					},
					EncryptedContent: lo.ToPtr("encrypted_data_here"),
				},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 1, consumed)
				require.Equal(t, "assistant", result.Role)
				require.NotNil(t, result.ReasoningContent)
				require.Equal(t, "Reasoning summary", *result.ReasoningContent)
				require.NotNil(t, result.ReasoningSignature)
				require.Equal(t, "encrypted_data_here", *result.ReasoningSignature)
			},
		},
		{
			name: "reasoning item with empty summary",
			items: []Item{
				{
					ID:      "reasoning_789",
					Type:    "reasoning",
					Summary: []ReasoningSummary{},
				},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 1, consumed)
				require.Equal(t, "assistant", result.Role)
				require.Nil(t, result.ReasoningContent)
			},
		},
		{
			name: "reasoning merged with function_call",
			items: []Item{
				{
					ID:   "reasoning_001",
					Type: "reasoning",
					Summary: []ReasoningSummary{
						{Type: "summary_text", Text: "I need to call the function."},
					},
				},
				{
					Type:      "function_call",
					CallID:    "call_123",
					Name:      "get_weather",
					Arguments: `{"location": "Tokyo"}`,
				},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 2, consumed)
				require.Equal(t, "assistant", result.Role)
				require.NotNil(t, result.ReasoningContent)
				require.Equal(t, "I need to call the function.", *result.ReasoningContent)
				require.Len(t, result.ToolCalls, 1)
				require.Equal(t, "call_123", result.ToolCalls[0].ID)
				require.Equal(t, "get_weather", result.ToolCalls[0].Function.Name)
			},
		},
		{
			name: "consecutive reasoning items merged with function_call",
			items: []Item{
				{
					ID:               "rs_first",
					Type:             "reasoning",
					Summary:          []ReasoningSummary{{Type: "summary_text", Text: "first"}},
					EncryptedContent: lo.ToPtr("gAAAA_FIRST_BLOB"),
				},
				{
					ID:               "rs_second",
					Type:             "reasoning",
					Summary:          []ReasoningSummary{{Type: "summary_text", Text: "second"}},
					EncryptedContent: lo.ToPtr("gAAAA_SECOND_BLOB"),
				},
				{
					Type:      "function_call",
					CallID:    "call_123",
					Name:      "get_weather",
					Arguments: `{}`,
				},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 3, consumed)
				require.Equal(t, []llm.ReasoningItem{
					{ID: "rs_first", Content: "first", Signature: "gAAAA_FIRST_BLOB"},
					{ID: "rs_second", Content: "second", Signature: "gAAAA_SECOND_BLOB"},
				}, result.ReasoningItems)
				require.Equal(t, "firstsecond", lo.FromPtr(result.ReasoningContent))
				require.Equal(t, "gAAAA_SECOND_BLOB", lo.FromPtr(result.ReasoningSignature))
				require.Len(t, result.ToolCalls, 1)
				require.Equal(t, "call_123", result.ToolCalls[0].ID)
			},
		},
		{
			name: "reasoning merged with parallel mixed tool calls",
			items: []Item{
				{
					ID:      "reasoning_parallel",
					Type:    "reasoning",
					Summary: []ReasoningSummary{{Type: "summary_text", Text: "use parallel tools"}},
				},
				{Type: "function_call", CallID: "call_function", Name: "lookup", Arguments: `{}`},
				{Type: "custom_tool_call", CallID: "call_custom", Name: "apply_patch", Input: lo.ToPtr("patch")},
				{Type: "tool_search_call", CallID: "call_search", Execution: "client", Arguments: `{}`},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 4, consumed)
				require.Len(t, result.ToolCalls, 3)
				require.Equal(t, []int{0, 1, 2}, []int{
					result.ToolCalls[0].Index,
					result.ToolCalls[1].Index,
					result.ToolCalls[2].Index,
				})
			},
		},
		{
			name: "reasoning merged with assistant text message",
			items: []Item{
				{
					ID:   "reasoning_002",
					Type: "reasoning",
					Summary: []ReasoningSummary{
						{Type: "summary_text", Text: "Thinking about the answer."},
					},
				},
				{
					Type: "message",
					Role: "assistant",
					Text: lo.ToPtr("The answer is 42."),
				},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 2, consumed)
				require.Equal(t, "assistant", result.Role)
				require.NotNil(t, result.ReasoningContent)
				require.Equal(t, "Thinking about the answer.", *result.ReasoningContent)
				require.NotNil(t, result.Content.Content)
				require.Equal(t, "The answer is 42.", *result.Content.Content)
			},
		},
		{
			name: "reasoning stops at user message",
			items: []Item{
				{
					ID:   "reasoning_003",
					Type: "reasoning",
					Summary: []ReasoningSummary{
						{Type: "summary_text", Text: "Thinking..."},
					},
				},
				{
					Type: "message",
					Role: "user",
					Text: lo.ToPtr("Next question"),
				},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 1, consumed)
				require.Equal(t, "assistant", result.Role)
				require.NotNil(t, result.ReasoningContent)
				require.Equal(t, "Thinking...", *result.ReasoningContent)
				require.Empty(t, result.ToolCalls)
			},
		},
		{
			name: "reasoning stops at function_call_output",
			items: []Item{
				{
					ID:   "reasoning_004",
					Type: "reasoning",
					Summary: []ReasoningSummary{
						{Type: "summary_text", Text: "Thinking..."},
					},
				},
				{
					Type:   "function_call_output",
					CallID: "call_456",
					Output: &Input{Text: lo.ToPtr("result")},
				},
			},
			startIdx: 0,
			validate: func(t *testing.T, result *llm.Message, consumed int, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 1, consumed)
				require.Equal(t, "assistant", result.Role)
				require.NotNil(t, result.ReasoningContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, consumed, err := convertReasoningWithFollowing(tt.items, tt.startIdx)
			tt.validate(t, result, consumed, err)
		})
	}
}

func TestBuildReasoningItems_OmitsEmptyEncryptedContent(t *testing.T) {
	items := buildReasoningItems(llm.Message{
		ReasoningItems: []llm.ReasoningItem{{ID: "rs_summary", Content: "summary only"}},
	})

	require.Len(t, items, 1)
	require.Equal(t, "rs_summary", items[0].ID)
	require.Nil(t, items[0].EncryptedContent)
}

func TestInboundTransformer_TransformRequest_WithReasoningInput(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		httpReq     *httpclient.Request
		expectError bool
		validate    func(t *testing.T, result *llm.Request)
	}{
		{
			name: "request with reasoning input item merged with assistant",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": [
						{
							"type": "message",
							"role": "user",
							"content": "What is 2+2?"
						},
						{
							"type": "reasoning",
							"id": "reasoning_abc",
							"summary": [
								{"type": "summary_text", "text": "Let me think about this math problem."}
							]
						},
						{
							"type": "message",
							"role": "assistant",
							"content": "The answer is 4."
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "o3", result.Model)
				// Reasoning + assistant message are merged into one message
				require.Len(t, result.Messages, 2)

				// First message: user
				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(t, "What is 2+2?", *result.Messages[0].Content.Content)

				// Second message: assistant with merged reasoning and text content
				require.Equal(t, "assistant", result.Messages[1].Role)
				require.NotNil(t, result.Messages[1].ReasoningContent)
				require.Equal(t, "Let me think about this math problem.", *result.Messages[1].ReasoningContent)
				require.NotNil(t, result.Messages[1].Content.Content)
				require.Equal(t, "The answer is 4.", *result.Messages[1].Content.Content)
			},
		},
		{
			name: "request with reasoning config",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": "Solve this complex problem",
					"reasoning": {
						"effort": "high",
						"summary": "detailed",
						"max_tokens": 10000
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "o3", result.Model)
				require.Equal(t, "high", result.ReasoningEffort)
				require.NotNil(t, result.ReasoningBudget)
				require.Equal(t, int64(10000), *result.ReasoningBudget)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformRequest(context.Background(), tt.httpReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestConvertToResponsesAPIResponse_AttachesAnnotationsToFirstTextItem(t *testing.T) {
	resp := convertToResponsesAPIResponse(&llm.Response{
		ID:      "resp_annotations",
		Created: 1677652288,
		Model:   "gpt-4o",
		Choices: []llm.Choice{{
			Message: &llm.Message{
				ID:   "msg_annotations",
				Role: "assistant",
				Annotations: []llm.Annotation{
					{
						Type:       "url_citation",
						StartIndex: lo.ToPtr(int64(0)),
						EndIndex:   lo.ToPtr(int64(5)),
						URLCitation: &llm.URLCitation{
							URL:   "https://example.com",
							Title: "Example",
						},
					},
				},
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("Hello")},
						{Type: "text", Text: lo.ToPtr(" world")},
					},
				},
			},
			FinishReason: lo.ToPtr("stop"),
		}},
	})

	require.Len(t, resp.Output, 1)
	require.NotNil(t, resp.Output[0].Content)
	require.Len(t, resp.Output[0].Content.Items, 2)
	require.Len(t, resp.Output[0].Content.Items[0].Annotations, 1)
	require.Empty(t, resp.Output[0].Content.Items[1].Annotations)
	require.Equal(t, "url_citation", resp.Output[0].Content.Items[0].Annotations[0].Type)
	require.NotNil(t, resp.Output[0].Content.Items[0].Annotations[0].URLCitation)
	require.Equal(t, "https://example.com", resp.Output[0].Content.Items[0].Annotations[0].URLCitation.URL)
}

func TestConvertToResponsesAPIResponse_PreservesMultipleReasoningItems(t *testing.T) {
	resp := convertToResponsesAPIResponse(&llm.Response{
		ID:      "resp_reasoning_items",
		Model:   "gpt-5",
		Created: 1,
		Choices: []llm.Choice{{
			Message: &llm.Message{
				Role: "assistant",
				ReasoningItems: []llm.ReasoningItem{
					{ID: "rs_first", Content: "first", Signature: "gAAAA_FIRST_BLOB"},
					{ID: "rs_second", Content: "second", Signature: "gAAAA_SECOND_BLOB"},
				},
				ToolCalls: []llm.ToolCall{{
					ID:   "call_tool",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "lookup",
						Arguments: "{}",
					},
				}},
			},
		}},
	})

	require.Len(t, resp.Output, 3)
	require.Equal(t, "reasoning", resp.Output[0].Type)
	require.Equal(t, "rs_first", resp.Output[0].ID)
	require.Len(t, resp.Output[0].Summary, 1)
	require.Equal(t, "first", resp.Output[0].Summary[0].Text)
	require.NotNil(t, resp.Output[0].EncryptedContent)
	require.Equal(t, "gAAAA_FIRST_BLOB", *resp.Output[0].EncryptedContent)

	require.Equal(t, "reasoning", resp.Output[1].Type)
	require.Equal(t, "rs_second", resp.Output[1].ID)
	require.Len(t, resp.Output[1].Summary, 1)
	require.Equal(t, "second", resp.Output[1].Summary[0].Text)
	require.NotNil(t, resp.Output[1].EncryptedContent)
	require.Equal(t, "gAAAA_SECOND_BLOB", *resp.Output[1].EncryptedContent)
	require.Equal(t, "function_call", resp.Output[2].Type)
}

func TestInboundTransformer_TransformResponse_WithReasoningContent(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.Response
		expectError bool
		validate    func(t *testing.T, result *httpclient.Response)
	}{
		{
			name: "response with reasoning content",
			chatResp: &llm.Response{
				ID:      "chatcmpl-reasoning",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "o3",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role:               "assistant",
							ReasoningContent:   lo.ToPtr("I analyzed the problem step by step."),
							ReasoningSignature: lo.ToPtr("encrypted_data_here"),
							Content: llm.MessageContent{
								Content: lo.ToPtr("The answer is 42."),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     50,
					CompletionTokens: 100,
					TotalTokens:      150,
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						ReasoningTokens: 80,
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				require.Equal(t, http.StatusOK, result.StatusCode)

				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.Equal(t, "response", resp.Object)
				require.Equal(t, "o3", resp.Model)

				// Should have reasoning output item and message output item
				require.Len(t, resp.Output, 2)

				// First output should be reasoning
				reasoningOutput := resp.Output[0]
				require.Equal(t, "reasoning", reasoningOutput.Type)
				require.Len(t, reasoningOutput.Summary, 1)
				require.Equal(t, "summary_text", reasoningOutput.Summary[0].Type)
				require.Equal(t, "I analyzed the problem step by step.", reasoningOutput.Summary[0].Text)
				require.NotNil(t, reasoningOutput.EncryptedContent)
				require.Equal(t, "encrypted_data_here", *reasoningOutput.EncryptedContent)

				// Second output should be message
				messageOutput := resp.Output[1]
				require.Equal(t, "message", messageOutput.Type)
				require.Equal(t, "assistant", messageOutput.Role)

				// Check usage includes reasoning tokens
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(80), resp.Usage.OutputTokenDetails.ReasoningTokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformResponse(context.Background(), tt.chatResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformRequest_MergesInterleavedToolOutputs(t *testing.T) {
	trans := NewInboundTransformer()

	result, err := trans.TransformRequest(context.Background(), &httpclient.Request{
		Body: []byte(`{
			"model": "gpt-4o",
			"input": [
				{"role": "user", "content": "run both"},
				{"type": "function_call", "call_id": "call_a", "name": "fn", "arguments": "{}"},
				{"type": "function_call", "call_id": "call_b", "name": "fn", "arguments": "{}"},
				{"type": "function_call_output", "call_id": "call_a", "output": "a1"},
				{"type": "function_call_output", "call_id": "call_b", "output": "b1"},
				{"type": "function_call_output", "call_id": "call_a", "output": "a2"},
				{"role": "user", "content": "next"}
			]
		}`),
	})

	require.NoError(t, err)
	// user, assistant(tool_calls), tool(call_a), tool(call_b), user = 5.
	require.Len(t, result.Messages, 5)
	require.Equal(t, "assistant", result.Messages[1].Role)
	require.Len(t, result.Messages[1].ToolCalls, 2)

	seen := make(map[string]int)
	for _, message := range result.Messages {
		if message.Role == "tool" {
			seen[lo.FromPtr(message.ToolCallID)]++
		}
	}
	require.Equal(t, map[string]int{"call_a": 1, "call_b": 1}, seen)

	require.Equal(t, "tool", result.Messages[2].Role)
	require.Equal(t, "call_a", lo.FromPtr(result.Messages[2].ToolCallID))
	require.Equal(t, "a1a2", flattenToolContent(result.Messages[2].Content))
	require.Equal(t, "tool", result.Messages[3].Role)
	require.Equal(t, "call_b", lo.FromPtr(result.Messages[3].ToolCallID))
	require.Equal(t, "b1", flattenToolContent(result.Messages[3].Content))
}
