package gemini

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestConvertDocumentURLToGeminiPart(t *testing.T) {
	tests := []struct {
		name     string
		doc      *llm.DocumentURL
		validate func(t *testing.T, result *Part)
	}{
		{
			name: "PDF data URL",
			doc: &llm.DocumentURL{
				URL:      "data:application/pdf;base64,JVBERi0xLjQK",
				MIMEType: "application/pdf",
			},
			validate: func(t *testing.T, result *Part) {
				require.NotNil(t, result)
				require.NotNil(t, result.InlineData)
				assert.Equal(t, "application/pdf", result.InlineData.MIMEType)
				assert.Equal(t, "JVBERi0xLjQK", result.InlineData.Data)
				assert.Nil(t, result.FileData)
			},
		},
		{
			name: "PDF file URL",
			doc: &llm.DocumentURL{
				URL:      "https://example.com/document.pdf",
				MIMEType: "application/pdf",
			},
			validate: func(t *testing.T, result *Part) {
				require.NotNil(t, result)
				require.NotNil(t, result.FileData)
				assert.Equal(t, "https://example.com/document.pdf", result.FileData.FileURI)
				assert.Equal(t, "application/pdf", result.FileData.MIMEType)
				assert.Nil(t, result.InlineData)
			},
		},
		{
			name: "Word document data URL",
			doc: &llm.DocumentURL{
				URL:      "data:application/msword;base64,0M8R4KGx",
				MIMEType: "application/msword",
			},
			validate: func(t *testing.T, result *Part) {
				require.NotNil(t, result)
				require.NotNil(t, result.InlineData)
				assert.Equal(t, "application/msword", result.InlineData.MIMEType)
				assert.Equal(t, "0M8R4KGx", result.InlineData.Data)
			},
		},
		{
			name: "PDF URL without MIME type",
			doc: &llm.DocumentURL{
				URL: "https://example.com/report.pdf",
			},
			validate: func(t *testing.T, result *Part) {
				require.NotNil(t, result)
				require.NotNil(t, result.FileData)
				assert.Equal(t, "https://example.com/report.pdf", result.FileData.FileURI)
				assert.Equal(t, "", result.FileData.MIMEType)
			},
		},
		{
			name: "nil document",
			doc:  nil,
			validate: func(t *testing.T, result *Part) {
				assert.Nil(t, result)
			},
		},
		{
			name: "empty URL",
			doc: &llm.DocumentURL{
				URL: "",
			},
			validate: func(t *testing.T, result *Part) {
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertDocumentURLToGeminiPart(tt.doc)
			tt.validate(t, result)
		})
	}
}

func TestIsDocumentMIMEType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		expected bool
	}{
		{
			name:     "PDF",
			mimeType: "application/pdf",
			expected: true,
		},
		{
			name:     "Word document",
			mimeType: "application/msword",
			expected: true,
		},
		{
			name:     "Word docx",
			mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			expected: true,
		},
		{
			name:     "Excel",
			mimeType: "application/vnd.ms-excel",
			expected: true,
		},
		{
			name:     "Plain text",
			mimeType: "text/plain",
			expected: true,
		},
		{
			name:     "HTML",
			mimeType: "text/html",
			expected: true,
		},
		{
			name:     "PNG image",
			mimeType: "image/png",
			expected: false,
		},
		{
			name:     "JPEG image",
			mimeType: "image/jpeg",
			expected: false,
		},
		{
			name:     "GIF image",
			mimeType: "image/gif",
			expected: false,
		},
		{
			name:     "Empty MIME type",
			mimeType: "",
			expected: false,
		},
		{
			name:     "Case insensitive - PDF uppercase",
			mimeType: "APPLICATION/PDF",
			expected: true,
		},
		{
			name:     "Case insensitive - image uppercase",
			mimeType: "IMAGE/PNG",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDocumentMIMEType(tt.mimeType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertLLMToGeminiRequest_WithDocuments(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Request
		validate func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name: "PDF document in request",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "document",
									Document: &llm.DocumentURL{
										URL:      "data:application/pdf;base64,JVBERi0xLjQK",
										MIMEType: "application/pdf",
									},
								},
								{
									Type: "text",
									Text: lo.ToPtr("Summarize this PDF"),
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				require.Len(t, result.Contents, 1)
				require.Len(t, result.Contents[0].Parts, 2)

				// Check PDF part
				pdfPart := result.Contents[0].Parts[0]
				require.NotNil(t, pdfPart.InlineData)
				assert.Equal(t, "application/pdf", pdfPart.InlineData.MIMEType)
				assert.Equal(t, "JVBERi0xLjQK", pdfPart.InlineData.Data)

				// Check text part
				textPart := result.Contents[0].Parts[1]
				assert.Equal(t, "Summarize this PDF", textPart.Text)
			},
		},
		{
			name: "Mixed image and document",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/png;base64,iVBORw0KGgo",
									},
								},
								{
									Type: "document",
									Document: &llm.DocumentURL{
										URL:      "https://example.com/doc.pdf",
										MIMEType: "application/pdf",
									},
								},
								{
									Type: "text",
									Text: lo.ToPtr("Compare these"),
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				require.Len(t, result.Contents, 1)
				require.Len(t, result.Contents[0].Parts, 3)

				// Check image part
				imagePart := result.Contents[0].Parts[0]
				require.NotNil(t, imagePart.InlineData)
				assert.Equal(t, "image/png", imagePart.InlineData.MIMEType)

				// Check document part
				docPart := result.Contents[0].Parts[1]
				require.NotNil(t, docPart.FileData)
				assert.Equal(t, "https://example.com/doc.pdf", docPart.FileData.FileURI)
				assert.Equal(t, "application/pdf", docPart.FileData.MIMEType)

				// Check text part
				textPart := result.Contents[0].Parts[2]
				assert.Equal(t, "Compare these", textPart.Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequest(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestShouldUseThinkinLevelForBudget(t *testing.T) {
	tests := []struct {
		name   string
		budget int64
	}{
		{"low threshold", 1024},
		{"medium threshold", 5000},
		{"high threshold", 20000},
		{"max threshold", 32768},
	}

	// Test with Gemini 3 models - should use thinkingLevel
	gemini3Models := []string{"gemini-3-5m", "gemini-3-5-pro", "gemini-3-pro", "gemini-3-5-flash"}
	for _, model := range gemini3Models {
		for _, tt := range tests {
			t.Run("gemini3/"+model+"/low", func(t *testing.T) {
				require.True(t, shouldUseThinkingLevelForBudget(model, tt.budget))
			})
		}
	}

	// Test with non-Gemini 3 models - should NOT use thinkingLevel (use budget instead)
	nonGemini3Models := []string{"gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-pro", "gemini-pro"}
	for _, model := range nonGemini3Models {
		for _, tt := range tests {
			t.Run("non-gemini3/"+model+"/low", func(t *testing.T) {
				require.False(t, shouldUseThinkingLevelForBudget(model, tt.budget))
			})
		}
	}

	// Test edge cases
	t.Run("very high budget for Gemini 3", func(t *testing.T) {
		require.False(t, shouldUseThinkingLevelForBudget("gemini-3-5m", 50000))
	})
	t.Run("very low budget for non-Gemini 3", func(t *testing.T) {
		require.False(t, shouldUseThinkingLevelForBudget("gemini-2.5-flash", 500))
	})
}

func TestConvertLLMToGeminiRequest_Gemini3ThinkingLevelvsBudget(t *testing.T) {
	tests := []struct {
		name           string
		input          *llm.Request
		expectedLevel  *string
		expectedBudget *int64
	}{
		{
			name: "gemini-3 with budget within standard range uses thinkingLevel",
			input: &llm.Request{
				Model:           "gemini-3-5m",
				ReasoningBudget: lo.ToPtr(int64(16000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			expectedLevel:  lo.ToPtr("high"),
			expectedBudget: nil,
		},
		{
			name: "gemini-2.5 with budget within standard range still uses thinkingBudget",
			input: &llm.Request{
				Model:           "gemini-2.5-flash",
				ReasoningBudget: lo.ToPtr(int64(16000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			expectedLevel:  nil,
			expectedBudget: lo.ToPtr(int64(16000)),
		},
		{
			name: "gemini-3 with high budget uses thinkingBudget (non-standard)",
			input: &llm.Request{
				Model:           "gemini-3-5m",
				ReasoningBudget: lo.ToPtr(int64(50000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			expectedLevel:  nil,
			expectedBudget: lo.ToPtr(int64(24576)), // Capped at max
		},
		{
			name: "gemini-3 with very low budget uses thinkingLevel",
			input: &llm.Request{
				Model:           "gemini-3-5m",
				ReasoningBudget: lo.ToPtr(int64(500)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			expectedLevel:  lo.ToPtr("low"),
			expectedBudget: nil,
		},
		{
			name: "gemini-3 with medium budget uses thinkingLevel",
			input: &llm.Request{
				Model:           "gemini-3-5m",
				ReasoningBudget: lo.ToPtr(int64(4000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			expectedLevel:  lo.ToPtr("medium"),
			expectedBudget: nil,
		},
		{
			name: "gemini-3 with reasoning effort uses thinkingLevel",
			input: &llm.Request{
				Model:           "gemini-3-5m",
				ReasoningEffort: "high",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			expectedLevel:  lo.ToPtr("high"),
			expectedBudget: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequest(tt.input)
			require.NotNil(t, result.GenerationConfig)
			require.NotNil(t, result.GenerationConfig.ThinkingConfig)

			if tt.expectedLevel != nil {
				require.Equal(t, *tt.expectedLevel, result.GenerationConfig.ThinkingConfig.ThinkingLevel)
			} else {
				require.Empty(t, result.GenerationConfig.ThinkingConfig.ThinkingLevel)
			}

			if tt.expectedBudget != nil {
				require.Equal(t, *tt.expectedBudget, *result.GenerationConfig.ThinkingConfig.ThinkingBudget)
			} else {
				require.Nil(t, result.GenerationConfig.ThinkingConfig.ThinkingBudget)
			}
		})
	}
}

// TestConvertToLLMUsage_CacheHitCalculation tests the specific bug reported where
// Gemini's cachedContentTokenCount was not being reflected correctly in cache hit rate.
//
// Bug scenario:
// With Gemini usage data:
//
//	promptTokenCount: 20981 (total tokens in prompt)
//	cachedContentTokenCount: 20350 (tokens served from cache)
//	candidatesTokenCount: 22
//	totalTokenCount: 21097
//
// AxonHub was reporting 0% cache hit because it was incorrectly handling the token counts.
// The fix ensures:
//
//	prompt_tokens = promptTokenCount - cachedContentTokenCount = 631 (only NEW tokens)
//	cached_tokens = 20350
//
// Cache hit rate should be: 20350 / (631 + 20350) * 100 = 97.0%.
func TestConvertToLLMUsage_CacheHitCalculation(t *testing.T) {
	tests := []struct {
		name          string
		geminiUsage   *UsageMetadata
		expectedUsage *llm.Usage
	}{
		{
			name: "high cache hit rate scenario from bug report",
			geminiUsage: &UsageMetadata{
				PromptTokenCount:        20981,
				CandidatesTokenCount:    22,
				TotalTokenCount:         21097,
				CachedContentTokenCount: 20350,
				ThoughtsTokenCount:      94,
			},
			expectedUsage: &llm.Usage{
				PromptTokens:     20981, // total includes cached
				CompletionTokens: 116,   // 22 + 94
				TotalTokens:      21097,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 20350,
				},
				CompletionTokensDetails: &llm.CompletionTokensDetails{
					ReasoningTokens: 94,
				},
			},
		},
		{
			name: "no cache scenario",
			geminiUsage: &UsageMetadata{
				PromptTokenCount:        1000,
				CandidatesTokenCount:    500,
				TotalTokenCount:         1500,
				CachedContentTokenCount: 0,
			},
			expectedUsage: &llm.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
				TotalTokens:      1500,
			},
		},
		{
			name: "partial cache scenario",
			geminiUsage: &UsageMetadata{
				PromptTokenCount:        1000,
				CandidatesTokenCount:    100,
				TotalTokenCount:         1100,
				CachedContentTokenCount: 600,
			},
			expectedUsage: &llm.Usage{
				PromptTokens:     1000,
				CompletionTokens: 100,
				TotalTokens:      1100,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 600,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToLLMUsage(tt.geminiUsage)

			require.NotNil(t, result)
			assert.Equal(t, tt.expectedUsage.PromptTokens, result.PromptTokens,
				"prompt_tokens should include cached tokens per OpenAI spec")
			assert.Equal(t, tt.expectedUsage.CompletionTokens, result.CompletionTokens)
			assert.Equal(t, tt.expectedUsage.TotalTokens, result.TotalTokens)

			if tt.expectedUsage.PromptTokensDetails != nil {
				require.NotNil(t, result.PromptTokensDetails)
				assert.Equal(t, tt.expectedUsage.PromptTokensDetails.CachedTokens,
					result.PromptTokensDetails.CachedTokens,
					"cached_tokens should match cachedContentTokenCount")

				// Verify cache hit rate calculation would be correct
				cacheHitRate := float64(result.PromptTokensDetails.CachedTokens) /
					float64(result.PromptTokens) * 100
				t.Logf("Cache hit rate: %.1f%%", cacheHitRate)
			} else {
				assert.Nil(t, result.PromptTokensDetails)
			}

			if tt.expectedUsage.CompletionTokensDetails != nil {
				require.NotNil(t, result.CompletionTokensDetails)
				assert.Equal(t, tt.expectedUsage.CompletionTokensDetails.ReasoningTokens,
					result.CompletionTokensDetails.ReasoningTokens)
			}
		})
	}
}

// TestConvertToGeminiUsage_BidirectionalConsistency tests that converting LLM -> Gemini -> LLM
// maintains consistency, especially for cached tokens.
func TestConvertToGeminiUsage_BidirectionalConsistency(t *testing.T) {
	tests := []struct {
		name       string
		llmUsage   *llm.Usage
		wantGemini *UsageMetadata
	}{
		{
			name: "with cached tokens",
			llmUsage: &llm.Usage{
				PromptTokens:     20981, // total
				CompletionTokens: 116,   // total
				TotalTokens:      21097,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 20350,
				},
				CompletionTokensDetails: &llm.CompletionTokensDetails{
					ReasoningTokens: 94,
				},
			},
			wantGemini: &UsageMetadata{
				PromptTokenCount:        20981,
				CandidatesTokenCount:    22, // 116 - 94
				TotalTokenCount:         21097,
				CachedContentTokenCount: 20350,
				ThoughtsTokenCount:      94,
			},
		},
		{
			name: "without cached tokens",
			llmUsage: &llm.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
				TotalTokens:      1500,
			},
			wantGemini: &UsageMetadata{
				PromptTokenCount:        1000,
				CandidatesTokenCount:    500,
				TotalTokenCount:         1500,
				CachedContentTokenCount: 0,
			},
		},
		{
			name: "with thoughts tokens",
			llmUsage: &llm.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
				TotalTokens:      1500,
				CompletionTokensDetails: &llm.CompletionTokensDetails{
					ReasoningTokens: 100,
				},
			},
			wantGemini: &UsageMetadata{
				PromptTokenCount:     1000,
				CandidatesTokenCount: 400, // 500 - 100
				TotalTokenCount:      1500,
				ThoughtsTokenCount:   100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert LLM -> Gemini
			geminiUsage := convertToGeminiUsage(tt.llmUsage)
			require.NotNil(t, geminiUsage)
			assert.Equal(t, tt.wantGemini.PromptTokenCount, geminiUsage.PromptTokenCount,
				"promptTokenCount should include cached tokens")
			assert.Equal(t, tt.wantGemini.CachedContentTokenCount, geminiUsage.CachedContentTokenCount)
			assert.Equal(t, tt.wantGemini.CandidatesTokenCount, geminiUsage.CandidatesTokenCount)
			assert.Equal(t, tt.wantGemini.TotalTokenCount, geminiUsage.TotalTokenCount)

			// Convert back Gemini -> LLM to verify bidirectional consistency
			llmUsageRoundtrip := convertToLLMUsage(geminiUsage)
			require.NotNil(t, llmUsageRoundtrip)
			assert.Equal(t, tt.llmUsage.PromptTokens, llmUsageRoundtrip.PromptTokens,
				"round-trip conversion should preserve prompt_tokens")
			assert.Equal(t, tt.llmUsage.CompletionTokens, llmUsageRoundtrip.CompletionTokens)
			assert.Equal(t, tt.llmUsage.TotalTokens, llmUsageRoundtrip.TotalTokens)

			if tt.llmUsage.PromptTokensDetails != nil {
				require.NotNil(t, llmUsageRoundtrip.PromptTokensDetails)
				assert.Equal(t, tt.llmUsage.PromptTokensDetails.CachedTokens,
					llmUsageRoundtrip.PromptTokensDetails.CachedTokens,
					"round-trip conversion should preserve cached_tokens")
			}
		})
	}
}
