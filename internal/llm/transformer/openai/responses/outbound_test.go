package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestNewOutboundTransformer(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		baseURL     string
		expectError bool
	}{
		{
			name:        "valid parameters",
			apiKey:      "test-api-key",
			baseURL:     "https://api.openai.com",
			expectError: false,
		},
		{
			name:        "empty api key",
			apiKey:      "",
			baseURL:     "https://api.openai.com",
			expectError: true,
		},
		{
			name:        "empty base url",
			apiKey:      "test-api-key",
			baseURL:     "",
			expectError: true,
		},
		{
			name:        "base url with trailing slash",
			apiKey:      "test-api-key",
			baseURL:     "https://api.openai.com/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformer(tt.baseURL, tt.apiKey)
			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, transformer)
			} else {
				require.NoError(t, err)
				require.NotNil(t, transformer)
				require.Equal(t, tt.apiKey, transformer.APIKey)
				// Base URL should have trailing slash removed
				expectedURL := tt.baseURL
				if expectedURL == "https://api.openai.com/" {
					expectedURL = "https://api.openai.com"
				}

				require.Equal(t, expectedURL, transformer.BaseURL)
			}
		})
	}
}

func TestOutboundTransformer_APIFormat(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.Equal(t, llm.APIFormatOpenAIResponse, transformer.APIFormat())
}

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name        string
		chatReq     *llm.Request
		expectError bool
		validate    func(t *testing.T, result *httpclient.Request, chatReq *llm.Request)
	}{
		{
			name:        "nil request",
			chatReq:     nil,
			expectError: true,
		},
		{
			name: "simple text request",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				require.Equal(t, http.MethodPost, result.Method)
				require.Equal(t, "https://api.openai.com/responses", result.URL)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.Equal(t, "application/json", result.Headers.Get("Accept"))
				require.NotNil(t, result.Auth)
				require.Equal(t, "bearer", result.Auth.Type)
				require.Equal(t, "test-api-key", result.Auth.APIKey)

				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Equal(t, chatReq.Model, req.Model)
				require.Equal(t, chatReq.Messages[0].Content.Content, req.Input.Text)
			},
		},
		{
			name: "request with system message",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: lo.ToPtr("You are a helpful assistant."),
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello!"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Equal(t, "You are a helpful assistant.", req.Instructions)
			},
		},
		{
			name: "request with multimodal content",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("What's in this image?"),
								},
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/jpeg;base64,/9j/4AAQSkZJRg...",
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with image generation tool",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate an image of a cat"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: llm.ToolTypeImageGeneration,
						ImageGeneration: &llm.ImageGeneration{
							Quality:           "high",
							Size:              "1024x1024",
							OutputFormat:      "png",
							OutputCompression: func() *int64 { v := int64(80); return &v }(),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.Len(t, req.Tools, 1)
				require.Equal(t, llm.ToolTypeImageGeneration, req.Tools[0].Type)
				require.Equal(t, "high", req.Tools[0].Quality)
				require.Equal(t, "1024x1024", req.Tools[0].Size)
				require.Equal(t, "png", req.Tools[0].OutputFormat)
				require.Equal(t, int64(80), *req.Tools[0].OutputCompression)
			},
		},
		{
			name: "request with unsupported tool type",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "unsupported_tool",
					},
				},
			},
			expectError: true,
		},
		{
			name: "request with streaming enabled",
			chatReq: &llm.Request{
				Model:  "gpt-4o",
				Stream: func() *bool { v := true; return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Stream)
				require.True(t, *req.Stream)
			},
		},
		{
			name: "request with parallel tool calls",
			chatReq: &llm.Request{
				Model:             "gpt-4o",
				ParallelToolCalls: lo.ToPtr(false),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.ParallelToolCalls)
				require.False(t, *req.ParallelToolCalls)
			},
		},
		{
			name: "request with text options",
			chatReq: &llm.Request{
				Model: "gpt-4o",
				ResponseFormat: &llm.ResponseFormat{
					Type: "json_object",
				},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Return JSON"; return &s }(),
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Request, chatReq *llm.Request) {
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				require.NotNil(t, req.Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformRequest(context.Background(), tt.chatReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result, tt.chatReq)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name        string
		httpResp    *httpclient.Response
		expectError bool
		validate    func(t *testing.T, result *llm.Response)
	}{
		{
			name:        "nil response",
			httpResp:    nil,
			expectError: true,
		},
		{
			name: "HTTP error status",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusBadRequest,
				Body:       []byte(`{"error": {"message": "Bad request"}}`),
			},
			expectError: true,
		},
		{
			name: "empty response body",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte{},
			},
			expectError: true,
		},
		{
			name: "invalid JSON response",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(`{invalid json}`),
			},
			expectError: true,
		},
		{
			name: "valid response with text output",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "resp_123",
					"object": "response",
					"created_at": 1759161016,
					"status": "completed",
					"model": "gpt-4o",
					"output": [
						{
							"id": "msg_123",
							"type": "message",
							"status": "completed",
							"content": [
								{
									"type": "output_text",
									"text": "Hello! How can I help you?"
								}
							],
							"role": "assistant"
						}
					],
					"usage": {
						"input_tokens": 10,
						"output_tokens": 20,
						"total_tokens": 30
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.Equal(t, "resp_123", result.ID)
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
				require.NotNil(t, result.Choices[0].Message.Content.Content)
				require.Equal(t, "Hello! How can I help you?", *result.Choices[0].Message.Content.Content)
				require.NotNil(t, result.Usage)
				require.Equal(t, int64(10), result.Usage.PromptTokens)
				require.Equal(t, int64(20), result.Usage.CompletionTokens)
				require.Equal(t, int64(30), result.Usage.TotalTokens)
			},
		},
		{
			name: "response with image generation result",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "resp_456",
					"object": "response",
					"created_at": 1759161016,
					"status": "completed",
					"model": "gpt-4o",
					"output": [
						{
							"id": "img_123",
							"type": "image_generation_call",
							"status": "completed",
							"result": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.Equal(t, "resp_456", result.ID)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
				require.Len(t, result.Choices[0].Message.Content.MultipleContent, 1)
				require.Equal(t, "image_url", result.Choices[0].Message.Content.MultipleContent[0].Type)
				require.NotNil(t, result.Choices[0].Message.Content.MultipleContent[0].ImageURL)
				require.Contains(t, result.Choices[0].Message.Content.MultipleContent[0].ImageURL.URL, "data:image/png;base64,")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(context.Background(), tt.httpResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformStreamChunk(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name        string
		event       *httpclient.StreamEvent
		expectError bool
		validate    func(t *testing.T, result *llm.Response)
	}{
		{
			name:        "nil event",
			event:       nil,
			expectError: true,
		},
		{
			name: "empty event data",
			event: &httpclient.StreamEvent{
				Data: []byte{},
			},
			expectError: true,
		},
		{
			name: "image generation partial event",
			event: &httpclient.StreamEvent{
				Data: []byte(`{
					"type": "response.image_generation_call.partial_image",
					"image_url": {
						"url": "data:image/png;base64,partial_image_data"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Delta.Role)
				require.Len(t, result.Choices[0].Delta.Content.MultipleContent, 1)
				require.Equal(t, "image_url", result.Choices[0].Delta.Content.MultipleContent[0].Type)
				require.Equal(t, "data:image/png;base64,partial_image_data", result.Choices[0].Delta.Content.MultipleContent[0].ImageURL.URL)
			},
		},
		{
			name: "image generation completed event",
			event: &httpclient.StreamEvent{
				Data: []byte(`{
					"type": "response.image_generation_call.completed",
					"result": "data:image/png;base64,completed_image_data"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Delta.Role)
				require.Len(t, result.Choices[0].Delta.Content.MultipleContent, 1)
				require.Equal(t, "image_url", result.Choices[0].Delta.Content.MultipleContent[0].Type)
				require.Equal(t, "data:image/png;base64,completed_image_data", result.Choices[0].Delta.Content.MultipleContent[0].ImageURL.URL)
			},
		},
		{
			name: "non-image event falls back to TransformResponse",
			event: &httpclient.StreamEvent{
				Data: []byte(`{
					"id": "resp_789",
					"object": "response",
					"created_at": 1759161016,
					"status": "completed",
					"model": "gpt-4o",
					"output": [
						{
							"id": "msg_789",
							"type": "message",
							"status": "completed",
							"content": [
								{
									"type": "output_text",
									"text": "Streaming response"
								}
							],
							"role": "assistant"
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.Equal(t, "resp_789", result.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformStreamChunk(context.Background(), tt.event)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest_WithTestData(t *testing.T) {
	tests := []struct {
		name        string
		requestFile string
		validate    func(t *testing.T, result *httpclient.Request, expectedReq *llm.Request)
	}{
		{
			name:        "image generation request transformation",
			requestFile: "image-generation.request.json",
			validate: func(t *testing.T, result *httpclient.Request, expectedReq *llm.Request) {
				// Verify basic HTTP request properties
				require.Equal(t, http.MethodPost, result.Method)
				require.Equal(t, "https://api.openai.com/responses", result.URL)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.Equal(t, "application/json", result.Headers.Get("Accept"))
				require.NotEmpty(t, result.Body)

				// Verify auth
				require.NotNil(t, result.Auth)
				require.Equal(t, "bearer", result.Auth.Type)
				require.Equal(t, "test-api-key", result.Auth.APIKey)

				// Parse the transformed request
				var req Request

				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)

				// Verify model
				require.Equal(t, expectedReq.Model, req.Model)

				// Verify tools transformation
				if len(expectedReq.Tools) > 0 {
					require.NotNil(t, req.Tools)
					require.Len(t, req.Tools, len(expectedReq.Tools))

					for i, tool := range expectedReq.Tools {
						require.Equal(t, tool.Type, req.Tools[i].Type)

						if tool.ImageGeneration != nil {
							require.Equal(t, tool.ImageGeneration.Quality, req.Tools[i].Quality)
							require.Equal(t, tool.ImageGeneration.Size, req.Tools[i].Size)
							require.Equal(t, tool.ImageGeneration.OutputFormat, req.Tools[i].OutputFormat)
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the test request data
			var expectedReq llm.Request

			err := xtest.LoadTestData(t, tt.requestFile, &expectedReq)
			if err != nil {
				t.Skipf("Test data file %s not found, skipping test", tt.requestFile)
				return
			}

			// Create transformer
			transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)

			// Transform the request
			result, err := transformer.TransformRequest(context.Background(), &expectedReq)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Run validation
			tt.validate(t, result, &expectedReq)
		})
	}
}

// loadTestDataRaw loads raw test data from a file in testdata directory.
func loadTestDataRaw(t *testing.T, filename string) ([]byte, error) {
	t.Helper()

	// Try to read from testdata directory
	testdataPath := "testdata/" + filename

	data, err := os.ReadFile(testdataPath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func TestOutboundTransformer_TransformResponse_WithTestData(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name         string
		responseFile string
		validate     func(t *testing.T, result *llm.Response)
	}{
		{
			name:         "stop response transformation",
			responseFile: "stop.response.json",
			validate: func(t *testing.T, result *llm.Response) {
				require.Equal(t, "chat.completion", result.Object)
				require.NotEmpty(t, result.ID)
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
				require.NotNil(t, result.Choices[0].Message.Content.Content)
				require.Contains(t, *result.Choices[0].Message.Content.Content, "weather")
				require.NotNil(t, result.Usage)
				require.Greater(t, result.Usage.TotalTokens, int64(0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the test response data
			responseData, err := loadTestDataRaw(t, tt.responseFile)
			if err != nil {
				t.Skipf("Test data file %s not found, skipping test", tt.responseFile)
				return
			}

			// Create HTTP response
			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       responseData,
			}

			// Transform the response
			result, err := transformer.TransformResponse(context.Background(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Run validation
			tt.validate(t, result)
		})
	}
}

func TestOutboundTransformer_TransformStreamChunk_WithTestData(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.openai.com", "test-api-key")

	tests := []struct {
		name       string
		streamFile string
		validate   func(t *testing.T, events []*httpclient.StreamEvent, results []*llm.Response)
	}{
		{
			name:       "stop response stream transformation",
			streamFile: "stop.response.stream.jsonl",
			validate: func(t *testing.T, events []*httpclient.StreamEvent, results []*llm.Response) {
				require.Greater(t, len(events), 0, "Should have stream events")
				require.Greater(t, len(results), 0, "Should have transformed results")

				// Check that we have various event types
				var hasTextDelta, hasResponseCompleted bool

				for _, event := range events {
					eventType := gjson.GetBytes(event.Data, "type").String()
					switch eventType {
					case "response.output_text.delta":
						hasTextDelta = true
					case "response.completed":
						hasResponseCompleted = true
					}
				}

				require.True(t, hasTextDelta, "Should have text delta events")
				require.True(t, hasResponseCompleted, "Should have response completed event")

				// Check final result structure
				finalResult := results[len(results)-1]
				if finalResult != nil {
					require.Equal(t, "chat.completion", finalResult.Object)

					if len(finalResult.Choices) > 0 {
						require.Equal(t, "assistant", finalResult.Choices[0].Message.Role)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load stream events
			events, err := xtest.LoadStreamChunks(t, tt.streamFile)
			if err != nil {
				t.Skipf("Test data file %s not found, skipping test: %v", tt.streamFile, err)
				return
			}

			// Transform each event
			var results []*llm.Response

			for _, event := range events {
				result, err := transformer.TransformStreamChunk(context.Background(), event)
				if err == nil && result != nil {
					results = append(results, result)
				}
			}

			// Run validation
			tt.validate(t, events, results)
		})
	}
}
