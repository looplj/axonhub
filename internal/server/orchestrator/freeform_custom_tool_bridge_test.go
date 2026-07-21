package orchestrator

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
	anthropictransformer "github.com/looplj/axonhub/llm/transformer/anthropic"
	chattransformer "github.com/looplj/axonhub/llm/transformer/openai"
	responsestransformer "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestPersistentOutboundTransformer_BridgesResponsesCustomToolToPortableChatFunction(t *testing.T) {
	processor := newFreeformCustomToolBridgeProcessor(t, llm.APIFormatOpenAIChatCompletion)
	request := responsesCustomToolRequest()

	httpReq, err := processor.TransformRequest(t.Context(), request)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name       string          `json:"name"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpReq.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "function", payload.Tools[0].Type)
	require.Equal(t, "exec", payload.Tools[0].Function.Name)
	require.JSONEq(t, `{"type":"object","properties":{"input":{"type":"string","description":"The complete freeform input for the custom tool. Do not wrap it in Markdown."}},"required":["input"],"additionalProperties":false}`, string(payload.Tools[0].Function.Parameters))
	require.Contains(t, llm.LossyDowngrades(request), llm.LossyDowngrade{
		SourceProtocol: llm.APIFormatOpenAIResponse,
		SourceField:    "tools[].type=custom.format",
		TargetProtocol: llm.APIFormatOpenAIChatCompletion,
		Reason:         llm.LossyDowngradeReasonNoEquivalentSemantics,
		Severity:       llm.LossyDowngradeSeverityWarning,
	})
}

func TestPersistentOutboundTransformer_BridgesResponsesCustomToolToAnthropicFunction(t *testing.T) {
	processor := newFreeformCustomToolBridgeProcessor(t, llm.APIFormatAnthropicMessage)
	request := responsesCustomToolRequest()

	httpReq, err := processor.TransformRequest(t.Context(), request)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"input_schema"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpReq.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "exec", payload.Tools[0].Name)
	require.JSONEq(t, `{"type":"object","properties":{"input":{"type":"string","description":"The complete freeform input for the custom tool. Do not wrap it in Markdown."}},"required":["input"],"additionalProperties":false}`, string(payload.Tools[0].InputSchema))
	require.Contains(t, llm.LossyDowngrades(request), llm.LossyDowngrade{
		SourceProtocol: llm.APIFormatOpenAIResponse,
		SourceField:    "tools[].type=custom.format",
		TargetProtocol: llm.APIFormatAnthropicMessage,
		Reason:         llm.LossyDowngradeReasonNoEquivalentSemantics,
		Severity:       llm.LossyDowngradeSeverityWarning,
	})
}

func TestPersistentOutboundTransformer_PreservesNativeChatCustomToolForVerifiedEndpoint(t *testing.T) {
	processor := newFreeformCustomToolBridgeProcessor(t, llm.APIFormatOpenAIChatCompletion)
	processor.state.ChannelModelsCandidates[0].Channel.Endpoints[0].SupportsOpenAIChatCustomTools = true

	httpReq, err := processor.TransformRequest(t.Context(), responsesCustomToolRequest())
	require.NoError(t, err)
	require.Nil(t, processor.customToolBridge)

	var payload struct {
		Tools []json.RawMessage `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpReq.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.JSONEq(t, `{"type":"custom","custom":{"name":"exec","description":"Run raw JavaScript.","format":{"type":"text"}}}`, string(payload.Tools[0]))
}

func TestPersistentOutboundTransformer_BridgesCustomToolHistoryForChatAndAnthropic(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIChatCompletion, llm.APIFormatAnthropicMessage} {
		t.Run(format.String(), func(t *testing.T) {
			processor := newFreeformCustomToolBridgeProcessor(t, format)
			request := responsesCustomToolRequest()
			request.Messages = append(request.Messages,
				llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{{
						ID:   "call_exec_history",
						Type: llm.ToolTypeResponsesCustomTool,
						ResponseCustomToolCall: &llm.ResponseCustomToolCall{
							CallID: "call_exec_history",
							Name:   "exec",
							Input:  "await tools.exec_command({cmd: 'pwd'})",
						},
					}},
				},
				llm.Message{
					Role:       "tool",
					ToolCallID: lo.ToPtr("call_exec_history"),
					Content:    llm.MessageContent{Content: lo.ToPtr("/workspace")},
				},
			)

			httpReq, err := processor.TransformRequest(t.Context(), request)
			require.NoError(t, err)

			if format == llm.APIFormatOpenAIChatCompletion {
				var payload struct {
					Messages []struct {
						Role       string            `json:"role"`
						ToolCallID *string           `json:"tool_call_id"`
						ToolCalls  []json.RawMessage `json:"tool_calls"`
					} `json:"messages"`
				}
				require.NoError(t, json.Unmarshal(httpReq.Body, &payload))
				require.Len(t, payload.Messages, 3)
				require.JSONEq(t, `[{"id":"call_exec_history","type":"function","function":{"name":"exec","arguments":"{\"input\":\"await tools.exec_command({cmd: 'pwd'})\"}"},"index":0}]`, string(mustMarshalJSON(t, payload.Messages[1].ToolCalls)))
				require.Equal(t, lo.ToPtr("call_exec_history"), payload.Messages[2].ToolCallID)
				return
			}

			var payload struct {
				Messages []struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				} `json:"messages"`
			}
			require.NoError(t, json.Unmarshal(httpReq.Body, &payload))
			require.Len(t, payload.Messages, 3)
			require.JSONEq(t, `[{"type":"tool_use","id":"call_exec_history","name":"exec","input":{"input":"await tools.exec_command({cmd: 'pwd'})"}}]`, string(payload.Messages[1].Content))
			require.JSONEq(t, `[{"type":"tool_result","tool_use_id":"call_exec_history","content":"/workspace"}]`, string(payload.Messages[2].Content))
		})
	}
}

func TestPersistentOutboundTransformer_BridgesNamedCustomToolChoice(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIChatCompletion, llm.APIFormatAnthropicMessage} {
		t.Run(format.String(), func(t *testing.T) {
			processor := newFreeformCustomToolBridgeProcessor(t, format)
			request := responsesCustomToolRequest()
			request.ToolChoice = &llm.ToolChoice{
				OpenAIChatCustomToolChoice: &llm.OpenAIChatCustomToolChoice{Name: "exec"},
			}

			httpReq, err := processor.TransformRequest(t.Context(), request)
			require.NoError(t, err)

			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(httpReq.Body, &payload))
			if format == llm.APIFormatOpenAIChatCompletion {
				require.JSONEq(t, `{"type":"function","function":{"name":"exec"}}`, string(payload["tool_choice"]))
			} else {
				require.JSONEq(t, `{"type":"tool","name":"exec"}`, string(payload["tool_choice"]))
			}
		})
	}
}

func TestPersistentOutboundTransformer_RejectsAmbiguousCustomAndFunctionNames(t *testing.T) {
	processor := newFreeformCustomToolBridgeProcessor(t, llm.APIFormatOpenAIChatCompletion)
	request := responsesCustomToolRequest()
	request.Tools = append(request.Tools, llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.Function{
			Name:       "exec",
			Parameters: json.RawMessage(`{"type":"object"}`),
		},
	})

	result, err := processor.TransformRequest(t.Context(), request)
	require.Nil(t, result)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.ErrorContains(t, err, `custom tool "exec" conflicts with an existing function tool`)
}

func TestPersistentOutboundTransformer_RehydratesChatFunctionCallAsResponsesCustomCall(t *testing.T) {
	processor := newFreeformCustomToolBridgeProcessor(t, llm.APIFormatOpenAIChatCompletion)
	httpReq, err := processor.TransformRequest(t.Context(), responsesCustomToolRequest())
	require.NoError(t, err)

	response, err := processor.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: 200,
		Request:    httpReq,
		Body: []byte(`{
			"id":"chatcmpl-freeform",
			"object":"chat.completion",
			"created":1,
			"model":"grok-4.5",
			"choices":[{
				"index":0,
				"message":{"role":"assistant","tool_calls":[{
					"id":"call_exec_1",
					"type":"function",
					"function":{"name":"exec","arguments":"{\"input\":\"await tools.exec_command({cmd: 'pwd'})\"}"}
				}]},
				"finish_reason":"tool_calls"
			}]
		}`),
	})
	require.NoError(t, err)
	require.Len(t, response.Choices, 1)
	require.Len(t, response.Choices[0].Message.ToolCalls, 1)

	toolCall := response.Choices[0].Message.ToolCalls[0]
	require.Equal(t, llm.ToolTypeResponsesCustomTool, toolCall.Type)
	require.NotNil(t, toolCall.ResponseCustomToolCall)
	require.Equal(t, "call_exec_1", toolCall.ResponseCustomToolCall.CallID)
	require.Equal(t, "exec", toolCall.ResponseCustomToolCall.Name)
	require.Equal(t, "await tools.exec_command({cmd: 'pwd'})", toolCall.ResponseCustomToolCall.Input)
}

func TestPersistentOutboundTransformer_RehydratesAnthropicToolUseAsResponsesCustomCall(t *testing.T) {
	processor := newFreeformCustomToolBridgeProcessor(t, llm.APIFormatAnthropicMessage)
	httpReq, err := processor.TransformRequest(t.Context(), responsesCustomToolRequest())
	require.NoError(t, err)

	response, err := processor.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: 200,
		Request:    httpReq,
		Body: []byte(`{
			"id":"msg_freeform",
			"type":"message",
			"role":"assistant",
			"model":"claude-test",
			"content":[{
				"type":"tool_use",
				"id":"toolu_exec_1",
				"name":"exec",
				"input":{"input":"await tools.exec_command({cmd: 'pwd'})"}
			}],
			"stop_reason":"tool_use",
			"usage":{"input_tokens":1,"output_tokens":1}
		}`),
	})
	require.NoError(t, err)
	require.Len(t, response.Choices, 1)
	require.Len(t, response.Choices[0].Message.ToolCalls, 1)

	toolCall := response.Choices[0].Message.ToolCalls[0]
	require.Equal(t, llm.ToolTypeResponsesCustomTool, toolCall.Type)
	require.NotNil(t, toolCall.ResponseCustomToolCall)
	require.Equal(t, "toolu_exec_1", toolCall.ResponseCustomToolCall.CallID)
	require.Equal(t, "exec", toolCall.ResponseCustomToolCall.Name)
	require.Equal(t, "await tools.exec_command({cmd: 'pwd'})", toolCall.ResponseCustomToolCall.Input)
}

func TestPersistentOutboundTransformer_BridgedProviderCallsReachResponsesClientAsCustomCalls(t *testing.T) {
	tests := []struct {
		name   string
		format llm.APIFormat
		body   string
		callID string
	}{
		{
			name:   "chat function",
			format: llm.APIFormatOpenAIChatCompletion,
			callID: "call_chat_exec",
			body: `{
				"id":"chatcmpl-freeform","object":"chat.completion","created":1,"model":"grok-4.5",
				"choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{
					"id":"call_chat_exec","type":"function",
					"function":{"name":"exec","arguments":"{\"input\":\"await tools.exec_command({cmd: 'pwd'})\"}"}
				}]},"finish_reason":"tool_calls"}]
			}`,
		},
		{
			name:   "anthropic tool use",
			format: llm.APIFormatAnthropicMessage,
			callID: "toolu_anthropic_exec",
			body: `{
				"id":"msg-freeform","type":"message","role":"assistant","model":"claude-test",
				"content":[{"type":"tool_use","id":"toolu_anthropic_exec","name":"exec","input":{"input":"await tools.exec_command({cmd: 'pwd'})"}}],
				"stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			processor := newFreeformCustomToolBridgeProcessor(t, test.format)
			httpReq, err := processor.TransformRequest(t.Context(), responsesCustomToolRequest())
			require.NoError(t, err)

			llmResponse, err := processor.TransformResponse(t.Context(), &httpclient.Response{
				StatusCode: 200,
				Request:    httpReq,
				Body:       []byte(test.body),
			})
			require.NoError(t, err)

			responsesInbound := responsestransformer.NewInboundTransformer()
			clientResponse, err := responsesInbound.TransformResponse(t.Context(), llmResponse)
			require.NoError(t, err)

			var payload struct {
				Output []struct {
					Type   string  `json:"type"`
					CallID string  `json:"call_id"`
					Name   string  `json:"name"`
					Input  *string `json:"input"`
				} `json:"output"`
			}
			require.NoError(t, json.Unmarshal(clientResponse.Body, &payload))
			require.Len(t, payload.Output, 1)
			require.Equal(t, "custom_tool_call", payload.Output[0].Type)
			require.Equal(t, test.callID, payload.Output[0].CallID)
			require.Equal(t, "exec", payload.Output[0].Name)
			require.Equal(t, lo.ToPtr("await tools.exec_command({cmd: 'pwd'})"), payload.Output[0].Input)
		})
	}
}

func TestFreeformCustomToolBridgeStream_RehydratesFunctionArgumentChunks(t *testing.T) {
	bridge := shared.NewFreeformCustomToolBridgeFromFunctionNames(map[string]string{
		"exec": "exec",
	})
	finishReason := "tool_calls"
	stream := newFreeformCustomToolBridgeStream(streams.SliceStream([]*llm.Response{
		{
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
					ID:       "call_stream_exec",
					Index:    0,
					Type:     llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "exec"},
				}}},
			}},
		},
		{
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
					Index:    0,
					Type:     llm.ToolTypeFunction,
					Function: llm.FunctionCall{Arguments: `{"input":"await tools.`},
				}}},
			}},
		},
		{
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
					Index:    0,
					Type:     llm.ToolTypeFunction,
					Function: llm.FunctionCall{Arguments: `exec_command()"}`},
				}}},
			}},
		},
		{Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{}, FinishReason: &finishReason}}},
	}), bridge)

	var inputs []string
	for stream.Next() {
		response := stream.Current()
		if response == nil || len(response.Choices) == 0 || response.Choices[0].Delta == nil || len(response.Choices[0].Delta.ToolCalls) == 0 {
			continue
		}
		toolCall := response.Choices[0].Delta.ToolCalls[0]
		require.Equal(t, llm.ToolTypeResponsesCustomTool, toolCall.Type)
		require.Empty(t, toolCall.Function.Name)
		require.NotNil(t, toolCall.ResponseCustomToolCall)
		if toolCall.ResponseCustomToolCall.Input != "" {
			inputs = append(inputs, toolCall.ResponseCustomToolCall.Input)
		}
	}
	require.NoError(t, stream.Err())
	require.Equal(t, []string{"await tools.exec_command()"}, inputs)
}

func TestFreeformCustomToolBridgeStream_ProducesResponsesCustomToolEvents(t *testing.T) {
	bridge := shared.NewFreeformCustomToolBridgeFromFunctionNames(map[string]string{
		"exec": "exec",
	})
	finishReason := "tool_calls"
	providerStream := newFreeformCustomToolBridgeStream(streams.SliceStream([]*llm.Response{
		{
			ID:      "chatcmpl-stream-freeform",
			Object:  "chat.completion.chunk",
			Created: 1,
			Model:   "grok-4.5",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
					ID:       "call_stream_exec",
					Index:    0,
					Type:     llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "exec", Arguments: `{"input":"await tools.exec_command()"}`},
				}}},
			}},
		},
		{ID: "chatcmpl-stream-freeform", Object: "chat.completion.chunk", Model: "grok-4.5", Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{}, FinishReason: &finishReason}}},
	}), bridge)

	responsesInbound := responsestransformer.NewInboundTransformer()
	clientStream, err := responsesInbound.TransformStream(t.Context(), providerStream)
	require.NoError(t, err)

	var doneItem struct {
		Type   string  `json:"type"`
		CallID string  `json:"call_id"`
		Name   string  `json:"name"`
		Input  *string `json:"input"`
	}
	found := false
	for clientStream.Next() {
		var event struct {
			Type string `json:"type"`
			Item *struct {
				Type   string  `json:"type"`
				CallID string  `json:"call_id"`
				Name   string  `json:"name"`
				Input  *string `json:"input"`
			} `json:"item"`
		}
		require.NoError(t, json.Unmarshal(clientStream.Current().Data, &event))
		if event.Type == "response.output_item.done" && event.Item != nil && event.Item.Type == "custom_tool_call" {
			doneItem = *event.Item
			found = true
		}
	}
	require.NoError(t, clientStream.Err())
	require.True(t, found, "expected a Responses custom_tool_call output item")
	require.Equal(t, "call_stream_exec", doneItem.CallID)
	require.Equal(t, "exec", doneItem.Name)
	require.NotNil(t, doneItem.Input)
	require.Equal(t, "await tools.exec_command()", *doneItem.Input)
}

func newFreeformCustomToolBridgeProcessor(t *testing.T, format llm.APIFormat) *PersistentOutboundTransformer {
	t.Helper()

	var outbound transformer.Outbound
	switch format {
	case llm.APIFormatOpenAIChatCompletion:
		created, err := chattransformer.NewOutboundTransformer("https://api.example.com", "test-key")
		require.NoError(t, err)
		outbound = created
	case llm.APIFormatAnthropicMessage:
		created, err := anthropictransformer.NewOutboundTransformer("https://api.example.com", "test-key")
		require.NoError(t, err)
		outbound = created
	default:
		t.Fatalf("unsupported format %q", format)
	}

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "bridge-target",
			Endpoints: []objects.ChannelEndpoint{{
				APIFormat: format.String(),
			}},
		},
		Outbound: outbound,
	}

	return &PersistentOutboundTransformer{
		wrapped: outbound,
		state: &PersistenceState{
			ChannelModelsCandidates: []*ChannelModelsCandidate{{
				Channel:   channel,
				APIFormat: format.String(),
				Models: []biz.ChannelModelEntry{{
					RequestModel: "grok-4.5",
					ActualModel:  "grok-4.5",
				}},
			}},
		},
	}
}

func responsesCustomToolRequest() *llm.Request {
	return &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Model:     "grok-4.5",
		MaxTokens: lo.ToPtr(int64(1024)),
		Messages: []llm.Message{{
			Role: "user",
			Content: llm.MessageContent{
				Content: lo.ToPtr("run the command"),
			},
		}},
		Tools: []llm.Tool{{
			Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomTool: &llm.ResponseCustomTool{
				Name:        "exec",
				Description: "Run raw JavaScript.",
				Format: &llm.ResponseCustomToolFormat{
					Type: "text",
				},
			},
		}},
	}
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return encoded
}
