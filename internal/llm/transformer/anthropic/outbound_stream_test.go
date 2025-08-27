package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestOutboundTransformer_StreamTransformation_WithTestData_Stop(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://example.com", "xxx")

	// Load test data from files
	streamEvents, err := xtest.LoadStreamChunks(t, "anthropic-stop.stream.jsonl")
	require.NoError(t, err)

	expectedResponses, err := xtest.LoadResponses(t, "llm-stop.stream.jsonl")
	require.NoError(t, err)

	// Create a mock stream
	mockStream := streams.SliceStream(streamEvents)

	// Transform the stream
	transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
	require.NoError(t, err)

	// Collect all transformed responses
	var actualResponses []*llm.Response

	for transformedStream.Next() {
		resp := transformedStream.Current()
		if resp != nil {
			actualResponses = append(actualResponses, resp)
		}
	}

	require.NoError(t, transformedStream.Err())

	// Verify the number of responses matches
	require.Equal(t, len(expectedResponses), len(actualResponses), "Number of responses should match")

	// Verify each response
	for i, expected := range expectedResponses {
		actual := actualResponses[i]

		// Verify basic fields
		require.Equal(t, expected.ID, actual.ID, "Response %d: ID should match", i)
		require.Equal(t, expected.Object, actual.Object, "Response %d: Object should match", i)
		require.Equal(t, expected.Model, actual.Model, "Response %d: Model should match", i)
		require.Equal(t, expected.Created, actual.Created, "Response %d: Created should match", i)

		// Verify choices
		require.Equal(t, len(expected.Choices), len(actual.Choices), "Response %d: Number of choices should match", i)

		if len(expected.Choices) > 0 && len(actual.Choices) > 0 {
			expectedChoice := expected.Choices[0]
			actualChoice := actual.Choices[0]

			require.Equal(t, expectedChoice.Index, actualChoice.Index, "Response %d: Choice index should match", i)
			require.Equal(t, expectedChoice.FinishReason, actualChoice.FinishReason, "Response %d: Finish reason should match", i)

			// Verify delta content
			if expectedChoice.Delta != nil && actualChoice.Delta != nil {
				require.Equal(t, expectedChoice.Delta.Role, actualChoice.Delta.Role, "Response %d: Delta role should match", i)

				if expectedChoice.Delta.Content.Content != nil && actualChoice.Delta.Content.Content != nil {
					require.Equal(t, *expectedChoice.Delta.Content.Content, *actualChoice.Delta.Content.Content, "Response %d: Delta content should match", i)
				}
			}
		}

		// Verify usage information
		if expected.Usage != nil && actual.Usage != nil {
			require.Equal(t, expected.Usage.PromptTokens, actual.Usage.PromptTokens, "Response %d: Prompt tokens should match", i)
			require.Equal(t, expected.Usage.CompletionTokens, actual.Usage.CompletionTokens, "Response %d: Completion tokens should match", i)
			require.Equal(t, expected.Usage.TotalTokens, actual.Usage.TotalTokens, "Response %d: Total tokens should match", i)
		}
	}

	// // Test aggregation as well
	aggregatedBytes, _, err := AggregateStreamChunks(t.Context(), streamEvents)
	require.NoError(t, err)

	var aggregatedResp Message

	err = json.Unmarshal(aggregatedBytes, &aggregatedResp)
	require.NoError(t, err)

	// Verify aggregated response
	require.Equal(t, "msg_bdrk_01Fbg5HKuVfmtT6mAMxQoCSn", aggregatedResp.ID)
	require.Equal(t, "message", aggregatedResp.Type)
	require.Equal(t, "claude-3-7-sonnet-20250219", aggregatedResp.Model)
	require.NotEmpty(t, aggregatedResp.Content)
	require.Equal(t, "assistant", aggregatedResp.Role)

	// Verify the complete content
	expectedContent := "1 2 3 4 5\n6 7 8 9 10\n11 12 13 14 15\n16 17 18 19 20"
	require.Equal(t, expectedContent, aggregatedResp.Content[0].Text)
}

func TestOutboundTransformer_StreamTransformation_WithTestData_Tool(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://example.com", "xxx")

	// Load test data from files
	streamEvents, err := xtest.LoadStreamChunks(t, "anthropic-tool.stream.jsonl")
	require.NoError(t, err)

	expectedResponses, err := xtest.LoadResponses(t, "llm-tool.stream.jsonl")
	require.NoError(t, err)

	// Create a mock stream
	mockStream := streams.SliceStream(streamEvents)

	// Transform the stream
	transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
	require.NoError(t, err)

	// Collect all transformed responses
	var actualResponses []*llm.Response

	for transformedStream.Next() {
		resp := transformedStream.Current()
		if resp != nil {
			actualResponses = append(actualResponses, resp)
		}
	}

	require.NoError(t, transformedStream.Err())

	// Debug: Print counts and first few responses
	t.Logf("Expected responses: %d, Actual responses: %d", len(expectedResponses), len(actualResponses))

	for i := 0; i < min(5, len(actualResponses)); i++ {
		respBytes, _ := json.Marshal(actualResponses[i])
		t.Logf("Actual response %d: %s", i, string(respBytes))
	}

	// Verify the number of responses matches
	require.Equal(t, len(expectedResponses), len(actualResponses), "Number of responses should match")

	// Verify each response
	for i, expected := range expectedResponses {
		actual := actualResponses[i]

		// Verify basic fields
		require.Equal(t, expected.ID, actual.ID, "Response %d: ID should match", i)
		require.Equal(t, expected.Object, actual.Object, "Response %d: Object should match", i)
		require.Equal(t, expected.Model, actual.Model, "Response %d: Model should match", i)
		require.Equal(t, expected.Created, actual.Created, "Response %d: Created should match", i)

		// Verify choices
		require.Equal(t, len(expected.Choices), len(actual.Choices), "Response %d: Number of choices should match", i)

		if len(expected.Choices) > 0 && len(actual.Choices) > 0 {
			expectedChoice := expected.Choices[0]
			actualChoice := actual.Choices[0]

			require.Equal(t, expectedChoice.Index, actualChoice.Index, "Response %d: Choice index should match", i)
			require.Equal(t, expectedChoice.FinishReason, actualChoice.FinishReason, "Response %d: Finish reason should match", i)

			// Verify delta content
			if expectedChoice.Delta != nil && actualChoice.Delta != nil {
				require.Equal(t, expectedChoice.Delta.Role, actualChoice.Delta.Role, "Response %d: Delta role should match", i)

				if expectedChoice.Delta.Content.Content != nil && actualChoice.Delta.Content.Content != nil {
					require.Equal(t, *expectedChoice.Delta.Content.Content, *actualChoice.Delta.Content.Content, "Response %d: Delta content should match", i)
				}

				// Verify tool calls
				if len(expectedChoice.Delta.ToolCalls) > 0 && len(actualChoice.Delta.ToolCalls) > 0 {
					for j, expectedToolCall := range expectedChoice.Delta.ToolCalls {
						if j < len(actualChoice.Delta.ToolCalls) {
							actualToolCall := actualChoice.Delta.ToolCalls[j]
							require.Equal(t, expectedToolCall.ID, actualToolCall.ID, "Response %d, ToolCall %d: ID should match", i, j)
							require.Equal(t, expectedToolCall.Type, actualToolCall.Type, "Response %d, ToolCall %d: Type should match", i, j)
							require.Equal(
								t,
								expectedToolCall.Function.Name,
								actualToolCall.Function.Name,
								"Response %d, ToolCall %d: Function name should match",
								i,
								j,
							)
							require.Equal(
								t,
								expectedToolCall.Function.Arguments,
								actualToolCall.Function.Arguments,
								"Response %d, ToolCall %d: Function arguments should match",
								i,
								j,
							)
						}
					}
				}
			}
		}

		// Verify usage information
		if expected.Usage != nil && actual.Usage != nil {
			require.Equal(t, expected.Usage.PromptTokens, actual.Usage.PromptTokens, "Response %d: Prompt tokens should match", i)
			require.Equal(t, expected.Usage.CompletionTokens, actual.Usage.CompletionTokens, "Response %d: Completion tokens should match", i)
			require.Equal(t, expected.Usage.TotalTokens, actual.Usage.TotalTokens, "Response %d: Total tokens should match", i)
		}
	}
}

func TestOutboundTransformer_StreamTransformation_WithTestData_Think(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://example.com", "xxx")

	// Load test data using xtest
	streamEvents, err := xtest.LoadStreamChunks(t, "anthropic-think.stream.jsonl")
	require.NoError(t, err)

	expectedResponses, err := xtest.LoadResponses(t, "llm-think.stream.jsonl")
	require.NoError(t, err)

	// Create a mock stream
	mockStream := streams.SliceStream(streamEvents)

	// Transform the stream
	transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
	require.NoError(t, err)

	// Collect all transformed responses
	var actualResponses []*llm.Response

	for transformedStream.Next() {
		resp := transformedStream.Current()
		actualResponses = append(actualResponses, resp)
	}

	require.NoError(t, transformedStream.Err())

	// Verify the number of responses matches
	require.Equal(t, len(expectedResponses), len(actualResponses), "Number of responses should match")

	// Verify each response
	for i, expected := range expectedResponses {
		actual := actualResponses[i]

		// Verify basic fields
		require.Equal(t, expected.ID, actual.ID, "Response %d: ID should match", i)
		require.Equal(t, expected.Object, actual.Object, "Response %d: Object should match", i)
		require.Equal(t, expected.Model, actual.Model, "Response %d: Model should match", i)
		require.Equal(t, expected.Created, actual.Created, "Response %d: Created should match", i)

		// Verify choices
		require.Equal(t, len(expected.Choices), len(actual.Choices), "Response %d: Number of choices should match", i)

		if len(expected.Choices) > 0 && len(actual.Choices) > 0 {
			expectedChoice := expected.Choices[0]
			actualChoice := actual.Choices[0]

			require.Equal(t, expectedChoice.Index, actualChoice.Index, "Response %d: Choice index should match", i)
			require.Equal(t, expectedChoice.FinishReason, actualChoice.FinishReason, "Response %d: Finish reason should match", i)

			// Verify delta content
			if expectedChoice.Delta != nil && actualChoice.Delta != nil {
				require.Equal(t, expectedChoice.Delta.Role, actualChoice.Delta.Role, "Response %d: Delta role should match", i)

				if expectedChoice.Delta.Content.Content != nil && actualChoice.Delta.Content.Content != nil {
					require.Equal(t, *expectedChoice.Delta.Content.Content, *actualChoice.Delta.Content.Content, "Response %d: Delta content should match", i)
				}

				if expectedChoice.Delta.ReasoningContent != nil && actualChoice.Delta.ReasoningContent != nil {
					require.Equal(
						t,
						expectedChoice.Delta.ReasoningContent,
						actualChoice.Delta.ReasoningContent,
						"Response %d: Delta reasoning content should match",
						i,
					)
				}

				if expectedChoice.Delta.ToolCalls != nil && actualChoice.Delta.ToolCalls != nil {
					require.Equal(t, expectedChoice.Delta.ToolCalls, actualChoice.Delta.ToolCalls, "Response %d: Delta tool calls should match", i)
				}
			}
		}

		// Verify usage information
		if expected.Usage != nil && actual.Usage != nil {
			require.Equal(t, expected.Usage.PromptTokens, actual.Usage.PromptTokens, "Response %d: Prompt tokens should match", i)
			require.Equal(t, expected.Usage.CompletionTokens, actual.Usage.CompletionTokens, "Response %d: Completion tokens should match", i)
			require.Equal(t, expected.Usage.TotalTokens, actual.Usage.TotalTokens, "Response %d: Total tokens should match", i)
		}
	}
}
