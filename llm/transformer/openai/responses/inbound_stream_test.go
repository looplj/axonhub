package responses

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xtest"
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

// TestReasoningItemSummaryFieldSerialization verifies that the Summary field
// in a reasoning Item is always serialized, even when empty.
// This is critical for Codex compatibility - Codex's ResponseItem::Reasoning
// requires the "summary" field to be present (it has no #[serde(default)]).
// Without this field, Codex fails to parse the item and reasoning streaming breaks.
func TestReasoningItemSummaryFieldSerialization(t *testing.T) {
	item := &Item{
		ID:      "item_test",
		Type:    "reasoning",
		Status:  lo.ToPtr("in_progress"),
		Summary: []ReasoningSummary{}, // Empty array
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	jsonStr := string(data)

	// Verify "summary" field is present in JSON output
	require.Contains(t, jsonStr, `"summary"`, "summary field must be present in JSON output for Codex compatibility")
	require.Contains(t, jsonStr, `"summary":[]`, "summary field should be serialized as empty array")

	// Verify the JSON can be unmarshaled back
	var unmarshaled Item

	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	require.NotNil(t, unmarshaled.Summary, "Summary should not be nil after unmarshal")
	require.Empty(t, unmarshaled.Summary, "Summary should be empty array")
}
