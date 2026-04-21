package responses

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm/streams"
)

// Compare each event.
var ignoreFields = cmp.FilterPath(func(p cmp.Path) bool {
	// Ignore dynamic fields that are generated at runtime
	if sf, ok := p.Last().(cmp.StructField); ok {
		switch sf.Name() {
		case "ID", "ItemID", "Obfuscation", "Logprobs", "Response":
			return true
		}
	}
	return false
}, cmp.Ignore())

func TestInboundTransformer_StreamTransformation_WithTestData(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name                 string
		inputStreamFile      string
		expectedStreamFile   string
		expectedResponseFile string
	}{
		{
			name:                 "stream transformation with text and multiple tool calls",
			inputStreamFile:      "llm-tool-2.stream.jsonl",
			expectedStreamFile:   "tool-2.stream.jsonl",
			expectedResponseFile: "tool-2.response.json",
		},
		{
			name:                 "stream transformation with custom tool call",
			inputStreamFile:      "llm-custom_tool.stream.jsonl",
			expectedStreamFile:   "custom_tool.stream.jsonl",
			expectedResponseFile: "custom_tool.stream.response.json",
		},
		{
			name:                 "stream transformation with encrypted reasoning only (no summary items)",
			inputStreamFile:      "llm-encrypted_only.stream.jsonl",
			expectedStreamFile:   "encrypted_only.stream.jsonl",
			expectedResponseFile: "encrypted_only.response.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the input file (LLM format responses)
			llmResponses, err := xtest.LoadLlmResponses(t, tt.inputStreamFile)
			require.NoError(t, err)

			// Load expected events from the expected stream file
			expectedEvents, err := xtest.LoadStreamChunks(t, tt.expectedStreamFile)
			require.NoError(t, err)

			// Create a mock stream from LLM responses
			mockStream := streams.SliceStream(llmResponses)

			// Transform the stream (LLM -> OpenAI Responses API)
			transformedStream, err := trans.TransformStream(t.Context(), mockStream)
			require.NoError(t, err)

			// Collect all transformed events
			var actualEvents []StreamEvent

			for transformedStream.Next() {
				event := transformedStream.Current()

				var ev StreamEvent

				err := json.Unmarshal(event.Data, &ev)
				require.NoError(t, err)

				actualEvents = append(actualEvents, ev)
			}

			require.NoError(t, transformedStream.Err())

			// Verify event count
			require.Equal(t, len(expectedEvents), len(actualEvents), "Event count should match expected")

			for i, expectedEvent := range expectedEvents {
				var expected StreamEvent

				err := json.Unmarshal(expectedEvent.Data, &expected)
				require.NoError(t, err)

				actual := actualEvents[i]

				if !xtest.Equal(expected, actual, ignoreFields) {
					t.Fatalf("event %d mismatch:\n%s", i, cmp.Diff(expected, actual, ignoreFields))
				}
			}

			// Verify the last event is response.completed and compare with expectedResponseFile
			if tt.expectedResponseFile != "" {
				require.NotEmpty(t, actualEvents, "Expected at least one event")

				lastEvent := actualEvents[len(actualEvents)-1]
				require.Equal(t, StreamEventTypeResponseCompleted, lastEvent.Type,
					"Last event should be response.completed")
				require.NotNil(t, lastEvent.Response, "response.completed event should have Response")

				// Load expected response from file
				var expectedResponse Response

				err := xtest.LoadTestData(t, tt.expectedResponseFile, &expectedResponse)
				require.NoError(t, err)

				// Compare the response in the event with the expected response file
				// Ignore dynamic fields like ID, ItemID
				responseIgnoreFields := cmp.FilterPath(func(p cmp.Path) bool {
					if sf, ok := p.Last().(cmp.StructField); ok {
						switch sf.Name() {
						case "ID", "ItemID", "Obfuscation", "Logprobs":
							return true
						}
					}

					return false
				}, cmp.Ignore())

				if !xtest.Equal(expectedResponse, *lastEvent.Response, responseIgnoreFields) {
					t.Fatalf("response.completed response mismatch:\n%s",
						cmp.Diff(expectedResponse, *lastEvent.Response, responseIgnoreFields))
				}
			}
		})
	}
}

func TestInboundTransformer_StreamTransformation_PreservesToolSearchItems(t *testing.T) {
	trans := NewInboundTransformer()

	callItem := Item{
		ID:        "tsc_123",
		Type:      "tool_search_call",
		CallID:    "call_abc123",
		Execution: "client",
		Status:    lo.ToPtr("completed"),
		Arguments: `{"goal":"Find the shipping ETA tool for order_42."}`,
	}
	outputItem := Item{
		ID:        "tso_123",
		Type:      "tool_search_output",
		CallID:    "call_abc123",
		Execution: "client",
		Status:    lo.ToPtr("completed"),
		Tools: []Tool{
			{
				Type:        "function",
				Name:        "get_shipping_eta",
				Description: "Look up shipping ETA details for an order.",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		},
	}

	callRaw, err := json.Marshal(callItem)
	require.NoError(t, err)

	outputRaw, err := json.Marshal(outputItem)
	require.NoError(t, err)

	mockStream := streams.SliceStream([]*llm.Response{
		{
			ID:      "resp_tool_search",
			Model:   "gpt-5.4",
			Created: 1700000000,
			Choices: []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type:        "tool_search_call",
									ServerBlock: callRaw,
								},
								{
									Type:        "tool_search_output",
									ServerBlock: outputRaw,
								},
							},
						},
					},
					FinishReason: lo.ToPtr("tool_calls"),
				},
			},
			Usage: &llm.Usage{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		},
	})

	transformedStream, err := trans.TransformStream(t.Context(), mockStream)
	require.NoError(t, err)

	var actualEvents []StreamEvent
	for transformedStream.Next() {
		event := transformedStream.Current()

		var ev StreamEvent
		err := json.Unmarshal(event.Data, &ev)
		require.NoError(t, err)

		actualEvents = append(actualEvents, ev)
	}
	require.NoError(t, transformedStream.Err())

	var toolSearchEvents []StreamEvent
	for _, ev := range actualEvents {
		if ev.Type != StreamEventTypeOutputItemAdded && ev.Type != StreamEventTypeOutputItemDone {
			continue
		}
		if ev.Item == nil {
			continue
		}
		if ev.Item.Type == "tool_search_call" || ev.Item.Type == "tool_search_output" {
			toolSearchEvents = append(toolSearchEvents, ev)
		}
	}

	require.Len(t, toolSearchEvents, 4)
	require.Equal(t, StreamEventTypeOutputItemAdded, toolSearchEvents[0].Type)
	require.Equal(t, "tool_search_call", toolSearchEvents[0].Item.Type)
	require.Equal(t, "in_progress", *toolSearchEvents[0].Item.Status)
	require.Equal(t, StreamEventTypeOutputItemDone, toolSearchEvents[1].Type)
	require.Equal(t, "tool_search_call", toolSearchEvents[1].Item.Type)
	require.Equal(t, "completed", *toolSearchEvents[1].Item.Status)
	require.JSONEq(t, callItem.Arguments, toolSearchEvents[1].Item.Arguments)

	require.Equal(t, StreamEventTypeOutputItemAdded, toolSearchEvents[2].Type)
	require.Equal(t, "tool_search_output", toolSearchEvents[2].Item.Type)
	require.Equal(t, "in_progress", *toolSearchEvents[2].Item.Status)
	require.Equal(t, StreamEventTypeOutputItemDone, toolSearchEvents[3].Type)
	require.Equal(t, "tool_search_output", toolSearchEvents[3].Item.Type)
	require.Equal(t, "completed", *toolSearchEvents[3].Item.Status)
	require.Len(t, toolSearchEvents[3].Item.Tools, 1)
	require.Equal(t, "get_shipping_eta", toolSearchEvents[3].Item.Tools[0].Name)

	lastEvent := actualEvents[len(actualEvents)-1]
	require.Equal(t, StreamEventTypeResponseCompleted, lastEvent.Type)
	require.NotNil(t, lastEvent.Response)

	var outputTypes []string
	for _, item := range lastEvent.Response.Output {
		if item.Type == "tool_search_call" || item.Type == "tool_search_output" {
			outputTypes = append(outputTypes, item.Type)
		}
	}
	require.Equal(t, []string{"tool_search_call", "tool_search_output"}, outputTypes)
}
