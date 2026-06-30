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
	chatoutbound "github.com/looplj/axonhub/llm/transformer/openai"
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
