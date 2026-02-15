package gemini

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm/streams"
)

func TestInboundTransformer_TransformStreamChunk(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name           string
		response       *llm.Response
		validateResult func(*testing.T, *httpclient.StreamEvent)
		expectedErr    bool
	}{
		{
			name:     "nil response returns nil",
			response: nil,
			validateResult: func(t *testing.T, event *httpclient.StreamEvent) {
				require.Nil(t, event)
			},
		},
		{
			name: "[DONE] marker returns nil event",
			response: &llm.Response{
				Object: "[DONE]",
			},
			validateResult: func(t *testing.T, event *httpclient.StreamEvent) {
				require.Nil(t, event)
			},
		},
		{
			name: "simple text response (streaming with Delta)",
			response: &llm.Response{
				ID:     "chatcmpl-123",
				Model:  "gemini-2.0-flash",
				Object: "chat.completion.chunk",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello, world!"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validateResult: func(t *testing.T, event *httpclient.StreamEvent) {
				require.NotNil(t, event)
				require.NotEmpty(t, event.Data)

				var geminiResp GenerateContentResponse

				err := json.Unmarshal(event.Data, &geminiResp)
				require.NoError(t, err)
				require.Equal(t, "chatcmpl-123", geminiResp.ResponseID)
				require.Len(t, geminiResp.Candidates, 1)
				require.NotNil(t, geminiResp.Candidates[0].Content)
				require.Len(t, geminiResp.Candidates[0].Content.Parts, 1)
				require.Equal(t, "Hello, world!", geminiResp.Candidates[0].Content.Parts[0].Text)
			},
		},
		{
			name: "response with reasoning content (streaming with Delta)",
			response: &llm.Response{
				ID:     "chatcmpl-456",
				Model:  "gemini-2.0-flash-thinking",
				Object: "chat.completion.chunk",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role:             "assistant",
							ReasoningContent: lo.ToPtr("Let me think about this..."),
							Content: llm.MessageContent{
								Content: lo.ToPtr("The answer is 42."),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validateResult: func(t *testing.T, event *httpclient.StreamEvent) {
				require.NotNil(t, event)

				var geminiResp GenerateContentResponse

				err := json.Unmarshal(event.Data, &geminiResp)
				require.NoError(t, err)
				require.Len(t, geminiResp.Candidates, 1)
				require.NotNil(t, geminiResp.Candidates[0].Content)

				// Should have thinking part and text part
				parts := geminiResp.Candidates[0].Content.Parts
				require.GreaterOrEqual(t, len(parts), 1)

				// Find thinking part
				var hasThinking, hasText bool

				for _, part := range parts {
					if part.Thought && part.Text == "Let me think about this..." {
						hasThinking = true
					}

					if !part.Thought && part.Text == "The answer is 42." {
						hasText = true
					}
				}

				require.True(t, hasThinking, "should have thinking part")
				require.True(t, hasText, "should have text part")
			},
		},
		{
			name: "response with tool calls (streaming with Delta)",
			response: &llm.Response{
				ID:     "chatcmpl-789",
				Model:  "gemini-2.0-flash",
				Object: "chat.completion.chunk",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call-123",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "get_weather",
										Arguments: `{"location":"Tokyo"}`,
									},
								},
							},
						},
						FinishReason: lo.ToPtr("tool_calls"),
					},
				},
			},
			validateResult: func(t *testing.T, event *httpclient.StreamEvent) {
				require.NotNil(t, event)

				var geminiResp GenerateContentResponse

				err := json.Unmarshal(event.Data, &geminiResp)
				require.NoError(t, err)
				require.Len(t, geminiResp.Candidates, 1)
				require.NotNil(t, geminiResp.Candidates[0].Content)

				// Find function call part
				var hasFunctionCall bool

				for _, part := range geminiResp.Candidates[0].Content.Parts {
					if part.FunctionCall != nil {
						hasFunctionCall = true

						require.Equal(t, "call-123", part.FunctionCall.ID)
						require.Equal(t, "get_weather", part.FunctionCall.Name)
						require.Equal(t, "Tokyo", part.FunctionCall.Args["location"])
					}
				}

				require.True(t, hasFunctionCall, "should have function call part")
			},
		},
		{
			name: "response with usage (streaming with Delta)",
			response: &llm.Response{
				ID:     "chatcmpl-usage",
				Model:  "gemini-2.0-flash",
				Object: "chat.completion.chunk",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Test"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			validateResult: func(t *testing.T, event *httpclient.StreamEvent) {
				require.NotNil(t, event)

				var geminiResp GenerateContentResponse

				err := json.Unmarshal(event.Data, &geminiResp)
				require.NoError(t, err)
				require.NotNil(t, geminiResp.UsageMetadata)
				require.Equal(t, int64(10), geminiResp.UsageMetadata.PromptTokenCount)
				require.Equal(t, int64(5), geminiResp.UsageMetadata.CandidatesTokenCount)
				require.Equal(t, int64(15), geminiResp.UsageMetadata.TotalTokenCount)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := transformer.TransformStreamChunk(context.Background(), tt.response)

			if tt.expectedErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.validateResult != nil {
				tt.validateResult(t, event)
			}
		})
	}
}

func TestInboundTransformer_TransformStream(t *testing.T) {
	transformer := NewInboundTransformer()

	// Create test LLM responses
	responses := []*llm.Response{
		{
			ID:     "chatcmpl-stream-1",
			Model:  "gemini-2.0-flash",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
		},
		{
			ID:     "chatcmpl-stream-1",
			Model:  "gemini-2.0-flash",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						Content: llm.MessageContent{
							Content: lo.ToPtr(", world!"),
						},
					},
					FinishReason: lo.ToPtr("stop"),
				},
			},
		},
	}

	// Create a stream from the responses
	inputStream := streams.SliceStream(responses)

	// Transform the stream
	outputStream, err := transformer.TransformStream(context.Background(), inputStream)
	require.NoError(t, err)
	require.NotNil(t, outputStream)

	// Collect results
	var results []*httpclient.StreamEvent
	for outputStream.Next() {
		results = append(results, outputStream.Current())
	}

	require.NoError(t, outputStream.Err())

	// Verify results
	require.Len(t, results, 2)

	// Verify first chunk
	var resp1 GenerateContentResponse

	err = json.Unmarshal(results[0].Data, &resp1)
	require.NoError(t, err)
	require.Len(t, resp1.Candidates, 1)

	// Verify second chunk
	var resp2 GenerateContentResponse

	err = json.Unmarshal(results[1].Data, &resp2)
	require.NoError(t, err)
	require.Len(t, resp2.Candidates, 1)
}

func TestInboundTransformer_AggregateStreamChunks(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name           string
		chunks         []*httpclient.StreamEvent
		validateResult func(*testing.T, []byte, llm.ResponseMeta)
		expectedErr    bool
	}{
		{
			name:   "empty chunks returns empty response",
			chunks: nil,
			validateResult: func(t *testing.T, data []byte, meta llm.ResponseMeta) {
				var resp GenerateContentResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
			},
		},
		{
			name: "aggregate simple text chunks",
			chunks: []*httpclient.StreamEvent{
				{
					Data: mustMarshal(&GenerateContentResponse{
						ResponseID:   "resp-agg-1",
						ModelVersion: "gemini-2.0-flash",
						Candidates: []*Candidate{
							{
								Index: 0,
								Content: &Content{
									Role:  "model",
									Parts: []*Part{{Text: "Hello"}},
								},
							},
						},
					}),
				},
				{
					Data: mustMarshal(&GenerateContentResponse{
						ResponseID:   "resp-agg-1",
						ModelVersion: "gemini-2.0-flash",
						Candidates: []*Candidate{
							{
								Index: 0,
								Content: &Content{
									Role:  "model",
									Parts: []*Part{{Text: ", world!"}},
								},
								FinishReason: "STOP",
							},
						},
						UsageMetadata: &UsageMetadata{
							PromptTokenCount:     10,
							CandidatesTokenCount: 5,
							TotalTokenCount:      15,
						},
					}),
				},
			},
			validateResult: func(t *testing.T, data []byte, meta llm.ResponseMeta) {
				var resp GenerateContentResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
				require.Equal(t, "resp-agg-1", resp.ResponseID)
				require.Len(t, resp.Candidates, 1)

				// Find text content
				var fullText strings.Builder

				for _, part := range resp.Candidates[0].Content.Parts {
					if !part.Thought {
						fullText.WriteString(part.Text)
					}
				}

				require.Equal(t, "Hello, world!", fullText.String())

				require.NotNil(t, resp.UsageMetadata)
				require.Equal(t, int64(10), resp.UsageMetadata.PromptTokenCount)

				// Verify meta
				require.Equal(t, "resp-agg-1", meta.ID)
				require.NotNil(t, meta.Usage)
				require.Equal(t, int64(10), meta.Usage.PromptTokens)
			},
		},
		{
			name: "aggregate with thinking content",
			chunks: []*httpclient.StreamEvent{
				{
					Data: mustMarshal(&GenerateContentResponse{
						ResponseID:   "resp-think-1",
						ModelVersion: "gemini-2.0-flash-thinking",
						Candidates: []*Candidate{
							{
								Index: 0,
								Content: &Content{
									Role:  "model",
									Parts: []*Part{{Text: "Let me think...", Thought: true}},
								},
							},
						},
					}),
				},
				{
					Data: mustMarshal(&GenerateContentResponse{
						ResponseID:   "resp-think-1",
						ModelVersion: "gemini-2.0-flash-thinking",
						Candidates: []*Candidate{
							{
								Index: 0,
								Content: &Content{
									Role:  "model",
									Parts: []*Part{{Text: "The answer is 42."}},
								},
								FinishReason: "STOP",
							},
						},
					}),
				},
			},
			validateResult: func(t *testing.T, data []byte, meta llm.ResponseMeta) {
				var resp GenerateContentResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
				require.Len(t, resp.Candidates, 1)

				// Find thinking and text parts
				var hasThinking, hasText bool

				for _, part := range resp.Candidates[0].Content.Parts {
					if part.Thought && part.Text == "Let me think..." {
						hasThinking = true
					}

					if !part.Thought && part.Text == "The answer is 42." {
						hasText = true
					}
				}

				require.True(t, hasThinking, "should have thinking part")
				require.True(t, hasText, "should have text part")
			},
		},
		{
			name: "aggregate with tool calls",
			chunks: []*httpclient.StreamEvent{
				{
					Data: mustMarshal(&GenerateContentResponse{
						ResponseID:   "resp-tool-1",
						ModelVersion: "gemini-2.0-flash",
						Candidates: []*Candidate{
							{
								Index: 0,
								Content: &Content{
									Role: "model",
									Parts: []*Part{
										{
											FunctionCall: &FunctionCall{
												ID:   "call-1",
												Name: "get_weather",
												Args: map[string]any{"location": "Tokyo"},
											},
										},
									},
								},
								FinishReason: "STOP",
							},
						},
					}),
				},
			},
			validateResult: func(t *testing.T, data []byte, meta llm.ResponseMeta) {
				var resp GenerateContentResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
				require.Len(t, resp.Candidates, 1)

				// Find function call part
				var hasFunctionCall bool

				for _, part := range resp.Candidates[0].Content.Parts {
					if part.FunctionCall != nil {
						hasFunctionCall = true

						require.Equal(t, "call-1", part.FunctionCall.ID)
						require.Equal(t, "get_weather", part.FunctionCall.Name)
					}
				}

				require.True(t, hasFunctionCall, "should have function call part")
			},
		},
		{
			name: "skip empty chunks",
			chunks: []*httpclient.StreamEvent{
				{
					Data: []byte{},
				},
				{
					Data: mustMarshal(&GenerateContentResponse{
						ResponseID:   "resp-skip-1",
						ModelVersion: "gemini-2.0-flash",
						Candidates: []*Candidate{
							{
								Index: 0,
								Content: &Content{
									Role:  "model",
									Parts: []*Part{{Text: "Valid response"}},
								},
								FinishReason: "STOP",
							},
						},
					}),
				},
			},
			validateResult: func(t *testing.T, data []byte, meta llm.ResponseMeta) {
				var resp GenerateContentResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
				require.Len(t, resp.Candidates, 1)

				var fullText strings.Builder

				for _, part := range resp.Candidates[0].Content.Parts {
					if !part.Thought {
						fullText.WriteString(part.Text)
					}
				}

				require.Equal(t, "Valid response", fullText.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, meta, err := transformer.AggregateStreamChunks(context.Background(), tt.chunks)

			if tt.expectedErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.validateResult != nil {
				tt.validateResult(t, data, meta)
			}
		})
	}
}

func TestInboundTransformer_StreamTransformation_WithTestData(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name               string
		inputStreamFile    string
		expectedStreamFile string
		expectedAggregated func(t *testing.T, result *GenerateContentResponse)
	}{
		{
			name:               "stream transformation with stop finish reason",
			inputStreamFile:    "llm-stop.stream.jsonl",
			expectedStreamFile: "gemini-stop.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Equal(t, "resp-gemini-stop-1", result.ResponseID)
				require.Equal(t, "gemini-2.0-flash", result.ModelVersion)
				require.Len(t, result.Candidates, 1)
				require.NotNil(t, result.Candidates[0].Content)

				// Verify the complete content
				var fullText strings.Builder

				for _, part := range result.Candidates[0].Content.Parts {
					if !part.Thought {
						fullText.WriteString(part.Text)
					}
				}

				require.Equal(t, "Hello, world!", fullText.String())
				require.Equal(t, "STOP", result.Candidates[0].FinishReason)
			},
		},
		{
			name:               "stream transformation with tool calls",
			inputStreamFile:    "llm-tool.stream.jsonl",
			expectedStreamFile: "gemini-tool.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Equal(t, "resp-gemini-tool-1", result.ResponseID)
				require.Len(t, result.Candidates, 1)

				// Find function call parts (aggregator may not preserve text when tool calls present)
				var hasFunctionCall bool

				for _, part := range result.Candidates[0].Content.Parts {
					if part.FunctionCall != nil {
						hasFunctionCall = true

						require.Equal(t, "get_weather", part.FunctionCall.Name)
						require.Equal(t, "Tokyo", part.FunctionCall.Args["location"])
					}
				}

				require.True(t, hasFunctionCall, "should have function call part")
			},
		},
		{
			name:               "stream transformation with thinking content",
			inputStreamFile:    "llm-think.stream.jsonl",
			expectedStreamFile: "gemini-think.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Equal(t, "resp-gemini-think-1", result.ResponseID)
				require.Len(t, result.Candidates, 1)

				// Find thinking and text parts
				var hasThinking, hasText bool

				for _, part := range result.Candidates[0].Content.Parts {
					if part.Thought {
						hasThinking = true

						require.Contains(t, part.Text, "think")
					}

					if !part.Thought && part.Text != "" {
						hasText = true

						require.Equal(t, "The answer is 42.", part.Text)
					}
				}

				require.True(t, hasThinking, "should have thinking part")
				require.True(t, hasText, "should have text part")
			},
		},
		{
			name:               "stream transformation with parallel tool calls",
			inputStreamFile:    "llm-parallel_tool.stream.jsonl",
			expectedStreamFile: "gemini-parallel_tool.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Equal(t, "resp-gemini-parallel-1", result.ResponseID)
				require.Len(t, result.Candidates, 1)

				// Count function calls
				var functionCallCount int

				for _, part := range result.Candidates[0].Content.Parts {
					if part.FunctionCall != nil {
						functionCallCount++
					}
				}

				require.Equal(t, 2, functionCallCount, "should have 2 function calls")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load LLM format responses
			llmResponses, err := xtest.LoadLlmResponses(t, tt.inputStreamFile)
			require.NoError(t, err)

			// Create a mock stream from LLM responses
			mockStream := streams.SliceStream(llmResponses)

			// Transform the stream (LLM -> Gemini)
			transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
			require.NoError(t, err)

			// Collect all transformed events
			var actualEvents []*httpclient.StreamEvent

			for transformedStream.Next() {
				event := transformedStream.Current()
				if event != nil && len(event.Data) > 0 {
					actualEvents = append(actualEvents, event)
				}
			}

			require.NoError(t, transformedStream.Err())

			// Test aggregation
			aggregatedBytes, meta, err := transformer.AggregateStreamChunks(t.Context(), actualEvents)
			require.NoError(t, err)
			require.NotEmpty(t, meta.ID)

			var aggregatedResp GenerateContentResponse

			err = json.Unmarshal(aggregatedBytes, &aggregatedResp)
			require.NoError(t, err)

			// Run custom validation if provided
			if tt.expectedAggregated != nil {
				tt.expectedAggregated(t, &aggregatedResp)
			}
		})
	}
}

func TestInboundTransformer_TransformStreamChunk_FinishReasons(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name                 string
		llmFinishReason      string
		expectedGeminiReason string
	}{
		{"stop to STOP", "stop", "STOP"},
		{"length to MAX_TOKENS", "length", "MAX_TOKENS"},
		{"content_filter to SAFETY", "content_filter", "SAFETY"},
		{"tool_calls to STOP", "tool_calls", "STOP"},
		{"unknown to STOP", "unknown", "STOP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &llm.Response{
				ID:     "test-123",
				Model:  "gemini-2.0-flash",
				Object: "chat.completion.chunk",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("test"),
							},
						},
						FinishReason: lo.ToPtr(tt.llmFinishReason),
					},
				},
			}

			event, err := transformer.TransformStreamChunk(context.Background(), response)
			require.NoError(t, err)
			require.NotNil(t, event)

			var geminiResp GenerateContentResponse

			err = json.Unmarshal(event.Data, &geminiResp)
			require.NoError(t, err)
			require.Len(t, geminiResp.Candidates, 1)
			require.Equal(t, tt.expectedGeminiReason, geminiResp.Candidates[0].FinishReason)
		})
	}
}
