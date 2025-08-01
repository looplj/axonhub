package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/internal/llm"
)

func TestAnthropicTransformers_Integration(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	tests := []struct {
		name                 string
		anthropicRequestJSON string
		expectedModel        string
		expectedMaxTokens    int64
	}{
		{
			name: "simple text message",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": "Hello, Claude!"
					}
				]
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 1024,
		},
		{
			name: "message with system prompt",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 2048,
				"system": "You are a helpful assistant.",
				"messages": [
					{
						"role": "user",
						"content": "What is the capital of France?"
					}
				],
				"temperature": 0.7
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 2048,
		},
		{
			name: "multimodal message",
			anthropicRequestJSON: `{
				"model": "claude-3-sonnet-20240229",
				"max_tokens": 1024,
				"messages": [
					{
						"role": "user",
						"content": [
							{
								"type": "text",
								"text": "What's in this image?"
							},
							{
								"type": "image",
								"source": {
									"type": "base64",
									"media_type": "image/jpeg",
									"data": "/9j/4AAQSkZJRg..."
								}
							}
						]
					}
				]
			}`,
			expectedModel:     "claude-3-sonnet-20240229",
			expectedMaxTokens: 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Transform Anthropic request to ChatCompletionRequest
			httpReq := &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(tt.anthropicRequestJSON),
			}

			chatReq, err := inboundTransformer.TransformRequest(context.Background(), httpReq)
			require.NoError(t, err)
			require.NotNil(t, chatReq)

			// Verify the transformation
			assert.Equal(t, tt.expectedModel, chatReq.Model)
			assert.Equal(t, tt.expectedMaxTokens, *chatReq.MaxTokens)
			assert.NotEmpty(t, chatReq.Messages)

			// Step 2: Transform ChatCompletionRequest to Anthropic outbound request
			outboundReq, err := outboundTransformer.TransformRequest(context.Background(), chatReq)
			require.NoError(t, err)
			require.NotNil(t, outboundReq)

			// Verify outbound request
			assert.Equal(t, http.MethodPost, outboundReq.Method)
			assert.Equal(t, "https://api.anthropic.com/v1/messages", outboundReq.URL)
			assert.Equal(t, "application/json", outboundReq.Headers.Get("Content-Type"))
			assert.Equal(t, "2023-06-01", outboundReq.Headers.Get("anthropic-version"))

			// Verify the outbound request body can be unmarshaled
			var anthropicReq MessageRequest
			err = json.Unmarshal(outboundReq.Body, &anthropicReq)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedModel, anthropicReq.Model)
			assert.Equal(t, tt.expectedMaxTokens, anthropicReq.MaxTokens)

			// Step 3: Simulate Anthropic response and transform back
			anthropicResponse := &MessageResponse{
				ID:   "msg_test_123",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{
						Type: "text",
						Text: func() *string { s := "This is a test response from Claude."; return &s }(),
					},
				},
				Model:      tt.expectedModel,
				StopReason: func() *string { s := "end_turn"; return &s }(),
				Usage: &Usage{
					InputTokens:  15,
					OutputTokens: 25,
				},
			}

			responseBody, err := json.Marshal(anthropicResponse)
			require.NoError(t, err)

			httpResp := &llm.GenericHttpResponse{
				StatusCode: http.StatusOK,
				Body:       responseBody,
			}

			// Step 4: Transform Anthropic response to ChatCompletionResponse
			chatResp, err := outboundTransformer.TransformResponse(context.Background(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, chatResp)

			// Verify chat response
			assert.Equal(t, "msg_test_123", chatResp.ID)
			assert.Equal(t, "chat.completion", chatResp.Object)
			assert.Equal(t, tt.expectedModel, chatResp.Model)
			assert.Equal(t, 1, len(chatResp.Choices))
			assert.Equal(t, "assistant", chatResp.Choices[0].Message.Role)
			assert.Equal(t, "This is a test response from Claude.", *chatResp.Choices[0].Message.Content.Content)
			assert.Equal(t, "stop", *chatResp.Choices[0].FinishReason)

			// Step 5: Transform ChatCompletionResponse back to Anthropic format
			finalHttpResp, err := inboundTransformer.TransformResponse(context.Background(), chatResp)
			require.NoError(t, err)
			require.NotNil(t, finalHttpResp)

			// Verify final response
			assert.Equal(t, http.StatusOK, finalHttpResp.StatusCode)
			assert.Equal(t, "application/json", finalHttpResp.Headers.Get("Content-Type"))

			var finalAnthropicResp MessageResponse
			err = json.Unmarshal(finalHttpResp.Body, &finalAnthropicResp)
			require.NoError(t, err)
			assert.Equal(t, "msg_test_123", finalAnthropicResp.ID)
			assert.Equal(t, "message", finalAnthropicResp.Type)
			assert.Equal(t, "assistant", finalAnthropicResp.Role)
			assert.Equal(t, tt.expectedModel, finalAnthropicResp.Model)
		})
	}
}

func TestAnthropicTransformers_StreamingIntegration(t *testing.T) {
	outboundTransformer := NewOutboundTransformer("", "")

	// Simulate streaming chunks from Anthropic
	chunks := [][]byte{
		[]byte(`{
			"type": "message_start",
			"message": {
				"id": "msg_stream_123",
				"type": "message",
				"role": "assistant",
				"content": [],
				"model": "claude-3-sonnet-20240229",
				"stop_reason": null,
				"stop_sequence": null,
				"usage": {"input_tokens": 10, "output_tokens": 0}
			}
		}`),
		[]byte(`{
			"type": "content_block_start",
			"index": 0,
			"content_block": {
				"type": "text",
				"text": ""
			}
		}`),
		[]byte(`{
			"type": "content_block_delta",
			"index": 0,
			"delta": {
				"type": "text_delta",
				"text": "Hello"
			}
		}`),
		[]byte(`{
			"type": "content_block_delta",
			"index": 0,
			"delta": {
				"type": "text_delta",
				"text": ", this is"
			}
		}`),
		[]byte(`{
			"type": "content_block_delta",
			"index": 0,
			"delta": {
				"type": "text_delta",
				"text": " a streaming response!"
			}
		}`),
		[]byte(`{
			"type": "content_block_stop",
			"index": 0
		}`),
		[]byte(`{
			"type": "message_delta",
			"delta": {
				"stop_reason": "end_turn",
				"stop_sequence": null
			},
			"usage": {"input_tokens": 10, "output_tokens": 25}
		}`),
		[]byte(`{
			"type": "message_stop"
		}`),
	}

	// Aggregate the streaming chunks
	chatResp, err := outboundTransformer.AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)
	require.NotNil(t, chatResp)

	// Verify the aggregated response
	assert.Equal(t, "msg_stream_123", chatResp.ID)
	assert.Equal(t, "chat.completion", chatResp.Object)
	assert.Equal(t, 1, len(chatResp.Choices))
	assert.Equal(t, "assistant", chatResp.Choices[0].Message.Role)
	assert.Equal(t, "Hello, this is a streaming response!", *chatResp.Choices[0].Message.Content.Content)
	assert.Equal(t, "stop", *chatResp.Choices[0].FinishReason)

	// Verify usage
	require.NotNil(t, chatResp.Usage)
	assert.Equal(t, 10, chatResp.Usage.PromptTokens)
	assert.Equal(t, 25, chatResp.Usage.CompletionTokens)
	assert.Equal(t, 35, chatResp.Usage.TotalTokens)
}

func TestAnthropicTransformers_ErrorHandling(t *testing.T) {
	inboundTransformer := NewInboundTransformer()
	outboundTransformer := NewOutboundTransformer("", "")

	t.Run("inbound error handling", func(t *testing.T) {
		// Test invalid JSON
		httpReq := &llm.GenericHttpRequest{
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: []byte(`invalid json`),
		}

		_, err := inboundTransformer.TransformRequest(context.Background(), httpReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode anthropic request")
	})

	t.Run("outbound error handling", func(t *testing.T) {
		// Test HTTP error response
		httpResp := &llm.GenericHttpResponse{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error": {"message": "Invalid request", "type": "invalid_request_error"}}`),
			Error: &llm.ResponseError{
				Message: "Invalid request",
				Type:    "invalid_request_error",
			},
		}

		_, err := outboundTransformer.TransformResponse(context.Background(), httpResp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP error 400")
	})
}

func TestAnthropicMessageContent_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		isValid bool
	}{
		{
			name:    "valid string",
			jsonStr: `"Hello, world!"`,
			isValid: true,
		},
		{
			name:    "valid array",
			jsonStr: `[{"type": "text", "text": "Hello"}]`,
			isValid: true,
		},
		{
			name:    "empty string",
			jsonStr: `""`,
			isValid: true,
		},
		{
			name:    "empty array",
			jsonStr: `[]`,
			isValid: true,
		},
		{
			name:    "null value",
			jsonStr: `null`,
			isValid: false,
		},
		{
			name:    "number value",
			jsonStr: `123`,
			isValid: false,
		},
		{
			name:    "boolean value",
			jsonStr: `true`,
			isValid: false,
		},
		{
			name:    "object value",
			jsonStr: `{"key": "value"}`,
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var content MessageContent
			err := json.Unmarshal([]byte(tt.jsonStr), &content)

			if tt.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
