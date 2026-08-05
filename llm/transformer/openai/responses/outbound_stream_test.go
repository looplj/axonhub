package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm/streams"
)

func TestOutboundTransformer_StreamTransformation_WithTestData(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	tests := []struct {
		name                 string
		inputStreamFile      string // OpenAI Responses API stream format
		expectedStreamFile   string // Expected LLM stream format
		expectedResponseFile string // Final LLM response format
	}{
		{
			name:                 "stream transformation with text and multiple tool calls",
			inputStreamFile:      "tool-2.stream.jsonl",
			expectedStreamFile:   "llm-tool-2.stream.jsonl",
			expectedResponseFile: "llm-tool-2.response.json",
		},
		{
			name:                 "stream transformation with encrypted reasoning",
			inputStreamFile:      "encrypted_content.stream.jsonl",
			expectedStreamFile:   "llm-encrypted_content.stream.jsonl",
			expectedResponseFile: "llm-encrypted_content.response.json",
		},
		{
			name:                 "stream transformation with custom tool call",
			inputStreamFile:      "custom_tool.stream.jsonl",
			expectedStreamFile:   "llm-custom_tool.stream.jsonl",
			expectedResponseFile: "llm-custom_tool.stream.response.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedEvents, err := xtest.LoadLlmResponses(t, tt.expectedStreamFile)
			require.NoError(t, err)

			// Load the input file (OpenAI Responses API format events)
			responsesAPIEvents, err := xtest.LoadStreamChunks(t, tt.inputStreamFile)
			require.NoError(t, err)

			// Transform the stream (OpenAI Responses API -> LLM format)
			transformedStream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(responsesAPIEvents))
			require.NoError(t, err)
			require.NoError(t, transformedStream.Err())

			// Collect all transformed events
			actualLLMResponses, err := streams.All(transformedStream)
			require.NoError(t, err)

			// Stream transformation may not be 1:1, so we verify key properties instead of exact count
			require.NotEmpty(t, actualLLMResponses, "Should have at least one response")

			// Verify the last event is DONE
			lastEvent := actualLLMResponses[len(actualLLMResponses)-1]
			require.Equal(t, llm.DoneResponse, lastEvent, "Last event should be DONE")

			// Verify non-DONE events have valid structure
			for _, resp := range actualLLMResponses {
				if resp != llm.DoneResponse {
					// Verify each response has the correct object type
					require.Contains(t, []string{"chat.completion", "chat.completion.chunk"}, resp.Object,
						"Response should be chat.completion or chat.completion.chunk")
				}
			}

			require.Len(t, actualLLMResponses, len(expectedEvents))

			// exclude the last DONE event
			for i, expectedEvent := range expectedEvents[:len(expectedEvents)-1] {
				if !xtest.Equal(expectedEvent, actualLLMResponses[i]) {
					t.Fatalf("event %d mismatch:\n%s", i, cmp.Diff(expectedEvent, actualLLMResponses[i]))
				}
			}

			// Verify the final response against expectedResponseFile
			if tt.expectedResponseFile != "" {
				// Find the last non-DONE response
				var lastResponse *llm.Response

				for i := len(actualLLMResponses) - 1; i >= 0; i-- {
					if actualLLMResponses[i] != llm.DoneResponse {
						lastResponse = actualLLMResponses[i]

						break
					}
				}

				require.NotNil(t, lastResponse, "Expected at least one non-DONE response")

				// Load expected final response from file
				var expectedFinalResponse llm.Response

				err := xtest.LoadTestData(t, tt.expectedResponseFile, &expectedFinalResponse)
				require.NoError(t, err)

				// Compare model and ID from the last response
				require.Equal(t, expectedFinalResponse.Model, lastResponse.Model,
					"Final response model should match")
				require.Equal(t, expectedFinalResponse.ID, lastResponse.ID,
					"Final response ID should match")
			}
		})
	}
}

func TestOutboundTransformer_StreamTransformation_ErrorEvent(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	responsesAPIEvents, err := xtest.LoadStreamChunks(t, "error.response.stream.jsonl")
	require.NoError(t, err)

	transformedStream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(responsesAPIEvents))
	require.NoError(t, err)

	_, err = streams.All(transformedStream)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Something went wrong")
}

func TestOutboundTransformer_TransformStream_UsesFinalEncryptedContentPerReasoningItem(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_reasoning_multi","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAA_added_1"}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAA_done_1"}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":1,"item":{"id":"rs_2","type":"reasoning","summary":[],"encrypted_content":"gAAAA_added_2"}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":1,"item":{"id":"rs_2","type":"reasoning","summary":[],"encrypted_content":"gAAAA_done_2"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_reasoning_multi","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
	}

	stream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	responses, err := streams.All(stream)
	require.NoError(t, err)

	var signatures []string
	var sourceIDs []string
	for _, resp := range responses {
		if resp == llm.DoneResponse || len(resp.Choices) == 0 || resp.Choices[0].Delta == nil {
			continue
		}
		if resp.Choices[0].Delta.ReasoningSignature == nil {
			continue
		}

		signatures = append(signatures, *resp.Choices[0].Delta.ReasoningSignature)
		metadata, ok := getResponsesReasoningItemMetadata(resp.TransformerMetadata)
		require.True(t, ok)
		require.True(t, metadata.Done)
		sourceIDs = append(sourceIDs, metadata.ID)
	}

	require.Equal(t, []string{"gAAAA_done_1", "gAAAA_done_2"}, signatures)
	require.Equal(t, []string{"rs_1", "rs_2"}, sourceIDs)
}

func TestOutboundTransformer_TransformStream_AssociatesReasoningSummaryWithItem(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_reasoning_summary","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_first","type":"reasoning","summary":[]}}`)},
		{Type: "response.reasoning_summary_text.delta", Data: []byte(`{"type":"response.reasoning_summary_text.delta","item_id":"rs_first","output_index":0,"summary_index":0,"delta":"first"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_first","type":"reasoning","summary":[],"encrypted_content":"gAAAA_FIRST_BLOB"}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":1,"item":{"id":"rs_second","type":"reasoning","summary":[]}}`)},
		{Type: "response.reasoning_summary_text.delta", Data: []byte(`{"type":"response.reasoning_summary_text.delta","item_id":"rs_second","output_index":1,"summary_index":0,"delta":"second"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":1,"item":{"id":"rs_second","type":"reasoning","summary":[],"encrypted_content":"gAAAA_SECOND_BLOB"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_reasoning_summary","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
	}

	stream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	responses, err := streams.All(stream)
	require.NoError(t, err)

	var summaries []string
	var summaryItemIDs []string
	for _, response := range responses {
		if response == llm.DoneResponse || len(response.Choices) == 0 || response.Choices[0].Delta == nil || response.Choices[0].Delta.ReasoningContent == nil {
			continue
		}
		summaries = append(summaries, *response.Choices[0].Delta.ReasoningContent)
		metadata, ok := getResponsesReasoningItemMetadata(response.TransformerMetadata)
		require.True(t, ok)
		summaryItemIDs = append(summaryItemIDs, metadata.ID)
	}

	require.Equal(t, []string{"first", "second"}, summaries)
	require.Equal(t, []string{"rs_first", "rs_second"}, summaryItemIDs)
}

// TestOutboundTransformer_TransformStream_HandlesReasoningText verifies that
// official reasoning_text events become unified reasoning deltas without duplication.
func TestOutboundTransformer_TransformStream_HandlesReasoningText(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_reasoning_text","object":"response","created_at":1700000000,"model":"deepseek-v4-flash","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_text","type":"reasoning","status":"in_progress","summary":[],"content":[]}}`)},
		{Type: "response.reasoning_text.delta", Data: []byte(`{"type":"response.reasoning_text.delta","item_id":"rs_text","output_index":0,"content_index":0,"delta":"first"}`)},
		{Type: "response.reasoning_text.delta", Data: []byte(`{"type":"response.reasoning_text.delta","item_id":"rs_text","output_index":0,"content_index":0,"delta":" second"}`)},
		{Type: "response.reasoning_text.done", Data: []byte(`{"type":"response.reasoning_text.done","item_id":"rs_text","output_index":0,"content_index":0,"text":"first second"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_text","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"first second"}]}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_reasoning_text","object":"response","created_at":1700000000,"model":"deepseek-v4-flash","status":"completed","output":[]}}`)},
	}

	stream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	responses, err := streams.All(stream)
	require.NoError(t, err)

	var deltas []string
	var itemIDs []string
	for _, response := range responses {
		if response == llm.DoneResponse || len(response.Choices) == 0 || response.Choices[0].Delta == nil || response.Choices[0].Delta.ReasoningContent == nil {
			continue
		}

		deltas = append(deltas, *response.Choices[0].Delta.ReasoningContent)
		metadata, ok := getResponsesReasoningItemMetadata(response.TransformerMetadata)
		require.True(t, ok)
		itemIDs = append(itemIDs, metadata.ID)
	}

	require.Equal(t, []string{"first", " second"}, deltas)
	require.Equal(t, []string{"rs_text", "rs_text"}, itemIDs)
}

func TestResponsesTransformer_StreamRoundTrip_PreservesCompactionSummary(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_compact_1","object":"response.compaction","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"msg_1","type":"message","status":"completed","content":[{"type":"input_text","text":"Preserve this fact."}],"role":"user"}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":1,"item":{"id":"cmp_1","type":"compaction_summary","encrypted_content":"encrypted-summary"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_compact_1","object":"response.compaction","created_at":1700000000,"model":"gpt-5","status":"completed","output":[{"id":"msg_1","type":"message","status":"completed","content":[{"type":"input_text","text":"Preserve this fact."}],"role":"user"},{"id":"cmp_1","type":"compaction_summary","encrypted_content":"encrypted-summary"}],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`)},
	}

	llmStream, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)

	llmResponses, err := streams.All(llmStream)
	require.NoError(t, err)

	var compactPart *llm.MessageContentPart
	for _, resp := range llmResponses {
		if resp == llm.DoneResponse || len(resp.Choices) == 0 || resp.Choices[0].Delta == nil {
			continue
		}
		for i := range resp.Choices[0].Delta.Content.MultipleContent {
			part := &resp.Choices[0].Delta.Content.MultipleContent[i]
			if part.Type == "compaction_summary" {
				compactPart = part
			}
		}
	}
	require.NotNil(t, compactPart)
	require.NotNil(t, compactPart.Compact)
	require.Equal(t, "cmp_1", compactPart.Compact.ID)
	require.Equal(t, "encrypted-summary", compactPart.Compact.EncryptedContent)

	inboundStream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream(llmResponses))
	require.NoError(t, err)

	var compactDone *Item
	var completed *Response
	for inboundStream.Next() {
		var event StreamEvent
		require.NoError(t, json.Unmarshal(inboundStream.Current().Data, &event))
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "compaction_summary" {
			compactDone = event.Item
		}
		if event.Type == StreamEventTypeResponseCompleted {
			completed = event.Response
		}
	}
	require.NoError(t, inboundStream.Err())
	require.NotNil(t, compactDone)
	require.Equal(t, "cmp_1", compactDone.ID)
	require.Equal(t, "encrypted-summary", lo.FromPtr(compactDone.EncryptedContent))
	require.NotNil(t, completed)
	require.Equal(t, "resp_compact_1", completed.ID)
	require.Len(t, completed.Output, 1)
	require.Equal(t, "compaction_summary", completed.Output[0].Type)
	require.Equal(t, "encrypted-summary", lo.FromPtr(completed.Output[0].EncryptedContent))
}

func TestOutboundTransformer_TransformStream_ResponseCancelledCompletes(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_cancelled","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Type: "response.cancelled", Data: []byte(`{"type":"response.cancelled","response":{"id":"resp_cancelled","object":"response","created_at":1700000000,"model":"gpt-5","status":"canceled","output":[]}}`)},
	}

	stream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	responses, err := streams.All(stream)
	require.NoError(t, err)
	require.Len(t, responses, 3)
	require.Equal(t, llm.DoneResponse, responses[2])
	require.Equal(t, "resp_cancelled", responses[1].ID)
	require.Equal(t, "gpt-5", responses[1].Model)
	require.Equal(t, int64(1700000000), responses[1].Created)
	require.NotEmpty(t, responses[1].Choices)
	require.NotNil(t, responses[1].Choices[0].FinishReason)
	require.Equal(t, "cancelled", *responses[1].Choices[0].FinishReason)
}

func TestOutboundTransformer_TransformStream_EmitsArgumentsProvidedOnlyInDone(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.created","response":{"id":"resp_done_arguments","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_done_arguments","type":"function_call","call_id":"call_done_arguments","name":"collaboration.spawn_agent","arguments":""}}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_done_arguments","output_index":0,"name":"collaboration.spawn_agent","arguments":"{\"task\":\"delegate this task\"}"}`)},
		{Data: []byte(`{"type":"response.completed","response":{"id":"resp_done_arguments","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
	}

	stream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	responses, err := streams.All(stream)
	require.NoError(t, err)

	var arguments string
	for _, response := range responses {
		if response == llm.DoneResponse || len(response.Choices) == 0 || response.Choices[0].Delta == nil {
			continue
		}

		for _, toolCall := range response.Choices[0].Delta.ToolCalls {
			arguments += toolCall.Function.Arguments
		}
	}

	require.JSONEq(t, `{"task":"delegate this task"}`, arguments)
}

func TestOutboundTransformer_TransformStream_AcceptsEquivalentFinalArguments(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	forwardedArguments := `{"task":"delegate this task","priority":1}`
	finalArguments := `{
  "priority": 1,
  "task": "delegate this task"
}`
	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.created","response":{"id":"resp_equivalent_arguments","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_equivalent_arguments","type":"function_call","call_id":"call_equivalent_arguments","name":"spawn_agent","arguments":""}}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_equivalent_arguments","output_index":0,"delta":"{\"task\":\"delegate this task\",\"priority\":1}"}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_equivalent_arguments","output_index":0,"arguments":"{\n  \"priority\": 1,\n  \"task\": \"delegate this task\"\n}"}`)},
		{Data: []byte(`{"type":"response.completed","response":{"id":"resp_equivalent_arguments","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
	}

	stream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	responses, err := streams.All(stream)
	require.NoError(t, err)

	var arguments string
	for _, response := range responses {
		if response == llm.DoneResponse || len(response.Choices) == 0 || response.Choices[0].Delta == nil {
			continue
		}

		for _, toolCall := range response.Choices[0].Delta.ToolCalls {
			arguments += toolCall.Function.Arguments
		}
	}

	require.Equal(t, forwardedArguments, arguments)
	require.JSONEq(t, finalArguments, arguments)
}

func TestOutboundTransformer_TransformStream_RejectsDistinctLargeJSONNumbers(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.created","response":{"id":"resp_large_numbers","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_large_numbers","type":"function_call","call_id":"call_large_numbers","name":"spawn_agent","arguments":""}}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_large_numbers","output_index":0,"delta":"{\"value\":9007199254740992}"}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_large_numbers","output_index":0,"arguments":"{\"value\":9007199254740993}"}`)},
	}

	stream, err := trans.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	_, err = streams.All(stream)
	require.ErrorContains(t, err, `function call arguments mismatch for call_id "call_large_numbers"`)
}

func TestResponsesStream_RoundTrip_PreservesToolIdentityProvidedOnlyInDone(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	upstreamEvents := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.created","response":{"id":"resp_done_identity","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_done_identity","type":"function_call","call_id":"call_done_identity","arguments":""}}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_done_identity","output_index":0,"name":"spawn_agent","namespace":"collaboration","arguments":"{\"task\":\"delegate this task\"}"}`)},
		{Data: []byte(`{"type":"response.completed","response":{"id":"resp_done_identity","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
	}

	unifiedStream, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)

	clientStream, err := NewInboundTransformer().TransformStream(t.Context(), unifiedStream)
	require.NoError(t, err)

	clientEvents, err := streams.All(clientStream)
	require.NoError(t, err)

	var addedItem *Item
	var completedItem *Item
	var completedArguments *StreamEvent
	for _, clientEvent := range clientEvents {
		if string(clientEvent.Data) == "[DONE]" {
			continue
		}

		var event StreamEvent
		require.NoError(t, json.Unmarshal(clientEvent.Data, &event))
		if event.Type == StreamEventTypeOutputItemAdded && event.Item != nil && event.Item.Type == "function_call" {
			addedItem = event.Item
		}
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "function_call" {
			completedItem = event.Item
		}
		if event.Type == StreamEventTypeFunctionCallArgumentsDone {
			completedArguments = &event
		}
	}

	require.NotNil(t, addedItem)
	require.NotNil(t, completedItem)
	require.NotNil(t, completedArguments)
	require.Equal(t, "call_done_identity", completedArguments.CallID)
	require.Equal(t, "spawn_agent", completedArguments.Name)
	require.Equal(t, "collaboration", completedArguments.Namespace)
	require.JSONEq(t, `{"task":"delegate this task"}`, completedArguments.Arguments)
	require.Equal(t, "spawn_agent", addedItem.Name)
	require.Equal(t, "collaboration", addedItem.Namespace)
	require.Equal(t, "spawn_agent", completedItem.Name)
	require.Equal(t, "collaboration", completedItem.Namespace)
	require.JSONEq(t, `{"task":"delegate this task"}`, completedItem.Arguments)
}

func TestResponsesStream_RoundTrip_DefersFunctionCallUntilLateIdentityArrives(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	arguments := `{"task":"delegate this task"}`
	upstreamEvents := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.created","response":{"id":"resp_late_identity","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_late_identity","type":"function_call","call_id":"call_late_identity","arguments":""}}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_late_identity","output_index":0,"delta":"{\"task\":\"delegate this task\"}"}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_late_identity","output_index":0,"name":"spawn_agent","namespace":"collaboration","arguments":"{\"task\":\"delegate this task\"}"}`)},
		{Data: []byte(`{"type":"response.completed","response":{"id":"resp_late_identity","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
	}

	unifiedStream, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)

	clientStream, err := NewInboundTransformer().TransformStream(t.Context(), unifiedStream)
	require.NoError(t, err)

	clientEvents, err := streams.All(clientStream)
	require.NoError(t, err)

	var addedItems []*Item
	var receivedArguments string
	for _, clientEvent := range clientEvents {
		if string(clientEvent.Data) == "[DONE]" {
			continue
		}

		var event StreamEvent
		require.NoError(t, json.Unmarshal(clientEvent.Data, &event))
		if event.Type == StreamEventTypeOutputItemAdded && event.Item != nil && event.Item.Type == "function_call" {
			addedItems = append(addedItems, event.Item)
		}
		if event.Type == StreamEventTypeFunctionCallArgumentsDelta {
			receivedArguments += event.Delta
		}
	}

	require.Len(t, addedItems, 1)
	require.Equal(t, "spawn_agent", addedItems[0].Name)
	require.Equal(t, "collaboration", addedItems[0].Namespace)
	require.JSONEq(t, arguments, receivedArguments)
}

func TestOutboundTransformer_TransformStream_PreservesFinalItemAnnotations(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type":"response.created",
				"response":{
					"id":"resp_stream_annotations",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-4o-search-preview",
					"status":"in_progress",
					"output":[]
				}
			}`),
		},
		{
			Type: "response.output_item.added",
			Data: []byte(`{
				"type":"response.output_item.added",
				"output_index":0,
				"item":{
					"id":"msg_stream_annotations",
					"type":"message",
					"status":"in_progress",
					"role":"assistant"
				}
			}`),
		},
		{
			Type: "response.content_part.added",
			Data: []byte(`{
				"type":"response.content_part.added",
				"item_id":"msg_stream_annotations",
				"output_index":0,
				"content_index":0,
				"part":{
					"type":"output_text",
					"text":""
				}
			}`),
		},
		{
			Type: "response.output_text.delta",
			Data: []byte(`{
				"type":"response.output_text.delta",
				"item_id":"msg_stream_annotations",
				"output_index":0,
				"content_index":0,
				"delta":"Search result"
			}`),
		},
		{
			Type: "response.output_text.done",
			Data: []byte(`{
				"type":"response.output_text.done",
				"item_id":"msg_stream_annotations",
				"output_index":0,
				"content_index":0,
				"text":"Search result"
			}`),
		},
		{
			Type: "response.output_item.done",
			Data: []byte(`{
				"type":"response.output_item.done",
				"output_index":0,
				"item":{
					"id":"msg_stream_annotations",
					"type":"message",
					"status":"completed",
					"role":"assistant",
					"content":[{
						"type":"output_text",
						"text":"Search result",
						"annotations":[{
							"type":"url_citation",
							"start_index":0,
							"end_index":6,
							"url_citation":{
								"url":"https://example.com/result",
								"title":"Example Result"
							}
						}]
					}]
				}
			}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type":"response.completed",
				"response":{
					"id":"resp_stream_annotations",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-4o-search-preview",
					"status":"completed",
					"output":[]
				}
			}`),
		},
	}

	stream, err := trans.TransformStream(context.Background(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	actual, err := streams.All(stream)
	require.NoError(t, err)
	require.NotEmpty(t, actual)

	var found []llm.Annotation
	for _, resp := range actual {
		if resp == llm.DoneResponse {
			continue
		}
		for _, choice := range resp.Choices {
			if choice.Message != nil && len(choice.Message.Annotations) > 0 {
				found = choice.Message.Annotations
				break
			}
			if choice.Delta != nil && len(choice.Delta.Annotations) > 0 {
				found = choice.Delta.Annotations
				break
			}
		}
		if len(found) > 0 {
			break
		}
	}

	require.Len(t, found, 1)
	require.Equal(t, "url_citation", found[0].Type)
	require.NotNil(t, found[0].URLCitation)
	require.Equal(t, "https://example.com/result", found[0].URLCitation.URL)
	require.Equal(t, "Example Result", found[0].URLCitation.Title)
	require.NotNil(t, found[0].StartIndex)
	require.NotNil(t, found[0].EndIndex)
	require.EqualValues(t, 0, *found[0].StartIndex)
	require.EqualValues(t, 6, *found[0].EndIndex)
}

func TestOutboundTransformer_TransformStream_PreservesWebSearchMetadataOnAnnotationChunk(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type":"response.created",
				"response":{
					"id":"resp_stream_web_search_annotations",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-4o-search-preview",
					"status":"in_progress",
					"output":[]
				}
			}`),
		},
		{
			Type: "response.output_item.done",
			Data: []byte(`{
				"type":"response.output_item.done",
				"output_index":0,
				"item":{
					"id":"ws_123",
					"type":"web_search_call",
					"status":"completed",
					"action":{
						"type":"search",
						"query":"latest ai news",
						"sources":[{"type":"url","url":"https://example.com/source","title":"Example Source"}]
					}
				}
			}`),
		},
		{
			Type: "response.output_item.done",
			Data: []byte(`{
				"type":"response.output_item.done",
				"output_index":1,
				"item":{
					"id":"msg_stream_web_search_annotations",
					"type":"message",
					"status":"completed",
					"role":"assistant",
					"content":[{
						"type":"output_text",
						"text":"Search result",
						"annotations":[{
							"type":"url_citation",
							"url_citation":{
								"url":"https://example.com/result",
								"title":"Example Result"
							}
						}]
					}]
				}
			}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type":"response.completed",
				"response":{
					"id":"resp_stream_web_search_annotations",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-4o-search-preview",
					"status":"completed",
					"output":[]
				}
			}`),
		},
	}

	stream, err := trans.TransformStream(context.Background(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	actual, err := streams.All(stream)
	require.NoError(t, err)
	require.NotEmpty(t, actual)

	var annotationChunk *llm.Response
	for _, resp := range actual {
		if resp == llm.DoneResponse {
			continue
		}
		for _, choice := range resp.Choices {
			if choice.Delta != nil && len(choice.Delta.Annotations) > 0 {
				annotationChunk = resp
				break
			}
		}
		if annotationChunk != nil {
			break
		}
	}

	require.NotNil(t, annotationChunk)
	require.NotNil(t, annotationChunk.TransformerMetadata)
	calls, ok := annotationChunk.TransformerMetadata[responsesWebSearchCallsTransformerMetadataKey]
	require.True(t, ok)
	require.NotNil(t, calls)
}

func TestOutboundTransformer_TransformStream_PreservesWebSearchMetadataWithoutAnnotations(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type":"response.created",
				"response":{
					"id":"resp_stream_web_search_no_annotations",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-4o-search-preview",
					"status":"in_progress",
					"output":[]
				}
			}`),
		},
		{
			Type: "response.output_item.done",
			Data: []byte(`{
				"type":"response.output_item.done",
				"output_index":0,
				"item":{
					"id":"ws_456",
					"type":"web_search_call",
					"status":"completed",
					"action":{
						"type":"search",
						"query":"latest ai news",
						"sources":[{"type":"url","url":"https://example.com/source","title":"Example Source"}]
					}
				}
			}`),
		},
		{
			Type: "response.output_item.done",
			Data: []byte(`{
				"type":"response.output_item.done",
				"output_index":1,
				"item":{
					"id":"msg_stream_web_search_no_annotations",
					"type":"message",
					"status":"completed",
					"role":"assistant",
					"content":[{
						"type":"output_text",
						"text":"Search result without inline citations"
					}]
				}
			}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type":"response.completed",
				"response":{
					"id":"resp_stream_web_search_no_annotations",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-4o-search-preview",
					"status":"completed",
					"output":[]
				}
			}`),
		},
	}

	stream, err := trans.TransformStream(context.Background(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	actual, err := streams.All(stream)
	require.NoError(t, err)
	require.NotEmpty(t, actual)

	var metadataChunk *llm.Response
	for _, resp := range actual {
		if resp == llm.DoneResponse {
			continue
		}
		if resp.TransformerMetadata != nil {
			if _, ok := resp.TransformerMetadata[responsesWebSearchCallsTransformerMetadataKey]; ok {
				metadataChunk = resp
				break
			}
		}
	}

	require.NotNil(t, metadataChunk)
	calls, ok := metadataChunk.TransformerMetadata[responsesWebSearchCallsTransformerMetadataKey]
	require.True(t, ok)
	require.NotNil(t, calls)
}

func TestOutboundTransformer_TransformStream_PreservesPreviousResponseID(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type":"response.created",
				"response":{
					"id":"resp_stream_prev",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-5.4",
					"status":"in_progress",
					"previous_response_id":"resp_prev_123",
					"output":[]
				}
			}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type":"response.completed",
				"response":{
					"id":"resp_stream_prev",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-5.4",
					"status":"completed",
					"previous_response_id":"resp_prev_123",
					"output":[],
					"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}
				}
			}`),
		},
	}

	stream, err := trans.TransformStream(context.Background(), nil, streams.SliceStream(events))
	require.NoError(t, err)

	actual, err := streams.All(stream)
	require.NoError(t, err)
	require.Len(t, actual, 4)

	require.NotNil(t, actual[0].PreviousResponseID)
	require.Equal(t, "resp_prev_123", *actual[0].PreviousResponseID)

	require.NotNil(t, actual[1].PreviousResponseID)
	require.Equal(t, "resp_prev_123", *actual[1].PreviousResponseID)

	require.NotNil(t, actual[2].PreviousResponseID)
	require.Equal(t, "resp_prev_123", *actual[2].PreviousResponseID)
	require.Equal(t, llm.DoneResponse, actual[3])
}
