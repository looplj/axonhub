package responses

import (
	"encoding/json"
	"errors"
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

func TestInboundTransformer_TransformStream_KeepsResponsesReasoningItemsSeparate(t *testing.T) {
	trans := NewInboundTransformer()

	stream, err := trans.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_reasoning_multi",
			Created: 1700000000,
			Model:   "gpt-5",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{Role: "assistant"},
			}},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_reasoning_multi",
			Created: 1700000000,
			Model:   "gpt-5",
			TransformerMetadata: map[string]any{
				responsesReasoningItemTransformerMetadataKey: responsesReasoningItemMetadata{ID: "rs_1", Done: true},
			},
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{ID: "rs_1", ReasoningSignature: lo.ToPtr("gAAAA_done_1")},
			}},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_reasoning_multi",
			Created: 1700000000,
			Model:   "gpt-5",
			TransformerMetadata: map[string]any{
				responsesReasoningItemTransformerMetadataKey: map[string]any{"id": "rs_2", "done": true},
			},
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{ID: "rs_2", ReasoningSignature: lo.ToPtr("gAAAA_done_2")},
			}},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_reasoning_multi",
			Created: 1700000000,
			Model:   "gpt-5",
			Choices: []llm.Choice{{
				Index:        0,
				Delta:        &llm.Message{},
				FinishReason: lo.ToPtr("stop"),
			}},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_reasoning_multi",
			Created: 1700000000,
			Model:   "gpt-5",
			Usage:   &llm.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		},
	}))
	require.NoError(t, err)

	var actualEvents []StreamEvent
	for stream.Next() {
		event := stream.Current()
		var ev StreamEvent
		err := json.Unmarshal(event.Data, &ev)
		require.NoError(t, err)
		actualEvents = append(actualEvents, ev)
	}
	require.NoError(t, stream.Err())

	var doneItems []Item
	for _, event := range actualEvents {
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "reasoning" {
			doneItems = append(doneItems, *event.Item)
		}
	}

	require.Len(t, doneItems, 2)
	require.Equal(t, "rs_1", doneItems[0].ID)
	require.Equal(t, "gAAAA_done_1", lo.FromPtr(doneItems[0].EncryptedContent))
	require.Equal(t, "rs_2", doneItems[1].ID)
	require.Equal(t, "gAAAA_done_2", lo.FromPtr(doneItems[1].EncryptedContent))

	lastEvent := actualEvents[len(actualEvents)-1]
	require.Equal(t, StreamEventTypeResponseCompleted, lastEvent.Type)
	require.NotNil(t, lastEvent.Response)
	require.Len(t, lastEvent.Response.Output, 2)
	require.Equal(t, "rs_1", lastEvent.Response.Output[0].ID)
	require.Equal(t, "gAAAA_done_1", lo.FromPtr(lastEvent.Response.Output[0].EncryptedContent))
	require.Equal(t, "rs_2", lastEvent.Response.Output[1].ID)
	require.Equal(t, "gAAAA_done_2", lo.FromPtr(lastEvent.Response.Output[1].EncryptedContent))
}

func TestInboundTransformer_TransformStream_EmitsAdaptedSpecialToolCalls(t *testing.T) {
	wrappedMetadata := map[string]any{"openai_responses_chat_wrapped_custom": true}
	source := streams.SliceStream([]*llm.Response{
		{ID: "resp_tools", Model: "glm-5.2", Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{Role: "assistant"}}}},
		{ID: "resp_tools", Model: "glm-5.2", Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
			{ID: "call_custom", Index: 0, ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_custom", Name: "apply_patch", Input: `{"input":"*** Begin`}, TransformerMetadata: wrappedMetadata},
			{ID: "call_search", Index: 1, ResponseToolSearchCall: &llm.ResponseToolSearchCall{CallID: "call_search", Execution: "client", Arguments: `{"query":"agents"}`}},
		}}}}},
		{ID: "resp_tools", Model: "glm-5.2", Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{
			{Index: 0, ResponseCustomToolCall: &llm.ResponseCustomToolCall{Input: ` Patch"}`}, TransformerMetadata: wrappedMetadata},
		}}}}},
		{ID: "resp_tools", Model: "glm-5.2", Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{}, FinishReason: lo.ToPtr("tool_calls")}}},
		llm.DoneResponse,
	})

	stream, err := NewInboundTransformer().TransformStream(t.Context(), source)
	require.NoError(t, err)
	var events []StreamEvent
	for stream.Next() {
		var event StreamEvent
		require.NoError(t, json.Unmarshal(stream.Current().Data, &event))
		events = append(events, event)
	}
	require.NoError(t, stream.Err())

	var customDone, searchDone *Item
	for i := range events {
		event := events[i]
		if event.Type == StreamEventTypeCustomToolCallInputDelta {
			require.NotContains(t, event.Delta, `{"input"`)
		}
		if event.Type != StreamEventTypeOutputItemDone || event.Item == nil {
			continue
		}
		switch event.Item.Type {
		case "custom_tool_call":
			customDone = event.Item
		case "tool_search_call":
			searchDone = event.Item
		}
	}
	require.NotNil(t, customDone)
	require.Equal(t, "*** Begin Patch", lo.FromPtr(customDone.Input))
	require.NotNil(t, searchDone)
	require.Equal(t, "client", searchDone.Execution)
	require.JSONEq(t, `{"query":"agents"}`, searchDone.Arguments)
}

func TestInboundTransformer_TransformStream_ReplacesItemScopedProvisionalSignature(t *testing.T) {
	trans := NewInboundTransformer()
	stream, err := trans.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			Object: "chat.completion.chunk",
			TransformerMetadata: map[string]any{
				responsesReasoningItemTransformerMetadataKey: responsesReasoningItemMetadata{ID: "rs_1"},
			},
			Choices: []llm.Choice{{Delta: &llm.Message{
				ID:                 "rs_1",
				ReasoningSignature: lo.ToPtr("gAAAA_PROVISIONAL_BLOB"),
			}}},
		},
		{
			Object: "chat.completion.chunk",
			TransformerMetadata: map[string]any{
				responsesReasoningItemTransformerMetadataKey: responsesReasoningItemMetadata{ID: "rs_1", Done: true},
			},
			Choices: []llm.Choice{{Delta: &llm.Message{
				ID:                 "rs_1",
				ReasoningSignature: lo.ToPtr("gAAAA_FINAL_BLOB"),
			}}},
		},
		{Object: "chat.completion.chunk", Choices: []llm.Choice{{Delta: &llm.Message{}, FinishReason: lo.ToPtr("stop")}}},
		{Object: "chat.completion.chunk", Usage: &llm.Usage{}},
	}))
	require.NoError(t, err)

	var doneItems []Item
	for stream.Next() {
		var event StreamEvent
		require.NoError(t, json.Unmarshal(stream.Current().Data, &event))
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "reasoning" {
			doneItems = append(doneItems, *event.Item)
		}
	}
	require.NoError(t, stream.Err())
	require.Len(t, doneItems, 1)
	require.Equal(t, "rs_1", doneItems[0].ID)
	require.Equal(t, "gAAAA_FINAL_BLOB", lo.FromPtr(doneItems[0].EncryptedContent))
	require.NotEqual(t, "gAAAA_PROVISIONAL_BLOBgAAAA_FINAL_BLOB", lo.FromPtr(doneItems[0].EncryptedContent))
}

func TestInboundTransformer_TransformStream_UsesItemMetadataForSummaryDeltas(t *testing.T) {
	trans := NewInboundTransformer()
	stream, err := trans.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			Object: "chat.completion.chunk",
			TransformerMetadata: map[string]any{
				responsesReasoningItemTransformerMetadataKey: map[string]any{"id": "rs_first"},
			},
			Choices: []llm.Choice{{Delta: &llm.Message{ReasoningContent: lo.ToPtr("first")}}},
		},
		{
			Object: "chat.completion.chunk",
			TransformerMetadata: map[string]any{
				responsesReasoningItemTransformerMetadataKey: map[string]any{"id": "rs_second"},
			},
			Choices: []llm.Choice{{Delta: &llm.Message{ReasoningContent: lo.ToPtr("second")}}},
		},
		{
			Object: "chat.completion.chunk",
			TransformerMetadata: map[string]any{
				responsesReasoningItemTransformerMetadataKey: map[string]any{"id": "rs_first", "done": true},
			},
			Choices: []llm.Choice{{Delta: &llm.Message{ID: "rs_first", ReasoningSignature: lo.ToPtr("gAAAA_FIRST_BLOB")}}},
		},
		{
			Object: "chat.completion.chunk",
			TransformerMetadata: map[string]any{
				responsesReasoningItemTransformerMetadataKey: map[string]any{"id": "rs_second", "done": true},
			},
			Choices: []llm.Choice{{Delta: &llm.Message{ID: "rs_second", ReasoningSignature: lo.ToPtr("gAAAA_SECOND_BLOB")}}},
		},
		{Object: "chat.completion.chunk", Choices: []llm.Choice{{Delta: &llm.Message{}, FinishReason: lo.ToPtr("stop")}}},
		{Object: "chat.completion.chunk", Usage: &llm.Usage{}},
	}))
	require.NoError(t, err)

	var doneItems []Item
	for stream.Next() {
		var event StreamEvent
		require.NoError(t, json.Unmarshal(stream.Current().Data, &event))
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "reasoning" {
			doneItems = append(doneItems, *event.Item)
		}
	}
	require.NoError(t, stream.Err())
	require.Len(t, doneItems, 2)
	require.Equal(t, "rs_first", doneItems[0].ID)
	require.Equal(t, "first", doneItems[0].Summary[0].Text)
	require.Equal(t, "gAAAA_FIRST_BLOB", lo.FromPtr(doneItems[0].EncryptedContent))
	require.Equal(t, "rs_second", doneItems[1].ID)
	require.Equal(t, "second", doneItems[1].Summary[0].Text)
	require.Equal(t, "gAAAA_SECOND_BLOB", lo.FromPtr(doneItems[1].EncryptedContent))
}

func TestInboundTransformer_TransformStream_PreservesWebSearchCallsFromChunkMetadata(t *testing.T) {
	trans := NewInboundTransformer()

	stream, err := trans.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_stream_web_search_no_annotations",
			Created: 1700000000,
			Model:   "gpt-4o-search-preview",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: lo.ToPtr("Search result without inline citations")},
				},
			}},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_stream_web_search_no_annotations",
			Created: 1700000000,
			Model:   "gpt-4o-search-preview",
			TransformerMetadata: map[string]any{
				responsesWebSearchCallsTransformerMetadataKey: []Item{{
					ID:     "ws_456",
					Type:   "web_search_call",
					Status: lo.ToPtr("completed"),
					Action: NewWebSearchAction(&WebSearchAction{
						Type:  "search",
						Query: "latest ai news",
						Sources: []WebSearchSource{{
							Type:  "url",
							URL:   "https://example.com/source",
							Title: "Example Source",
						}},
					}),
				}},
			},
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: lo.ToPtr("stop"),
			}},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_stream_web_search_no_annotations",
			Created: 1700000000,
			Model:   "gpt-4o-search-preview",
			Usage:   &llm.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		},
	}))
	require.NoError(t, err)

	var actualEvents []StreamEvent
	for stream.Next() {
		event := stream.Current()
		var ev StreamEvent
		err := json.Unmarshal(event.Data, &ev)
		require.NoError(t, err)
		actualEvents = append(actualEvents, ev)
	}
	require.NoError(t, stream.Err())
	require.NotEmpty(t, actualEvents)

	lastEvent := actualEvents[len(actualEvents)-1]
	require.Equal(t, StreamEventTypeResponseCompleted, lastEvent.Type)
	require.NotNil(t, lastEvent.Response)
	require.Len(t, lastEvent.Response.Output, 2)
	require.Equal(t, "web_search_call", lastEvent.Response.Output[0].Type)
	require.Equal(t, "ws_456", lastEvent.Response.Output[0].ID)
	require.NotNil(t, lastEvent.Response.Output[0].Action)
	require.NotNil(t, lastEvent.Response.Output[0].Action.WebSearch)
	require.Equal(t, "latest ai news", lastEvent.Response.Output[0].Action.WebSearch.Query)
	require.Equal(t, "message", lastEvent.Response.Output[1].Type)
	require.NotNil(t, lastEvent.Response.Output[1].Content)
	require.Len(t, lastEvent.Response.Output[1].Content.Items, 1)
	require.Equal(t, "Search result without inline citations", lo.FromPtr(lastEvent.Response.Output[1].Content.Items[0].Text))
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
