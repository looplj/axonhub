package responses

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestResponsesStreamRoundTrip_PreservesFunctionCallArgumentsFromFinalEvents(t *testing.T) {
	const arguments = `{"description":"Inspect the repository and report the relevant files.","model":"gpt-5.5"}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_spawn_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_spawn_1","type":"function_call","status":"in_progress","call_id":"call_spawn_1","name":"spawn_agent","namespace":"collaboration","arguments":""}}`)},
		{Type: "response.function_call_arguments.done", Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_spawn_1","output_index":0,"arguments":` + strconv.Quote(arguments) + `}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_spawn_1","type":"function_call","status":"completed","call_id":"call_spawn_1","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_spawn_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_spawn_1","type":"function_call","status":"completed","call_id":"call_spawn_1","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, "function_call", completed.Output[0].Type)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_OutputItemDoneCarriesCompleteFunctionCall(t *testing.T) {
	const arguments = `{"description":"Persist the complete tool call."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_done_item","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_done_item","type":"function_call","status":"in_progress","call_id":"call_done_item","name":"spawn_agent","namespace":"collaboration","arguments":""}}`)},
		{Type: "response.function_call_arguments.done", Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_done_item","output_index":0,"arguments":` + strconv.Quote(arguments) + `}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_done_item","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_done_item","type":"function_call","status":"completed","call_id":"call_done_item","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}]}}`)},
	}

	var doneItem *Item
	for _, event := range roundTripResponseEvents(t, upstreamEvents) {
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "function_call" {
			doneItem = event.Item
		}
	}
	require.NotNil(t, doneItem)
	require.Equal(t, "fc_done_item", doneItem.ID)
	require.Equal(t, "call_done_item", doneItem.CallID)
	require.Equal(t, "spawn_agent", doneItem.Name)
	require.Equal(t, "collaboration", doneItem.Namespace)
	require.Equal(t, arguments, doneItem.Arguments)
}

func TestResponsesStreamRoundTrip_PreservesFunctionCallArgumentsWhenArgumentsDoneIsMissing(t *testing.T) {
	const arguments = `{"description":"Audit the stream transformer."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_spawn_2","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_spawn_2","type":"function_call","status":"in_progress","call_id":"call_spawn_2","name":"spawn_agent","namespace":"collaboration","arguments":""}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_spawn_2","type":"function_call","status":"completed","call_id":"call_spawn_2","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_spawn_2","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_spawn_2","type":"function_call","status":"completed","call_id":"call_spawn_2","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_PreservesFunctionCallArgumentsFromCompletedSnapshot(t *testing.T) {
	const arguments = `{"description":"Use the terminal response snapshot."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_spawn_3","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_spawn_3","type":"function_call","status":"in_progress","call_id":"call_spawn_3","name":"spawn_agent","namespace":"collaboration","arguments":""}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_spawn_3","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_spawn_3","type":"function_call","status":"completed","call_id":"call_spawn_3","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_PreservesFunctionCallFromOutputItemDoneWithoutAdded(t *testing.T) {
	const arguments = `{"description":"Recover the final tool item."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_final_only_done","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_final_only_done","type":"function_call","status":"completed","call_id":"call_final_only_done","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_final_only_done","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, "fc_final_only_done", completed.Output[0].ID)
	require.Equal(t, "call_final_only_done", completed.Output[0].CallID)
	require.Equal(t, "spawn_agent", completed.Output[0].Name)
	require.Equal(t, "collaboration", completed.Output[0].Namespace)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_PreservesCustomToolFromCompletedSnapshotWithoutAdded(t *testing.T) {
	const input = "patch"

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_final_only_completed","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_final_only_completed","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_final_only_completed","type":"custom_tool_call","status":"completed","call_id":"call_final_only_completed","name":"apply_patch","namespace":"mcp__codex","input":` + strconv.Quote(input) + `}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, "ctc_final_only_completed", completed.Output[0].ID)
	require.Equal(t, "call_final_only_completed", completed.Output[0].CallID)
	require.Equal(t, "apply_patch", completed.Output[0].Name)
	require.Equal(t, "mcp__codex", completed.Output[0].Namespace)
	require.NotNil(t, completed.Output[0].Input)
	require.Equal(t, input, *completed.Output[0].Input)
}

func TestResponsesOutboundStream_CreatesToolCallFromFinalItemWithoutAdded(t *testing.T) {
	const arguments = `{"description":"Recover outbound state."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_outbound_final_only","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_outbound_final_only","type":"function_call","status":"completed","call_id":"call_outbound_final_only","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_outbound_final_only","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[]}}`)},
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)

	var toolCall *llm.ToolCall
	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			if choice.Delta == nil || len(choice.Delta.ToolCalls) == 0 {
				continue
			}
			candidate := choice.Delta.ToolCalls[0]
			toolCall = &candidate
		}
	}
	require.NotNil(t, toolCall)
	require.Equal(t, "call_outbound_final_only", toolCall.ID)
	require.Equal(t, "fc_outbound_final_only", toolCall.ResponseItemID)
	require.Equal(t, "spawn_agent", toolCall.Function.Name)
	require.Equal(t, "collaboration", toolCall.Function.Namespace)
	require.Equal(t, arguments, toolCall.Function.Arguments)
}

func TestResponsesStreamRoundTrip_GeneratesStableItemIDWhenAddedOmitsIt(t *testing.T) {
	const arguments = `{"description":"Recover item identity."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_missing_item_id","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","status":"in_progress","call_id":"call_missing_item_id","name":"spawn_agent","namespace":"collaboration","arguments":""}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_missing_item_id","type":"function_call","status":"completed","call_id":"call_missing_item_id","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_missing_item_id","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_missing_item_id","type":"function_call","status":"completed","call_id":"call_missing_item_id","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}]}}`)},
	}

	var completed *Response
	var addedItemID, doneItemID string
	var addedCount, doneCount int
	for _, event := range roundTripResponseEvents(t, upstreamEvents) {
		if event.Type == StreamEventTypeOutputItemAdded && event.Item != nil && event.Item.Type == "function_call" {
			addedCount++
			addedItemID = event.Item.ID
			require.True(t, strings.HasPrefix(addedItemID, "fc_"))
			require.NotEqual(t, "call_missing_item_id", addedItemID)
			require.Equal(t, "call_missing_item_id", event.Item.CallID)
		}
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "function_call" {
			doneCount++
			doneItemID = event.Item.ID
			require.Equal(t, "call_missing_item_id", event.Item.CallID)
		}
		if event.Type == StreamEventTypeResponseCompleted {
			completed = event.Response
		}
	}
	require.Equal(t, 1, addedCount)
	require.Equal(t, 1, doneCount)
	require.Equal(t, addedItemID, doneItemID)
	require.NotNil(t, completed)
	require.Len(t, completed.Output, 1)
	require.Equal(t, addedItemID, completed.Output[0].ID)
	require.Equal(t, "call_missing_item_id", completed.Output[0].CallID)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_AssociatesFunctionDeltaByOutputIndexWhenAddedOmitsItemID(t *testing.T) {
	const arguments = `{"description":"Keep the streamed arguments."}`

	completed := roundTripCompletedResponse(t, []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_partial_function_identity","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","status":"in_progress","call_id":"call_partial_function_identity","name":"spawn_agent","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_partial_function_identity","output_index":0,"delta":` + strconv.Quote(arguments) + `}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_partial_function_identity","type":"function_call","status":"completed","call_id":"call_partial_function_identity","name":"spawn_agent","arguments":""}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_partial_function_identity","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[]}}`)},
	})

	require.Len(t, completed.Output, 1)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_AssociatesCustomDeltaByOutputIndexWhenAddedOmitsItemID(t *testing.T) {
	const input = "patch"

	completed := roundTripCompletedResponse(t, []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_partial_custom_identity","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"custom_tool_call","status":"in_progress","call_id":"call_partial_custom_identity","name":"apply_patch","input":""}}`)},
		{Type: "response.custom_tool_call_input.delta", Data: []byte(`{"type":"response.custom_tool_call_input.delta","item_id":"ctc_partial_custom_identity","output_index":0,"delta":"patch"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"ctc_partial_custom_identity","type":"custom_tool_call","status":"completed","call_id":"call_partial_custom_identity","name":"apply_patch"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_partial_custom_identity","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[]}}`)},
	})

	require.Len(t, completed.Output, 1)
	require.NotNil(t, completed.Output[0].Input)
	require.Equal(t, input, *completed.Output[0].Input)
}

func TestResponsesStreamRoundTrip_DoesNotMergeDifferentToolTypesThatReuseOutputIndex(t *testing.T) {
	completed := roundTripCompletedResponse(t, []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_reused_index_types","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","status":"in_progress","call_id":"call_reused_function","name":"wait","arguments":""}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ctc_reused_custom","type":"custom_tool_call","status":"in_progress","name":"apply_patch","input":""}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_reused_function","type":"function_call","status":"completed","call_id":"call_reused_function","name":"wait","arguments":"{}"}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"ctc_reused_custom","type":"custom_tool_call","status":"completed","call_id":"call_reused_custom","name":"apply_patch","input":"patch"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_reused_index_types","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[]}}`)},
	})

	require.Len(t, completed.Output, 2)
	itemsByCallID := map[string]Item{}
	for _, item := range completed.Output {
		itemsByCallID[item.CallID] = item
	}
	require.Equal(t, "function_call", itemsByCallID["call_reused_function"].Type)
	require.Equal(t, "{}", itemsByCallID["call_reused_function"].Arguments)
	require.Equal(t, "custom_tool_call", itemsByCallID["call_reused_custom"].Type)
	require.Equal(t, "patch", lo.FromPtr(itemsByCallID["call_reused_custom"].Input))
}

func TestResponsesStreamRoundTrip_DoesNotMergeSameTypeToolsThatReuseOutputIndex(t *testing.T) {
	completed := roundTripCompletedResponse(t, []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_reused_index_functions","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","status":"in_progress","call_id":"call_reused_function_a","name":"wait","arguments":""}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_reused_function_b","type":"function_call","status":"in_progress","name":"spawn_agent","arguments":""}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_reused_function_a","type":"function_call","status":"completed","call_id":"call_reused_function_a","name":"wait","arguments":"{}"}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_reused_function_b","type":"function_call","status":"completed","call_id":"call_reused_function_b","name":"spawn_agent","arguments":"{\"description\":\"second\"}"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_reused_index_functions","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[]}}`)},
	})

	require.Len(t, completed.Output, 2)
	itemsByCallID := map[string]Item{}
	for _, item := range completed.Output {
		itemsByCallID[item.CallID] = item
	}
	require.Equal(t, "{}", itemsByCallID["call_reused_function_a"].Arguments)
	require.Equal(t, `{"description":"second"}`, itemsByCallID["call_reused_function_b"].Arguments)
}

func TestResponsesOutboundStream_RejectsAmbiguousLateItemIDAfterOutputIndexReuse(t *testing.T) {
	upstream, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := upstream.TransformStream(t.Context(), nil, streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_ambiguous_index","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","status":"in_progress","call_id":"call_ambiguous_a","name":"wait","arguments":""}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","status":"in_progress","call_id":"call_ambiguous_b","name":"spawn_agent","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_ambiguous_a","output_index":0,"delta":"{}"}`)},
	}))
	require.NoError(t, err)
	_, err = streams.All(canonical)
	require.ErrorContains(t, err, "ambiguous tool call output_index 0")
}

func TestResponsesStreamRoundTrip_FinalItemSuppliesMissingCallID(t *testing.T) {
	const arguments = `{"description":"Recover call identity."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_missing_call_id","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_missing_call_id","type":"function_call","status":"in_progress","name":"spawn_agent","namespace":"collaboration","arguments":""}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_missing_call_id","type":"function_call","status":"completed","call_id":"call_missing_call_id","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_missing_call_id","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_missing_call_id","type":"function_call","status":"completed","call_id":"call_missing_call_id","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}]}}`)},
	}

	var completed *Response
	var addedCount, doneCount int
	for _, event := range roundTripResponseEvents(t, upstreamEvents) {
		if event.Type == StreamEventTypeOutputItemAdded && event.Item != nil && event.Item.Type == "function_call" {
			addedCount++
			require.Equal(t, "fc_missing_call_id", event.Item.ID)
			require.Equal(t, "call_missing_call_id", event.Item.CallID)
		}
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "function_call" {
			doneCount++
			require.Equal(t, "fc_missing_call_id", event.Item.ID)
			require.Equal(t, "call_missing_call_id", event.Item.CallID)
		}
		if event.Type == StreamEventTypeResponseCompleted {
			completed = event.Response
		}
	}
	require.Equal(t, 1, addedCount)
	require.Equal(t, 1, doneCount)
	require.NotNil(t, completed)
	require.Len(t, completed.Output, 1)
	require.Equal(t, "fc_missing_call_id", completed.Output[0].ID)
	require.Equal(t, "call_missing_call_id", completed.Output[0].CallID)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_EmptyFinalArgumentsDoNotEraseStreamedArguments(t *testing.T) {
	const arguments = `{"description":"Keep streamed arguments."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_empty_final","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_empty_final","type":"function_call","status":"in_progress","call_id":"call_empty_final","name":"spawn_agent","namespace":"collaboration","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_empty_final","output_index":0,"delta":` + strconv.Quote(arguments) + `}`)},
		{Type: "response.function_call_arguments.done", Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_empty_final","output_index":0,"arguments":""}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_empty_final","type":"function_call","status":"completed","call_id":"call_empty_final","name":"spawn_agent","namespace":"collaboration","arguments":""}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_empty_final","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_AcceptsConflictingFinalFunctionArguments(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_conflict","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_conflict","type":"function_call","status":"in_progress","call_id":"call_conflict","name":"spawn_agent","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_conflict","output_index":0,"delta":"{\"description\":\"first\"}"}`)},
		{Type: "response.function_call_arguments.done", Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_conflict","output_index":0,"arguments":"{\"description\":\"function-done\"}"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_conflict","type":"function_call","status":"completed","call_id":"call_conflict","name":"spawn_agent","arguments":"{\"description\":\"different\"}"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_conflict","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_conflict","type":"function_call","status":"completed","call_id":"call_conflict","name":"spawn_agent","arguments":"{\"description\":\"different\"}"}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, `{"description":"different"}`, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_CompletedSnapshotCorrectsStreamedFunctionArguments(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_completed_correction","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_completed_correction","type":"function_call","status":"in_progress","call_id":"call_completed_correction","name":"search","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_completed_correction","output_index":0,"delta":"{}"}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_completed_correction","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_completed_correction","type":"function_call","status":"completed","call_id":"call_completed_correction","name":"search","arguments":"{\"query\":\"x\"}"}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, `{"query":"x"}`, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_AcceptsConflictingFinalCustomToolInput(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_custom_conflict","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ctc_conflict","type":"custom_tool_call","status":"in_progress","call_id":"call_custom_conflict","name":"apply_patch","input":""}}`)},
		{Type: "response.custom_tool_call_input.delta", Data: []byte(`{"type":"response.custom_tool_call_input.delta","item_id":"ctc_conflict","output_index":0,"delta":"draft"}`)},
		{Type: "response.custom_tool_call_input.done", Data: []byte(`{"type":"response.custom_tool_call_input.done","item_id":"ctc_conflict","output_index":0,"input":"final"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"ctc_conflict","type":"custom_tool_call","status":"completed","call_id":"call_custom_conflict","name":"apply_patch","input":"output-item-final"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_custom_conflict","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_conflict","type":"custom_tool_call","status":"completed","call_id":"call_custom_conflict","name":"apply_patch","input":"output-item-final"}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.NotNil(t, completed.Output[0].Input)
	require.Equal(t, "output-item-final", *completed.Output[0].Input)
}

func TestResponsesStreamRoundTrip_ExplicitEmptyFinalCustomToolInputClearsStreamedInput(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_custom_empty_final","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ctc_custom_empty_final","type":"custom_tool_call","status":"in_progress","call_id":"call_custom_empty_final","name":"apply_patch","input":""}}`)},
		{Type: "response.custom_tool_call_input.delta", Data: []byte(`{"type":"response.custom_tool_call_input.delta","item_id":"ctc_custom_empty_final","output_index":0,"delta":"draft"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"ctc_custom_empty_final","type":"custom_tool_call","status":"completed","call_id":"call_custom_empty_final","name":"apply_patch","input":""}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_custom_empty_final","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_custom_empty_final","type":"custom_tool_call","status":"completed","call_id":"call_custom_empty_final","name":"apply_patch","input":""}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.NotNil(t, completed.Output[0].Input)
	require.Empty(t, *completed.Output[0].Input)
}

func TestResponsesStreamRoundTrip_CompletedSnapshotExplicitEmptyCustomInputClearsStreamedInput(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_custom_empty_completed","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ctc_custom_empty_completed","type":"custom_tool_call","status":"in_progress","call_id":"call_custom_empty_completed","name":"apply_patch","input":""}}`)},
		{Type: "response.custom_tool_call_input.delta", Data: []byte(`{"type":"response.custom_tool_call_input.delta","item_id":"ctc_custom_empty_completed","output_index":0,"delta":"draft"}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_custom_empty_completed","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_custom_empty_completed","type":"custom_tool_call","status":"completed","call_id":"call_custom_empty_completed","name":"apply_patch","input":""}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.NotNil(t, completed.Output[0].Input)
	require.Empty(t, *completed.Output[0].Input)
}

func TestResponsesOutboundStream_DoesNotEmitProvisionalToolCallOnCompletedResponse(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_provisional_tool","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_provisional_tool","type":"function_call","status":"in_progress","call_id":"call_provisional_tool","name":"search","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_provisional_tool","output_index":0,"delta":"{}"}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_provisional_tool","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[]}}`)},
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)

	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			if choice.Delta != nil {
				require.Empty(t, choice.Delta.ToolCalls)
			}
		}
	}
}

func TestResponsesOutboundStream_DoesNotEmitToolCallWhenResponseFails(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_failed_tool","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_failed_tool","type":"function_call","status":"in_progress","call_id":"call_failed_tool","name":"search","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_failed_tool","output_index":0,"delta":"{}"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_failed_tool","type":"function_call","status":"completed","call_id":"call_failed_tool","name":"search","arguments":"{}"}}`)},
		{Type: "response.failed", Data: []byte(`{"type":"response.failed","response":{"id":"resp_failed_tool","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"failed","output":[]}}`)},
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)

	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			if choice.Delta != nil {
				require.Empty(t, choice.Delta.ToolCalls)
			}
		}
	}
}

func TestResponsesOutboundStream_DuplicateCompletedIsIdempotent(t *testing.T) {
	completedEvent := []byte(`{"type":"response.completed","response":{"id":"resp_duplicate_completed","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`)
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_duplicate_completed","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.completed", Data: completedEvent},
		{Type: "response.completed", Data: completedEvent},
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)

	var finishCount, usageCount int
	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			if choice.FinishReason != nil {
				finishCount++
			}
		}
		if chunk.Usage != nil {
			usageCount++
		}
	}
	require.Equal(t, 1, finishCount)
	require.Equal(t, 1, usageCount)
}

func TestResponsesStreamRoundTrip_PreservesMultipleFinalOnlyTools(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_multiple_final_only","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_multiple_final_only","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_multiple_1","type":"function_call","status":"completed","call_id":"call_multiple_1","name":"spawn_agent","namespace":"collaboration","arguments":"{\"description\":\"one\"}"},{"id":"ctc_multiple_2","type":"custom_tool_call","status":"completed","call_id":"call_multiple_2","name":"apply_patch","namespace":"mcp__codex","input":"patch"}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 2)
	require.Equal(t, "fc_multiple_1", completed.Output[0].ID)
	require.Equal(t, "call_multiple_1", completed.Output[0].CallID)
	require.Equal(t, "ctc_multiple_2", completed.Output[1].ID)
	require.Equal(t, "call_multiple_2", completed.Output[1].CallID)
}

func TestResponsesOutboundStream_DuplicateOutputItemDoneDoesNotRepeatArguments(t *testing.T) {
	const arguments = `{"description":"emit once"}`
	finalItem := `{"id":"fc_duplicate_done","type":"function_call","status":"completed","call_id":"call_duplicate_done","name":"spawn_agent","arguments":` + strconv.Quote(arguments) + `}`
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_duplicate_done","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_duplicate_done","type":"function_call","status":"in_progress","call_id":"call_duplicate_done","name":"spawn_agent","arguments":""}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":` + finalItem + `}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":` + finalItem + `}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_duplicate_done","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[` + finalItem + `]}}`)},
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)

	var argumentChunks int
	var streamedArguments strings.Builder
	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			if choice.Delta == nil {
				continue
			}
			for _, toolCall := range choice.Delta.ToolCalls {
				if toolCall.Function.Arguments != "" {
					argumentChunks++
					streamedArguments.WriteString(toolCall.Function.Arguments)
				}
			}
		}
	}
	require.Equal(t, 1, argumentChunks)
	require.Equal(t, arguments, streamedArguments.String())
}

func TestResponsesStreamRoundTrip_PreservesCustomToolInputFromFinalEvents(t *testing.T) {
	const input = "*** Begin Patch\n*** Update File: main.go\n@@\n-old\n+new\n*** End Patch"

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_patch_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ctc_patch_1","type":"custom_tool_call","status":"in_progress","call_id":"call_patch_1","name":"apply_patch","input":""}}`)},
		{Type: "response.custom_tool_call_input.done", Data: []byte(`{"type":"response.custom_tool_call_input.done","item_id":"ctc_patch_1","output_index":0,"input":` + strconv.Quote(input) + `}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"ctc_patch_1","type":"custom_tool_call","status":"completed","call_id":"call_patch_1","name":"apply_patch","input":` + strconv.Quote(input) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_patch_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_patch_1","type":"custom_tool_call","status":"completed","call_id":"call_patch_1","name":"apply_patch","input":` + strconv.Quote(input) + `}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, "custom_tool_call", completed.Output[0].Type)
	require.NotNil(t, completed.Output[0].Input)
	require.Equal(t, input, *completed.Output[0].Input)
}

func TestResponsesStreamRoundTrip_PreservesFunctionCallMetadataFromFinalEvents(t *testing.T) {
	const arguments = `{"description":"Run the delegated task."}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_metadata_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_metadata_1","type":"function_call","status":"in_progress","call_id":"call_metadata_1","name":"","namespace":"","arguments":""}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_metadata_1","type":"function_call","status":"completed","call_id":"call_metadata_1","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_metadata_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_metadata_1","type":"function_call","status":"completed","call_id":"call_metadata_1","name":"spawn_agent","namespace":"collaboration","arguments":` + strconv.Quote(arguments) + `}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, "spawn_agent", completed.Output[0].Name)
	require.Equal(t, "collaboration", completed.Output[0].Namespace)
	require.Equal(t, arguments, completed.Output[0].Arguments)
}

func TestResponsesStreamRoundTrip_PreservesCustomToolMetadataFromFinalEvents(t *testing.T) {
	const input = "patch"

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_metadata_2","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ctc_metadata_1","type":"custom_tool_call","status":"in_progress","call_id":"call_metadata_2","name":"","namespace":"","input":""}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"ctc_metadata_1","type":"custom_tool_call","status":"completed","call_id":"call_metadata_2","name":"apply_patch","namespace":"mcp__codex","input":` + strconv.Quote(input) + `}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_metadata_2","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_metadata_1","type":"custom_tool_call","status":"completed","call_id":"call_metadata_2","name":"apply_patch","namespace":"mcp__codex","input":` + strconv.Quote(input) + `}]}}`)},
	}

	completed := roundTripCompletedResponse(t, upstreamEvents)
	require.Len(t, completed.Output, 1)
	require.Equal(t, "custom_tool_call", completed.Output[0].Type)
	require.Equal(t, "apply_patch", completed.Output[0].Name)
	require.Equal(t, "mcp__codex", completed.Output[0].Namespace)
	require.NotNil(t, completed.Output[0].Input)
	require.Equal(t, input, *completed.Output[0].Input)
}

func TestResponsesStreamRoundTrip_PreservesToolItemIDsSeparatelyFromCallIDs(t *testing.T) {
	tests := []struct {
		name       string
		itemID     string
		callID     string
		streamData []*httpclient.StreamEvent
	}{
		{
			name:   "function call",
			itemID: "fc_identity_1",
			callID: "call_identity_1",
			streamData: []*httpclient.StreamEvent{
				{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_identity_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
				{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_identity_1","type":"function_call","status":"in_progress","call_id":"call_identity_1","name":"wait","namespace":"collaboration","arguments":""}}`)},
				{Type: "response.function_call_arguments.done", Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_identity_1","output_index":0,"arguments":"{}"}`)},
				{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_identity_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_identity_1","type":"function_call","status":"completed","call_id":"call_identity_1","name":"wait","namespace":"collaboration","arguments":"{}"}]}}`)},
			},
		},
		{
			name:   "custom tool call",
			itemID: "ctc_identity_1",
			callID: "call_identity_2",
			streamData: []*httpclient.StreamEvent{
				{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_identity_2","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
				{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ctc_identity_1","type":"custom_tool_call","status":"in_progress","call_id":"call_identity_2","name":"apply_patch","input":""}}`)},
				{Type: "response.custom_tool_call_input.done", Data: []byte(`{"type":"response.custom_tool_call_input.done","item_id":"ctc_identity_1","output_index":0,"input":"patch"}`)},
				{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_identity_2","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_identity_1","type":"custom_tool_call","status":"completed","call_id":"call_identity_2","name":"apply_patch","input":"patch"}]}}`)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completed := roundTripCompletedResponse(t, tt.streamData)
			require.Len(t, completed.Output, 1)
			require.Equal(t, tt.itemID, completed.Output[0].ID)
			require.Equal(t, tt.callID, completed.Output[0].CallID)
		})
	}
}

func roundTripCompletedResponse(t *testing.T, upstreamEvents []*httpclient.StreamEvent) *Response {
	t.Helper()

	var completed *Response
	for _, event := range roundTripResponseEvents(t, upstreamEvents) {
		if event.Type == StreamEventTypeResponseCompleted {
			completed = event.Response
		}
	}
	require.NotNil(t, completed)
	return completed
}

func roundTripResponseEvents(t *testing.T, upstreamEvents []*httpclient.StreamEvent) []StreamEvent {
	t.Helper()

	upstream, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := upstream.TransformStream(context.Background(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)

	client, err := NewInboundTransformer().TransformStream(context.Background(), canonical)
	require.NoError(t, err)

	var events []StreamEvent
	for client.Next() {
		var event StreamEvent
		require.NoError(t, json.Unmarshal(client.Current().Data, &event))
		events = append(events, event)
	}
	require.NoError(t, client.Err())
	return events
}

func TestResponsesInboundStream_GeneratesProtocolValidToolItemIDs(t *testing.T) {
	tests := []struct {
		name       string
		toolCall   llm.ToolCall
		wantPrefix string
	}{
		{
			name: "function call",
			toolCall: llm.ToolCall{
				ID:    "call_generated_1",
				Type:  "function",
				Index: 0,
				Function: llm.FunctionCall{
					Name:      "wait",
					Arguments: `{"timeout_ms":1000}`,
				},
			},
			wantPrefix: "fc_",
		},
		{
			name: "custom tool call",
			toolCall: llm.ToolCall{
				ID:    "call_generated_2",
				Type:  llm.ToolTypeResponsesCustomTool,
				Index: 0,
				ResponseCustomToolCall: &llm.ResponseCustomToolCall{
					CallID: "call_generated_2",
					Name:   "apply_patch",
					Input:  "patch",
				},
			},
			wantPrefix: "ctc_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
				{
					Object:  "chat.completion.chunk",
					ID:      "resp_generated_ids",
					Model:   "test-model",
					Created: 1700000000,
					Choices: []llm.Choice{{
						Index: 0,
						Delta: &llm.Message{
							Role:      "assistant",
							ToolCalls: []llm.ToolCall{tt.toolCall},
						},
					}},
				},
				{
					Object:  "chat.completion.chunk",
					ID:      "resp_generated_ids",
					Model:   "test-model",
					Created: 1700000000,
					Choices: []llm.Choice{{
						Index:        0,
						Delta:        &llm.Message{},
						FinishReason: lo.ToPtr("tool_calls"),
					}},
				},
			}))
			require.NoError(t, err)

			var itemID string
			for stream.Next() {
				var event StreamEvent
				require.NoError(t, json.Unmarshal(stream.Current().Data, &event))
				if event.Type == StreamEventTypeOutputItemAdded && event.Item != nil {
					itemID = event.Item.ID
				}
			}
			require.NoError(t, stream.Err())
			require.True(t, strings.HasPrefix(itemID, tt.wantPrefix), "generated item id %q must use %q", itemID, tt.wantPrefix)
			require.NotEqual(t, tt.toolCall.ID, itemID)
		})
	}
}

func TestResponsesInboundStream_DoesNotDuplicateInitialCustomToolInput(t *testing.T) {
	const input = "patch"

	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_custom_input",
			Model:   "test-model",
			Created: 1700000000,
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{{
						ID:             "call_custom_input",
						ResponseItemID: "ctc_custom_input",
						Type:           llm.ToolTypeResponsesCustomTool,
						Index:          0,
						ResponseCustomToolCall: &llm.ResponseCustomToolCall{
							CallID: "call_custom_input",
							Name:   "apply_patch",
							Input:  input,
						},
					}},
				},
			}},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_custom_input",
			Model:   "test-model",
			Created: 1700000000,
			Choices: []llm.Choice{{
				Index:        0,
				Delta:        &llm.Message{},
				FinishReason: lo.ToPtr("tool_calls"),
			}},
		},
	}))
	require.NoError(t, err)

	var doneItem *Item
	for stream.Next() {
		var event StreamEvent
		require.NoError(t, json.Unmarshal(stream.Current().Data, &event))
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "custom_tool_call" {
			doneItem = event.Item
		}
	}
	require.NoError(t, stream.Err())
	require.NotNil(t, doneItem)
	require.NotNil(t, doneItem.Input)
	require.Equal(t, input, *doneItem.Input)
}

func TestResponsesNonStreamRoundTrip_PreservesToolItemIDsSeparatelyFromCallIDs(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		itemID string
		callID string
	}{
		{
			name:   "function call",
			body:   `{"id":"resp_nonstream_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_nonstream_1","type":"function_call","status":"completed","call_id":"call_nonstream_1","name":"wait","namespace":"collaboration","arguments":"{}"}]}`,
			itemID: "fc_nonstream_1",
			callID: "call_nonstream_1",
		},
		{
			name:   "custom tool call",
			body:   `{"id":"resp_nonstream_2","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_nonstream_1","type":"custom_tool_call","status":"completed","call_id":"call_nonstream_2","name":"apply_patch","input":"patch"}]}`,
			itemID: "ctc_nonstream_1",
			callID: "call_nonstream_2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)
			canonical, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
				StatusCode: 200,
				Body:       []byte(tt.body),
			})
			require.NoError(t, err)
			require.Len(t, canonical.Choices, 1)
			require.NotNil(t, canonical.Choices[0].Message)
			require.Len(t, canonical.Choices[0].Message.ToolCalls, 1)
			require.Equal(t, tt.itemID, canonical.Choices[0].Message.ToolCalls[0].ResponseItemID)

			clientResponse, err := NewInboundTransformer().TransformResponse(t.Context(), canonical)
			require.NoError(t, err)
			var roundTripped Response
			require.NoError(t, json.Unmarshal(clientResponse.Body, &roundTripped))
			require.Len(t, roundTripped.Output, 1)
			require.Equal(t, tt.itemID, roundTripped.Output[0].ID)
			require.Equal(t, tt.callID, roundTripped.Output[0].CallID)
		})
	}
}

func TestResponsesNonStreamRoundTrip_PreservesCustomToolNamespace(t *testing.T) {
	body := `{"id":"resp_nonstream_namespace","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"ctc_nonstream_namespace","type":"custom_tool_call","status":"completed","call_id":"call_nonstream_namespace","name":"apply_patch","namespace":"mcp__codex","input":"patch"}]}`

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: 200,
		Body:       []byte(body),
	})
	require.NoError(t, err)
	require.Len(t, canonical.Choices, 1)
	require.NotNil(t, canonical.Choices[0].Message)
	require.Len(t, canonical.Choices[0].Message.ToolCalls, 1)
	require.NotNil(t, canonical.Choices[0].Message.ToolCalls[0].ResponseCustomToolCall)
	require.Equal(t, "mcp__codex", canonical.Choices[0].Message.ToolCalls[0].ResponseCustomToolCall.Namespace)

	inbound, err := NewInboundTransformer().TransformResponse(t.Context(), canonical)
	require.NoError(t, err)
	var roundTripped Response
	require.NoError(t, json.Unmarshal(inbound.Body, &roundTripped))
	require.Len(t, roundTripped.Output, 1)
	require.Equal(t, "mcp__codex", roundTripped.Output[0].Namespace)
}

func TestResponsesRequestRoundTrip_PreservesToolItemIDsSeparatelyFromCallIDs(t *testing.T) {
	requestBody := []byte(`{"model":"gpt-5.5","input":[{"id":"fc_request_1","type":"function_call","call_id":"call_request_1","name":"wait","namespace":"collaboration","arguments":"{}"},{"id":"fco_request_1","type":"function_call_output","call_id":"call_request_1","output":"done"},{"id":"ctc_request_1","type":"custom_tool_call","call_id":"call_request_2","name":"apply_patch","namespace":"mcp__codex","input":"patch"},{"id":"ctco_request_1","type":"custom_tool_call_output","call_id":"call_request_2","output":"done"}]}`)

	inbound, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: requestBody})
	require.NoError(t, err)
	require.Len(t, inbound.Messages, 4)
	require.Equal(t, "fc_request_1", inbound.Messages[0].ToolCalls[0].ResponseItemID)
	require.Equal(t, "fco_request_1", inbound.Messages[1].ID)
	require.Equal(t, "ctc_request_1", inbound.Messages[2].ToolCalls[0].ResponseItemID)
	require.Equal(t, "ctco_request_1", inbound.Messages[3].ID)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	replayed, err := outbound.TransformRequest(t.Context(), inbound)
	require.NoError(t, err)

	var replayedRequest Request
	require.NoError(t, json.Unmarshal(replayed.Body, &replayedRequest))
	require.Len(t, replayedRequest.Input.Items, 4)
	require.Equal(t, "fc_request_1", replayedRequest.Input.Items[0].ID)
	require.Equal(t, "fco_request_1", replayedRequest.Input.Items[1].ID)
	require.Equal(t, "ctc_request_1", replayedRequest.Input.Items[2].ID)
	require.Equal(t, "mcp__codex", replayedRequest.Input.Items[2].Namespace)
	require.Equal(t, "ctco_request_1", replayedRequest.Input.Items[3].ID)
}
