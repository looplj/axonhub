package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
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
		"background": true,
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
	require.Equal(t, true, body["background"])

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

func TestCodexMCPRequestRoundTrip_InputDirtyPreservesUnmodifiedCodexItems(t *testing.T) {
	rawRequest := []byte(`{
		"model": "gpt-5.1-codex-mini",
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
			{"type": "namespace", "name": "filesystem", "tools": [{"type": "function", "name": "read_file", "defer_loading": true}]}
		],
		"stream": true,
		"store": false
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: rawRequest})
	require.NoError(t, err)

	injected := "Injected prompt"
	llmReq.Messages = append([]llm.Message{{
		Role:    "developer",
		Content: llm.MessageContent{Content: &injected},
	}}, llmReq.Messages...)
	llm.MarkOpenAIResponsesInputDirty(llmReq)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(httpReq.Body, &body))

	input := body["input"].([]any)
	require.Len(t, input, 4)
	require.Equal(t, "Injected prompt", input[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])

	functionCall := input[2].(map[string]any)
	require.Equal(t, "function_call", functionCall["type"])
	require.Equal(t, "filesystem", functionCall["namespace"])

	mcpOutput := input[3].(map[string]any)
	require.Equal(t, "mcp_tool_call_output", mcpOutput["type"])
	require.Contains(t, mcpOutput, "result")
}

func TestCodexMCPRequestRoundTrip_InputDirtyPreservesNestedInputParts(t *testing.T) {
	rawRequest := []byte(`{
		"model": "gpt-5.1-codex-mini",
		"input": [
			{
				"type": "message",
				"role": "user",
				"content": [
					{"type": "input_text", "text": "Read this file"},
					{
						"type": "input_file",
						"file_id": "file_codex_123",
						"codex_part_extra": {"kept": true}
					}
				]
			}
		],
		"stream": true,
		"store": false
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: rawRequest})
	require.NoError(t, err)
	require.NotNil(t, llmReq.Messages[0].ProtocolExtensions)

	injected := "Injected prompt"
	llmReq.Messages = append([]llm.Message{{
		Role:    "developer",
		Content: llm.MessageContent{Content: &injected},
	}}, llmReq.Messages...)
	llm.MarkOpenAIResponsesInputDirty(llmReq)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(httpReq.Body, &body))

	input := body["input"].([]any)
	require.Len(t, input, 2)
	require.Equal(t, "Injected prompt", input[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])

	content := input[1].(map[string]any)["content"].([]any)
	require.Len(t, content, 2)
	inputFile := content[1].(map[string]any)
	require.Equal(t, "input_file", inputFile["type"])
	require.Equal(t, "file_codex_123", inputFile["file_id"])
	require.Equal(t, map[string]any{"kept": true}, inputFile["codex_part_extra"])
}

func TestCodexMCPRequestRoundTrip_InputDirtyDoesNotRestoreMaskedRawMessage(t *testing.T) {
	rawRequest := []byte(`{
		"model": "gpt-5.1-codex-mini",
		"input": [
			{
				"type": "message",
				"role": "user",
				"content": [{"type": "input_text", "text": "token is secret-123"}],
				"codex_extra": true
			}
		],
		"stream": false,
		"store": false
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: rawRequest})
	require.NoError(t, err)
	require.NotNil(t, llmReq.Messages[0].ProtocolExtensions)

	masked := "token is [MASKED]"
	llmReq.Messages[0].Content = llm.MessageContent{Content: &masked}
	// The masked message must drop raw extensions, otherwise dirty rebuild would restore the sensitive original text.
	llmReq.Messages[0].ProtocolExtensions = nil
	llm.MarkOpenAIResponsesInputDirty(llmReq)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	require.NotContains(t, string(httpReq.Body), "secret-123")
	require.Contains(t, string(httpReq.Body), "[MASKED]")
}

func TestCodexMCPRequestRoundTrip_ToolsDirtyRebuildsTools(t *testing.T) {
	rawRequest := []byte(`{
		"model": "gpt-5.1-codex-mini",
		"input": "hello",
		"tools": [
			{"type": "namespace", "name": "filesystem"},
			{"type": "tool_search"},
			{"type": "local_shell"}
		]
	}`)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: rawRequest})
	require.NoError(t, err)

	llmReq.Tools = []llm.Tool{{
		Type: llm.ToolTypeFunction,
		Function: llm.Function{
			Name:       "safe_tool",
			Parameters: json.RawMessage(`{"type":"object"}`),
		},
	}}
	llm.MarkOpenAIResponsesToolsDirty(llmReq)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := outbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(httpReq.Body, &body))

	tools := body["tools"].([]any)
	require.Len(t, tools, 1)
	require.Equal(t, "function", tools[0].(map[string]any)["type"])
	require.Equal(t, "safe_tool", tools[0].(map[string]any)["name"])
}

func TestCodexMCPResponseRoundTrip_NonPassThroughPreservesResponseMetadata(t *testing.T) {
	upstreamBody := []byte(`{
		"id": "resp_codex_1",
		"object": "response",
		"created_at": 1770000000,
		"status": "completed",
		"model": "gpt-5.1-codex-mini",
		"metadata": {"codex_session_id": "s1"},
		"codex_response_extra": {"kept": true},
		"output": [
			{
				"id": "search_1",
				"type": "tool_search_call",
				"status": "completed",
				"query": "filesystem tools",
				"result": {"tools": ["read_file"]}
			}
		],
		"usage": {
			"input_tokens": 10,
			"output_tokens": 5,
			"total_tokens": 15
		}
	}`)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: 200,
		Body:       upstreamBody,
	})
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(httpResp.Body, &body))
	require.Equal(t, map[string]any{"codex_session_id": "s1"}, body["metadata"])
	require.Equal(t, map[string]any{"kept": true}, body["codex_response_extra"])

	output := body["output"].([]any)
	require.Len(t, output, 1)
	require.Equal(t, "tool_search_call", output[0].(map[string]any)["type"])
	require.Equal(t, "filesystem tools", output[0].(map[string]any)["query"])
}

func TestCodexMCPResponseRoundTrip_PreservesNestedOutputTextFields(t *testing.T) {
	upstreamBody := []byte(`{
		"id": "resp_codex_output_text",
		"object": "response",
		"created_at": 1770000000,
		"status": "completed",
		"model": "gpt-5.1-codex-mini",
		"output": [
			{
				"id": "msg_output_text",
				"type": "message",
				"status": "completed",
				"role": "assistant",
				"content": [
					{
						"type": "output_text",
						"text": "done",
						"annotations": [{"type": "container_file_citation", "file_id": "file_codex_1"}],
						"logprobs": []
					}
				]
			}
		],
		"usage": {
			"input_tokens": 10,
			"output_tokens": 5,
			"total_tokens": 15
		}
	}`)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: 200,
		Body:       upstreamBody,
	})
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(httpResp.Body, &body))

	output := body["output"].([]any)
	require.Len(t, output, 1)
	content := output[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 1)
	outputText := content[0].(map[string]any)
	require.Equal(t, "output_text", outputText["type"])
	require.Equal(t, []any{map[string]any{"type": "container_file_citation", "file_id": "file_codex_1"}}, outputText["annotations"])
	require.Equal(t, []any{}, outputText["logprobs"])
}

func TestCodexMCPStreamRoundTrip_RawEventsUpdateCompletedOutputAndSequence(t *testing.T) {
	rawChunk := func(raw string) *llm.Response {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &ev))
		return &llm.Response{
			ProtocolExtensions: rawEventProtocolExtensionsFromRaw(&ev, []byte(raw)),
		}
	}

	source := streams.SliceStream([]*llm.Response{
		rawChunk(`{
			"type": "response.created",
			"sequence_number": 0,
			"response": {
				"id": "resp_stream_1",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"output": []
			}
		}`),
		rawChunk(`{
			"type": "response.in_progress",
			"sequence_number": 1,
			"response": {
				"id": "resp_stream_1",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"output": []
			}
		}`),
		rawChunk(`{
			"type": "response.output_item.added",
			"sequence_number": 2,
			"output_index": 0,
			"item": {
				"id": "mcp_out_1",
				"type": "mcp_tool_call_output",
				"call_id": "call_mcp_1",
				"status": "in_progress",
				"result": {"content": [{"type": "text", "text": "searching"}]}
			}
		}`),
		rawChunk(`{
			"type": "response.output_item.done",
			"sequence_number": 3,
			"output_index": 0,
			"item": {
				"id": "mcp_out_1",
				"type": "mcp_tool_call_output",
				"call_id": "call_mcp_1",
				"status": "completed",
				"result": {"content": [{"type": "text", "text": "ok"}], "isError": false}
			}
		}`),
		{
			ID:      "resp_stream_1",
			Object:  "chat.completion.chunk",
			Model:   "gpt-5.1-codex-mini",
			Created: 1770000000,
			Choices: []llm.Choice{{
				Delta: &llm.Message{
					Role:    "assistant",
					Content: llm.MessageContent{Content: lo.ToPtr("done")},
				},
			}},
		},
		{
			ID:      "resp_stream_1",
			Object:  "chat.completion.chunk",
			Model:   "gpt-5.1-codex-mini",
			Created: 1770000000,
			Choices: []llm.Choice{{
				FinishReason: lo.ToPtr("stop"),
			}},
		},
		{
			ID:      "resp_stream_1",
			Object:  "chat.completion.chunk",
			Model:   "gpt-5.1-codex-mini",
			Created: 1770000000,
			Usage: &llm.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	})

	inbound := NewInboundTransformer()
	out, err := inbound.TransformStream(context.Background(), source)
	require.NoError(t, err)

	var events []StreamEvent
	for out.Next() {
		event := out.Current()
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &ev))
		events = append(events, ev)
	}
	require.NoError(t, out.Err())

	require.GreaterOrEqual(t, len(events), 5)
	require.Equal(t, 0, events[0].SequenceNumber)
	require.Equal(t, 1, events[1].SequenceNumber)
	require.Equal(t, 2, events[2].SequenceNumber)
	require.Equal(t, 3, events[3].SequenceNumber)
	require.Equal(t, StreamEventTypeOutputItemAdded, events[4].Type)
	require.Equal(t, 4, events[4].SequenceNumber)

	createdCount := 0
	inProgressCount := 0
	for _, event := range events {
		switch event.Type {
		case StreamEventTypeResponseCreated:
			createdCount++
		case StreamEventTypeResponseInProgress:
			inProgressCount++
		}
	}
	require.Equal(t, 1, createdCount)
	require.Equal(t, 1, inProgressCount)

	completed := events[len(events)-1]
	require.Equal(t, StreamEventTypeResponseCompleted, completed.Type)
	require.NotNil(t, completed.Response)
	require.GreaterOrEqual(t, len(completed.Response.Output), 2)
	require.Equal(t, "mcp_tool_call_output", completed.Response.Output[0].Type)
	require.NotNil(t, completed.Response.Output[0].Result)
	require.Equal(t, "message", completed.Response.Output[1].Type)
}

func TestCodexMCPStreamRoundTrip_OutboundCarriesTerminalResponseRawEvent(t *testing.T) {
	rawEvent := func(raw string) *httpclient.StreamEvent {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &ev))
		return &httpclient.StreamEvent{Type: string(ev.Type), Data: []byte(raw)}
	}

	upstream := streams.SliceStream([]*httpclient.StreamEvent{
		rawEvent(`{
			"type": "response.created",
			"sequence_number": 0,
			"response": {
				"id": "resp_stream_terminal",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"output": []
			}
		}`),
		rawEvent(`{
			"type": "response.output_item.added",
			"sequence_number": 1,
			"output_index": 0,
			"item": {
				"id": "search_1",
				"type": "tool_search_call",
				"status": "in_progress",
				"query": "filesystem tools"
			}
		}`),
		rawEvent(`{
			"type": "response.completed",
			"sequence_number": 2,
			"response": {
				"id": "resp_stream_terminal",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "completed",
				"metadata": {"codex_session_id": "s1"},
				"codex_terminal_extra": {"kept": true},
				"output": [
					{
						"id": "search_1",
						"type": "tool_search_call",
						"status": "completed",
						"query": "filesystem tools",
						"result": {"tools": ["read_file"]}
					}
				],
				"usage": {
					"input_tokens": 10,
					"output_tokens": 5,
					"total_tokens": 15
				}
			}
		}`),
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmStream, err := outbound.TransformStream(context.Background(), upstream)
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	out, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	var completed *StreamEvent
	var completedRaw map[string]any
	for out.Next() {
		event := out.Current()
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &ev))
		if ev.Type != StreamEventTypeResponseCompleted {
			continue
		}

		completed = &ev
		require.NoError(t, json.Unmarshal(event.Data, &completedRaw))
	}
	require.NoError(t, out.Err())
	require.NotNil(t, completed)
	require.Equal(t, 2, completed.SequenceNumber)
	require.NotNil(t, completed.Response)
	require.Equal(t, map[string]string{"codex_session_id": "s1"}, completed.Response.Metadata)
	require.Len(t, completed.Response.Output, 1)
	require.Equal(t, "tool_search_call", completed.Response.Output[0].Type)

	responseRaw := completedRaw["response"].(map[string]any)
	require.Equal(t, map[string]any{"kept": true}, responseRaw["codex_terminal_extra"])
}

func TestCodexMCPStreamRoundTrip_RawTerminalEventClosesSyntheticTextItem(t *testing.T) {
	rawEvent := func(raw string) *httpclient.StreamEvent {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &ev))
		return &httpclient.StreamEvent{Type: string(ev.Type), Data: []byte(raw)}
	}

	upstream := streams.SliceStream([]*httpclient.StreamEvent{
		rawEvent(`{
			"type": "response.created",
			"sequence_number": 0,
			"response": {
				"id": "resp_stream_close",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"output": []
			}
		}`),
		rawEvent(`{
			"type": "response.output_item.added",
			"sequence_number": 1,
			"output_index": 0,
			"item": {
				"id": "msg_stream_close",
				"type": "message",
				"status": "in_progress",
				"role": "assistant"
			}
		}`),
		rawEvent(`{
			"type": "response.content_part.added",
			"sequence_number": 2,
			"item_id": "msg_stream_close",
			"output_index": 0,
			"content_index": 0,
			"part": {"type": "output_text", "text": ""}
		}`),
		rawEvent(`{
			"type": "response.output_text.delta",
			"sequence_number": 3,
			"item_id": "msg_stream_close",
			"output_index": 0,
			"content_index": 0,
			"delta": "done"
		}`),
		rawEvent(`{
			"type": "response.completed",
			"sequence_number": 4,
			"response": {
				"id": "resp_stream_close",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "completed",
				"metadata": {"codex_session_id": "s1"},
				"output": [
					{
						"id": "msg_stream_close",
						"type": "message",
						"status": "completed",
						"role": "assistant",
						"content": [{"type": "output_text", "text": "done"}]
					}
				],
				"usage": {
					"input_tokens": 10,
					"output_tokens": 5,
					"total_tokens": 15
				}
			}
		}`),
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmStream, err := outbound.TransformStream(context.Background(), upstream)
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	out, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	var events []StreamEvent
	for out.Next() {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(out.Current().Data, &ev))
		events = append(events, ev)
	}
	require.NoError(t, out.Err())

	indexByType := map[StreamEventType]int{}
	for i, event := range events {
		if i > 0 {
			require.Greater(t, event.SequenceNumber, events[i-1].SequenceNumber)
		}
		if _, exists := indexByType[event.Type]; !exists {
			indexByType[event.Type] = i
		}
	}

	completedIndex, ok := indexByType[StreamEventTypeResponseCompleted]
	require.True(t, ok)
	outputTextDoneIndex, ok := indexByType[StreamEventTypeOutputTextDone]
	require.True(t, ok)
	contentPartDoneIndex, ok := indexByType[StreamEventTypeContentPartDone]
	require.True(t, ok)
	outputItemDoneIndex, ok := indexByType[StreamEventTypeOutputItemDone]
	require.True(t, ok)
	require.Less(t, outputTextDoneIndex, completedIndex)
	require.Less(t, contentPartDoneIndex, completedIndex)
	require.Less(t, outputItemDoneIndex, completedIndex)
	require.Equal(t, map[string]string{"codex_session_id": "s1"}, events[completedIndex].Response.Metadata)
}

func TestCodexMCPStreamRoundTrip_OutboundCarriesKnownTerminalResponseFields(t *testing.T) {
	rawEvent := func(raw string) *httpclient.StreamEvent {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &ev))
		return &httpclient.StreamEvent{Type: string(ev.Type), Data: []byte(raw)}
	}

	upstream := streams.SliceStream([]*httpclient.StreamEvent{
		rawEvent(`{
			"type": "response.created",
			"sequence_number": 0,
			"response": {
				"id": "resp_stream_known_fields",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"output": []
			}
		}`),
		rawEvent(`{
			"type": "response.completed",
			"sequence_number": 1,
			"response": {
				"id": "resp_stream_known_fields",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "completed",
				"instructions": "Keep exact terminal fields.",
				"parallel_tool_calls": true,
				"service_tier": "default",
				"truncation": "auto",
				"output": [],
				"usage": {
					"input_tokens": 10,
					"output_tokens": 5,
					"total_tokens": 15
				}
			}
		}`),
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmStream, err := outbound.TransformStream(context.Background(), upstream)
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	out, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	var completedRaw map[string]any
	for out.Next() {
		event := out.Current()
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &ev))
		if ev.Type == StreamEventTypeResponseCompleted {
			require.NoError(t, json.Unmarshal(event.Data, &completedRaw))
		}
	}
	require.NoError(t, out.Err())
	require.NotNil(t, completedRaw)

	responseRaw := completedRaw["response"].(map[string]any)
	require.Equal(t, "Keep exact terminal fields.", responseRaw["instructions"])
	require.Equal(t, true, responseRaw["parallel_tool_calls"])
	require.Equal(t, "default", responseRaw["service_tier"])
	require.Equal(t, "auto", responseRaw["truncation"])
}

func TestCodexMCPStreamRoundTrip_PreservesKnownDeltaRawFields(t *testing.T) {
	rawEvent := func(raw string) *httpclient.StreamEvent {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &ev))
		return &httpclient.StreamEvent{Type: string(ev.Type), Data: []byte(raw)}
	}

	upstream := streams.SliceStream([]*httpclient.StreamEvent{
		rawEvent(`{
			"type": "response.created",
			"sequence_number": 0,
			"response": {
				"id": "resp_stream_delta",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"output": []
			}
		}`),
		rawEvent(`{
			"type": "response.output_text.delta",
			"sequence_number": 1,
			"item_id": "msg_stream_delta",
			"output_index": 0,
			"content_index": 0,
			"delta": "hello",
			"logprobs": [],
			"obfuscation": "text-obfuscation",
			"codex_delta_extra": {"kept": true}
		}`),
		rawEvent(`{
			"type": "response.output_item.added",
			"sequence_number": 2,
			"output_index": 1,
			"item": {
				"id": "fc_stream_delta",
				"type": "function_call",
				"status": "in_progress",
				"call_id": "call_stream_delta",
				"name": "read_file"
			}
		}`),
		rawEvent(`{
			"type": "response.function_call_arguments.delta",
			"sequence_number": 3,
			"item_id": "fc_stream_delta",
			"output_index": 1,
			"content_index": 0,
			"delta": "{\"path\":\"README.md\"}",
			"obfuscation": "args-obfuscation",
			"codex_args_extra": {"kept": true}
		}`),
		rawEvent(`{
			"type": "response.completed",
			"sequence_number": 4,
			"response": {
				"id": "resp_stream_delta",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "completed",
				"output": [],
				"usage": {
					"input_tokens": 10,
					"output_tokens": 5,
					"total_tokens": 15
				}
			}
		}`),
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmStream, err := outbound.TransformStream(context.Background(), upstream)
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	out, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	var textDeltaRaw map[string]any
	var argsDeltaRaw map[string]any
	for out.Next() {
		event := out.Current()
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &ev))
		switch ev.Type {
		case StreamEventTypeOutputTextDelta:
			require.NoError(t, json.Unmarshal(event.Data, &textDeltaRaw))
		case StreamEventTypeFunctionCallArgumentsDelta:
			require.NoError(t, json.Unmarshal(event.Data, &argsDeltaRaw))
		}
	}
	require.NoError(t, out.Err())

	require.NotNil(t, textDeltaRaw)
	require.Equal(t, "text-obfuscation", textDeltaRaw["obfuscation"])
	require.Equal(t, []any{}, textDeltaRaw["logprobs"])
	require.Equal(t, map[string]any{"kept": true}, textDeltaRaw["codex_delta_extra"])

	require.NotNil(t, argsDeltaRaw)
	require.Equal(t, "args-obfuscation", argsDeltaRaw["obfuscation"])
	require.Equal(t, map[string]any{"kept": true}, argsDeltaRaw["codex_args_extra"])
}

func TestCodexMCPStreamRoundTrip_PreservesContentPartRawFields(t *testing.T) {
	rawEvent := func(raw string) *httpclient.StreamEvent {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &ev))
		return &httpclient.StreamEvent{Type: string(ev.Type), Data: []byte(raw)}
	}

	upstream := streams.SliceStream([]*httpclient.StreamEvent{
		rawEvent(`{
			"type": "response.created",
			"sequence_number": 0,
			"response": {
				"id": "resp_stream_part",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"output": []
			}
		}`),
		rawEvent(`{
			"type": "response.content_part.added",
			"sequence_number": 1,
			"item_id": "msg_stream_part",
			"output_index": 0,
			"content_index": 0,
			"part": {
				"type": "output_text",
				"text": "",
				"annotations": [{"type": "container_file_citation", "file_id": "file_codex_1"}],
				"logprobs": [],
				"codex_part_extra": {"kept": true}
			}
		}`),
		rawEvent(`{
			"type": "response.content_part.done",
			"sequence_number": 2,
			"item_id": "msg_stream_part",
			"output_index": 0,
			"content_index": 1,
			"part": {
				"type": "refusal",
				"refusal": "not allowed"
			}
		}`),
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmStream, err := outbound.TransformStream(context.Background(), upstream)
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	out, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	var addedRaw map[string]any
	var doneRaw map[string]any
	for out.Next() {
		event := out.Current()
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &ev))
		switch ev.Type {
		case StreamEventTypeContentPartAdded:
			require.NoError(t, json.Unmarshal(event.Data, &addedRaw))
		case StreamEventTypeContentPartDone:
			require.NoError(t, json.Unmarshal(event.Data, &doneRaw))
		}
	}
	require.NoError(t, out.Err())

	require.NotNil(t, addedRaw)
	addedPart := addedRaw["part"].(map[string]any)
	require.Equal(t, []any{map[string]any{"type": "container_file_citation", "file_id": "file_codex_1"}}, addedPart["annotations"])
	require.Equal(t, []any{}, addedPart["logprobs"])
	require.Equal(t, map[string]any{"kept": true}, addedPart["codex_part_extra"])

	require.NotNil(t, doneRaw)
	donePart := doneRaw["part"].(map[string]any)
	require.Equal(t, "refusal", donePart["type"])
	require.Equal(t, "not allowed", donePart["refusal"])
}

func TestCodexMCPStreamRoundTrip_PreservesLifecycleRawEvents(t *testing.T) {
	rawEvent := func(raw string) *httpclient.StreamEvent {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &ev))
		return &httpclient.StreamEvent{Type: string(ev.Type), Data: []byte(raw)}
	}

	upstream := streams.SliceStream([]*httpclient.StreamEvent{
		rawEvent(`{
			"type": "response.created",
			"sequence_number": 0,
			"response": {
				"id": "resp_stream_lifecycle",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"background": false,
				"store": false,
				"service_tier": "default",
				"tools": [{"type": "local_shell"}],
				"metadata": {"codex_session_id": "s1"},
				"codex_created_extra": {"kept": true},
				"output": []
			}
		}`),
		rawEvent(`{
			"type": "response.in_progress",
			"sequence_number": 1,
			"response": {
				"id": "resp_stream_lifecycle",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "in_progress",
				"background": false,
				"store": false,
				"service_tier": "default",
				"tools": [{"type": "local_shell"}],
				"metadata": {"codex_session_id": "s1"},
				"codex_progress_extra": {"kept": true},
				"output": []
			}
		}`),
		rawEvent(`{
			"type": "response.completed",
			"sequence_number": 2,
			"response": {
				"id": "resp_stream_lifecycle",
				"object": "response",
				"created_at": 1770000000,
				"model": "gpt-5.1-codex-mini",
				"status": "completed",
				"output": []
			}
		}`),
	})

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmStream, err := outbound.TransformStream(context.Background(), upstream)
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	out, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	var createdRaw map[string]any
	var inProgressRaw map[string]any
	for out.Next() {
		event := out.Current()
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &ev))
		switch ev.Type {
		case StreamEventTypeResponseCreated:
			require.NoError(t, json.Unmarshal(event.Data, &createdRaw))
		case StreamEventTypeResponseInProgress:
			require.NoError(t, json.Unmarshal(event.Data, &inProgressRaw))
		}
	}
	require.NoError(t, out.Err())

	require.NotNil(t, createdRaw)
	createdResponse := createdRaw["response"].(map[string]any)
	require.Equal(t, false, createdResponse["background"])
	require.Equal(t, false, createdResponse["store"])
	require.Equal(t, "default", createdResponse["service_tier"])
	require.Equal(t, map[string]any{"codex_session_id": "s1"}, createdResponse["metadata"])
	require.Equal(t, map[string]any{"kept": true}, createdResponse["codex_created_extra"])
	require.Len(t, createdResponse["tools"].([]any), 1)

	require.NotNil(t, inProgressRaw)
	inProgressResponse := inProgressRaw["response"].(map[string]any)
	require.Equal(t, false, inProgressResponse["background"])
	require.Equal(t, false, inProgressResponse["store"])
	require.Equal(t, "default", inProgressResponse["service_tier"])
	require.Equal(t, map[string]any{"codex_session_id": "s1"}, inProgressResponse["metadata"])
	require.Equal(t, map[string]any{"kept": true}, inProgressResponse["codex_progress_extra"])
	require.Len(t, inProgressResponse["tools"].([]any), 1)
}
