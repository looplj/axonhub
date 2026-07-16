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
	chatopenai "github.com/looplj/axonhub/llm/transformer/openai"
)

// A1: Chat custom call history must bridge to Responses request items as
// custom_tool_call + matching custom_tool_call_output, preserving name/input/call_id.
// Public seam: Chat inbound HTTP request → canonical → Responses outbound HTTP body.
func TestA1_ChatCustomToolHistoryBridgesToResponsesRequest(t *testing.T) {
	chatInbound := chatopenai.NewInboundTransformer()
	llmReq, err := chatInbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gpt-5",
			"messages": [
				{"role":"user","content":"run sql"},
				{"role":"assistant","tool_calls":[{"id":"call_c1","type":"custom","custom":{"name":"run_sql","input":"SELECT 1"}}]},
				{"role":"tool","tool_call_id":"call_c1","content":"ok"}
			]
		}`),
	})
	require.NoError(t, err)

	responsesOutbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := responsesOutbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(httpReq.Body, &payload))

	var input []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload["input"], &input))
	require.GreaterOrEqual(t, len(input), 3)

	// Find custom call + output by type; order should keep call before its output.
	var callItem, outputItem map[string]json.RawMessage
	callIdx, outputIdx := -1, -1
	for i, item := range input {
		var typ string
		require.NoError(t, json.Unmarshal(item["type"], &typ))
		switch typ {
		case "custom_tool_call":
			if callItem == nil {
				callItem = item
				callIdx = i
			}
		case "custom_tool_call_output":
			if outputItem == nil {
				outputItem = item
				outputIdx = i
			}
		case "function_call", "function_call_output":
			t.Fatalf("Chat custom tool lifecycle must not degrade to %s", typ)
		}
	}
	require.NotNil(t, callItem, "expected custom_tool_call item")
	require.NotNil(t, outputItem, "expected custom_tool_call_output item")
	require.Less(t, callIdx, outputIdx, "custom_tool_call must precede matching output")

	require.JSONEq(t, `"call_c1"`, string(callItem["call_id"]))
	require.JSONEq(t, `"run_sql"`, string(callItem["name"]))
	require.JSONEq(t, `"SELECT 1"`, string(callItem["input"]))

	require.JSONEq(t, `"call_c1"`, string(outputItem["call_id"]))
	// output may be string or object; accept string "ok"
	var output any
	require.NoError(t, json.Unmarshal(outputItem["output"], &output))
	switch v := output.(type) {
	case string:
		require.Equal(t, "ok", v)
	default:
		// Some paths wrap as {"text":"ok"}-like; require the raw contains ok.
		require.Contains(t, string(outputItem["output"]), "ok")
	}
}

// A2: Chat-provider custom tool response must reconstruct as Responses custom_tool_call.
// Public seam: Chat outbound HTTP response → canonical → Responses inbound HTTP body.
func TestA2_ChatProviderCustomResponseBridgesToResponsesClientResponse(t *testing.T) {
	chatOutbound, err := chatopenai.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	llmResp, err := chatOutbound.TransformResponse(context.Background(), &httpclient.Response{
		Body: []byte(`{
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
						"id": "call_c1",
						"type": "custom",
						"custom": {"name": "run_sql", "input": "SELECT 1"}
					}]
				},
				"finish_reason": "tool_calls"
			}]
		}`),
	})
	require.NoError(t, err)

	responsesInbound := NewInboundTransformer()
	clientResp, err := responsesInbound.TransformResponse(context.Background(), llmResp)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(clientResp.Body, &payload))

	var output []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload["output"], &output))

	var callItem map[string]json.RawMessage
	for _, item := range output {
		var typ string
		require.NoError(t, json.Unmarshal(item["type"], &typ))
		switch typ {
		case "custom_tool_call":
			callItem = item
		case "function_call":
			t.Fatalf("Chat custom tool response must not reconstruct as function_call")
		}
	}
	require.NotNil(t, callItem, "expected custom_tool_call output item")
	require.JSONEq(t, `"call_c1"`, string(callItem["call_id"]))
	require.JSONEq(t, `"run_sql"`, string(callItem["name"]))
	require.JSONEq(t, `"SELECT 1"`, string(callItem["input"]))
}

// A3: Chat-provider function/custom responses must emit distinct Responses output
// item id that is non-empty and never aliases call_id when ResponseItemID is empty.
// Public seam: same as A2.
func TestA3_ChatProviderToolResponseOutputIdentityIsDistinctFromCallID(t *testing.T) {
	cases := []struct {
		name         string
		upstreamBody string
		wantType     string
		wantCallID   string
		wantName     string
	}{
		{
			name: "function_call",
			upstreamBody: `{
				"id": "chatcmpl-fn",
				"object": "chat.completion",
				"created": 1,
				"model": "gpt-5",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": null,
						"tool_calls": [{
							"id": "call_1",
							"type": "function",
							"function": {"name": "weather", "arguments": "{}"}
						}]
					},
					"finish_reason": "tool_calls"
				}]
			}`,
			wantType:   "function_call",
			wantCallID: "call_1",
			wantName:   "weather",
		},
		{
			name: "custom_tool_call",
			upstreamBody: `{
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
							"id": "call_c1",
							"type": "custom",
							"custom": {"name": "run_sql", "input": "SELECT 1"}
						}]
					},
					"finish_reason": "tool_calls"
				}]
			}`,
			wantType:   "custom_tool_call",
			wantCallID: "call_c1",
			wantName:   "run_sql",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chatOutbound, err := chatopenai.NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			llmResp, err := chatOutbound.TransformResponse(context.Background(), &httpclient.Response{Body: []byte(tc.upstreamBody)})
			require.NoError(t, err)

			// Chat sources do not carry Responses-native item ids.
			require.NotEmpty(t, llmResp.Choices)
			require.NotNil(t, llmResp.Choices[0].Message)
			require.NotEmpty(t, llmResp.Choices[0].Message.ToolCalls)
			require.Empty(t, llmResp.Choices[0].Message.ToolCalls[0].ResponseItemID)

			responsesInbound := NewInboundTransformer()
			clientResp, err := responsesInbound.TransformResponse(context.Background(), llmResp)
			require.NoError(t, err)

			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(clientResp.Body, &payload))
			var output []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(payload["output"], &output))

			var callItem map[string]json.RawMessage
			for _, item := range output {
				var typ string
				require.NoError(t, json.Unmarshal(item["type"], &typ))
				if typ == tc.wantType {
					callItem = item
					break
				}
			}
			require.NotNil(t, callItem, "expected %s output item", tc.wantType)

			require.JSONEq(t, `"`+tc.wantCallID+`"`, string(callItem["call_id"]))
			require.JSONEq(t, `"`+tc.wantName+`"`, string(callItem["name"]))

			rawID, ok := callItem["id"]
			require.True(t, ok, "Responses output item id must be present")
			var itemID string
			require.NoError(t, json.Unmarshal(rawID, &itemID))
			require.NotEmpty(t, itemID, "Responses output item id must be non-empty")
			require.NotEqual(t, tc.wantCallID, itemID, "item id must not alias call_id")
		})
	}
}

// A4: Chat-provider stream chunks must reconstruct Responses stream events with:
// - distinct non-empty item ids when ResponseItemID is missing (function + custom)
// - OpenAIChatCustomToolCall emitted as custom_tool_call events, not function_call
// - start/delta/done for one call sharing the same generated item id
// Public seam: Chat outbound stream chunks → canonical llm.Response stream → Responses inbound stream.
func TestA4_ChatProviderStreamToolCallsBridgeToResponsesStream(t *testing.T) {
	type caseSpec struct {
		name           string
		chunks         [][]byte
		wantItemType   string
		wantCallID     string
		wantName       string
		wantDeltaEvent string
		wantDoneEvent  string
		wantDeltaText  string
	}

	cases := []caseSpec{
		{
			name: "function_call",
			chunks: [][]byte{
				[]byte(`{
					"id":"chatcmpl-fn-stream","object":"chat.completion.chunk","created":1,"model":"gpt-5",
					"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"weather","arguments":""}}]},"finish_reason":null}]
				}`),
				[]byte(`{
					"id":"chatcmpl-fn-stream","object":"chat.completion.chunk","created":1,"model":"gpt-5",
					"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"q\":\"SF\"}"}}]},"finish_reason":null}]
				}`),
				[]byte(`{
					"id":"chatcmpl-fn-stream","object":"chat.completion.chunk","created":1,"model":"gpt-5",
					"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]
				}`),
			},
			wantItemType:   "function_call",
			wantCallID:     "call_1",
			wantName:       "weather",
			wantDeltaEvent: string(StreamEventTypeFunctionCallArgumentsDelta),
			wantDoneEvent:  string(StreamEventTypeFunctionCallArgumentsDone),
			wantDeltaText:  `{"q":"SF"}`,
		},
		{
			name: "custom_tool_call",
			chunks: [][]byte{
				[]byte(`{
					"id":"chatcmpl-custom-stream","object":"chat.completion.chunk","created":1,"model":"gpt-5",
					"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_c1","type":"custom","custom":{"name":"run_sql","input":""}}]},"finish_reason":null}]
				}`),
				[]byte(`{
					"id":"chatcmpl-custom-stream","object":"chat.completion.chunk","created":1,"model":"gpt-5",
					"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"custom","custom":{"input":"SELECT 1"}}]},"finish_reason":null}]
				}`),
				[]byte(`{
					"id":"chatcmpl-custom-stream","object":"chat.completion.chunk","created":1,"model":"gpt-5",
					"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]
				}`),
			},
			wantItemType:   "custom_tool_call",
			wantCallID:     "call_c1",
			wantName:       "run_sql",
			wantDeltaEvent: string(StreamEventTypeCustomToolCallInputDelta),
			wantDoneEvent:  string(StreamEventTypeCustomToolCallInputDone),
			wantDeltaText:  "SELECT 1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chatOutbound, err := chatopenai.NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)

			var llmChunks []*llm.Response
			for _, body := range tc.chunks {
				chunk, err := chatOutbound.TransformResponse(context.Background(), &httpclient.Response{Body: body})
				require.NoError(t, err)
				// Chat stream sources do not carry Responses-native item ids.
				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
					for _, toolCall := range chunk.Choices[0].Delta.ToolCalls {
						require.Empty(t, toolCall.ResponseItemID)
					}
				}
				llmChunks = append(llmChunks, chunk)
			}

			responsesInbound := NewInboundTransformer()
			stream, err := responsesInbound.TransformStream(context.Background(), streams.SliceStream(llmChunks))
			require.NoError(t, err)

			var events []StreamEvent
			for stream.Next() {
				evData := stream.Current()
				var ev StreamEvent
				require.NoError(t, json.Unmarshal(evData.Data, &ev))
				events = append(events, ev)
			}
			require.NoError(t, stream.Err())
			require.NotEmpty(t, events)

			var addedItem *Item
			var doneItem *Item
			var deltaItemIDs []string
			var doneEventItemID string
			sawFunctionCall := false

			for i := range events {
				ev := events[i]
				switch {
				case ev.Type == StreamEventTypeOutputItemAdded && ev.Item != nil && ev.Item.Type == tc.wantItemType:
					addedItem = ev.Item
				case ev.Type == StreamEventTypeOutputItemDone && ev.Item != nil && ev.Item.Type == tc.wantItemType:
					doneItem = ev.Item
				case string(ev.Type) == tc.wantDeltaEvent:
					require.NotNil(t, ev.ItemID)
					deltaItemIDs = append(deltaItemIDs, *ev.ItemID)
					if tc.wantDeltaText != "" {
						require.Equal(t, tc.wantDeltaText, ev.Delta)
					}
				case string(ev.Type) == tc.wantDoneEvent:
					require.NotNil(t, ev.ItemID)
					doneEventItemID = *ev.ItemID
				case ev.Item != nil && ev.Item.Type == "function_call" && tc.wantItemType == "custom_tool_call":
					sawFunctionCall = true
				case string(ev.Type) == string(StreamEventTypeFunctionCallArgumentsDelta) && tc.wantItemType == "custom_tool_call":
					sawFunctionCall = true
				}
			}

			require.False(t, sawFunctionCall, "Chat custom stream must not emit function_call events")
			require.NotNil(t, addedItem, "expected output_item.added for %s", tc.wantItemType)
			require.NotNil(t, doneItem, "expected output_item.done for %s", tc.wantItemType)
			require.Equal(t, tc.wantCallID, addedItem.CallID)
			require.Equal(t, tc.wantName, addedItem.Name)
			require.NotEmpty(t, addedItem.ID, "stream item id must be non-empty")
			require.NotEqual(t, tc.wantCallID, addedItem.ID, "stream item id must not alias call_id")
			require.Equal(t, addedItem.ID, doneItem.ID, "start/done must share the same item id")
			require.NotEmpty(t, deltaItemIDs, "expected at least one delta event")
			for _, id := range deltaItemIDs {
				require.Equal(t, addedItem.ID, id, "delta must reuse start item id")
			}
			require.Equal(t, addedItem.ID, doneEventItemID, "done event must reuse start item id")
		})
	}
}

// A5: Chat custom tool declarations must bridge to Responses tools[].type=custom
// with name/description/format (text or grammar). Public seam:
// Chat inbound HTTP request → canonical → Responses outbound HTTP body.
func TestA5_ChatCustomToolDeclarationsBridgeToResponsesTools(t *testing.T) {
	chatInbound := chatopenai.NewInboundTransformer()
	llmReq, err := chatInbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gpt-5",
			"messages": [{"role":"user","content":"use tools"}],
			"tools": [
				{
					"type": "custom",
					"custom": {
						"name": "run_sql",
						"description": "Execute a constrained query",
						"format": {
							"type": "grammar",
							"grammar": {"syntax": "lark", "definition": "start: SELECT"}
						}
					}
				},
				{
					"type": "custom",
					"custom": {
						"name": "freeform_note",
						"description": "Capture free text",
						"format": {"type": "text"}
					}
				},
				{
					"type": "function",
					"function": {
						"name": "get_weather",
						"description": "Weather lookup",
						"parameters": {"type":"object","properties":{}}
					}
				}
			]
		}`),
	})
	require.NoError(t, err)

	responsesOutbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	httpReq, err := responsesOutbound.TransformRequest(context.Background(), llmReq)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(httpReq.Body, &payload))
	require.Contains(t, payload, "tools")

	var tools []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload["tools"], &tools))
	require.Len(t, tools, 3)

	byName := map[string]map[string]json.RawMessage{}
	for _, tool := range tools {
		var typ, name string
		require.NoError(t, json.Unmarshal(tool["type"], &typ))
		require.NoError(t, json.Unmarshal(tool["name"], &name))
		byName[name] = tool
		if name == "run_sql" || name == "freeform_note" {
			require.Equal(t, "custom", typ)
		}
	}

	runSQL, ok := byName["run_sql"]
	require.True(t, ok, "run_sql custom tool must be present")
	require.JSONEq(t, `"Execute a constrained query"`, string(runSQL["description"]))
	require.JSONEq(t, `{"type":"grammar","syntax":"lark","definition":"start: SELECT"}`, string(runSQL["format"]))
	_, hasCustomNested := runSQL["custom"]
	require.False(t, hasCustomNested, "Responses custom tools use flat fields, not Chat nested custom object")

	note, ok := byName["freeform_note"]
	require.True(t, ok, "freeform_note custom tool must be present")
	require.JSONEq(t, `"Capture free text"`, string(note["description"]))
	require.JSONEq(t, `{"type":"text"}`, string(note["format"]))

	weather, ok := byName["get_weather"]
	require.True(t, ok)
	require.JSONEq(t, `"function"`, string(weather["type"]))
}
