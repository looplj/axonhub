package aisdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestDataStreamTransformer_StreamTransformation_WithTestData(t *testing.T) {
	transformer := NewDataStreamTransformer()

	tests := []struct {
		name               string
		inputStreamFile    string
		expectedStreamFile string
		expectedAggregated func(t *testing.T, result *UIMessage)
	}{
		{
			name:               "stream transformation with stop finish reason",
			inputStreamFile:    "llm-stop.stream.json",
			expectedStreamFile: "aisdk-strop.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *UIMessage) {
				t.Helper()
				// Verify aggregated response
				require.Equal(t, "gen-1754577344-bfGaoVZhBY3iT78Psu02", result.ID)
				require.Equal(t, "assistant", result.Role)
				require.NotEmpty(t, result.Parts)

				// Verify the complete content by aggregating all text deltas
				expectedContent := "Sure! Here’s the output from 1 to 20, with 5 numbers on each line:\n\n```\n1 2 3 4 5\n6 7 8 9 10\n11 12 13 14 15\n16 17 18 19 20\n```"

				// Find text parts and aggregate content
				var aggregatedText string
				for _, part := range result.Parts {
					if part.Type == "text" {
						aggregatedText += part.Text
					}
				}
				require.Equal(t, expectedContent, aggregatedText)
			},
		},
		{
			name:               "stream transformation with parallel multiple tool calls",
			inputStreamFile:    "llm-parallel_multiple_tool.stream.jsonl",
			expectedStreamFile: "",
			expectedAggregated: func(t *testing.T, result *UIMessage) {
				t.Helper()
				// Verify aggregated response basic fields
				require.Equal(t, "chatcmpl-C2WBYGbjjGZj4CJNJI1FSlzO8U4vj", result.ID)
				require.Equal(t, "assistant", result.Role)
				// No text or reasoning parts expected in this case
				require.Len(t, result.Parts, 0)
			},
		},
		{
			name:               "stream transformation with thinking content and parallel tool calls",
			inputStreamFile:    "llm-think.stream.jsonl",
			expectedStreamFile: "",
			expectedAggregated: func(t *testing.T, result *UIMessage) {
				t.Helper()
				// Verify aggregated response
				require.Equal(t, "msg_bdrk_01DDaPSX8bJqM5dRkdv32TkC", result.ID)
				require.Equal(t, "assistant", result.Role)
				require.NotEmpty(t, result.Parts)

				// Expect first part is reasoning and the second is text
				// Aggregate reasoning and text content
				var reasoningText, textText string
				for _, p := range result.Parts {
					switch p.Type {
					case "reasoning":
						reasoningText += p.Text
					case "text":
						textText += p.Text
					}
				}

				expectedThinking := "The user is asking for the weather in San Francisco, CA. To get the weather, I need to:\n\n1. First get the coordinates (latitude and longitude) of San Francisco, CA using the get_coordinates function\n2. Then get the temperature unit for the US using get_temperature_unit function \n3. Finally use the get_weather function with the coordinates and appropriate unit\n\nLet me start with getting the coordinates and temperature unit."
				require.Equal(t, expectedThinking, reasoningText)

				expectedText := "I'll help you get the weather for San Francisco, CA. Let me first get the coordinates and determine the appropriate temperature unit for the US."
				require.Equal(t, expectedText, textText)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The input file contains LLM format responses (OpenAI-like)
			llmResponses, err := xtest.LoadResponses(t, tt.inputStreamFile)
			require.NoError(t, err)

			var expectedEvents []*httpclient.StreamEvent

			if tt.expectedStreamFile != "" {
				// The expected file contains expected AI SDK data stream events
				var loadErr error

				expectedEvents, loadErr = xtest.LoadStreamChunks(t, tt.expectedStreamFile)
				require.NoError(t, loadErr)
			}

			// Create a mock stream from LLM responses
			mockStream := streams.SliceStream(llmResponses)

			// Transform the stream (LLM -> AI SDK Data Stream)
			transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
			require.NoError(t, err)

			// Collect all transformed events
			var actualEvents []*httpclient.StreamEvent

			for transformedStream.Next() {
				event := transformedStream.Current()
				actualEvents = append(actualEvents, event)
			}

			require.NoError(t, transformedStream.Err())

			if tt.expectedStreamFile != "" {
				// Verify the number of events matches
				require.Equal(t, len(expectedEvents), len(actualEvents), "Number of events should match")

				// Verify each event
				for i, expected := range expectedEvents {
					actual := actualEvents[i]

					// Verify event type
					require.Equal(t, expected.Type, actual.Type, "Event %d: Type should match", i)

					// Parse and compare event data
					// Skip terminator events which are not JSON
					if string(expected.Data) == "[DONE]" || string(actual.Data) == "[DONE]" {
						continue
					}

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
						"Event %d: Stream event type should match",
						i,
					)

					// Verify specific fields based on event type
					switch expectedStreamEvent.Type {
					case "start":
						require.Equal(t, expectedStreamEvent.MessageID, actualStreamEvent.MessageID, "Event %d: Message ID should match", i)

					case "start-step":
						// No specific fields to verify for start-step

					case "text-start":
						// Text IDs are dynamically generated, so we just verify they exist and have the right prefix
						require.NotEmpty(t, actualStreamEvent.ID, "Event %d: Text ID should not be empty", i)
						require.Contains(t, actualStreamEvent.ID, "text_", "Event %d: Text ID should have text_ prefix", i)

					case "text-delta":
						// For text-delta, we verify the delta content matches and ID is consistent within the same text block
						require.NotEmpty(t, actualStreamEvent.ID, "Event %d: Text ID should not be empty", i)
						require.Contains(t, actualStreamEvent.ID, "text_", "Event %d: Text ID should have text_ prefix", i)
						require.Equal(t, expectedStreamEvent.Delta, actualStreamEvent.Delta, "Event %d: Delta text should match", i)

					case "text-end":
						// For text-end, we verify the ID is consistent with the text block
						require.NotEmpty(t, actualStreamEvent.ID, "Event %d: Text ID should not be empty", i)
						require.Contains(t, actualStreamEvent.ID, "text_", "Event %d: Text ID should have text_ prefix", i)

					case "tool-input-start":
						require.Equal(t, expectedStreamEvent.ToolCallID, actualStreamEvent.ToolCallID, "Event %d: Tool call ID should match", i)
						require.Equal(t, expectedStreamEvent.ToolName, actualStreamEvent.ToolName, "Event %d: Tool name should match", i)

					case "tool-input-delta":
						require.Equal(t, expectedStreamEvent.ToolCallID, actualStreamEvent.ToolCallID, "Event %d: Tool call ID should match", i)
						require.Equal(t, expectedStreamEvent.InputTextDelta, actualStreamEvent.InputTextDelta, "Event %d: Input text delta should match", i)

					case "tool-input-available":
						require.Equal(t, expectedStreamEvent.ToolCallID, actualStreamEvent.ToolCallID, "Event %d: Tool call ID should match", i)
						require.Equal(t, expectedStreamEvent.ToolName, actualStreamEvent.ToolName, "Event %d: Tool name should match", i)
						// Compare input as JSON
						require.JSONEq(t, string(expectedStreamEvent.Input), string(actualStreamEvent.Input), "Event %d: Tool input should match", i)

					case "reasoning-start":
						// Reasoning IDs are dynamically generated, so we just verify they exist and have the right prefix
						require.NotEmpty(t, actualStreamEvent.ID, "Event %d: Reasoning ID should not be empty", i)
						require.Contains(t, actualStreamEvent.ID, "reasoning_", "Event %d: Reasoning ID should have reasoning_ prefix", i)

					case "reasoning-delta":
						// For reasoning-delta, we verify the delta content matches and ID is consistent within the same reasoning block
						require.NotEmpty(t, actualStreamEvent.ID, "Event %d: Reasoning ID should not be empty", i)
						require.Contains(t, actualStreamEvent.ID, "reasoning_", "Event %d: Reasoning ID should have reasoning_ prefix", i)
						require.Equal(t, expectedStreamEvent.Delta, actualStreamEvent.Delta, "Event %d: Reasoning delta should match", i)

					case "reasoning-end":
						// For reasoning-end, we verify the ID is consistent with the reasoning block
						require.NotEmpty(t, actualStreamEvent.ID, "Event %d: Reasoning ID should not be empty", i)
						require.Contains(t, actualStreamEvent.ID, "reasoning_", "Event %d: Reasoning ID should have reasoning_ prefix", i)

					case "finish-step":
						// No specific fields to verify for finish-step

					case "finish":
						// No specific fields to verify for finish

					default:
						t.Logf("Unknown event type: %s", expectedStreamEvent.Type)
					}
				}
			}

			// Test aggregation
			aggregatedBytes, _, err := transformer.AggregateStreamChunks(t.Context(), actualEvents)
			require.NoError(t, err)

			var aggregatedResp UIMessage

			err = json.Unmarshal(aggregatedBytes, &aggregatedResp)
			require.NoError(t, err)

			// Run custom validation if provided
			if tt.expectedAggregated != nil {
				tt.expectedAggregated(t, &aggregatedResp)
			}
		})
	}
}
