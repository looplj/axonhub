package responses

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestInboundTransformer_TransformStream_EmitsUpstreamErrorEvents(t *testing.T) {
	tests := []struct {
		name      string
		source    streams.Stream[*llm.Response]
		wantTypes []StreamEventType
		assert    func(t *testing.T, events []StreamEvent)
	}{
		{
			name:      "emits error event before response starts",
			source:    &errorResponseStream{err: errors.New("upstream boom")},
			wantTypes: []StreamEventType{StreamEventTypeError},
			assert: func(t *testing.T, events []StreamEvent) {
				require.Equal(t, "stream_error", events[0].Code)
				require.Equal(t, "upstream boom", events[0].Message)
			},
		},
		{
			name: "emits response.failed after response starts",
			source: &errorResponseStream{
				items: []*llm.Response{{
					ID:      "resp_123",
					Model:   "gpt-test",
					Created: 123,
				}},
				err: errors.New("upstream boom"),
			},
			wantTypes: []StreamEventType{
				StreamEventTypeResponseCreated,
				StreamEventTypeResponseInProgress,
				StreamEventTypeResponseFailed,
			},
			assert: func(t *testing.T, events []StreamEvent) {
				failed := events[len(events)-1]
				require.NotNil(t, failed.Response)
				require.NotNil(t, failed.Response.Status)
				require.Equal(t, "failed", *failed.Response.Status)
				require.NotNil(t, failed.Response.Error)
				require.Equal(t, "stream_error", failed.Response.Error.Code)
				require.Equal(t, "upstream boom", failed.Response.Error.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformedStream, err := NewInboundTransformer().TransformStream(t.Context(), tt.source)
			require.NoError(t, err)

			actualEvents := make([]StreamEvent, 0, len(tt.wantTypes))
			for range 10 {
				if !transformedStream.Next() {
					break
				}

				event := transformedStream.Current()
				require.NotNil(t, event)

				var actual StreamEvent
				err := json.Unmarshal(event.Data, &actual)
				require.NoError(t, err)

				actualEvents = append(actualEvents, actual)
			}

			require.Len(t, actualEvents, len(tt.wantTypes))
			for i, wantType := range tt.wantTypes {
				require.Equal(t, wantType, actualEvents[i].Type)
			}

			require.False(t, transformedStream.Next())
			require.NoError(t, transformedStream.Err())

			tt.assert(t, actualEvents)
		})
	}
}

type errorResponseStream struct {
	items []*llm.Response
	index int
	err   error
}

func (s *errorResponseStream) Next() bool {
	if s.index < len(s.items) {
		s.index++
		return true
	}

	return false
}

func (s *errorResponseStream) Current() *llm.Response {
	if s.index == 0 || s.index > len(s.items) {
		return nil
	}

	return s.items[s.index-1]
}

func (s *errorResponseStream) Err() error {
	if s.index >= len(s.items) {
		return s.err
	}

	return nil
}

func (s *errorResponseStream) Close() error {
	return nil
}
