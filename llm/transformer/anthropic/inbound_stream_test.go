package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm/streams"
)

func TestInboundStream_EmitsCitationsDeltaBeforeContentBlockStop(t *testing.T) {
	transformer := NewInboundTransformer()

	text := "Annotated answer"
	stop := "stop"

	input := []*llm.Response{
		{
			ID:     "msg_annotations_stream",
			Object: "chat.completion.chunk",
			Model:  "claude-sonnet-4-6",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &text,
					},
					Annotations: []llm.Annotation{
						{
							Type: "url_citation",
							URLCitation: &llm.URLCitation{
								URL:   "https://example.com/1",
								Title: "Source One",
							},
						},
						{
							Type: "url_citation",
							URLCitation: &llm.URLCitation{
								URL:   "https://example.com/2",
								Title: "Source Two",
							},
						},
					},
				},
			}},
		},
		{
			ID:     "msg_annotations_stream",
			Object: "chat.completion.chunk",
			Model:  "claude-sonnet-4-6",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: &stop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectInboundStreamEvents(t, transformer, input)

	assertCitationsDeltaBeforeContentBlockStop(t, events, []TextCitation{
		{
			Type:  "url_citation",
			URL:   "https://example.com/1",
			Title: "Source One",
		},
		{
			Type:  "url_citation",
			URL:   "https://example.com/2",
			Title: "Source Two",
		},
	})
}

func TestInboundStream_EmitsCitationsDeltaWhenAnnotationsArriveBeforeText(t *testing.T) {
	transformer := NewInboundTransformer()

	text := "Annotated after metadata"
	stop := "stop"

	input := []*llm.Response{
		{
			ID:     "msg_annotations_before_text",
			Object: "chat.completion.chunk",
			Model:  "claude-sonnet-4-6",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Annotations: []llm.Annotation{
						{
							Type: "url_citation",
							URLCitation: &llm.URLCitation{
								URL:   "https://example.com/metadata-first",
								Title: "Metadata First",
							},
						},
					},
				},
			}},
		},
		{
			ID:     "msg_annotations_before_text",
			Object: "chat.completion.chunk",
			Model:  "claude-sonnet-4-6",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &text,
					},
				},
			}},
		},
		{
			ID:     "msg_annotations_before_text",
			Object: "chat.completion.chunk",
			Model:  "claude-sonnet-4-6",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: &stop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectInboundStreamEvents(t, transformer, input)

	assertCitationsDeltaBeforeContentBlockStop(t, events, []TextCitation{{
		Type:  "url_citation",
		URL:   "https://example.com/metadata-first",
		Title: "Metadata First",
	}})
}

func TestInboundStream_EmitsCitationsDeltaFromChoiceMessageAnnotations(t *testing.T) {
	transformer := NewInboundTransformer()

	text := "Annotated via choice.message"
	stop := "stop"

	input := []*llm.Response{
		{
			ID:     "msg_choice_message_annotations",
			Object: "chat.completion.chunk",
			Model:  "claude-sonnet-4-6",
			Choices: []llm.Choice{{
				Index: 0,
				Message: &llm.Message{
					Annotations: []llm.Annotation{
						{
							Type: "url_citation",
							URLCitation: &llm.URLCitation{
								URL:   "https://example.com/message-annotations",
								Title: "Message Annotation",
							},
						},
					},
				},
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &text,
					},
				},
			}},
		},
		{
			ID:     "msg_choice_message_annotations",
			Object: "chat.completion.chunk",
			Model:  "claude-sonnet-4-6",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: &stop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectInboundStreamEvents(t, transformer, input)

	assertCitationsDeltaBeforeContentBlockStop(t, events, []TextCitation{{
		Type:  "url_citation",
		URL:   "https://example.com/message-annotations",
		Title: "Message Annotation",
	}})
}

func collectInboundStreamEvents(t *testing.T, transformer *InboundTransformer, input []*llm.Response) []StreamEvent {
	t.Helper()

	stream, err := transformer.TransformStream(t.Context(), streams.SliceStream(input))
	require.NoError(t, err)

	var events []StreamEvent
	for stream.Next() {
		raw := stream.Current()
		var event StreamEvent
		err := json.Unmarshal(raw.Data, &event)
		require.NoError(t, err)
		events = append(events, event)
	}
	require.NoError(t, stream.Err())

	return events
}

func assertCitationsDeltaBeforeContentBlockStop(t *testing.T, events []StreamEvent, expected []TextCitation) {
	t.Helper()

	var (
		citationEventIndexes []int
		contentBlockStopIndex = -1
		actualCitations       []TextCitation
	)

	for i, event := range events {
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type != nil && *event.Delta.Type == "citations_delta" {
			citationEventIndexes = append(citationEventIndexes, i)
			require.NotNil(t, event.Delta.Citation)
			actualCitations = append(actualCitations, *event.Delta.Citation)
			require.Nil(t, event.Delta.Citation.EncryptedIndex)
			require.Nil(t, event.Delta.Citation.CitedText)
		}
		if event.Type == "content_block_stop" && event.Index != nil && *event.Index == 0 && contentBlockStopIndex == -1 {
			contentBlockStopIndex = i
		}
	}

	require.Len(t, citationEventIndexes, len(expected))
	require.NotEqual(t, -1, contentBlockStopIndex)
	for _, idx := range citationEventIndexes {
		require.Less(t, idx, contentBlockStopIndex)
	}
	require.Equal(t, expected, actualCitations)
}

func TestInboundStream_NormalizesOpenAIWebSearchCitationTypeFromChunkMetadata(t *testing.T) {
	transformer := NewInboundTransformer()

	text := "Annotated answer"
	stop := "stop"

	input := []*llm.Response{
		{
			ID:     "msg_annotations_stream_web_search",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o-search-preview",
			TransformerMetadata: map[string]any{
				"openai_responses_web_search_calls": []map[string]any{{
					"id":     "ws_123",
					"type":   "web_search_call",
					"status": "completed",
					"action": map[string]any{
						"type":  "search",
						"query": "latest ai news",
					},
				}},
			},
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: &text,
					},
					Annotations: []llm.Annotation{{
						Type: "url_citation",
						URLCitation: &llm.URLCitation{
							URL:   "https://example.com/result",
							Title: "Example Result",
						},
					}},
				},
			}},
		},
		{
			ID:     "msg_annotations_stream_web_search",
			Object: "chat.completion.chunk",
			Model:  "gpt-4o-search-preview",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: &stop,
			}},
			Usage: &llm.Usage{},
		},
	}

	events := collectInboundStreamEvents(t, transformer, input)

	assertCitationsDeltaBeforeContentBlockStop(t, events, []TextCitation{{
		Type:  "web_search_result_location",
		URL:   "https://example.com/result",
		Title: "Example Result",
	}})
}

func TestInboundTransformer_StreamTransformation_WithTestData(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name                string
		inputStreamFile     string
		expectedInputTokens int64
		expectedStreamFile  string
		expectedAggregated  func(t *testing.T, result *Message)
	}{
		{
			name:                "stream transformation with stop finish reason",
			inputStreamFile:     "llm-stop.stream.jsonl",
			expectedStreamFile:  "anthropic-stop.stream.jsonl",
			expectedInputTokens: 21,
			expectedAggregated: func(t *testing.T, result *Message) {
				t.Helper()
				// Verify aggregated response
				require.Equal(t, "msg_bdrk_01Fbg5HKuVfmtT6mAMxQoCSn", result.ID)
				require.Equal(t, "message", result.Type)
				require.Equal(t, "claude-3-7-sonnet-20250219", result.Model)
				require.NotEmpty(t, result.Content)
				require.Equal(t, "assistant", result.Role)

				// Verify the complete content
				expectedContent := "1 2 3 4 5\n6 7 8 9 10\n11 12 13 14 15\n16 17 18 19 20"
				require.Equal(t, expectedContent, *result.Content[0].Text)
			},
		},
		{
			name:                "stream transformation with parallel multiple tool calls",
			inputStreamFile:     "llm-parallel_multiple_tool.stream.jsonl",
			expectedStreamFile:  "anthropic-parallel_multiple_tool.stream.jsonl",
			expectedInputTokens: 104,
			expectedAggregated: func(t *testing.T, result *Message) {
				t.Helper()
				// Verify aggregated response
				require.Equal(t, "chatcmpl-C2WBYGbjjGZj4CJNJI1FSlzO8U4vj", result.ID)
				require.Equal(t, "message", result.Type)
				require.Equal(t, "gpt-4o-2024-11-20", result.Model)
				require.NotEmpty(t, result.Content)
				require.Equal(t, "assistant", result.Role)
				require.Equal(t, "tool_use", *result.StopReason)

				// Verify we have 2 tool use content blocks
				require.Len(t, result.Content, 2)

				// Verify first tool call (get_user_city)
				require.Equal(t, "tool_use", result.Content[0].Type)
				require.Equal(t, "call_tooG2dAMZaICWBfsYU5LYyvs", result.Content[0].ID)
				require.Equal(t, "get_user_city", *result.Content[0].Name)

				var cityInput map[string]any

				err := json.Unmarshal(result.Content[0].Input, &cityInput)
				require.NoError(t, err)
				require.Equal(t, "123", cityInput["user_id"])

				// Verify second tool call (get_user_language)
				require.Equal(t, "tool_use", result.Content[1].Type)
				require.Equal(t, "call_Ul0yUvKCpLfl5c32FHPcASEB", result.Content[1].ID)
				require.Equal(t, "get_user_language", *result.Content[1].Name)

				var langInput map[string]any

				err = json.Unmarshal(result.Content[1].Input, &langInput)
				require.NoError(t, err)
				require.Equal(t, "123", langInput["user_id"])
			},
		},
		{
			name:                "stream transformation with thinking content and parallel tool calls",
			inputStreamFile:     "llm-think.stream.jsonl",
			expectedStreamFile:  "anthropic-think.stream.jsonl",
			expectedInputTokens: 587,
			expectedAggregated: func(t *testing.T, result *Message) {
				t.Helper()
				// Verify aggregated response
				require.Equal(t, "msg_bdrk_01DDaPSX8bJqM5dRkdv32TkC", result.ID)
				require.Equal(t, "message", result.Type)
				require.Equal(t, "claude-sonnet-4-20250514", result.Model)
				require.NotEmpty(t, result.Content)
				require.Equal(t, "assistant", result.Role)
				require.Equal(t, "tool_use", *result.StopReason)

				// Verify we have 4 content blocks: thinking, text, and 2 tool uses
				require.Len(t, result.Content, 4)

				// Verify thinking content block
				require.Equal(t, "thinking", result.Content[0].Type)

				expectedThinking := "The user is asking for the weather in San Francisco, CA. To get the weather, I need to:\n\n1. First get the coordinates (latitude and longitude) of San Francisco, CA using the get_coordinates function\n2. Then get the temperature unit for the US using get_temperature_unit function \n3. Finally use the get_weather function with the coordinates and appropriate unit\n\nLet me start with getting the coordinates and temperature unit."
				require.Equal(t, expectedThinking, *result.Content[0].Thinking)

				// Verify text content block
				require.Equal(t, "text", result.Content[1].Type)

				expectedText := "I'll help you get the weather for San Francisco, CA. Let me first get the coordinates and determine the appropriate temperature unit for the US."
				require.Equal(t, expectedText, *result.Content[1].Text)

				// Verify first tool call (get_coordinates)
				require.Equal(t, "tool_use", result.Content[2].Type)
				require.Equal(t, "toolu_bdrk_01RjxXDSvxn69XRfWLjn6Sur", result.Content[2].ID)
				require.Equal(t, "get_coordinates", *result.Content[2].Name)

				var coordInput map[string]any

				err := json.Unmarshal(result.Content[2].Input, &coordInput)
				require.NoError(t, err)
				require.Equal(t, "San Francisco, CA", coordInput["location"])

				// Verify second tool call (get_temperature_unit)
				require.Equal(t, "tool_use", result.Content[3].Type)
				require.Equal(t, "toolu_bdrk_01E6Gr52e4i9TLwsDn8Sgimg", result.Content[3].ID)
				require.Equal(t, "get_temperature_unit", *result.Content[3].Name)

				var unitInput map[string]any

				err = json.Unmarshal(result.Content[3].Input, &unitInput)
				require.NoError(t, err)
				require.Equal(t, "United States", unitInput["country"])
			},
		},
		{
			name:                "stream transformation with OpenRouter content and tool call",
			inputStreamFile:     "or-tool.stream.jsonl",
			expectedStreamFile:  "anthropic-or-tool.stream.jsonl",
			expectedInputTokens: 18810,
			expectedAggregated: func(t *testing.T, result *Message) {
				t.Helper()
				// Verify aggregated response
				require.Equal(t, "gen-1761365834-YlzLUYrcuUQ4OsjtP1qS", result.ID)
				require.Equal(t, "message", result.Type)
				require.Equal(t, "minimax/minimax-m2:free", result.Model)
				require.NotEmpty(t, result.Content)
				require.Equal(t, "assistant", result.Role)
				require.Equal(t, "tool_use", *result.StopReason)

				// Verify we have 2 content blocks: text, and 1 tool use
				require.Len(t, result.Content, 2)

				// Verify text content block
				require.Equal(t, "text", result.Content[0].Type)
				require.Contains(t, *result.Content[0].Text, "代码Review结果")

				// Verify tool call (TodoWrite)
				require.Equal(t, "tool_use", result.Content[1].Type)
				require.Equal(t, "call_function_6091710012_1", result.Content[1].ID)
				require.Equal(t, "TodoWrite", *result.Content[1].Name)

				var toolInput map[string]any

				err := json.Unmarshal(result.Content[1].Input, &toolInput)
				require.NoError(t, err)
				require.NotNil(t, toolInput["todos"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The input file contains OpenAI format responses
			openaiResponses, err := xtest.LoadLlmResponses(t, tt.inputStreamFile)
			require.NoError(t, err)

			// The expected file contains expected Anthropic format events
			expectedEvents, err := xtest.LoadStreamChunks(t, tt.expectedStreamFile)
			require.NoError(t, err)

			// Create a mock stream from OpenAI responses
			mockStream := streams.SliceStream(openaiResponses)

			// Transform the stream (OpenAI -> Anthropic)
			transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
			require.NoError(t, err)

			// Collect all transformed events
			var actualEvents []*httpclient.StreamEvent

			for transformedStream.Next() {
				event := transformedStream.Current()
				actualEvents = append(actualEvents, event)
			}

			require.NoError(t, transformedStream.Err())

			// Verify the number of events matches
			require.Equal(t, len(expectedEvents), len(actualEvents), "Number of events should match")

			// Verify each event
			for i, expected := range expectedEvents {
				actual := actualEvents[i]

				// Verify event type
				require.Equal(t, expected.Type, actual.Type, "Event %d: Type should match", i)

				// Parse and compare event data
				var expectedStreamEvent StreamEvent

				err := json.Unmarshal(expected.Data, &expectedStreamEvent)
				require.NoError(t, err)

				var actualStreamEvent StreamEvent

				err = json.Unmarshal(actual.Data, &actualStreamEvent)
				require.NoError(t, err)

				// Verify stream event type
				require.Equal(
					t,
					expectedStreamEvent.Type,
					actualStreamEvent.Type,
					"Event %d: Stream event type should match, expected: %v, actual: %v",
					i,
					string(xjson.MustMarshal(expectedStreamEvent)),
					string(xjson.MustMarshal(actualStreamEvent)),
				)

				// Verify specific fields based on event type
				switch expectedStreamEvent.Type {
				case "message_start":
					require.NotNil(t, expectedStreamEvent.Message)
					require.NotNil(t, actualStreamEvent.Message)
					require.Equal(t, expectedStreamEvent.Message.ID, actualStreamEvent.Message.ID, "Event %d: Message ID should match", i)
					require.Equal(t, expectedStreamEvent.Message.Model, actualStreamEvent.Message.Model, "Event %d: Model should match", i)
					require.Equal(t, expectedStreamEvent.Message.Role, actualStreamEvent.Message.Role, "Event %d: Role should match", i)

					if expectedStreamEvent.Message.Usage != nil && actualStreamEvent.Message.Usage != nil {
						require.Equal(
							t,
							expectedStreamEvent.Message.Usage.InputTokens,
							actualStreamEvent.Message.Usage.InputTokens,
							"Event %d: Input tokens should match",
							i,
						)
						require.Equal(
							t,
							expectedStreamEvent.Message.Usage.OutputTokens,
							actualStreamEvent.Message.Usage.OutputTokens,
							"Event %d: Output tokens should match",
							i,
						)
					}

				case "content_block_start":
					require.NotNil(t, expectedStreamEvent.ContentBlock)
					require.NotNil(t, actualStreamEvent.ContentBlock)
					require.Equal(t, expectedStreamEvent.ContentBlock.Type, actualStreamEvent.ContentBlock.Type, "Event %d: Content block type should match", i)

					// Additional validation for tool_use content blocks
					if expectedStreamEvent.ContentBlock.Type == "tool_use" {
						require.Equal(t, expectedStreamEvent.ContentBlock.ID, actualStreamEvent.ContentBlock.ID, "Event %d: Tool use ID should match", i)

						if expectedStreamEvent.ContentBlock.Name != nil && actualStreamEvent.ContentBlock.Name != nil {
							require.Equal(
								t,
								*expectedStreamEvent.ContentBlock.Name,
								*actualStreamEvent.ContentBlock.Name,
								"Event %d: Tool use name should match",
								i,
							)
						}
					}

				case "content_block_delta":
					require.NotNil(t, expectedStreamEvent.Delta)
					require.NotNil(t, actualStreamEvent.Delta)

					if !xtest.Equal(expectedStreamEvent.Delta, actualStreamEvent.Delta, cmpopts.IgnoreFields(StreamDelta{}, "Signature")) {
						t.Errorf("Index: %d, Diff: %s ", i, cmp.Diff(expectedStreamEvent.Delta, actualStreamEvent.Delta))
					}

				case "content_block_stop":
					require.Equal(
						t,
						expectedStreamEvent.Index,
						actualStreamEvent.Index,
						"Event %d: Index should match, expected: %v, actual: %v",
						i,
						*expectedStreamEvent.Index,
						*actualStreamEvent.Index,
					)

				case "message_delta":
					require.NotNil(t, expectedStreamEvent.Delta)
					require.NotNil(t, actualStreamEvent.Delta)

					require.Equal(t, expectedStreamEvent.Delta.StopReason, actualStreamEvent.Delta.StopReason)

					if !xtest.Equal(expectedStreamEvent.Delta, actualStreamEvent.Delta, cmpopts.IgnoreFields(StreamDelta{}, "Signature")) {
						t.Errorf("Index: %d, Diff: %s ", i, cmp.Diff(expectedStreamEvent.Delta, actualStreamEvent.Delta))
					}

					if expectedStreamEvent.Usage != nil && actualStreamEvent.Usage != nil {
						// Aggregate input tokens from the message_start event.
						require.Equal(t, tt.expectedInputTokens, actualStreamEvent.Usage.InputTokens, "Event %d: Usage input tokens should match", i)
						require.Equal(
							t,
							expectedStreamEvent.Usage.OutputTokens,
							actualStreamEvent.Usage.OutputTokens,
							"Event %d: Usage output tokens should match",
							i,
						)
					}

				case "message_stop":
					// No specific fields to verify for message_stop
				}
			}

			// Test aggregation
			aggregatedBytes, _, err := transformer.AggregateStreamChunks(t.Context(), actualEvents)
			require.NoError(t, err)

			var aggregatedResp Message

			err = json.Unmarshal(aggregatedBytes, &aggregatedResp)
			require.NoError(t, err)

			// Run custom validation if provided
			if tt.expectedAggregated != nil {
				tt.expectedAggregated(t, &aggregatedResp)
			}
		})
	}
}
