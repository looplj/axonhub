package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestNewInboundTransformer(t *testing.T) {
	transformer := NewInboundTransformer()
	require.NotNil(t, transformer)
}

func TestInboundTransformer_APIFormat(t *testing.T) {
	transformer := NewInboundTransformer()
	require.Equal(t, llm.APIFormatOpenAIResponse, transformer.APIFormat())
}

func TestInboundTransformer_TransformRequest(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		httpReq     *httpclient.Request
		expectError bool
		validate    func(t *testing.T, result *llm.Request)
	}{
		{
			name:        "nil request",
			httpReq:     nil,
			expectError: true,
		},
		{
			name: "empty body",
			httpReq: &httpclient.Request{
				Body: []byte{},
			},
			expectError: true,
		},
		{
			name: "invalid JSON",
			httpReq: &httpclient.Request{
				Body: []byte(`{invalid json}`),
			},
			expectError: true,
		},
		{
			name: "missing model",
			httpReq: &httpclient.Request{
				Body: []byte(`{"input": "Hello"}`),
			},
			expectError: true,
		},
		{
			name: "simple text input",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello, world!"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Messages, 1)
				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(t, "Hello, world!", *result.Messages[0].Content.Content)
			},
		},
		{
			name: "request with instructions",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"instructions": "You are a helpful assistant.",
					"input": "Hello!"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "gpt-4o", result.Model)
				require.Len(t, result.Messages, 2)
				require.Equal(t, "system", result.Messages[0].Role)
				require.Equal(t, "You are a helpful assistant.", *result.Messages[0].Content.Content)
				require.Equal(t, "user", result.Messages[1].Role)
				require.Equal(t, "Hello!", *result.Messages[1].Content.Content)
			},
		},
		{
			name: "request with temperature and top_p",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"temperature": 0.7,
					"top_p": 0.9
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "gpt-4o", result.Model)
				require.NotNil(t, result.Temperature)
				require.Equal(t, 0.7, *result.Temperature)
				require.NotNil(t, result.TopP)
				require.Equal(t, 0.9, *result.TopP)
			},
		},
		{
			name: "request with max_output_tokens",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"max_output_tokens": 1000
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.MaxCompletionTokens)
				require.Equal(t, int64(1000), *result.MaxCompletionTokens)
			},
		},
		{
			name: "request with function tools",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "What's the weather?",
					"tools": [
						{
							"type": "function",
							"name": "get_weather",
							"description": "Get weather information",
							"parameters": {
								"type": "object",
								"properties": {
									"location": {"type": "string"}
								}
							}
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Len(t, result.Tools, 1)
				require.Equal(t, "function", result.Tools[0].Type)
				require.Equal(t, "get_weather", result.Tools[0].Function.Name)
				require.Equal(t, "Get weather information", result.Tools[0].Function.Description)
			},
		},
		{
			name: "request with image generation tool",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Generate an image of a cat",
					"tools": [
						{
							"type": "image_generation",
							"quality": "high",
							"size": "1024x1024"
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Len(t, result.Tools, 1)
				require.Equal(t, llm.ToolTypeImageGeneration, result.Tools[0].Type)
				require.NotNil(t, result.Tools[0].ImageGeneration)
				require.Equal(t, "high", result.Tools[0].ImageGeneration.Quality)
				require.Equal(t, "1024x1024", result.Tools[0].ImageGeneration.Size)
			},
		},
		{
			name: "request with reasoning",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": "Solve this problem",
					"reasoning": {
						"effort": "high",
						"max_tokens": 5000
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "high", result.ReasoningEffort)
				require.NotNil(t, result.ReasoningBudget)
				require.Equal(t, int64(5000), *result.ReasoningBudget)
			},
		},
		{
			name: "request with tool choice mode",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"tool_choice": {
						"mode": "auto"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.ToolChoice)
				require.NotNil(t, result.ToolChoice.ToolChoice)
				require.Equal(t, "auto", *result.ToolChoice.ToolChoice)
			},
		},
		{
			name: "request with specific tool choice",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"tool_choice": {
						"type": "function",
						"name": "get_weather"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.ToolChoice)
				require.NotNil(t, result.ToolChoice.NamedToolChoice)
				require.Equal(t, "function", result.ToolChoice.NamedToolChoice.Type)
				require.Equal(t, "get_weather", result.ToolChoice.NamedToolChoice.Function.Name)
			},
		},
		{
			name: "request with metadata",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"metadata": {
						"user_id": "123",
						"session_id": "abc"
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.Metadata)
				require.Equal(t, "123", result.Metadata["user_id"])
				require.Equal(t, "abc", result.Metadata["session_id"])
			},
		},
		{
			name: "request with store and service_tier",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"store": true,
					"service_tier": "default"
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.Store)
				require.True(t, *result.Store)
				require.NotNil(t, result.ServiceTier)
				require.Equal(t, "default", *result.ServiceTier)
			},
		},
		{
			name: "request with text format",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Return JSON",
					"text": {
						"format": {
							"type": "json_object"
						}
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.ResponseFormat)
				require.Equal(t, "json_object", result.ResponseFormat.Type)
			},
		},
		{
			name: "request with stream options",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"stream": true,
					"stream_options": {
						"include_obfuscation": true
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.Stream)
				require.True(t, *result.Stream)
				require.NotNil(t, result.StreamOptions)
			},
		},
		{
			name: "request with top_logprobs",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "gpt-4o",
					"input": "Hello",
					"top_logprobs": 5
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.NotNil(t, result.TopLogprobs)
				require.Equal(t, int64(5), *result.TopLogprobs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformRequest(context.Background(), tt.httpReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, llm.APIFormatOpenAIResponse, result.RawAPIFormat)

				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformResponse(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.Response
		expectError bool
		validate    func(t *testing.T, result *httpclient.Response)
	}{
		{
			name:        "nil response",
			chatResp:    nil,
			expectError: true,
		},
		{
			name: "simple text response",
			chatResp: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello! How can I help you?"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				require.Equal(t, http.StatusOK, result.StatusCode)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))

				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.Equal(t, "response", resp.Object)
				require.Equal(t, "chatcmpl-123", resp.ID)
				require.Equal(t, "gpt-4o", resp.Model)
				require.NotNil(t, resp.Status)
				require.Equal(t, "completed", *resp.Status)
				require.Len(t, resp.Output, 1)
				output := resp.Output[0]
				require.Equal(t, "message", output.Type)
				require.Equal(t, "assistant", output.Role)
			},
		},
		{
			name: "response with tool calls",
			chatResp: &llm.Response{
				ID:      "chatcmpl-456",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "get_weather",
										Arguments: `{"location": "San Francisco"}`,
									},
								},
							},
						},
						FinishReason: lo.ToPtr("tool_calls"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				require.Equal(t, http.StatusOK, result.StatusCode)

				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Status)
				require.Equal(t, "completed", *resp.Status)
			},
		},
		{
			name: "response with usage details",
			chatResp: &llm.Response{
				ID:      "chatcmpl-789",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Response with usage"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
					PromptTokensDetails: &llm.PromptTokensDetails{
						CachedTokens: 20,
					},
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						ReasoningTokens: 10,
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(100), resp.Usage.InputTokens)
				require.Equal(t, int64(50), resp.Usage.OutputTokens)
				require.Equal(t, int64(150), resp.Usage.TotalTokens)
				require.Equal(t, int64(20), resp.Usage.InputTokenDetails.CachedTokens)
				require.Equal(t, int64(10), resp.Usage.OutputTokenDetails.ReasoningTokens)
			},
		},
		{
			name: "response with length finish reason",
			chatResp: &llm.Response{
				ID:      "chatcmpl-length",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4o",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Truncated response..."),
							},
						},
						FinishReason: lo.ToPtr("length"),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.NotNil(t, resp.Status)
				require.Equal(t, "incomplete", *resp.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformResponse(context.Background(), tt.chatResp)

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

func TestInboundTransformer_TransformError(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name     string
		err      error
		validate func(t *testing.T, result *httpclient.Error)
	}{
		{
			name: "nil error",
			err:  nil,
			validate: func(t *testing.T, result *httpclient.Error) {
				require.Equal(t, http.StatusInternalServerError, result.StatusCode)
			},
		},
		{
			name: "invalid request error",
			err:  transformer.ErrInvalidRequest,
			validate: func(t *testing.T, result *httpclient.Error) {
				require.Equal(t, http.StatusBadRequest, result.StatusCode)
				require.Contains(t, string(result.Body), "invalid_request_error")
			},
		},
		{
			name: "invalid model error",
			err:  transformer.ErrInvalidModel,
			validate: func(t *testing.T, result *httpclient.Error) {
				require.Equal(t, http.StatusUnprocessableEntity, result.StatusCode)
				require.Contains(t, string(result.Body), "invalid_model_error")
			},
		},
		{
			name: "llm response error",
			err: &llm.ResponseError{
				StatusCode: http.StatusTooManyRequests,
				Detail: llm.ErrorDetail{
					Message: "Rate limit exceeded",
					Type:    "rate_limit_error",
					Code:    "rate_limit",
				},
			},
			validate: func(t *testing.T, result *httpclient.Error) {
				require.Equal(t, http.StatusTooManyRequests, result.StatusCode)
				require.Contains(t, string(result.Body), "Rate limit exceeded")
				require.Contains(t, string(result.Body), "rate_limit_error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trans.TransformError(context.Background(), tt.err)
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestConvertToolChoiceToLLM(t *testing.T) {
	tests := []struct {
		name     string
		input    *ToolChoice
		validate func(t *testing.T, result *llm.ToolChoice)
	}{
		{
			name:  "nil input",
			input: nil,
			validate: func(t *testing.T, result *llm.ToolChoice) {
				require.Nil(t, result)
			},
		},
		{
			name: "mode only",
			input: &ToolChoice{
				Mode: lo.ToPtr("auto"),
			},
			validate: func(t *testing.T, result *llm.ToolChoice) {
				require.NotNil(t, result)
				require.NotNil(t, result.ToolChoice)
				require.Equal(t, "auto", *result.ToolChoice)
				require.Nil(t, result.NamedToolChoice)
			},
		},
		{
			name: "specific function",
			input: &ToolChoice{
				Type: lo.ToPtr("function"),
				Name: lo.ToPtr("get_weather"),
			},
			validate: func(t *testing.T, result *llm.ToolChoice) {
				require.NotNil(t, result)
				require.Nil(t, result.ToolChoice)
				require.NotNil(t, result.NamedToolChoice)
				require.Equal(t, "function", result.NamedToolChoice.Type)
				require.Equal(t, "get_weather", result.NamedToolChoice.Function.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToolChoiceToLLM(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertToMessageContentParts(t *testing.T) {
	tests := []struct {
		name     string
		input    Input
		validate func(t *testing.T, result []llm.MessageContentPart)
	}{
		{
			name:  "text input returns one part",
			input: Input{Text: lo.ToPtr("Hello world")},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "input_text", result[0].Type)
				require.Equal(t, "Hello world", *result[0].Text)
			},
		},
		{
			name:  "single input_text item returns one part",
			input: Input{Items: []Item{{Type: "input_text", Text: lo.ToPtr("Hello world")}}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "Hello world", *result[0].Text)
			},
		},
		{
			name:  "single text item returns one part",
			input: Input{Items: []Item{{Type: "text", Text: lo.ToPtr("Hello world")}}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "Hello world", *result[0].Text)
			},
		},
		{
			name: "multiple items returns multiple parts",
			input: Input{Items: []Item{
				{Type: "input_text", Text: lo.ToPtr("First")},
				{Type: "input_text", Text: lo.ToPtr("Second")},
			}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 2)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "First", *result[0].Text)
				require.Equal(t, "text", result[1].Type)
				require.Equal(t, "Second", *result[1].Text)
			},
		},
		{
			name: "single input_image returns one part",
			input: Input{Items: []Item{
				{Type: "input_image", ImageURL: lo.ToPtr("https://example.com/image.png")},
			}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "image_url", result[0].Type)
				require.NotNil(t, result[0].ImageURL)
				require.Equal(t, "https://example.com/image.png", result[0].ImageURL.URL)
			},
		},
		{
			name: "mixed text and image returns multiple parts",
			input: Input{Items: []Item{
				{Type: "input_text", Text: lo.ToPtr("Look at this image:")},
				{Type: "input_image", ImageURL: lo.ToPtr("https://example.com/image.png")},
			}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 2)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "Look at this image:", *result[0].Text)
				require.Equal(t, "image_url", result[1].Type)
				require.NotNil(t, result[1].ImageURL)
				require.Equal(t, "https://example.com/image.png", result[1].ImageURL.URL)
			},
		},
		{
			name:  "empty items returns empty slice",
			input: Input{Items: []Item{}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Empty(t, result)
			},
		},
		{
			name: "output_text item returns one part",
			input: Input{Items: []Item{
				{Type: "output_text", Text: lo.ToPtr("Generated text")},
			}},
			validate: func(t *testing.T, result []llm.MessageContentPart) {
				require.Len(t, result, 1)
				require.Equal(t, "text", result[0].Type)
				require.Equal(t, "Generated text", *result[0].Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToMessageContentParts(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertToMessageContent(t *testing.T) {
	tests := []struct {
		name     string
		input    Input
		validate func(t *testing.T, result llm.MessageContent)
	}{
		{
			name:  "text input returns simple Content",
			input: Input{Text: lo.ToPtr("Hello world")},
			validate: func(t *testing.T, result llm.MessageContent) {
				require.NotNil(t, result.Content)
				require.Equal(t, "Hello world", *result.Content)
				require.Nil(t, result.MultipleContent)
			},
		},
		{
			name:  "single input_text item returns simple Content",
			input: Input{Items: []Item{{Type: "input_text", Text: lo.ToPtr("Hello world")}}},
			validate: func(t *testing.T, result llm.MessageContent) {
				require.NotNil(t, result.Content)
				require.Equal(t, "Hello world", *result.Content)
				require.Nil(t, result.MultipleContent)
			},
		},
		{
			name: "multiple items returns MultipleContent",
			input: Input{Items: []Item{
				{Type: "input_text", Text: lo.ToPtr("First")},
				{Type: "input_text", Text: lo.ToPtr("Second")},
			}},
			validate: func(t *testing.T, result llm.MessageContent) {
				require.Nil(t, result.Content)
				require.Len(t, result.MultipleContent, 2)
				require.Equal(t, "text", result.MultipleContent[0].Type)
				require.Equal(t, "First", *result.MultipleContent[0].Text)
			},
		},
		{
			name:  "single input_image returns MultipleContent",
			input: Input{Items: []Item{{Type: "input_image", ImageURL: lo.ToPtr("https://example.com/image.png")}}},
			validate: func(t *testing.T, result llm.MessageContent) {
				require.Nil(t, result.Content)
				require.Len(t, result.MultipleContent, 1)
				require.Equal(t, "image_url", result.MultipleContent[0].Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToMessageContent(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertItemToMessage_Reasoning(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		validate func(t *testing.T, result *llm.Message, err error)
	}{
		{
			name: "reasoning item with summary",
			item: &Item{
				ID:   "reasoning_123",
				Type: "reasoning",
				Summary: []ReasoningSummary{
					{Type: "summary_text", Text: "First, I need to analyze the problem."},
					{Type: "summary_text", Text: " Then, I will solve it step by step."},
				},
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "assistant", result.Role)
				require.NotNil(t, result.ReasoningContent)
				require.Equal(t, "First, I need to analyze the problem. Then, I will solve it step by step.", *result.ReasoningContent)
			},
		},
		{
			name: "reasoning item with encrypted content",
			item: &Item{
				ID:   "reasoning_456",
				Type: "reasoning",
				Summary: []ReasoningSummary{
					{Type: "summary_text", Text: "Reasoning summary"},
				},
				EncryptedContent: lo.ToPtr("encrypted_data_here"),
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "assistant", result.Role)
				require.NotNil(t, result.ReasoningContent)
				require.Equal(t, "Reasoning summary", *result.ReasoningContent)
				require.NotNil(t, result.ReasoningSignature)
				require.Equal(t, "encrypted_data_here", *result.ReasoningSignature)
			},
		},
		{
			name: "reasoning item with empty summary",
			item: &Item{
				ID:      "reasoning_789",
				Type:    "reasoning",
				Summary: []ReasoningSummary{},
			},
			validate: func(t *testing.T, result *llm.Message, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "assistant", result.Role)
				require.Nil(t, result.ReasoningContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertItemToMessage(tt.item)
			tt.validate(t, result, err)
		})
	}
}

func TestInboundTransformer_TransformRequest_WithReasoningInput(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		httpReq     *httpclient.Request
		expectError bool
		validate    func(t *testing.T, result *llm.Request)
	}{
		{
			name: "request with reasoning input item",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": [
						{
							"type": "message",
							"role": "user",
							"content": "What is 2+2?"
						},
						{
							"type": "reasoning",
							"id": "reasoning_abc",
							"summary": [
								{"type": "summary_text", "text": "Let me think about this math problem."}
							]
						},
						{
							"type": "message",
							"role": "assistant",
							"content": "The answer is 4."
						}
					]
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "o3", result.Model)
				require.Len(t, result.Messages, 3)

				// First message: user
				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(t, "What is 2+2?", *result.Messages[0].Content.Content)

				// Second message: reasoning (converted to assistant with ReasoningContent)
				require.Equal(t, "assistant", result.Messages[1].Role)
				require.NotNil(t, result.Messages[1].ReasoningContent)
				require.Equal(t, "Let me think about this math problem.", *result.Messages[1].ReasoningContent)

				// Third message: assistant
				require.Equal(t, "assistant", result.Messages[2].Role)
				require.Equal(t, "The answer is 4.", *result.Messages[2].Content.Content)
			},
		},
		{
			name: "request with reasoning config",
			httpReq: &httpclient.Request{
				Body: []byte(`{
					"model": "o3",
					"input": "Solve this complex problem",
					"reasoning": {
						"effort": "high",
						"summary": "detailed",
						"max_tokens": 10000
					}
				}`),
			},
			expectError: false,
			validate: func(t *testing.T, result *llm.Request) {
				require.Equal(t, "o3", result.Model)
				require.Equal(t, "high", result.ReasoningEffort)
				require.NotNil(t, result.ReasoningBudget)
				require.Equal(t, int64(10000), *result.ReasoningBudget)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformRequest(context.Background(), tt.httpReq)

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

func TestInboundTransformer_TransformResponse_WithReasoning(t *testing.T) {
	trans := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.Response
		expectError bool
		validate    func(t *testing.T, result *httpclient.Response)
	}{
		{
			name: "response with reasoning content",
			chatResp: &llm.Response{
				ID:      "chatcmpl-reasoning",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "o3",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role:             "assistant",
							ReasoningContent: lo.ToPtr("I analyzed the problem step by step."),
							Content: llm.MessageContent{
								Content: lo.ToPtr("The answer is 42."),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     50,
					CompletionTokens: 100,
					TotalTokens:      150,
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						ReasoningTokens: 80,
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, result *httpclient.Response) {
				require.Equal(t, http.StatusOK, result.StatusCode)

				var resp Response

				err := json.Unmarshal(result.Body, &resp)
				require.NoError(t, err)
				require.Equal(t, "response", resp.Object)
				require.Equal(t, "o3", resp.Model)

				// Should have reasoning output item and message output item
				require.Len(t, resp.Output, 2)

				// First output should be reasoning
				reasoningOutput := resp.Output[0]
				require.Equal(t, "reasoning", reasoningOutput.Type)
				require.Len(t, reasoningOutput.Summary, 1)
				require.Equal(t, "summary_text", reasoningOutput.Summary[0].Type)
				require.Equal(t, "I analyzed the problem step by step.", reasoningOutput.Summary[0].Text)

				// Second output should be message
				messageOutput := resp.Output[1]
				require.Equal(t, "message", messageOutput.Type)
				require.Equal(t, "assistant", messageOutput.Role)

				// Check usage includes reasoning tokens
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(80), resp.Usage.OutputTokenDetails.ReasoningTokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trans.TransformResponse(context.Background(), tt.chatResp)

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
