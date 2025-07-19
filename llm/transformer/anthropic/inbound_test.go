package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/llm"
)

func TestInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		httpReq     *llm.GenericHttpRequest
		expected    *llm.ChatCompletionRequest
		expectError bool
	}{
		{
			name: "valid simple text request",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"messages": [
						{
							"role": "user",
							"content": "Hello, Claude!"
						}
					]
				}`),
			},
			expected: &llm.ChatCompletionRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.ChatCompletionMessage{
					{
						Role: "user",
						Content: llm.ChatCompletionMessageContent{
							Content: func() *string { s := "Hello, Claude!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with system message",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"system": "You are a helpful assistant.",
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expected: &llm.ChatCompletionRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.ChatCompletionMessage{
					{
						Role: "system",
						Content: llm.ChatCompletionMessageContent{
							Content: func() *string { s := "You are a helpful assistant."; return &s }(),
						},
					},
					{
						Role: "user",
						Content: llm.ChatCompletionMessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with multimodal content",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
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
				}`),
			},
			expected: &llm.ChatCompletionRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.ChatCompletionMessage{
					{
						Role: "user",
						Content: llm.ChatCompletionMessageContent{
							MultipleContent: []llm.ContentPart{
								{
									Type: "text",
									Text: func() *string { s := "What's in this image?"; return &s }(),
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
			name: "request with temperature and stop sequences",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"temperature": 0.7,
					"stop_sequences": ["Human:", "Assistant:"],
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expected: &llm.ChatCompletionRequest{
				Model:       "claude-3-sonnet-20240229",
				MaxTokens:   func() *int64 { v := int64(1024); return &v }(),
				Temperature: func() *float64 { v := 0.7; return &v }(),
				Stop: &llm.Stop{
					MultipleStop: []string{"Human:", "Assistant:"},
				},
				Messages: []llm.ChatCompletionMessage{
					{
						Role: "user",
						Content: llm.ChatCompletionMessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "nil request",
			httpReq:     nil,
			expectError: true,
		},
		{
			name: "empty body",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte{},
			},
			expectError: true,
		},
		{
			name: "missing model",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"max_tokens": 1024,
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expectError: true,
		},
		{
			name: "missing max_tokens",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expectError: true,
		},
		{
			name: "missing messages",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024
				}`),
			},
			expectError: true,
		},
		{
			name: "invalid content type",
			httpReq: &llm.GenericHttpRequest{
				Headers: http.Header{
					"Content-Type": []string{"text/plain"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformRequest(context.Background(), tt.httpReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tt.expected.Model, result.Model)
				require.Equal(t, tt.expected.MaxTokens, result.MaxTokens)
				require.Equal(t, tt.expected.Temperature, result.Temperature)
				require.Equal(t, len(tt.expected.Messages), len(result.Messages))

				for i, expectedMsg := range tt.expected.Messages {
					require.Equal(t, expectedMsg.Role, result.Messages[i].Role)
					require.Equal(t, expectedMsg.Content.Content, result.Messages[i].Content.Content)
					require.Equal(t, len(expectedMsg.Content.MultipleContent), len(result.Messages[i].Content.MultipleContent))
				}

				if tt.expected.Stop != nil {
					require.NotNil(t, result.Stop)
					require.Equal(t, tt.expected.Stop.Stop, result.Stop.Stop)
					require.Equal(t, tt.expected.Stop.MultipleStop, result.Stop.MultipleStop)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.ChatCompletionResponse
		expectError bool
	}{
		{
			name: "valid response",
			chatResp: &llm.ChatCompletionResponse{
				ID:      "msg_123",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.ChatCompletionChoice{
					{
						Index: 0,
						Message: &llm.ChatCompletionMessage{
							Role: "assistant",
							Content: llm.ChatCompletionMessageContent{
								Content: func() *string { s := "Hello! How can I help you?"; return &s }(),
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			expectError: false,
		},
		{
			name: "response with multimodal content",
			chatResp: &llm.ChatCompletionResponse{
				ID:      "msg_456",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.ChatCompletionChoice{
					{
						Index: 0,
						Message: &llm.ChatCompletionMessage{
							Role: "assistant",
							Content: llm.ChatCompletionMessageContent{
								MultipleContent: []llm.ContentPart{
									{
										Type: "text",
										Text: func() *string { s := "I can see an image."; return &s }(),
									},
								},
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
		},
		{
			name:        "nil response",
			chatResp:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(context.Background(), tt.chatResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.StatusOK, result.StatusCode)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.NotEmpty(t, result.Body)

				// Verify the response can be unmarshaled to AnthropicResponse
				var anthropicResp MessageResponse
				err := json.Unmarshal(result.Body, &anthropicResp)
				require.NoError(t, err)
				require.Equal(t, tt.chatResp.ID, anthropicResp.ID)
				require.Equal(t, "message", anthropicResp.Type)
				require.Equal(t, "assistant", anthropicResp.Role)
				require.Equal(t, tt.chatResp.Model, anthropicResp.Model)
			}
		})
	}
}

func TestAnthropicMessageContent_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		content MessageContent
		jsonStr string
	}{
		{
			name: "string content",
			content: MessageContent{
				Content: func() *string { s := "Hello, world!"; return &s }(),
			},
			jsonStr: `"Hello, world!"`,
		},
		{
			name: "array content",
			content: MessageContent{
				MultipleContent: []ContentBlock{
					{
						Type: "text",
						Text: func() *string { s := "Hello"; return &s }(),
					},
				},
			},
			jsonStr: `[{"type":"text","text":"Hello"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Marshal
			data, err := json.Marshal(tt.content)
			require.NoError(t, err)
			require.JSONEq(t, tt.jsonStr, string(data))

			// Test Unmarshal
			var content MessageContent
			err = json.Unmarshal([]byte(tt.jsonStr), &content)
			require.NoError(t, err)
			require.Equal(t, tt.content.Content, content.Content)
			require.Equal(t, len(tt.content.MultipleContent), len(content.MultipleContent))
		})
	}
}

func TestInboundTransformer_TransformStreamChunk(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.ChatCompletionResponse
		expectError bool
		checkEvent  func(t *testing.T, event *llm.GenericStreamEvent)
	}{
		{
			name: "message_start event",
			chatResp: &llm.ChatCompletionResponse{
				ID:      "msg_123",
				Object:  "message_start",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "message_start", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "message_start", streamEvent.Type)
				require.NotNil(t, streamEvent.Message)
				require.Equal(t, "msg_123", streamEvent.Message.ID)
				require.Equal(t, "assistant", streamEvent.Message.Role)
				require.Equal(t, "claude-3-sonnet-20240229", streamEvent.Message.Model)

				require.NotNil(t, streamEvent.Message.Usage)
				require.Equal(t, 10, streamEvent.Message.Usage.InputTokens)
				require.Equal(t, 20, streamEvent.Message.Usage.OutputTokens)
			},
		},
		{
			name: "content_block_start event",
			chatResp: &llm.ChatCompletionResponse{
				ID:     "msg_123",
				Object: "content_block_start",
				Model:  "claude-3-sonnet-20240229",
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "content_block_start", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "content_block_start", streamEvent.Type)
				require.NotNil(t, streamEvent.ContentBlock)
				require.Equal(t, "text", streamEvent.ContentBlock.Type)
			},
		},
		{
			name: "content_block_delta event",
			chatResp: &llm.ChatCompletionResponse{
				ID:     "msg_123",
				Object: "content_block_delta",
				Model:  "claude-3-sonnet-20240229",
				Choices: []llm.ChatCompletionChoice{
					{
						Index: 0,
						Delta: &llm.ChatCompletionMessage{
							Role: "assistant",
							Content: llm.ChatCompletionMessageContent{
								Content: func() *string { s := "Hello"; return &s }(),
							},
						},
					},
				},
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "content_block_delta", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "content_block_delta", streamEvent.Type)
				require.NotNil(t, streamEvent.Delta)
				require.NotNil(t, streamEvent.Delta.Text)
				require.Equal(t, "Hello", *streamEvent.Delta.Text)
			},
		},
		{
			name: "content_block_stop event",
			chatResp: &llm.ChatCompletionResponse{
				ID:     "msg_123",
				Object: "content_block_stop",
				Model:  "claude-3-sonnet-20240229",
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "content_block_stop", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "content_block_stop", streamEvent.Type)
			},
		},
		{
			name: "message_delta event with stop reason",
			chatResp: &llm.ChatCompletionResponse{
				ID:     "msg_123",
				Object: "message_delta",
				Model:  "claude-3-sonnet-20240229",
				Choices: []llm.ChatCompletionChoice{
					{
						Index:        0,
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "message_delta", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "message_delta", streamEvent.Type)
				require.NotNil(t, streamEvent.Delta)
				require.NotNil(t, streamEvent.Delta.StopReason)
				require.Equal(t, "end_turn", *streamEvent.Delta.StopReason)
				require.NotNil(t, streamEvent.Usage)
				require.Equal(t, 10, streamEvent.Usage.InputTokens)
				require.Equal(t, 20, streamEvent.Usage.OutputTokens)
			},
		},
		{
			name: "message_delta event with length reason",
			chatResp: &llm.ChatCompletionResponse{
				ID:     "msg_123",
				Object: "message_delta",
				Model:  "claude-3-sonnet-20240229",
				Choices: []llm.ChatCompletionChoice{
					{
						Index:        0,
						FinishReason: func() *string { s := "length"; return &s }(),
					},
				},
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "message_delta", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "message_delta", streamEvent.Type)
				require.NotNil(t, streamEvent.Delta)
				require.NotNil(t, streamEvent.Delta.StopReason)
				require.Equal(t, "max_tokens", *streamEvent.Delta.StopReason)
			},
		},
		{
			name: "message_delta event with tool_calls reason",
			chatResp: &llm.ChatCompletionResponse{
				ID:     "msg_123",
				Object: "message_delta",
				Model:  "claude-3-sonnet-20240229",
				Choices: []llm.ChatCompletionChoice{
					{
						Index:        0,
						FinishReason: func() *string { s := "tool_calls"; return &s }(),
					},
				},
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "message_delta", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "message_delta", streamEvent.Type)
				require.NotNil(t, streamEvent.Delta)
				require.NotNil(t, streamEvent.Delta.StopReason)
				require.Equal(t, "tool_use", *streamEvent.Delta.StopReason)
			},
		},
		{
			name: "message_stop event",
			chatResp: &llm.ChatCompletionResponse{
				ID:     "msg_123",
				Object: "message_stop",
				Model:  "claude-3-sonnet-20240229",
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "message_stop", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "message_stop", streamEvent.Type)
			},
		},
		{
			name: "default/data event with content",
			chatResp: &llm.ChatCompletionResponse{
				ID:     "msg_123",
				Object: "", // Empty object defaults to "data"
				Model:  "claude-3-sonnet-20240229",
				Choices: []llm.ChatCompletionChoice{
					{
						Index: 0,
						Delta: &llm.ChatCompletionMessage{
							Role: "assistant",
							Content: llm.ChatCompletionMessageContent{
								Content: lo.ToPtr("Hello world"),
							},
						},
					},
				},
			},
			expectError: false,
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Empty(t, event.Type)
				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Empty(t, streamEvent.Type)
				require.NotNil(t, streamEvent.Delta)
				require.NotNil(t, streamEvent.Delta.Text)
				require.Equal(t, "Hello world", *streamEvent.Delta.Text)
			},
		},
		{
			name: "empty choices",
			chatResp: &llm.ChatCompletionResponse{
				ID:      "msg_123",
				Object:  "content_block_delta",
				Model:   "claude-3-sonnet-20240229",
				Choices: []llm.ChatCompletionChoice{},
			},
			expectError: false, // Should not error, just create empty event
			checkEvent: func(t *testing.T, event *llm.GenericStreamEvent) {
				require.Equal(t, "content_block_delta", event.Type)

				// Unmarshal the data to check the event
				var streamEvent StreamEvent
				err := json.Unmarshal(event.Data, &streamEvent)
				require.NoError(t, err)

				require.Equal(t, "content_block_delta", streamEvent.Type)
				// Delta should be nil since no choices
				require.Nil(t, streamEvent.Delta)
			},
		},
		{
			name:        "nil response",
			chatResp:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformStreamChunk(context.Background(), tt.chatResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.Data)

				if tt.checkEvent != nil {
					tt.checkEvent(t, result)
				}
			}
		})
	}
}
