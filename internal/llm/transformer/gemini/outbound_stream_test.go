package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

func TestOutboundTransformer_TransformStreamChunk(t *testing.T) {
	transformer := &OutboundTransformer{
		config: Config{
			BaseURL:    DefaultBaseURL,
			APIVersion: DefaultAPIVersion,
		},
	}

	tests := []struct {
		name           string
		event          *httpclient.StreamEvent
		expectedResp   *llm.Response
		expectedErr    bool
		validateResult func(*testing.T, *llm.Response)
	}{
		{
			name:  "nil event returns nil",
			event: nil,
			validateResult: func(t *testing.T, resp *llm.Response) {
				require.Nil(t, resp)
			},
		},
		{
			name: "empty data returns nil",
			event: &httpclient.StreamEvent{
				Data: []byte{},
			},
			validateResult: func(t *testing.T, resp *llm.Response) {
				require.Nil(t, resp)
			},
		},
		{
			name: "[DONE] marker returns DoneResponse",
			event: &httpclient.StreamEvent{
				Data: []byte("[DONE]"),
			},
			validateResult: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Equal(t, "[DONE]", resp.Object)
			},
		},
		{
			name: "simple text response",
			event: &httpclient.StreamEvent{
				Data: mustMarshal(&GenerateContentResponse{
					ResponseID:   "resp-123",
					ModelVersion: "gemini-2.0-flash",
					Candidates: []*Candidate{
						{
							Index: 0,
							Content: &Content{
								Role: "model",
								Parts: []*Part{
									{Text: "Hello, world!"},
								},
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
			validateResult: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Equal(t, "resp-123", resp.ID)
				require.Equal(t, "gemini-2.0-flash", resp.Model)
				require.Equal(t, "chat.completion.chunk", resp.Object)
				require.Len(t, resp.Choices, 1)
				require.NotNil(t, resp.Choices[0].Delta)
				require.NotNil(t, resp.Choices[0].Delta.Content.Content)
				require.Equal(t, "Hello, world!", *resp.Choices[0].Delta.Content.Content)
				require.NotNil(t, resp.Choices[0].FinishReason)
				require.Equal(t, "stop", *resp.Choices[0].FinishReason)
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(10), resp.Usage.PromptTokens)
				require.Equal(t, int64(5), resp.Usage.CompletionTokens)
			},
		},
		{
			name: "response with thinking content",
			event: &httpclient.StreamEvent{
				Data: mustMarshal(&GenerateContentResponse{
					ResponseID:   "resp-456",
					ModelVersion: "gemini-2.0-flash-thinking",
					Candidates: []*Candidate{
						{
							Index: 0,
							Content: &Content{
								Role: "model",
								Parts: []*Part{
									{Text: "Let me think about this...", Thought: true},
									{Text: "The answer is 42."},
								},
							},
							FinishReason: "STOP",
						},
					},
				}),
			},
			validateResult: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Len(t, resp.Choices, 1)
				require.NotNil(t, resp.Choices[0].Delta)
				require.NotNil(t, resp.Choices[0].Delta.ReasoningContent)
				require.Equal(t, "Let me think about this...", *resp.Choices[0].Delta.ReasoningContent)
				require.NotNil(t, resp.Choices[0].Delta.Content.Content)
				require.Equal(t, "The answer is 42.", *resp.Choices[0].Delta.Content.Content)
			},
		},
		{
			name: "response with function call",
			event: &httpclient.StreamEvent{
				Data: mustMarshal(&GenerateContentResponse{
					ResponseID:   "resp-789",
					ModelVersion: "gemini-2.0-flash",
					Candidates: []*Candidate{
						{
							Index: 0,
							Content: &Content{
								Role: "model",
								Parts: []*Part{
									{
										FunctionCall: &FunctionCall{
											ID:   "call-123",
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
			validateResult: func(t *testing.T, resp *llm.Response) {
				require.NotNil(t, resp)
				require.Len(t, resp.Choices, 1)
				require.NotNil(t, resp.Choices[0].Delta)
				require.Len(t, resp.Choices[0].Delta.ToolCalls, 1)
				require.Equal(t, "call-123", resp.Choices[0].Delta.ToolCalls[0].ID)
				require.Equal(t, "get_weather", resp.Choices[0].Delta.ToolCalls[0].Function.Name)
				require.Contains(t, resp.Choices[0].Delta.ToolCalls[0].Function.Arguments, "Tokyo")
			},
		},
		{
			name: "invalid JSON returns error",
			event: &httpclient.StreamEvent{
				Data: []byte("invalid json"),
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := transformer.TransformStreamChunk(context.Background(), tt.event)

			if tt.expectedErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.validateResult != nil {
				tt.validateResult(t, resp)
			}
		})
	}
}

func TestOutboundTransformer_TransformStream(t *testing.T) {
	transformer := &OutboundTransformer{
		config: Config{
			BaseURL:    DefaultBaseURL,
			APIVersion: DefaultAPIVersion,
		},
	}

	// Create test stream events
	events := []*httpclient.StreamEvent{
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-stream-1",
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
				ResponseID:   "resp-stream-1",
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
	}

	// Create a stream from the events
	inputStream := streams.SliceStream(events)

	// Transform the stream
	outputStream, err := transformer.TransformStream(context.Background(), inputStream)
	require.NoError(t, err)
	require.NotNil(t, outputStream)

	// Collect results
	var results []*llm.Response
	for outputStream.Next() {
		results = append(results, outputStream.Current())
	}

	require.NoError(t, outputStream.Err())

	// Verify results - streaming uses Delta instead of Message
	require.GreaterOrEqual(t, len(results), 2)
	require.NotNil(t, results[0].Choices[0].Delta)
	require.Equal(t, "Hello", *results[0].Choices[0].Delta.Content.Content)
	require.NotNil(t, results[1].Choices[0].Delta)
	require.Equal(t, ", world!", *results[1].Choices[0].Delta.Content.Content)
}

func TestOutboundTransformer_AggregateStreamChunks(t *testing.T) {
	transformer := &OutboundTransformer{
		config: Config{
			BaseURL:    DefaultBaseURL,
			APIVersion: DefaultAPIVersion,
		},
	}

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
				var resp llm.Response

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
				// First parse as Gemini format
				var geminiResp GenerateContentResponse

				err := json.Unmarshal(data, &geminiResp)
				require.NoError(t, err)

				// Convert to LLM format (non-streaming for aggregated result)
				llmResp := convertGeminiToLLMResponse(&geminiResp, false)

				require.Equal(t, "resp-agg-1", llmResp.ID)
				require.Len(t, llmResp.Choices, 1)
				require.NotNil(t, llmResp.Choices[0].Message.Content.Content)
				require.Equal(t, "Hello, world!", *llmResp.Choices[0].Message.Content.Content)
				require.NotNil(t, llmResp.Usage)
				require.Equal(t, int64(10), llmResp.Usage.PromptTokens)
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
				// First parse as Gemini format
				var geminiResp GenerateContentResponse

				err := json.Unmarshal(data, &geminiResp)
				require.NoError(t, err)

				// Convert to LLM format (non-streaming for aggregated result)
				llmResp := convertGeminiToLLMResponse(&geminiResp, false)

				require.Len(t, llmResp.Choices, 1)
				require.NotNil(t, llmResp.Choices[0].Message.ReasoningContent)
				require.Equal(t, "Let me think...", *llmResp.Choices[0].Message.ReasoningContent)
				require.NotNil(t, llmResp.Choices[0].Message.Content.Content)
				require.Equal(t, "The answer is 42.", *llmResp.Choices[0].Message.Content.Content)
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
				// First parse as Gemini format
				var geminiResp GenerateContentResponse

				err := json.Unmarshal(data, &geminiResp)
				require.NoError(t, err)

				// Convert to LLM format (non-streaming for aggregated result)
				llmResp := convertGeminiToLLMResponse(&geminiResp, false)

				require.Len(t, llmResp.Choices, 1)
				require.Len(t, llmResp.Choices[0].Message.ToolCalls, 1)
				require.Equal(t, "call-1", llmResp.Choices[0].Message.ToolCalls[0].ID)
				require.Equal(t, "get_weather", llmResp.Choices[0].Message.ToolCalls[0].Function.Name)
			},
		},
		{
			name: "skip [DONE] marker",
			chunks: []*httpclient.StreamEvent{
				{
					Data: mustMarshal(&GenerateContentResponse{
						ResponseID:   "resp-done-1",
						ModelVersion: "gemini-2.0-flash",
						Candidates: []*Candidate{
							{
								Index: 0,
								Content: &Content{
									Role:  "model",
									Parts: []*Part{{Text: "Hello"}},
								},
								FinishReason: "STOP",
							},
						},
					}),
				},
				{
					Data: []byte("[DONE]"),
				},
			},
			validateResult: func(t *testing.T, data []byte, meta llm.ResponseMeta) {
				// First parse as Gemini format
				var geminiResp GenerateContentResponse

				err := json.Unmarshal(data, &geminiResp)
				require.NoError(t, err)

				// Convert to LLM format (non-streaming for aggregated result)
				llmResp := convertGeminiToLLMResponse(&geminiResp, false)

				require.Len(t, llmResp.Choices, 1)
				require.Equal(t, "Hello", *llmResp.Choices[0].Message.Content.Content)
			},
		},
		{
			name: "skip invalid JSON chunks",
			chunks: []*httpclient.StreamEvent{
				{
					Data: []byte("invalid json"),
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
				// First parse as Gemini format
				var geminiResp GenerateContentResponse

				err := json.Unmarshal(data, &geminiResp)
				require.NoError(t, err)

				// Convert to LLM format (non-streaming for aggregated result)
				llmResp := convertGeminiToLLMResponse(&geminiResp, false)

				require.Len(t, llmResp.Choices, 1)
				require.Equal(t, "Valid response", *llmResp.Choices[0].Message.Content.Content)
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

func TestOutboundTransformer_TransformStreamChunk_FinishReasons(t *testing.T) {
	transformer := &OutboundTransformer{
		config: Config{
			BaseURL:    DefaultBaseURL,
			APIVersion: DefaultAPIVersion,
		},
	}

	tests := []struct {
		name                 string
		geminiFinishReason   string
		expectedFinishReason string
	}{
		{"STOP to stop", "STOP", "stop"},
		{"MAX_TOKENS to length", "MAX_TOKENS", "length"},
		{"SAFETY to content_filter", "SAFETY", "content_filter"},
		{"RECITATION to content_filter", "RECITATION", "content_filter"},
		{"unknown to stop", "UNKNOWN", "stop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &httpclient.StreamEvent{
				Data: mustMarshal(&GenerateContentResponse{
					ResponseID: "test-response-id",
					Candidates: []*Candidate{
						{
							Index: 0,
							Content: &Content{
								Role:  "model",
								Parts: []*Part{{Text: "test"}},
							},
							FinishReason: tt.geminiFinishReason,
						},
					},
				}),
			}

			resp, err := transformer.TransformStreamChunk(context.Background(), event)
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.NotNil(t, resp.Choices[0].FinishReason)
			require.Equal(t, tt.expectedFinishReason, *resp.Choices[0].FinishReason)
		})
	}
}

func TestOutboundTransformer_StreamTransformation_WithTestData(t *testing.T) {
	transformer := &OutboundTransformer{
		config: Config{
			BaseURL:    DefaultBaseURL,
			APIVersion: DefaultAPIVersion,
		},
	}

	tests := []struct {
		name               string
		inputStreamFile    string
		expectedStreamFile string
		expectedAggregated func(t *testing.T, result *llm.Response)
	}{
		{
			name:               "stream transformation with stop finish reason",
			inputStreamFile:    "gemini-stop.stream.jsonl",
			expectedStreamFile: "llm-stop.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "resp-gemini-stop-1", result.ID)
				require.Equal(t, "gemini-2.0-flash", result.Model)
				require.Equal(t, "chat.completion", result.Object)
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].Message)
				require.NotNil(t, result.Choices[0].Message.Content.Content)
				require.Equal(t, "Hello, world!", *result.Choices[0].Message.Content.Content)
				require.NotNil(t, result.Choices[0].FinishReason)
				require.Equal(t, "stop", *result.Choices[0].FinishReason)

				// Verify usage
				require.NotNil(t, result.Usage)
				require.Equal(t, int64(10), result.Usage.PromptTokens)
				require.Equal(t, int64(5), result.Usage.CompletionTokens)
			},
		},
		{
			name:               "stream transformation with tool calls",
			inputStreamFile:    "gemini-tool.stream.jsonl",
			expectedStreamFile: "llm-tool.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "resp-gemini-tool-1", result.ID)
				require.Len(t, result.Choices, 1)

				// Should have tool calls (when tool calls present, content is nil per aggregator logic)
				require.Len(t, result.Choices[0].Message.ToolCalls, 1)
				require.Equal(t, "get_weather", result.Choices[0].Message.ToolCalls[0].Function.Name)
				require.Contains(t, result.Choices[0].Message.ToolCalls[0].Function.Arguments, "Tokyo")

				require.NotNil(t, result.Choices[0].FinishReason)
				require.Equal(t, "tool_calls", *result.Choices[0].FinishReason)
			},
		},
		{
			name:               "stream transformation with thinking content",
			inputStreamFile:    "gemini-think.stream.jsonl",
			expectedStreamFile: "llm-think.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "resp-gemini-think-1", result.ID)
				require.Len(t, result.Choices, 1)

				// Should have reasoning content
				require.NotNil(t, result.Choices[0].Message.ReasoningContent)
				require.Contains(t, *result.Choices[0].Message.ReasoningContent, "think")

				// Should have text content
				require.NotNil(t, result.Choices[0].Message.Content.Content)
				require.Equal(t, "The answer is 42.", *result.Choices[0].Message.Content.Content)

				require.NotNil(t, result.Choices[0].FinishReason)
				require.Equal(t, "stop", *result.Choices[0].FinishReason)
			},
		},
		{
			name:               "stream transformation with parallel tool calls",
			inputStreamFile:    "gemini-parallel_tool.stream.jsonl",
			expectedStreamFile: "llm-parallel_tool.stream.jsonl",
			expectedAggregated: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "resp-gemini-parallel-1", result.ID)
				require.Len(t, result.Choices, 1)

				// Should have 2 tool calls
				require.Len(t, result.Choices[0].Message.ToolCalls, 2)
				require.Equal(t, "get_weather", result.Choices[0].Message.ToolCalls[0].Function.Name)
				require.Equal(t, "get_time", result.Choices[0].Message.ToolCalls[1].Function.Name)

				require.NotNil(t, result.Choices[0].FinishReason)
				require.Equal(t, "tool_calls", *result.Choices[0].FinishReason)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load Gemini format stream events
			geminiEvents, err := loadGeminiStreamChunks(t, tt.inputStreamFile)
			require.NoError(t, err)

			// Create a mock stream from Gemini events
			mockStream := streams.SliceStream(geminiEvents)

			// Transform the stream (Gemini -> LLM)
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

			// Test aggregation
			aggregatedBytes, meta, err := transformer.AggregateStreamChunks(t.Context(), geminiEvents)
			require.NoError(t, err)
			require.NotEmpty(t, meta.ID)

			// First parse as Gemini format
			var geminiResp GenerateContentResponse

			err = json.Unmarshal(aggregatedBytes, &geminiResp)
			require.NoError(t, err)

			// Convert to LLM format (non-streaming for aggregated result)
			aggregatedResp := convertGeminiToLLMResponse(&geminiResp, false)

			// Run custom validation if provided
			if tt.expectedAggregated != nil {
				tt.expectedAggregated(t, aggregatedResp)
			}
		})
	}
}

// loadGeminiStreamChunks loads Gemini stream chunks from a JSONL file in testdata directory.
func loadGeminiStreamChunks(t *testing.T, filename string) ([]*httpclient.StreamEvent, error) {
	t.Helper()

	file, err := os.Open("testdata/" + filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var chunks []*httpclient.StreamEvent

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var temp struct {
			LastEventID string `json:"LastEventID"`
			Type        string `json:"Type"`
			Data        string `json:"Data"`
		}

		if err := json.Unmarshal([]byte(line), &temp); err != nil {
			return nil, err
		}

		chunks = append(chunks, &httpclient.StreamEvent{
			LastEventID: temp.LastEventID,
			Type:        temp.Type,
			Data:        []byte(temp.Data),
		})
	}

	return chunks, scanner.Err()
}

// Helper function to marshal JSON for tests.
func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}

// Ensure lo is used to satisfy linter.
var _ = lo.ToPtr[string]
