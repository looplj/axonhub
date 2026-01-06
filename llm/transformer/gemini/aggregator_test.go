package gemini

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestAggregateStreamChunks_EmptyChunks(t *testing.T) {
	data, meta, err := AggregateStreamChunks(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, data)
	require.Empty(t, meta.ID)
}

func TestAggregateStreamChunks_SimpleText(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-123",
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
				ResponseID:   "resp-123",
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

	t.Run("Gemini format output", func(t *testing.T) {
		data, meta, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Equal(t, "resp-123", resp.ResponseID)
		require.Equal(t, "gemini-2.0-flash", resp.ModelVersion)
		require.Len(t, resp.Candidates, 1)
		require.NotNil(t, resp.Candidates[0].Content)

		// Find text content
		var fullText string

		for _, part := range resp.Candidates[0].Content.Parts {
			if !part.Thought {
				fullText += part.Text
			}
		}

		require.Equal(t, "Hello, world!", fullText)
		require.Equal(t, "STOP", resp.Candidates[0].FinishReason)

		require.NotNil(t, resp.UsageMetadata)
		require.Equal(t, int64(10), resp.UsageMetadata.PromptTokenCount)
		require.Equal(t, int64(5), resp.UsageMetadata.CandidatesTokenCount)

		// Verify meta
		require.Equal(t, "resp-123", meta.ID)
		require.NotNil(t, meta.Usage)
		require.Equal(t, int64(10), meta.Usage.PromptTokens)
	})
}

func TestAggregateStreamChunks_WithThinking(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
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
	}

	t.Run("Gemini format output", func(t *testing.T) {
		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Len(t, resp.Candidates, 1)
		require.NotNil(t, resp.Candidates[0].Content)

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
	})
}

func TestAggregateStreamChunks_WithToolCalls(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
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
	}

	t.Run("Gemini format output", func(t *testing.T) {
		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Len(t, resp.Candidates, 1)

		// Find function call part
		var hasFunctionCall bool

		for _, part := range resp.Candidates[0].Content.Parts {
			if part.FunctionCall != nil {
				hasFunctionCall = true

				require.Equal(t, "call-1", part.FunctionCall.ID)
				require.Equal(t, "get_weather", part.FunctionCall.Name)
				require.Equal(t, "Tokyo", part.FunctionCall.Args["location"])
			}
		}

		require.True(t, hasFunctionCall, "should have function call part")
	})
}

func TestAggregateStreamChunks_SkipInvalidChunks(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		nil,
		{Data: []byte{}},
		{Data: []byte("[DONE]")},
		{Data: []byte("invalid json")},
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-valid",
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
	}

	t.Run("Gemini format output", func(t *testing.T) {
		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Len(t, resp.Candidates, 1)
		require.Equal(t, "Valid response", resp.Candidates[0].Content.Parts[0].Text)
	})
}

func TestAggregateStreamChunks_MultipleToolCalls(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-multi-tool",
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
					},
				},
			}),
		},
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-multi-tool",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									FunctionCall: &FunctionCall{
										ID:   "call-2",
										Name: "get_time",
										Args: map[string]any{"timezone": "JST"},
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			}),
		},
	}

	t.Run("Gemini format output", func(t *testing.T) {
		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Len(t, resp.Candidates, 1)
		require.Len(t, resp.Candidates[0].Content.Parts, 2)

		// Verify first tool call
		require.Equal(t, "call-1", resp.Candidates[0].Content.Parts[0].FunctionCall.ID)
		require.Equal(t, "get_weather", resp.Candidates[0].Content.Parts[0].FunctionCall.Name)

		// Verify second tool call
		require.Equal(t, "call-2", resp.Candidates[0].Content.Parts[1].FunctionCall.ID)
		require.Equal(t, "get_time", resp.Candidates[0].Content.Parts[1].FunctionCall.Name)
	})
}

func TestAggregateStreamChunks_WithInlineData(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-image-1",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									InlineData: &Blob{
										MIMEType: "image/png",
										Data:     "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			}),
		},
	}

	t.Run("Gemini format output with single image", func(t *testing.T) {
		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Len(t, resp.Candidates, 1)
		require.NotNil(t, resp.Candidates[0].Content)
		require.Len(t, resp.Candidates[0].Content.Parts, 1)

		part := resp.Candidates[0].Content.Parts[0]
		require.NotNil(t, part.InlineData)
		require.Equal(t, "image/png", part.InlineData.MIMEType)
		require.NotEmpty(t, part.InlineData.Data)
	})
}

func TestAggregateStreamChunks_WithMultipleInlineData(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-multi-image-1",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									InlineData: &Blob{
										MIMEType: "image/png",
										Data:     "first_image_data",
									},
								},
							},
						},
					},
				},
			}),
		},
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-multi-image-1",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									InlineData: &Blob{
										MIMEType: "image/jpeg",
										Data:     "second_image_data",
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			}),
		},
	}

	t.Run("Gemini format output with multiple images", func(t *testing.T) {
		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Len(t, resp.Candidates, 1)
		require.NotNil(t, resp.Candidates[0].Content)
		require.Len(t, resp.Candidates[0].Content.Parts, 2)

		// Verify first image
		require.NotNil(t, resp.Candidates[0].Content.Parts[0].InlineData)
		require.Equal(t, "image/png", resp.Candidates[0].Content.Parts[0].InlineData.MIMEType)
		require.Equal(t, "first_image_data", resp.Candidates[0].Content.Parts[0].InlineData.Data)

		// Verify second image
		require.NotNil(t, resp.Candidates[0].Content.Parts[1].InlineData)
		require.Equal(t, "image/jpeg", resp.Candidates[0].Content.Parts[1].InlineData.MIMEType)
		require.Equal(t, "second_image_data", resp.Candidates[0].Content.Parts[1].InlineData.Data)
	})
}

func TestAggregateStreamChunks_WithTextAndInlineData(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-text-image-1",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Here is the generated image:"},
							},
						},
					},
				},
			}),
		},
		{
			Data: mustMarshal(&GenerateContentResponse{
				ResponseID:   "resp-text-image-1",
				ModelVersion: "gemini-2.0-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									InlineData: &Blob{
										MIMEType: "image/png",
										Data:     "generated_image_data",
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			}),
		},
	}

	t.Run("Gemini format output with text and image", func(t *testing.T) {
		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Len(t, resp.Candidates, 1)
		require.NotNil(t, resp.Candidates[0].Content)
		require.Len(t, resp.Candidates[0].Content.Parts, 2)

		// Verify text part
		require.Equal(t, "Here is the generated image:", resp.Candidates[0].Content.Parts[0].Text)

		// Verify image part
		require.NotNil(t, resp.Candidates[0].Content.Parts[1].InlineData)
		require.Equal(t, "image/png", resp.Candidates[0].Content.Parts[1].InlineData.MIMEType)
		require.Equal(t, "generated_image_data", resp.Candidates[0].Content.Parts[1].InlineData.Data)
	})
}

func TestAggregateStreamChunks_FinishReasons(t *testing.T) {
	tests := []struct {
		name               string
		geminiFinishReason string
	}{
		{"STOP", "STOP"},
		{"MAX_TOKENS", "MAX_TOKENS"},
		{"SAFETY", "SAFETY"},
		{"RECITATION", "RECITATION"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := []*httpclient.StreamEvent{
				{
					Data: mustMarshal(&GenerateContentResponse{
						ResponseID:   "resp-finish",
						ModelVersion: "gemini-2.0-flash",
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
				},
			}

			// Test Gemini format preserves finish reason
			geminiData, _, err := AggregateStreamChunks(context.Background(), chunks)
			require.NoError(t, err)

			var geminiResp GenerateContentResponse

			err = json.Unmarshal(geminiData, &geminiResp)
			require.NoError(t, err)
			require.Equal(t, tt.geminiFinishReason, geminiResp.Candidates[0].FinishReason)
		})
	}
}

func TestAggregateStreamChunks_WithGroundingMetadata(t *testing.T) {
	t.Run("grounding metadata in final chunk", func(t *testing.T) {
		chunks := []*httpclient.StreamEvent{
			{
				Data: mustMarshal(&GenerateContentResponse{
					ResponseID:   "resp-grounding",
					ModelVersion: "gemini-2.0-flash",
					Candidates: []*Candidate{
						{
							Index: 0,
							Content: &Content{
								Role:  "model",
								Parts: []*Part{{Text: "Based on my search, "}},
							},
						},
					},
				}),
			},
			{
				Data: mustMarshal(&GenerateContentResponse{
					ResponseID:   "resp-grounding",
					ModelVersion: "gemini-2.0-flash",
					Candidates: []*Candidate{
						{
							Index: 0,
							Content: &Content{
								Role:  "model",
								Parts: []*Part{{Text: "here is the answer."}},
							},
							FinishReason: "STOP",
							GroundingMetadata: &GroundingMetadata{
								WebSearchQueries: []string{"latest AI news"},
								GroundingChunks: []*GroundingChunk{
									{
										Web: &GroundingChunkWeb{
											URI:    "https://example.com/article",
											Title:  "AI News",
											Domain: "example.com",
										},
									},
								},
								GroundingSupports: []*GroundingSupport{
									{
										Segment: &Segment{
											StartIndex: 0,
											EndIndex:   40,
											Text:       "Based on my search, here is the answer.",
										},
										GroundingChunkIndices: []int32{0},
										ConfidenceScores:      []float32{0.95},
									},
								},
								SearchEntryPoint: &SearchEntryPoint{
									RenderedContent: "<div>Search results</div>",
								},
								RetrievalMetadata: &RetrievalMetadata{
									GoogleSearchDynamicRetrievalScore: 0.92,
								},
							},
						},
					},
				}),
			},
		}

		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Len(t, resp.Candidates, 1)
		require.NotNil(t, resp.Candidates[0].GroundingMetadata)

		gm := resp.Candidates[0].GroundingMetadata
		require.Equal(t, []string{"latest AI news"}, gm.WebSearchQueries)
		require.Len(t, gm.GroundingChunks, 1)
		require.Equal(t, "https://example.com/article", gm.GroundingChunks[0].Web.URI)
		require.Equal(t, "AI News", gm.GroundingChunks[0].Web.Title)
		require.Len(t, gm.GroundingSupports, 1)
		require.Equal(t, []int32{0}, gm.GroundingSupports[0].GroundingChunkIndices)
		require.NotNil(t, gm.SearchEntryPoint)
		require.Equal(t, "<div>Search results</div>", gm.SearchEntryPoint.RenderedContent)
		require.NotNil(t, gm.RetrievalMetadata)
		require.InDelta(t, 0.92, gm.RetrievalMetadata.GoogleSearchDynamicRetrievalScore, 0.01)
	})

	t.Run("grounding metadata with retrieved context", func(t *testing.T) {
		chunks := []*httpclient.StreamEvent{
			{
				Data: mustMarshal(&GenerateContentResponse{
					ResponseID:   "resp-retrieval",
					ModelVersion: "gemini-2.0-flash",
					Candidates: []*Candidate{
						{
							Index: 0,
							Content: &Content{
								Role:  "model",
								Parts: []*Part{{Text: "According to the document..."}},
							},
							FinishReason: "STOP",
							GroundingMetadata: &GroundingMetadata{
								GroundingChunks: []*GroundingChunk{
									{
										RetrievedContext: &GroundingChunkRetrievedContext{
											URI:   "gs://bucket/document.pdf",
											Title: "Important Document",
											Text:  "Relevant excerpt from the document.",
										},
									},
								},
							},
						},
					},
				}),
			},
		}

		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.NotNil(t, resp.Candidates[0].GroundingMetadata)
		gm := resp.Candidates[0].GroundingMetadata
		require.Len(t, gm.GroundingChunks, 1)
		require.NotNil(t, gm.GroundingChunks[0].RetrievedContext)
		require.Equal(t, "gs://bucket/document.pdf", gm.GroundingChunks[0].RetrievedContext.URI)
		require.Equal(t, "Important Document", gm.GroundingChunks[0].RetrievedContext.Title)
	})

	t.Run("no grounding metadata", func(t *testing.T) {
		chunks := []*httpclient.StreamEvent{
			{
				Data: mustMarshal(&GenerateContentResponse{
					ResponseID:   "resp-no-grounding",
					ModelVersion: "gemini-2.0-flash",
					Candidates: []*Candidate{
						{
							Index: 0,
							Content: &Content{
								Role:  "model",
								Parts: []*Part{{Text: "Simple response"}},
							},
							FinishReason: "STOP",
						},
					},
				}),
			},
		}

		data, _, err := AggregateStreamChunks(context.Background(), chunks)
		require.NoError(t, err)

		var resp GenerateContentResponse

		err = json.Unmarshal(data, &resp)
		require.NoError(t, err)

		require.Nil(t, resp.Candidates[0].GroundingMetadata)
	})
}
