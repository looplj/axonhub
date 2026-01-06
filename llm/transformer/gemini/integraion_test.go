package gemini

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestGeminiTransformers_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")

	tests := []struct {
		name               string
		requestPath        string
		requestQuery       url.Values
		geminiRequestJSON  string
		expectedModel      string
		expectedMaxTokens  int64
		expectedPath       string
		expectedModalities []string
	}{
		{
			name:         "simple text message",
			requestPath:  "/v1beta/models/gemini-2.5-flash:generateContent",
			requestQuery: url.Values{},
			geminiRequestJSON: `{
				"contents": [
					{
						"role": "user",
						"parts": [
							{"text": "Hello, Gemini!"}
						]
					}
				],
				"generationConfig": {
					"maxOutputTokens": 1024
				}
			}`,
			expectedModel:     "gemini-2.5-flash",
			expectedMaxTokens: 1024,
			expectedPath:      "/v1beta/models/gemini-2.5-flash:generateContent",
		},
		{
			name:         "message with system instruction",
			requestPath:  "/v1beta/models/gemini-2.5-flash:generateContent",
			requestQuery: url.Values{},
			geminiRequestJSON: `{
				"systemInstruction": {
					"parts": [
						{"text": "You are a helpful assistant."}
					]
				},
				"contents": [
					{
						"role": "user",
						"parts": [
							{"text": "What is the capital of France?"}
						]
					}
				],
				"generationConfig": {
					"maxOutputTokens": 2048,
					"temperature": 0.7
				}
			}`,
			expectedModel:     "gemini-2.5-flash",
			expectedMaxTokens: 2048,
			expectedPath:      "/v1beta/models/gemini-2.5-flash:generateContent",
		},
		{
			name:         "message with modalities",
			requestPath:  "/v1beta/models/gemini-2.5-flash-image-preview:generateContent",
			requestQuery: url.Values{},
			geminiRequestJSON: `{
				"contents": [
					{
						"role": "user",
						"parts": [
							{"text": "Draw a green apple"}
						]
					}
				],
				"generationConfig": {
					"maxOutputTokens": 1024,
					"temperature": 1,
					"topP": 1,
					"responseModalities": ["TEXT", "IMAGE"]
				}
			}`,
			expectedModel:      "gemini-2.5-flash-image-preview",
			expectedMaxTokens:  1024,
			expectedPath:       "/v1beta/models/gemini-2.5-flash-image-preview:generateContent",
			expectedModalities: []string{"text", "image"},
		},
		{
			name:         "streaming request with alt query",
			requestPath:  "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
			requestQuery: url.Values{"alt": []string{"sse"}},
			geminiRequestJSON: `{
				"contents": [
					{
						"role": "user",
						"parts": [
							{"text": "Hello streaming!"}
						]
					}
				],
				"generationConfig": {
					"maxOutputTokens": 512
				}
			}`,
			expectedModel:     "gemini-2.5-flash",
			expectedMaxTokens: 512,
			expectedPath:      "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Transform Gemini request to LLM Request
			httpReq := &httpclient.Request{
				Path:  tt.requestPath,
				Query: tt.requestQuery,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(tt.geminiRequestJSON),
			}

			chatReq, err := inboundTransformer.TransformRequest(t.Context(), httpReq)
			require.NoError(t, err)
			require.NotNil(t, chatReq)

			// Verify the transformation
			require.Equal(t, tt.expectedModel, chatReq.Model)
			require.Equal(t, tt.expectedMaxTokens, *chatReq.MaxTokens)
			require.NotEmpty(t, chatReq.Messages)

			// Verify streaming behavior based on request path
			expectedStream := strings.Contains(tt.requestPath, "streamGenerateContent")

			require.NotNil(t, chatReq.Stream)
			require.Equal(t, expectedStream, *chatReq.Stream)

			// Verify modalities if expected
			if len(tt.expectedModalities) > 0 {
				require.Equal(t, tt.expectedModalities, chatReq.Modalities)
			}

			// Step 2: Transform LLM Request to Gemini outbound request
			outboundReq, err := outboundTransformer.TransformRequest(t.Context(), chatReq)
			require.NoError(t, err)
			require.NotNil(t, outboundReq)

			// Verify outbound request
			require.Equal(t, http.MethodPost, outboundReq.Method)
			require.Equal(t, "application/json", outboundReq.Headers.Get("Content-Type"))

			// Verify the URL matches expected path
			expectedURL := "https://generativelanguage.googleapis.com" + tt.expectedPath
			require.Equal(t, expectedURL, outboundReq.URL)

			// Verify the outbound request body can be unmarshaled
			var geminiReq GenerateContentRequest

			err = json.Unmarshal(outboundReq.Body, &geminiReq)
			require.NoError(t, err)

			// Verify modalities are preserved
			if len(tt.expectedModalities) > 0 {
				require.NotNil(t, geminiReq.GenerationConfig)
				require.Equal(t, []string{"TEXT", "IMAGE"}, geminiReq.GenerationConfig.ResponseModalities)
			}

			// Step 3: Simulate Gemini response and transform back
			geminiResponse := &GenerateContentResponse{
				ResponseID:   "test-response-123",
				ModelVersion: tt.expectedModel,
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "This is a test response from Gemini."},
							},
						},
						FinishReason: "STOP",
					},
				},
				UsageMetadata: &UsageMetadata{
					PromptTokenCount:     15,
					CandidatesTokenCount: 25,
					TotalTokenCount:      40,
				},
			}

			responseBody, err := json.Marshal(geminiResponse)
			require.NoError(t, err)

			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       responseBody,
			}

			// Step 4: Transform Gemini response to LLM Response
			chatResp, err := outboundTransformer.TransformResponse(t.Context(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, chatResp)

			// Verify chat response
			require.Equal(t, "test-response-123", chatResp.ID)
			require.Equal(t, "chat.completion", chatResp.Object)
			require.Equal(t, tt.expectedModel, chatResp.Model)
			require.Len(t, chatResp.Choices, 1)
			require.Equal(t, "assistant", chatResp.Choices[0].Message.Role)
			require.Equal(t, "This is a test response from Gemini.", *chatResp.Choices[0].Message.Content.Content)
			require.Equal(t, "stop", *chatResp.Choices[0].FinishReason)

			// Step 5: Transform LLM Response back to Gemini format
			finalHttpResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
			require.NoError(t, err)
			require.NotNil(t, finalHttpResp)

			// Verify final response
			require.Equal(t, http.StatusOK, finalHttpResp.StatusCode)
			require.Equal(t, "application/json", finalHttpResp.Headers.Get("Content-Type"))

			var finalGeminiResp GenerateContentResponse

			err = json.Unmarshal(finalHttpResp.Body, &finalGeminiResp)
			require.NoError(t, err)
			require.Equal(t, "test-response-123", finalGeminiResp.ResponseID)
			require.Equal(t, tt.expectedModel, finalGeminiResp.ModelVersion)
			require.Equal(t, "model", finalGeminiResp.Candidates[0].Content.Role)
		})
	}
}

func TestTransformRequest_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")

	tests := []struct {
		name        string
		requestFile string
	}{
		{
			name:        "simple request",
			requestFile: "gemini-simple.request.json",
		},
		{
			name:        "tools request",
			requestFile: "gemini-tools.request.json",
		},
		{
			name:        "thinking request",
			requestFile: "gemini-thinking.request.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wantReq GenerateContentRequest

			err := xtest.LoadTestData(t, tt.requestFile, &wantReq)
			require.NoError(t, err)

			var buf bytes.Buffer

			encoder := json.NewEncoder(&buf)
			encoder.SetEscapeHTML(false)

			if err := encoder.Encode(wantReq); err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			// Use a mock path for the inbound transformer
			httpReq := &httpclient.Request{
				Path: "/v1beta/models/gemini-2.5-flash:generateContent",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: buf.Bytes(),
			}

			chatReq, err := inboundTransformer.TransformRequest(t.Context(), httpReq)
			require.NoError(t, err)
			require.NotNil(t, chatReq)

			outboundReq, err := outboundTransformer.TransformRequest(t.Context(), chatReq)
			require.NoError(t, err)

			var gotReq GenerateContentRequest

			err = json.Unmarshal(outboundReq.Body, &gotReq)
			require.NoError(t, err)

			// Custom comparator for json.RawMessage that compares semantic equality
			jsonRawMessageComparer := cmp.Comparer(func(x, y json.RawMessage) bool {
				if len(x) == 0 && len(y) == 0 {
					return true
				}

				if len(x) == 0 || len(y) == 0 {
					return false
				}

				var xVal, yVal interface{}
				if err := json.Unmarshal(x, &xVal); err != nil {
					return false
				}

				if err := json.Unmarshal(y, &yVal); err != nil {
					return false
				}

				return cmp.Equal(xVal, yVal)
			})

			if !cmp.Equal(wantReq, gotReq, jsonRawMessageComparer) {
				t.Errorf("wantReq != gotReq\n%s", cmp.Diff(wantReq, gotReq, jsonRawMessageComparer))
			}
		})
	}
}

func TestGeminiImageResponse_Integration(t *testing.T) {
	outboundTransformer, _ := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")
	inboundTransformer := NewInboundTransformer()

	tests := []struct {
		name           string
		geminiResponse *GenerateContentResponse
		validate       func(t *testing.T, chatResp *llm.Response)
	}{
		{
			name: "image only response",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-image-only",
				ModelVersion: "gemini-2.5-flash-image-preview",
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
				UsageMetadata: &UsageMetadata{
					PromptTokenCount:     6,
					CandidatesTokenCount: 1299,
					TotalTokenCount:      1305,
					PromptTokensDetails: []*ModalityTokenCount{
						{Modality: "TEXT", TokenCount: 6},
					},
					CandidatesTokensDetails: []*ModalityTokenCount{
						{Modality: "IMAGE", TokenCount: 1290},
					},
				},
			},
			validate: func(t *testing.T, chatResp *llm.Response) {
				t.Helper()
				require.Equal(t, "resp-image-only", chatResp.ID)
				require.Len(t, chatResp.Choices, 1)
				require.Equal(t, "assistant", chatResp.Choices[0].Message.Role)

				// Verify image content
				require.NotNil(t, chatResp.Choices[0].Message.Content.MultipleContent)
				require.Len(t, chatResp.Choices[0].Message.Content.MultipleContent, 1)
				require.Equal(t, "image_url", chatResp.Choices[0].Message.Content.MultipleContent[0].Type)
				require.NotNil(t, chatResp.Choices[0].Message.Content.MultipleContent[0].ImageURL)
				require.Equal(t,
					"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
					chatResp.Choices[0].Message.Content.MultipleContent[0].ImageURL.URL,
				)

				// Verify modality token details
				require.Len(t, chatResp.Usage.PromptModalityTokenDetails, 1)
				require.Equal(t, "TEXT", chatResp.Usage.PromptModalityTokenDetails[0].Modality)
				require.Len(t, chatResp.Usage.CompletionModalityTokenDetails, 1)
				require.Equal(t, "IMAGE", chatResp.Usage.CompletionModalityTokenDetails[0].Modality)
			},
		},
		{
			name: "text and image mixed response",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-text-image",
				ModelVersion: "gemini-2.5-flash-image-preview",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Here is the green apple you requested:"},
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
				UsageMetadata: &UsageMetadata{
					PromptTokenCount:     10,
					CandidatesTokenCount: 1500,
					TotalTokenCount:      1510,
				},
			},
			validate: func(t *testing.T, chatResp *llm.Response) {
				t.Helper()
				require.Equal(t, "resp-text-image", chatResp.ID)
				require.Len(t, chatResp.Choices, 1)

				// Should have multipart content with text first, then image
				require.NotNil(t, chatResp.Choices[0].Message.Content.MultipleContent)
				require.Len(t, chatResp.Choices[0].Message.Content.MultipleContent, 2)

				// First part should be text
				require.Equal(t, "text", chatResp.Choices[0].Message.Content.MultipleContent[0].Type)
				require.Equal(t, "Here is the green apple you requested:", *chatResp.Choices[0].Message.Content.MultipleContent[0].Text)

				// Second part should be image
				require.Equal(t, "image_url", chatResp.Choices[0].Message.Content.MultipleContent[1].Type)
				require.Contains(t, chatResp.Choices[0].Message.Content.MultipleContent[1].ImageURL.URL, "data:image/png;base64,")
			},
		},
		{
			name: "multiple images response",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-multi-image",
				ModelVersion: "gemini-2.5-flash-image-preview",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									InlineData: &Blob{
										MIMEType: "image/png",
										Data:     "image1base64data",
									},
								},
								{
									InlineData: &Blob{
										MIMEType: "image/jpeg",
										Data:     "image2base64data",
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, chatResp *llm.Response) {
				t.Helper()
				require.Equal(t, "resp-multi-image", chatResp.ID)
				require.Len(t, chatResp.Choices, 1)

				// Should have 2 images
				require.NotNil(t, chatResp.Choices[0].Message.Content.MultipleContent)
				require.Len(t, chatResp.Choices[0].Message.Content.MultipleContent, 2)

				// First image (PNG)
				require.Equal(t, "image_url", chatResp.Choices[0].Message.Content.MultipleContent[0].Type)
				require.Equal(t, "data:image/png;base64,image1base64data", chatResp.Choices[0].Message.Content.MultipleContent[0].ImageURL.URL)

				// Second image (JPEG)
				require.Equal(t, "image_url", chatResp.Choices[0].Message.Content.MultipleContent[1].Type)
				require.Equal(t, "data:image/jpeg;base64,image2base64data", chatResp.Choices[0].Message.Content.MultipleContent[1].ImageURL.URL)
			},
		},
		{
			name: "different mime types",
			geminiResponse: &GenerateContentResponse{
				ResponseID:   "resp-mime-types",
				ModelVersion: "gemini-2.5-flash-image-preview",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									InlineData: &Blob{
										MIMEType: "image/webp",
										Data:     "webpdata",
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, chatResp *llm.Response) {
				t.Helper()
				require.Len(t, chatResp.Choices[0].Message.Content.MultipleContent, 1)
				require.Equal(t, "data:image/webp;base64,webpdata", chatResp.Choices[0].Message.Content.MultipleContent[0].ImageURL.URL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseBody, err := json.Marshal(tt.geminiResponse)
			require.NoError(t, err)

			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       responseBody,
			}

			// Transform Gemini response to LLM Response
			chatResp, err := outboundTransformer.TransformResponse(t.Context(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, chatResp)

			tt.validate(t, chatResp)
		})
	}

	// Test round-trip: Gemini -> LLM -> Gemini
	t.Run("image response round trip", func(t *testing.T) {
		geminiResponse := &GenerateContentResponse{
			ResponseID:   "resp-roundtrip",
			ModelVersion: "gemini-2.5-flash-image-preview",
			Candidates: []*Candidate{
				{
					Index: 0,
					Content: &Content{
						Role: "model",
						Parts: []*Part{
							{Text: "Here is your image:"},
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
			UsageMetadata: &UsageMetadata{
				PromptTokenCount:     10,
				CandidatesTokenCount: 1300,
				TotalTokenCount:      1310,
			},
		}

		responseBody, err := json.Marshal(geminiResponse)
		require.NoError(t, err)

		httpResp := &httpclient.Response{
			StatusCode: http.StatusOK,
			Body:       responseBody,
		}

		// Step 1: Gemini -> LLM
		chatResp, err := outboundTransformer.TransformResponse(t.Context(), httpResp)
		require.NoError(t, err)

		// Step 2: LLM -> Gemini
		finalHttpResp, err := inboundTransformer.TransformResponse(t.Context(), chatResp)
		require.NoError(t, err)

		var finalGeminiResp GenerateContentResponse

		err = json.Unmarshal(finalHttpResp.Body, &finalGeminiResp)
		require.NoError(t, err)

		// Verify round-trip preserved the structure
		require.Equal(t, "resp-roundtrip", finalGeminiResp.ResponseID)
		require.Len(t, finalGeminiResp.Candidates, 1)
		require.Equal(t, "model", finalGeminiResp.Candidates[0].Content.Role)
		require.Len(t, finalGeminiResp.Candidates[0].Content.Parts, 2)

		// First part should be text
		require.Equal(t, "Here is your image:", finalGeminiResp.Candidates[0].Content.Parts[0].Text)

		// Second part should be inline data (image)
		require.NotNil(t, finalGeminiResp.Candidates[0].Content.Parts[1].InlineData)
		require.Equal(t, "image/png", finalGeminiResp.Candidates[0].Content.Parts[1].InlineData.MIMEType)
		require.Equal(
			t,
			"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			finalGeminiResp.Candidates[0].Content.Parts[1].InlineData.Data,
		)
	})
}

func TestModalitiesConversion(t *testing.T) {
	tests := []struct {
		name             string
		llmModalities    []string
		geminiModalities []string
	}{
		{
			name:             "text only",
			llmModalities:    []string{"text"},
			geminiModalities: []string{"TEXT"},
		},
		{
			name:             "text and image",
			llmModalities:    []string{"text", "image"},
			geminiModalities: []string{"TEXT", "IMAGE"},
		},
		{
			name:             "text, image and audio",
			llmModalities:    []string{"text", "image", "audio"},
			geminiModalities: []string{"TEXT", "IMAGE", "AUDIO"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test LLM -> Gemini conversion
			gotGemini := convertLLMModalitiesToGemini(tt.llmModalities)
			require.Equal(t, tt.geminiModalities, gotGemini)

			// Test Gemini -> LLM conversion
			gotLLM := convertGeminiModalitiesToLLM(tt.geminiModalities)
			require.Equal(t, tt.llmModalities, gotLLM)
		})
	}
}

func TestModalitiesRoundTrip(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer, _ := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")

	// Create a request with modalities
	geminiReq := GenerateContentRequest{
		Contents: []*Content{
			{
				Role: "user",
				Parts: []*Part{
					{Text: "Draw a green apple"},
				},
			},
		},
		GenerationConfig: &GenerationConfig{
			MaxOutputTokens:    1024,
			Temperature:        lo.ToPtr(1.0),
			TopP:               lo.ToPtr(1.0),
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}

	reqBody, err := json.Marshal(geminiReq)
	require.NoError(t, err)

	// Transform Gemini -> LLM
	httpReq := &httpclient.Request{
		Path: "/v1beta/models/gemini-2.5-flash-image-preview:generateContent",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: reqBody,
	}

	chatReq, err := inboundTransformer.TransformRequest(t.Context(), httpReq)
	require.NoError(t, err)
	require.Equal(t, []string{"text", "image"}, chatReq.Modalities)

	// Transform LLM -> Gemini
	outboundReq, err := outboundTransformer.TransformRequest(t.Context(), chatReq)
	require.NoError(t, err)

	var gotGeminiReq GenerateContentRequest

	err = json.Unmarshal(outboundReq.Body, &gotGeminiReq)
	require.NoError(t, err)

	// Verify modalities are preserved
	require.NotNil(t, gotGeminiReq.GenerationConfig)
	require.Equal(t, []string{"TEXT", "IMAGE"}, gotGeminiReq.GenerationConfig.ResponseModalities)
}
