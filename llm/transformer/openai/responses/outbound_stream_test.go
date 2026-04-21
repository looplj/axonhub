package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
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
			transformedStream, err := trans.TransformStream(t.Context(), streams.SliceStream(responsesAPIEvents))
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

	transformedStream, err := trans.TransformStream(t.Context(), streams.SliceStream(responsesAPIEvents))
	require.NoError(t, err)

	_, err = streams.All(transformedStream)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Something went wrong")
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

	stream, err := trans.TransformStream(context.Background(), streams.SliceStream(events))
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

func TestOutboundTransformer_TransformStream_PreservesToolSearchItems(t *testing.T) {
	trans, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type":"response.created",
				"response":{
					"id":"resp_tool_search",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-5.4",
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
					"id":"tsc_123",
					"type":"tool_search_call",
					"call_id":"call_abc123",
					"execution":"client",
					"status":"completed",
					"arguments":{"goal":"Find the shipping ETA tool for order_42."}
				}
			}`),
		},
		{
			Type: "response.output_item.added",
			Data: []byte(`{
				"type":"response.output_item.added",
				"output_index":1,
				"item":{
					"id":"tso_123",
					"type":"tool_search_output",
					"call_id":"call_abc123",
					"execution":"client",
					"status":"completed",
					"tools":[{
						"type":"function",
						"name":"get_shipping_eta",
						"description":"Look up shipping ETA details for an order.",
						"parameters":{"type":"object"}
					}]
				}
			}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type":"response.completed",
				"response":{
					"id":"resp_tool_search",
					"object":"response",
					"created_at":1700000000,
					"model":"gpt-5.4",
					"status":"completed",
					"output":[],
					"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}
				}
			}`),
		},
	}

	stream, err := trans.TransformStream(context.Background(), streams.SliceStream(events))
	require.NoError(t, err)

	actual, err := streams.All(stream)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(actual), 5)

	var toolSearchChunks []*llm.Response
	for _, resp := range actual {
		if resp == nil || resp == llm.DoneResponse || len(resp.Choices) == 0 || resp.Choices[0].Delta == nil {
			continue
		}

		if len(resp.Choices[0].Delta.Content.MultipleContent) > 0 {
			partType := resp.Choices[0].Delta.Content.MultipleContent[0].Type
			if partType == "tool_search_call" || partType == "tool_search_output" {
				toolSearchChunks = append(toolSearchChunks, resp)
			}
		}
	}

	require.Len(t, toolSearchChunks, 2)

	var callItem Item
	err = json.Unmarshal(toolSearchChunks[0].Choices[0].Delta.Content.MultipleContent[0].ServerBlock, &callItem)
	require.NoError(t, err)
	require.Equal(t, "tool_search_call", callItem.Type)
	require.Equal(t, "call_abc123", callItem.CallID)
	require.JSONEq(t, `{"goal":"Find the shipping ETA tool for order_42."}`, callItem.Arguments)

	var outputItem Item
	err = json.Unmarshal(toolSearchChunks[1].Choices[0].Delta.Content.MultipleContent[0].ServerBlock, &outputItem)
	require.NoError(t, err)
	require.Equal(t, "tool_search_output", outputItem.Type)
	require.Equal(t, "call_abc123", outputItem.CallID)
	require.Len(t, outputItem.Tools, 1)
	require.Equal(t, "get_shipping_eta", outputItem.Tools[0].Name)
}
