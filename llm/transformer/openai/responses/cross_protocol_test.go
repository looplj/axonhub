package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/deepseek"
	geminioai "github.com/looplj/axonhub/llm/transformer/gemini/openai"
	"github.com/looplj/axonhub/llm/transformer/nanogpt"
	chatoutbound "github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// TestCrossProtocol_NamespaceMapSurvivesRoundTrip verifies that the namespace
// tool map stored in TransformerMetadata during responses inbound survives a
// cross-protocol round-trip through the chat outbound (responses client → chat
// upstream → back to responses client). The chat outbound must propagate
// TransformerMetadata on both the request (so it reaches the upstream) and the
// response (so the responses inbound can restore the namespace group identity).
//
// This was previously a known gap (chat outbound did not clone request
// TransformerMetadata to the response). The fix adds shared.PropagateRequestMetadata
// / MergeResponseMetadata calls to the chat, anthropic, and gemini outbounds.

func TestCrossProtocol_ChatOutboundEmitsLossyDowngradeDiagnostics(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model": "gpt-4o",
			"input": []map[string]any{
				{"type": "additional_tools", "tools": []map[string]any{{"type": "tool_search", "name": "search_docs"}}},
				{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "hello"}}},
			},
			"tools": []map[string]any{
				{
					"type":  "namespace",
					"name":  "mcp__node_repl",
					"tools": []map[string]any{{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}}},
				},
				{"type": "tool_search", "name": "search_docs", "namespace": "docs"},
				{"type": "future_tool", "name": "future"},
			},
		}),
	}

	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"

	chatOut, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	_, err = chatOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	diagnosticsPtr := llm.ResponsesLossySummaryOf(llmReq)
	ok := diagnosticsPtr != nil
	var diagnostics shared.ResponsesLossyDowngradeDiagnostics
	if ok {
		diagnostics = *diagnosticsPtr
	}
	require.True(t, ok)
	require.True(t, diagnostics.LossyDowngrade)
	require.Equal(t, 1, diagnostics.NamespaceToolCount)
	require.Equal(t, 1, diagnostics.ToolSearchToolCount)
	require.Equal(t, 1, diagnostics.UnknownToolCount)
	require.Equal(t, 1, diagnostics.AdditionalToolsCount)

	downgrades := llm.LossyDowngrades(llmReq)
	require.NotEmpty(t, downgrades)
	fields := map[string]bool{}
	for _, d := range downgrades {
		fields[d.SourceField] = true
		require.Equal(t, llm.APIFormatOpenAIChatCompletion, d.TargetProtocol)
	}
	require.True(t, fields["tools[].type=namespace"])
	require.True(t, fields["tools[].type=tool_search"])
	require.True(t, fields["input[].type=additional_tools"])
	require.True(t, fields["tools[] raw-only native tool"])
}

func TestCrossProtocol_ChatOutboundEmitsLossyDowngradeDiagnosticsForUnknownTopLevelOnly(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model":                  "gpt-4o",
			"input":                  "hello",
			"codex_future_top_level": map[string]any{"enabled": true},
		}),
	}

	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"

	chatOut, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	_, err = chatOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	diagnosticsPtr := llm.ResponsesLossySummaryOf(llmReq)
	ok := diagnosticsPtr != nil
	var diagnostics shared.ResponsesLossyDowngradeDiagnostics
	if ok {
		diagnostics = *diagnosticsPtr
	}
	require.True(t, ok)
	require.True(t, diagnostics.LossyDowngrade)
	require.Equal(t, 1, diagnostics.UnknownTopLevelFieldCount)
	require.Equal(t, 0, diagnostics.NamespaceToolCount)
	require.Equal(t, 0, diagnostics.ToolSearchToolCount)
	require.Equal(t, 0, diagnostics.UnknownToolCount)
	require.Equal(t, 0, diagnostics.AdditionalToolsCount)
}

func TestCrossProtocol_ChatOutboundEmitsLossyDowngradeDiagnosticsForKnownRawOnlyTool(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model": "gpt-4o",
			"input": "run code",
			"tools": []map[string]any{
				{"type": "code_interpreter", "container": map[string]any{"type": "auto"}},
			},
		}),
	}

	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"

	chatOut, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	_, err = chatOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	diagnosticsPtr := llm.ResponsesLossySummaryOf(llmReq)
	ok := diagnosticsPtr != nil
	var diagnostics shared.ResponsesLossyDowngradeDiagnostics
	if ok {
		diagnostics = *diagnosticsPtr
	}
	require.True(t, ok)
	require.True(t, diagnostics.LossyDowngrade)
	require.Equal(t, 1, diagnostics.RawOnlyToolCount)
	require.Equal(t, 0, diagnostics.UnknownToolCount)

	downgrades := llm.LossyDowngrades(llmReq)
	require.NotEmpty(t, downgrades)
	found := false
	for _, d := range downgrades {
		if d.SourceField == "tools[] raw-only native tool" {
			found = true
			require.Equal(t, llm.APIFormatOpenAIChatCompletion, d.TargetProtocol)
		}
	}
	require.True(t, found)
}

func TestCrossProtocol_ChatOutboundEmitsLossyDowngradeDiagnosticsForClientMetadataOnly(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model":           "gpt-4o",
			"input":           "hello",
			"client_metadata": map[string]any{"codex_version": "1.2.3"},
		}),
	}

	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"

	chatOut, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	_, err = chatOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	diagnosticsPtr := llm.ResponsesLossySummaryOf(llmReq)
	ok := diagnosticsPtr != nil
	var diagnostics shared.ResponsesLossyDowngradeDiagnostics
	if ok {
		diagnostics = *diagnosticsPtr
	}
	require.True(t, ok)
	require.True(t, diagnostics.LossyDowngrade)
	require.Equal(t, 1, diagnostics.ClientMetadataCount)
	require.Equal(t, 0, diagnostics.UnknownTopLevelFieldCount)
	require.Equal(t, 0, diagnostics.NamespaceToolCount)
	require.Equal(t, 0, diagnostics.ToolSearchToolCount)
	require.Equal(t, 0, diagnostics.UnknownToolCount)
	require.Equal(t, 0, diagnostics.AdditionalToolsCount)
}

func TestCrossProtocol_ResponsesCustomToolHistoryBridgesToOpenAIChat(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	llmReq, err := responsesInbound.TransformRequest(context.Background(), &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model": "gpt-5",
			"input": []map[string]any{
				{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "apply the patch"}}},
				{"type": "custom_tool_call", "id": "item_call_1", "call_id": "call_patch_1", "name": "apply_patch", "input": "*** Begin Patch\n*** End Patch"},
				{"type": "custom_tool_call_output", "id": "item_output_1", "call_id": "call_patch_1", "output": "Done"},
			},
			"tools": []map[string]any{{
				"type":        "custom",
				"name":        "apply_patch",
				"description": "Apply a patch",
				"format": map[string]any{
					"type":       "grammar",
					"syntax":     "lark",
					"definition": "start: patch",
				},
			}},
		}),
	})
	require.NoError(t, err)

	chatOutbound, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := chatOutbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(httpReq.Body, &payload))
	require.JSONEq(t, `[{"type":"custom","custom":{"name":"apply_patch","description":"Apply a patch","format":{"type":"grammar","grammar":{"syntax":"lark","definition":"start: patch"}}}}]`, string(payload["tools"]))

	var messages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload["messages"], &messages))
	require.Len(t, messages, 3)
	require.JSONEq(t, `[{"id":"call_patch_1","type":"custom","custom":{"name":"apply_patch","input":"*** Begin Patch\n*** End Patch"}}]`, string(messages[1]["tool_calls"]))
	require.JSONEq(t, `"call_patch_1"`, string(messages[2]["tool_call_id"]))
	require.JSONEq(t, `"Done"`, string(messages[2]["content"]))
}

func TestCrossProtocol_NamespaceMapSurvivesRoundTrip(t *testing.T) {
	// --- Step 1: responses inbound (request) — namespace map is recorded ---
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model": "gpt-4o",
			"input": "use the tool",
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}

	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	_, ok := llmReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, ok, "namespace map must be recorded by responses inbound")

	// --- Step 2: chat outbound (request) — TransformerMetadata propagated ---
	chatOut, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"

	httpReq, err := chatOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	t.Run("chat outbound request carries TransformerMetadata", func(t *testing.T) {
		require.NotNil(t, httpReq.TransformerMetadata)
		_, exists := httpReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
		require.True(t, exists, "chat outbound must propagate namespace map on request")
	})

	// --- Step 3: mock chat upstream response (function_call with flattened name) ---
	chatRespBody := mustMarshal(t, map[string]any{
		"id":      "chatcompl_1",
		"object":  "chat.completion",
		"model":   "gpt-4o",
		"created": 1700000000,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []map[string]any{
						{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      "mcp__node_repl__run",
								"arguments": `{"x":1}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	})

	httpResp := &httpclient.Response{
		StatusCode: 200,
		Body:       chatRespBody,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Request:    httpReq,
	}

	// --- Step 4: chat outbound (response) — TransformerMetadata cloned ---
	llmResp, err := chatOut.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)

	t.Run("chat outbound response carries namespace map", func(t *testing.T) {
		require.NotNil(t, llmResp.TransformerMetadata)
		_, exists := llmResp.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
		require.True(t, exists, "chat outbound must clone request TransformerMetadata to response")
	})

	// --- Step 5: responses inbound (response) — namespace restored ---
	clientResp, err := responsesInbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)

	var respPayload Response
	require.NoError(t, json.Unmarshal(clientResp.Body, &respPayload))

	t.Run("namespace restored in cross-protocol return", func(t *testing.T) {
		var fcItem *Item
		for i := range respPayload.Output {
			if respPayload.Output[i].Type == "function_call" {
				fcItem = &respPayload.Output[i]
				break
			}
		}
		require.NotNil(t, fcItem, "function_call item must exist")
		require.Equal(t, "run", fcItem.Name, "name must be restored to leaf")
		require.Equal(t, "mcp__node_repl", fcItem.Namespace, "namespace must be restored to group")
	})
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

var _ transformer.Outbound = (*chatoutbound.OutboundTransformer)(nil)

// TestCrossProtocol_Stream_NamespaceMapSurvivesRoundTrip verifies the streaming
// cross-protocol path: a responses client streaming through a chat upstream must
// still get the namespace group identity restored on function_call items. The
// chat outbound's TransformStream now wraps its output with
// shared.PropagateStreamMetadata so the request's TransformerMetadata rides on
// the first chunk, and the responses inbound's mergeTransformerMetadata picks it
// up before initToolCall runs.
func TestCrossProtocol_Stream_NamespaceMapSurvivesRoundTrip(t *testing.T) {
	responsesInbound := NewInboundTransformer()

	// Step 1: responses inbound (request) — namespace map recorded
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model":  "gpt-4o",
			"input":  "use the tool",
			"stream": true,
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}
	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)

	// Step 2: chat outbound (request)
	chatOut, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"
	httpReq, err := chatOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	// Step 3: mock chat SSE stream (function_call with flattened name)
	chatSSEEvents := []*httpclient.StreamEvent{
		{Type: "chat.completion.chunk", Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","created":1700000000,"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"mcp__node_repl__run","arguments":"{\"x\":1}"}}]},"finish_reason":null}]}`)},
		{Type: "chat.completion.chunk", Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","created":1700000000,"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)},
		{Type: "data", Data: []byte("[DONE]")},
	}

	// Step 4: chat outbound (stream) — should propagate namespace map on first chunk
	llmStream, err := chatOut.TransformStream(context.Background(), httpReq, streams.SliceStream(chatSSEEvents))
	require.NoError(t, err)

	var llmChunks []*llm.Response
	for llmStream.Next() {
		ch := llmStream.Current()
		if ch != nil {
			llmChunks = append(llmChunks, ch)
		}
	}
	require.NoError(t, llmStream.Err())
	require.NotEmpty(t, llmChunks)

	// Verify at least one chunk carries the namespace map
	var sawMap bool
	for _, ch := range llmChunks {
		if ch.TransformerMetadata != nil {
			if _, ok := ch.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]; ok {
				sawMap = true
				break
			}
		}
	}
	require.True(t, sawMap, "chat outbound stream must propagate namespace map on a chunk")

	// Step 5: responses inbound (stream) — should restore namespace
	respStream, err := responsesInbound.TransformStream(context.Background(), streams.SliceStream(llmChunks))
	require.NoError(t, err)

	var actualEvents []StreamEvent
	for respStream.Next() {
		ev := respStream.Current()
		var se StreamEvent
		if err := json.Unmarshal(ev.Data, &se); err == nil {
			actualEvents = append(actualEvents, se)
		}
	}
	require.NoError(t, respStream.Err())

	// Find the function_call item
	var fcItem *Item
	for i := range actualEvents {
		ev := actualEvents[i]
		if (ev.Type == StreamEventTypeOutputItemAdded || ev.Type == StreamEventTypeOutputItemDone) &&
			ev.Item != nil && ev.Item.Type == "function_call" {
			fcItem = ev.Item
			if ev.Type == StreamEventTypeOutputItemDone {
				break
			}
		}
	}
	require.NotNil(t, fcItem, "expected a function_call output item in stream")
	require.Equal(t, "run", fcItem.Name, "streaming cross-protocol: name must be restored to leaf")
	require.Equal(t, "mcp__node_repl", fcItem.Namespace, "streaming cross-protocol: namespace must be restored")
}

// TestCrossProtocol_DeepSeekIndependentOutboundPropagatesMetadata verifies that
// deepseek — which builds its own httpclient.Request independently (not delegating
// to the chat outbound's TransformRequest) — now calls
// shared.PropagateRequestMetadata so the namespace map survives the round-trip.
// Regression guard for the 5 independently-constructing chat-family outbounds
// (deepseek/moonshot/zai/doubao/openrouter) identified by acceptance audit.
func TestCrossProtocol_DeepSeekIndependentOutboundPropagatesMetadata(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model": "deepseek-chat",
			"input": "use the tool",
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}
	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)

	deepseekOut, err := deepseek.NewOutboundTransformer("https://api.deepseek.com", "test-key")
	require.NoError(t, err)
	llmReq.Model = "deepseek-chat"

	httpReq, err := deepseekOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	require.NotNil(t, httpReq.TransformerMetadata,
		"deepseek outbound must propagate TransformerMetadata on independently-constructed request")
	_, exists := httpReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, exists, "deepseek outbound must carry the namespace tool map")
}

// TestCrossProtocol_OpenRouterResponseMetadataRoundTrip verifies that openrouter —
// which builds its own TransformResponse/TransformStream for the chat path (NOT
// delegating to the chat base, unlike deepseek/moonshot/zai/doubao) — propagates
// TransformerMetadata back on the response so a responses client's namespace tool
// map survives the cross-protocol round-trip.
//
// Regression guard: openrouter's self-built chat response previously skipped
// shared.MergeResponseMetadata, so the namespace map was lost on the return trip
// and tool-call names could not be restored.
func TestCrossProtocol_OpenRouterResponseMetadataRoundTrip(t *testing.T) {
	// Step 1: responses inbound (request) — namespace map recorded
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model": "openai/gpt-4o",
			"input": "use the tool",
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}
	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	_, ok := llmReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, ok, "namespace map must be recorded by responses inbound")

	// Step 2: openrouter outbound (request) — TransformerMetadata propagated
	orOut, err := openrouter.NewOutboundTransformer("https://openrouter.ai/api/v1", "test-key")
	require.NoError(t, err)
	llmReq.Model = "openai/gpt-4o"
	httpReq, err := orOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.NotNil(t, httpReq.TransformerMetadata, "openrouter outbound must propagate TransformerMetadata on request")
	_, exists := httpReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, exists, "openrouter outbound must carry the namespace tool map")

	// Step 3: mock chat upstream response (function_call with flattened name)
	chatRespBody := mustMarshal(t, map[string]any{
		"id":      "chatcompl_1",
		"object":  "chat.completion",
		"model":   "openai/gpt-4o",
		"created": 1700000000,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []map[string]any{
						{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      "mcp__node_repl__run",
								"arguments": `{"x":1}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	})
	httpResp := &httpclient.Response{
		StatusCode: 200,
		Body:       chatRespBody,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Request:    httpReq,
	}

	// Step 4: openrouter outbound (response) — TransformerMetadata must be cloned
	llmResp, err := orOut.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp.TransformerMetadata, "openrouter outbound must clone request TransformerMetadata to response")
	_, exists = llmResp.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, exists, "openrouter outbound response must carry the namespace tool map")

	// Step 5: responses inbound (response) — namespace restored
	clientResp, err := responsesInbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)
	var respPayload Response
	require.NoError(t, json.Unmarshal(clientResp.Body, &respPayload))
	var fcItem *Item
	for i := range respPayload.Output {
		if respPayload.Output[i].Type == "function_call" {
			fcItem = &respPayload.Output[i]
			break
		}
	}
	require.NotNil(t, fcItem, "function_call item must exist")
	require.Equal(t, "run", fcItem.Name, "name must be restored to leaf")
	require.Equal(t, "mcp__node_repl", fcItem.Namespace, "namespace must be restored to group")
}

// TestCrossProtocol_OpenRouterStreamMetadataRoundTrip verifies the streaming
// cross-protocol path through openrouter's self-built TransformStream: the
// namespace tool map must ride on a stream chunk so the responses inbound stream
// can restore the namespace group identity.
func TestCrossProtocol_OpenRouterStreamMetadataRoundTrip(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model":  "openai/gpt-4o",
			"input":  "use the tool",
			"stream": true,
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}
	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)

	orOut, err := openrouter.NewOutboundTransformer("https://openrouter.ai/api/v1", "test-key")
	require.NoError(t, err)
	llmReq.Model = "openai/gpt-4o"
	httpReq, err := orOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	// mock chat SSE stream (function_call with flattened name)
	chatSSEEvents := []*httpclient.StreamEvent{
		{Type: "chat.completion.chunk", Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"openai/gpt-4o","created":1700000000,"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"mcp__node_repl__run","arguments":"{\"x\":1}"}}]},"finish_reason":null}]}`)},
		{Type: "chat.completion.chunk", Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"openai/gpt-4o","created":1700000000,"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)},
		{Type: "data", Data: []byte("[DONE]")},
	}

	llmStream, err := orOut.TransformStream(context.Background(), httpReq, streams.SliceStream(chatSSEEvents))
	require.NoError(t, err)

	var llmChunks []*llm.Response
	for llmStream.Next() {
		ch := llmStream.Current()
		if ch != nil {
			llmChunks = append(llmChunks, ch)
		}
	}
	require.NoError(t, llmStream.Err())
	require.NotEmpty(t, llmChunks)

	var sawMap bool
	for _, ch := range llmChunks {
		if ch.TransformerMetadata != nil {
			if _, ok := ch.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]; ok {
				sawMap = true
				break
			}
		}
	}
	require.True(t, sawMap, "openrouter outbound stream must propagate namespace map on a chunk")

	// responses inbound (stream) — restore namespace
	respStream, err := responsesInbound.TransformStream(context.Background(), streams.SliceStream(llmChunks))
	require.NoError(t, err)
	var actualEvents []StreamEvent
	for respStream.Next() {
		ev := respStream.Current()
		var se StreamEvent
		if err := json.Unmarshal(ev.Data, &se); err == nil {
			actualEvents = append(actualEvents, se)
		}
	}
	require.NoError(t, respStream.Err())
	var fcItem *Item
	for i := range actualEvents {
		ev := actualEvents[i]
		if (ev.Type == StreamEventTypeOutputItemAdded || ev.Type == StreamEventTypeOutputItemDone) &&
			ev.Item != nil && ev.Item.Type == "function_call" {
			fcItem = ev.Item
			if ev.Type == StreamEventTypeOutputItemDone {
				break
			}
		}
	}
	require.NotNil(t, fcItem, "expected a function_call output item in stream")
	require.Equal(t, "run", fcItem.Name, "streaming cross-protocol: name must be restored to leaf")
	require.Equal(t, "mcp__node_repl", fcItem.Namespace, "streaming cross-protocol: namespace must be restored")
}

// TestCrossProtocol_NanoGPTResponseMetadataRoundTrip verifies that nanogpt — which
// builds its own TransformResponse/TransformStream for the chat path (NOT delegating
// to the chat base) — propagates TransformerMetadata back on the response so a
// responses client's namespace tool map survives the cross-protocol round-trip.
//
// Regression guard: nanogpt's self-built chat response/stream previously skipped
// shared.MergeResponseMetadata / shared.PropagateStreamMetadata (same class of gap
// as the openrouter fix).
func TestCrossProtocol_NanoGPTResponseMetadataRoundTrip(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model": "gpt-4o",
			"input": "use the tool",
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}
	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	_, ok := llmReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, ok, "namespace map must be recorded by responses inbound")

	nanoOut, err := nanogpt.NewOutboundTransformer("https://api.nanogpt.com/v1", "test-key")
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"
	httpReq, err := nanoOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)
	require.NotNil(t, httpReq.TransformerMetadata, "nanogpt outbound must propagate TransformerMetadata on request")
	_, exists := httpReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, exists, "nanogpt outbound must carry the namespace tool map")

	chatRespBody := mustMarshal(t, map[string]any{
		"id":      "chatcompl_1",
		"object":  "chat.completion",
		"model":   "gpt-4o",
		"created": 1700000000,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []map[string]any{
						{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      "mcp__node_repl__run",
								"arguments": `{"x":1}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	})
	httpResp := &httpclient.Response{
		StatusCode: 200,
		Body:       chatRespBody,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Request:    httpReq,
	}

	llmResp, err := nanoOut.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.NotNil(t, llmResp.TransformerMetadata, "nanogpt outbound must clone request TransformerMetadata to response")
	_, exists = llmResp.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, exists, "nanogpt outbound response must carry the namespace tool map")

	clientResp, err := responsesInbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)
	var respPayload Response
	require.NoError(t, json.Unmarshal(clientResp.Body, &respPayload))
	var fcItem *Item
	for i := range respPayload.Output {
		if respPayload.Output[i].Type == "function_call" {
			fcItem = &respPayload.Output[i]
			break
		}
	}
	require.NotNil(t, fcItem, "function_call item must exist")
	require.Equal(t, "run", fcItem.Name, "name must be restored to leaf")
	require.Equal(t, "mcp__node_repl", fcItem.Namespace, "namespace must be restored to group")
}

// TestCrossProtocol_NanoGPTStreamMetadataRoundTrip verifies the streaming
// cross-protocol path through nanogpt's self-built TransformStream: the namespace
// tool map must ride on a stream chunk so the responses inbound stream can restore
// the namespace group identity.
func TestCrossProtocol_NanoGPTStreamMetadataRoundTrip(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model":  "gpt-4o",
			"input":  "use the tool",
			"stream": true,
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}
	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)

	nanoOut, err := nanogpt.NewOutboundTransformer("https://api.nanogpt.com/v1", "test-key")
	require.NoError(t, err)
	llmReq.Model = "gpt-4o"
	httpReq, err := nanoOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	chatSSEEvents := []*httpclient.StreamEvent{
		{Type: "chat.completion.chunk", Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","created":1700000000,"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"mcp__node_repl__run","arguments":"{\"x\":1}"}}]},"finish_reason":null}]}`)},
		{Type: "chat.completion.chunk", Data: []byte(`{"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4o","created":1700000000,"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)},
		{Type: "data", Data: []byte("[DONE]")},
	}

	llmStream, err := nanoOut.TransformStream(context.Background(), httpReq, streams.SliceStream(chatSSEEvents))
	require.NoError(t, err)

	var llmChunks []*llm.Response
	for llmStream.Next() {
		ch := llmStream.Current()
		if ch != nil {
			llmChunks = append(llmChunks, ch)
		}
	}
	require.NoError(t, llmStream.Err())
	require.NotEmpty(t, llmChunks)

	var sawMap bool
	for _, ch := range llmChunks {
		if ch.TransformerMetadata != nil {
			if _, ok := ch.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]; ok {
				sawMap = true
				break
			}
		}
	}
	require.True(t, sawMap, "nanogpt outbound stream must propagate namespace map on a chunk")

	respStream, err := responsesInbound.TransformStream(context.Background(), streams.SliceStream(llmChunks))
	require.NoError(t, err)
	var actualEvents []StreamEvent
	for respStream.Next() {
		ev := respStream.Current()
		var se StreamEvent
		if err := json.Unmarshal(ev.Data, &se); err == nil {
			actualEvents = append(actualEvents, se)
		}
	}
	require.NoError(t, respStream.Err())
	var fcItem *Item
	for i := range actualEvents {
		ev := actualEvents[i]
		if (ev.Type == StreamEventTypeOutputItemAdded || ev.Type == StreamEventTypeOutputItemDone) &&
			ev.Item != nil && ev.Item.Type == "function_call" {
			fcItem = ev.Item
			if ev.Type == StreamEventTypeOutputItemDone {
				break
			}
		}
	}
	require.NotNil(t, fcItem, "expected a function_call output item in stream")
	require.Equal(t, "run", fcItem.Name, "streaming cross-protocol: name must be restored to leaf")
	require.Equal(t, "mcp__node_repl", fcItem.Namespace, "streaming cross-protocol: namespace must be restored")
}

// TestCrossProtocol_GeminiOpenAIResponseMetadataRoundTrip verifies that
// gemini_openai — which builds its own TransformRequest (not delegating to the
// chat base) — propagates TransformerMetadata onto the request so a responses
// client's namespace tool map survives the cross-protocol round-trip.
//
// Regression guard: gemini_openai's self-built TransformRequest previously did
// not call shared.PropagateRequestMetadata, so the namespace map was lost before
// reaching the upstream and the response-side restoration (handled by the
// embedded openai base) had nothing to merge.
func TestCrossProtocol_GeminiOpenAIResponseMetadataRoundTrip(t *testing.T) {
	responsesInbound := NewInboundTransformer()
	inboundReq := &httpclient.Request{
		Body: mustMarshal(t, map[string]any{
			"model": "gemini-2.5-flash",
			"input": "use the tool",
			"tools": []map[string]any{
				{
					"type": "namespace",
					"name": "mcp__node_repl",
					"tools": []map[string]any{
						{"type": "function", "name": "run", "parameters": map[string]any{"type": "object"}},
					},
				},
			},
		}),
	}
	llmReq, err := responsesInbound.TransformRequest(context.Background(), inboundReq)
	require.NoError(t, err)
	_, ok := llmReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
	require.True(t, ok, "namespace map must be recorded by responses inbound")

	geminiOaiOut, err := geminioai.NewOutboundTransformer("https://generativelanguage.googleapis.com/v1beta/openai", "test-key")
	require.NoError(t, err)
	llmReq.Model = "gemini-2.5-flash"
	httpReq, err := geminiOaiOut.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	t.Run("gemini_openai request carries TransformerMetadata", func(t *testing.T) {
		require.NotNil(t, httpReq.TransformerMetadata, "gemini_openai outbound must propagate TransformerMetadata on request")
		_, exists := httpReq.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
		require.True(t, exists, "gemini_openai outbound must carry the namespace tool map")
	})

	chatRespBody := mustMarshal(t, map[string]any{
		"id":      "chatcompl_1",
		"object":  "chat.completion",
		"model":   "gemini-2.5-flash",
		"created": 1700000000,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role": "assistant",
					"tool_calls": []map[string]any{
						{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      "mcp__node_repl__run",
								"arguments": `{"x":1}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	})
	httpResp := &httpclient.Response{
		StatusCode: 200,
		Body:       chatRespBody,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Request:    httpReq,
	}

	llmResp, err := geminiOaiOut.TransformResponse(context.Background(), httpResp)
	require.NoError(t, err)

	t.Run("gemini_openai response carries namespace map", func(t *testing.T) {
		require.NotNil(t, llmResp.TransformerMetadata, "gemini_openai outbound must clone request TransformerMetadata to response")
		_, exists := llmResp.TransformerMetadata[responsesNamespaceToolMapTransformerMetadataKey]
		require.True(t, exists, "gemini_openai outbound response must carry the namespace tool map")
	})

	clientResp, err := responsesInbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)
	var respPayload Response
	require.NoError(t, json.Unmarshal(clientResp.Body, &respPayload))
	var fcItem *Item
	for i := range respPayload.Output {
		if respPayload.Output[i].Type == "function_call" {
			fcItem = &respPayload.Output[i]
			break
		}
	}
	require.NotNil(t, fcItem, "function_call item must exist")
	require.Equal(t, "run", fcItem.Name, "name must be restored to leaf")
	require.Equal(t, "mcp__node_repl", fcItem.Namespace, "namespace must be restored to group")
}
