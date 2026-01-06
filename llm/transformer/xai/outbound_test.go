package xai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func TestOutboundTransformer_TransformStream_FilterEmptyEvents(t *testing.T) {
	tests := []struct {
		name           string
		inputEvents    []*llm.Response
		expectedEvents []*llm.Response
		description    string
	}{
		{
			name: "should filter out completely empty event",
			inputEvents: []*llm.Response{
				{
					ID:      "test-1",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{}, // Empty delta
						},
					},
				},
			},
			expectedEvents: []*llm.Response{},
			description:    "Empty delta should be filtered out",
		},
		{
			name: "should keep event with text content",
			inputEvents: []*llm.Response{
				{
					ID:      "test-2",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Content: llm.MessageContent{
									Content: stringPtr("Hello"),
								},
							},
						},
					},
				},
			},
			expectedEvents: []*llm.Response{
				{
					ID:      "test-2",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Content: llm.MessageContent{
									Content: stringPtr("Hello"),
								},
							},
						},
					},
				},
			},
			description: "Event with text content should be kept",
		},
		{
			name: "should keep event with role",
			inputEvents: []*llm.Response{
				{
					ID:      "test-3",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Role: "assistant",
							},
						},
					},
				},
			},
			expectedEvents: []*llm.Response{
				{
					ID:      "test-3",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Role: "assistant",
							},
						},
					},
				},
			},
			description: "Event with role should be kept",
		},
		{
			name: "should keep event with finish reason",
			inputEvents: []*llm.Response{
				{
					ID:      "test-4",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index:        0,
							Delta:        &llm.Message{},
							FinishReason: stringPtr("stop"),
						},
					},
				},
			},
			expectedEvents: []*llm.Response{
				{
					ID:      "test-4",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index:        0,
							Delta:        &llm.Message{},
							FinishReason: stringPtr("stop"),
						},
					},
				},
			},
			description: "Event with finish reason should be kept",
		},
		{
			name: "should keep event with tool calls",
			inputEvents: []*llm.Response{
				{
					ID:      "test-5",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								ToolCalls: []llm.ToolCall{
									{
										ID:   "call_123",
										Type: "function",
										Function: llm.FunctionCall{
											Name:      "test_function",
											Arguments: "{}",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedEvents: []*llm.Response{
				{
					ID:      "test-5",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								ToolCalls: []llm.ToolCall{
									{
										ID:   "call_123",
										Type: "function",
										Function: llm.FunctionCall{
											Name:      "test_function",
											Arguments: "{}",
										},
									},
								},
							},
						},
					},
				},
			},
			description: "Event with tool calls should be kept",
		},
		{
			name: "should keep event with refusal",
			inputEvents: []*llm.Response{
				{
					ID:      "test-6",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Refusal: "I cannot help with that",
							},
						},
					},
				},
			},
			expectedEvents: []*llm.Response{
				{
					ID:      "test-6",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Refusal: "I cannot help with that",
							},
						},
					},
				},
			},
			description: "Event with refusal should be kept",
		},
		{
			name: "should keep event with reasoning content",
			inputEvents: []*llm.Response{
				{
					ID:      "test-7",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								ReasoningContent: stringPtr("Let me think about this..."),
							},
						},
					},
				},
			},
			expectedEvents: []*llm.Response{
				{
					ID:      "test-7",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								ReasoningContent: stringPtr("Let me think about this..."),
							},
						},
					},
				},
			},
			description: "Event with reasoning content should be kept",
		},
		{
			name: "should keep event with multiple content parts",
			inputEvents: []*llm.Response{
				{
					ID:      "test-8",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Content: llm.MessageContent{
									MultipleContent: []llm.MessageContentPart{
										{
											Type: "text",
											Text: stringPtr("Hello"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedEvents: []*llm.Response{
				{
					ID:      "test-8",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Content: llm.MessageContent{
									MultipleContent: []llm.MessageContentPart{
										{
											Type: "text",
											Text: stringPtr("Hello"),
										},
									},
								},
							},
						},
					},
				},
			},
			description: "Event with multiple content parts should be kept",
		},
		{
			name: "should always keep done response",
			inputEvents: []*llm.Response{
				llm.DoneResponse,
			},
			expectedEvents: []*llm.Response{
				llm.DoneResponse,
			},
			description: "Done response should always be kept",
		},
		{
			name: "should filter out event with no choices",
			inputEvents: []*llm.Response{
				{
					ID:      "test-9",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{}, // No choices
				},
			},
			expectedEvents: []*llm.Response{},
			description:    "Event with no choices should be filtered out",
		},
		{
			name: "should filter out event with nil delta",
			inputEvents: []*llm.Response{
				{
					ID:      "test-10",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: nil, // Nil delta
						},
					},
				},
			},
			expectedEvents: []*llm.Response{},
			description:    "Event with nil delta should be filtered out",
		},
		{
			name: "mixed events - should filter appropriately",
			inputEvents: []*llm.Response{
				// Empty event - should be filtered
				{
					ID:      "test-11a",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{},
						},
					},
				},
				// Event with content - should be kept
				{
					ID:      "test-11b",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Content: llm.MessageContent{
									Content: stringPtr("Hello"),
								},
							},
						},
					},
				},
				// Another empty event - should be filtered
				{
					ID:      "test-11c",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{},
						},
					},
				},
			},
			expectedEvents: []*llm.Response{
				{
					ID:      "test-11b",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "grok-code-fast-1",
					Choices: []llm.Choice{
						{
							Index: 0,
							Delta: &llm.Message{
								Content: llm.MessageContent{
									Content: stringPtr("Hello"),
								},
							},
						},
					},
				},
			},
			description: "Mixed events should be filtered appropriately",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock transformer
			config := &Config{
				BaseURL: DefaultBaseURL,
				APIKey:  "test-key",
			}
			transformer, err := NewOutboundTransformerWithConfig(config)
			require.NoError(t, err)

			xaiTransformer := transformer.(*OutboundTransformer)

			// Create a mock stream from input events
			inputStream := createMockLLMStream(tt.inputEvents)

			// Apply the filter
			filteredStream, err := xaiTransformer.applyStreamFilter(context.Background(), inputStream)
			require.NoError(t, err)

			// Collect all events from the filtered stream
			var actualEvents []*llm.Response
			for filteredStream.Next() {
				actualEvents = append(actualEvents, filteredStream.Current())
			}

			require.NoError(t, filteredStream.Err())

			// Compare results
			assert.Equal(t, len(tt.expectedEvents), len(actualEvents), tt.description)

			for i, expected := range tt.expectedEvents {
				if i < len(actualEvents) {
					assert.Equal(t, expected, actualEvents[i], "Event %d: %s", i, tt.description)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformStream_RealXAIEmptyEvent(t *testing.T) {
	// Test the specific empty event format from XAI that was causing issues
	emptyEventJSON := `{
		"id": "c3f2e709-9d83-5dba-aff3-e0a2e5dcefdf_us-east-1",
		"object": "chat.completion.chunk",
		"created": 1758555599,
		"model": "grok-code-fast-1",
		"choices": [
			{
				"index": 0,
				"delta": {}
			}
		],
		"system_fingerprint": "fp_10f00c862d"
	}`

	var emptyEvent llm.Response

	err := json.Unmarshal([]byte(emptyEventJSON), &emptyEvent)
	require.NoError(t, err)

	// Create transformer
	config := &Config{
		BaseURL: DefaultBaseURL,
		APIKey:  "test-key",
	}
	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	xaiTransformer := transformer.(*OutboundTransformer)

	// Create stream with the empty event
	inputStream := createMockLLMStream([]*llm.Response{&emptyEvent})

	// Apply filter
	filteredStream, err := xaiTransformer.applyStreamFilter(context.Background(), inputStream)
	require.NoError(t, err)

	// Collect events
	var actualEvents []*llm.Response
	for filteredStream.Next() {
		actualEvents = append(actualEvents, filteredStream.Current())
	}

	require.NoError(t, filteredStream.Err())

	// The empty event should be filtered out
	assert.Equal(t, 0, len(actualEvents), "XAI empty event should be filtered out")
}

// Helper function to create a mock LLM stream.
func createMockLLMStream(events []*llm.Response) streams.Stream[*llm.Response] {
	return streams.SliceStream(events)
}

// Helper method to apply the stream filter (extracted from TransformStream for testing).
func (t *OutboundTransformer) applyStreamFilter(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return streams.Filter(stream, func(event *llm.Response) bool {
		// Always allow the done response
		if event.Object == llm.DoneResponse.Object {
			return true
		}

		// Filter out events with no choices
		if len(event.Choices) == 0 {
			return false
		}

		choice := event.Choices[0]

		// Filter out events with no delta
		if choice.Delta == nil {
			return false
		}

		delta := choice.Delta

		// Check if delta has meaningful content
		hasContent := false

		// Check for text content
		if delta.Content.Content != nil && *delta.Content.Content != "" {
			hasContent = true
		}

		// Check for multiple content parts
		if len(delta.Content.MultipleContent) > 0 {
			hasContent = true
		}

		// Check for tool calls
		if len(delta.ToolCalls) > 0 {
			hasContent = true
		}

		// Check for role (important for the first message)
		if delta.Role != "" {
			hasContent = true
		}

		// Check for finish reason
		if choice.FinishReason != nil {
			hasContent = true
		}

		// Check for refusal
		if delta.Refusal != "" {
			hasContent = true
		}

		// Check for reasoning content (for models that support it)
		if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
			hasContent = true
		}

		return hasContent
	}), nil
}

// Helper function to create string pointers.
func stringPtr(s string) *string {
	return &s
}
