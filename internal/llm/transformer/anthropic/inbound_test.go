package anthropic

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

func TestInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		httpReq     *httpclient.Request
		expected    *llm.Request
		expectError bool
	}{
		{
			name: "valid simple text request",
			httpReq: &httpclient.Request{
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
			expected: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, Claude!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with system message",
			httpReq: &httpclient.Request{
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
			expected: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: func() *string { s := "You are a helpful assistant."; return &s }(),
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with multimodal content",
			httpReq: &httpclient.Request{
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
			expected: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
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
			httpReq: &httpclient.Request{
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
			expected: &llm.Request{
				Model:       "claude-3-sonnet-20240229",
				MaxTokens:   func() *int64 { v := int64(1024); return &v }(),
				Temperature: func() *float64 { v := 0.7; return &v }(),
				Stop: &llm.Stop{
					MultipleStop: []string{"Human:", "Assistant:"},
				},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
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
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte{},
			},
			expectError: true,
		},
		{
			name: "missing model",
			httpReq: &httpclient.Request{
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
			httpReq: &httpclient.Request{
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
			httpReq: &httpclient.Request{
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
			httpReq: &httpclient.Request{
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
			result, err := transformer.TransformRequest(t.Context(), tt.httpReq)

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
		chatResp    *llm.Response
		expectError bool
	}{
		{
			name: "valid response",
			chatResp: &llm.Response{
				ID:      "msg_123",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
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
			chatResp: &llm.Response{
				ID:      "msg_456",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
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
			result, err := transformer.TransformResponse(t.Context(), tt.chatResp)

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
				var anthropicResp Message

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

func TestInboundTransformer_ErrorHandling(t *testing.T) {
	transformer := NewInboundTransformer()

	t.Run("TransformRequest error cases", func(t *testing.T) {
		tests := []struct {
			name        string
			httpReq     *httpclient.Request
			expectError bool
			errorMsg    string
		}{
			{
				name:        "nil request",
				httpReq:     nil,
				expectError: true,
				errorMsg:    "http request is nil",
			},
			{
				name: "empty body",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte{},
				},
				expectError: true,
				errorMsg:    "request body is empty",
			},
			{
				name: "invalid content type",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"text/plain"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": []}`,
					),
				},
				expectError: true,
				errorMsg:    "unsupported content type",
			},
			{
				name: "no content type header",
				httpReq: &httpclient.Request{
					Headers: http.Header{},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": []}`,
					),
				},
				expectError: true,
				errorMsg:    "unsupported content type",
			},
			{
				name: "invalid JSON",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(`{invalid json}`),
				},
				expectError: true,
				errorMsg:    "failed to decode anthropic request",
			},
			{
				name: "missing model field",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "model is required",
			},
			{
				name: "empty model field",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "", "max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "model is required",
			},
			{
				name: "missing messages field",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024}`),
				},
				expectError: true,
				errorMsg:    "messages are required",
			},
			{
				name: "empty messages array",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": []}`,
					),
				},
				expectError: true,
				errorMsg:    "messages are required",
			},
			{
				name: "invalid max_tokens (negative)",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": -1, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "max_tokens is required and must be positive",
			},
			{
				name: "invalid max_tokens (zero)",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 0, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "max_tokens is required and must be positive",
			},
			{
				name: "missing max_tokens field",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "max_tokens is required and must be positive",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformRequest(t.Context(), tt.httpReq)
				if tt.expectError {
					require.Error(t, err)
					require.Contains(t, err.Error(), tt.errorMsg)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("TransformResponse error cases", func(t *testing.T) {
		tests := []struct {
			name        string
			chatResp    *llm.Response
			expectError bool
			errorMsg    string
		}{
			{
				name:        "nil response",
				chatResp:    nil,
				expectError: true,
				errorMsg:    "chat completion response is nil",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformResponse(t.Context(), tt.chatResp)
				if tt.expectError {
					require.Error(t, err)
					require.Contains(t, err.Error(), tt.errorMsg)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestInboundTransformer_ValidationEdgeCases(t *testing.T) {
	transformer := NewInboundTransformer()

	t.Run("Message content validation", func(t *testing.T) {
		tests := []struct {
			name        string
			httpReq     *httpclient.Request
			expectError bool
		}{
			{
				name: "null content in message",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": [{"role": "user", "content": null}]}`,
					),
				},
				expectError: true, // Should error on null content
			},
			{
				name: "invalid content type",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": [{"role": "user", "content": 123}]}`,
					),
				},
				expectError: true, // Should error on invalid content type
			},
			{
				name: "invalid system prompt type",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "system": 123, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true, // Should error on invalid system type
			},
			{
				name: "invalid system prompt array type",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "system": [{"type": "invalid"}], "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true, // Should error on invalid system prompt array
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformRequest(t.Context(), tt.httpReq)
				if tt.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
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
						Text: "Hello",
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

func TestInboundTransformer_StreamTransformation_WithTestData_Stop(t *testing.T) {
	transformer := NewInboundTransformer()

	// Load test data from files
	// The response-stop.stream.jsonl contains OpenAI format responses
	openaiData, err := os.ReadFile("testdata/llm-stop.stream.jsonl")
	require.NoError(t, err)

	// The anthropic-stop.stream.jsonl contains expected Anthropic format events
	expectedData, err := os.ReadFile("testdata/anthropic-stop.stream.jsonl")
	require.NoError(t, err)

	// Parse OpenAI stream responses
	openaiLines := strings.Split(strings.TrimSpace(string(openaiData)), "\n")

	var openaiResponses []*llm.Response

	for _, line := range openaiLines {
		if line != "" {
			// Check if this is a DONE event
			if strings.Contains(line, `"Data":"[DONE]"`) {
				// This is a DONE event, add the DoneResponse
				openaiResponses = append(openaiResponses, llm.DoneResponse)
			} else {
				// Parse the StreamEvent to get the Data field
				var event struct {
					Type string `json:"Type"`
					Data string `json:"Data"`
				}

				err := json.Unmarshal([]byte(line), &event)
				require.NoError(t, err)

				// Parse the Data field as llm.Response
				var resp llm.Response

				err = json.Unmarshal([]byte(event.Data), &resp)
				require.NoError(t, err)

				openaiResponses = append(openaiResponses, &resp)
			}
		}
	}

	// Parse expected Anthropic stream events
	expectedLines := strings.Split(strings.TrimSpace(string(expectedData)), "\n")

	var expectedEvents []*httpclient.StreamEvent

	for _, line := range expectedLines {
		if line != "" {
			var event struct {
				Type string `json:"Type"`
				Data string `json:"Data"`
			}

			err := json.Unmarshal([]byte(line), &event)
			require.NoError(t, err)

			expectedEvents = append(expectedEvents, &httpclient.StreamEvent{
				Type: event.Type,
				Data: []byte(event.Data),
			})
		}
	}

	// Create a mock stream from OpenAI responses
	mockStream := streams.SliceStream(openaiResponses)

	// Transform the stream (OpenAI -> Anthropic)
	transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
	require.NoError(t, err)

	// Collect all transformed events
	var actualEvents []*httpclient.StreamEvent

	for transformedStream.Next() {
		event := transformedStream.Current()
		actualEvents = append(actualEvents, event)
	}

	require.NoError(t, transformedStream.Err())

	// Verify the number of events matches
	require.Equal(t, len(expectedEvents), len(actualEvents), "Number of events should match")

	// Verify each event
	for i, expected := range expectedEvents {
		actual := actualEvents[i]

		// Verify event type
		assert.Equal(t, expected.Type, actual.Type, "Event %d: Type should match", i)

		// Parse and compare event data
		var expectedStreamEvent StreamEvent

		err := json.Unmarshal(expected.Data, &expectedStreamEvent)
		require.NoError(t, err)

		var actualStreamEvent StreamEvent

		err = json.Unmarshal(actual.Data, &actualStreamEvent)
		require.NoError(t, err)

		// Verify stream event type
		assert.Equal(t, expectedStreamEvent.Type, actualStreamEvent.Type, "Event %d: Stream event type should match", i)

		// Verify specific fields based on event type
		switch expectedStreamEvent.Type {
		case "message_start":
			require.NotNil(t, expectedStreamEvent.Message)
			require.NotNil(t, actualStreamEvent.Message)
			assert.Equal(t, expectedStreamEvent.Message.ID, actualStreamEvent.Message.ID, "Event %d: Message ID should match", i)
			assert.Equal(t, expectedStreamEvent.Message.Model, actualStreamEvent.Message.Model, "Event %d: Model should match", i)
			assert.Equal(t, expectedStreamEvent.Message.Role, actualStreamEvent.Message.Role, "Event %d: Role should match", i)

			if expectedStreamEvent.Message.Usage != nil && actualStreamEvent.Message.Usage != nil {
				assert.Equal(t, int64(1), actualStreamEvent.Message.Usage.InputTokens, "Event %d: Input tokens should match", i)
				assert.Equal(
					t,
					expectedStreamEvent.Message.Usage.OutputTokens,
					actualStreamEvent.Message.Usage.OutputTokens,
					"Event %d: Output tokens should match",
					i,
				)
			}

		case "content_block_start":
			require.NotNil(t, expectedStreamEvent.ContentBlock)
			require.NotNil(t, actualStreamEvent.ContentBlock)
			assert.Equal(t, expectedStreamEvent.ContentBlock.Type, actualStreamEvent.ContentBlock.Type, "Event %d: Content block type should match", i)

		case "content_block_delta":
			require.NotNil(t, expectedStreamEvent.Delta)
			require.NotNil(t, actualStreamEvent.Delta)

			if expectedStreamEvent.Delta.Text != nil && actualStreamEvent.Delta.Text != nil {
				assert.Equal(t, *expectedStreamEvent.Delta.Text, *actualStreamEvent.Delta.Text, "Event %d: Delta text should match", i)
			}

		case "content_block_stop":
			assert.Equal(t, expectedStreamEvent.Index, actualStreamEvent.Index, "Event %d: Index should match", i)

		case "message_delta":
			require.NotNil(t, expectedStreamEvent.Delta)
			require.NotNil(t, actualStreamEvent.Delta)

			if expectedStreamEvent.Delta.StopReason != nil && actualStreamEvent.Delta.StopReason != nil {
				assert.Equal(t, *expectedStreamEvent.Delta.StopReason, *actualStreamEvent.Delta.StopReason, "Event %d: Stop reason should match", i)
			}

			if expectedStreamEvent.Usage != nil && actualStreamEvent.Usage != nil {
				// Aggregate input tokens from the message_start event.
				assert.Equal(t, int64(21), actualStreamEvent.Usage.InputTokens, "Event %d: Usage input tokens should match", i)
				assert.Equal(t, expectedStreamEvent.Usage.OutputTokens, actualStreamEvent.Usage.OutputTokens, "Event %d: Usage output tokens should match", i)
			}

		case "message_stop":
			// No specific fields to verify for message_stop
		}
	}

	aggregatedBytes, err := transformer.AggregateStreamChunks(t.Context(), actualEvents)
	require.NoError(t, err)

	var aggregatedResp Message

	err = json.Unmarshal(aggregatedBytes, &aggregatedResp)
	require.NoError(t, err)

	// Verify aggregated response
	assert.Equal(t, "msg_bdrk_01Fbg5HKuVfmtT6mAMxQoCSn", aggregatedResp.ID)
	assert.Equal(t, "message", aggregatedResp.Type)
	assert.Equal(t, "claude-3-7-sonnet-20250219", aggregatedResp.Model)
	assert.NotEmpty(t, aggregatedResp.Content)
	assert.Equal(t, "assistant", aggregatedResp.Role)

	// Verify the complete content
	expectedContent := "1 2 3 4 5\n6 7 8 9 10\n11 12 13 14 15\n16 17 18 19 20"
	assert.Equal(t, expectedContent, aggregatedResp.Content[0].Text)
}
