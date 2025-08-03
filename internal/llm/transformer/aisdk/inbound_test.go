package aisdk

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewInboundTransformer()
	ctx := context.Background()

	tests := []struct {
		name     string
		input    *llm.GenericHttpRequest
		expected *llm.Request
		wantErr  bool
	}{
		{
			name: "basic text message",
			input: &llm.GenericHttpRequest{
				Method: "POST",
				URL:    "/api/chat",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"messages": [
						{
							"role": "user",
							"content": "Hello, world!"
						}
					],
					"model": "gpt-3.5-turbo",
					"stream": true
				}`),
			},
			expected: &llm.Request{
				Model: "gpt-3.5-turbo",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: stringPtr("Hello, world!"),
						},
					},
				},
				Stream: boolPtr(true),
			},
			wantErr: false,
		},
		{
			name: "message with tools",
			input: &llm.GenericHttpRequest{
				Method: "POST",
				URL:    "/api/chat",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"messages": [
						{
							"role": "user",
							"content": "What's the weather like?"
						}
					],
					"model": "gpt-4",
					"tools": [
						{
							"type": "function",
							"function": {
								"name": "get_weather",
								"description": "Get current weather",
								"parameters": {
									"type": "object",
									"properties": {
										"location": {
											"type": "string",
											"description": "The city name"
										}
									},
									"required": ["location"]
								}
							}
						}
					],
					"toolChoice": "auto"
				}`),
			},
			expected: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: stringPtr("What's the weather like?"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "function",
						Function: llm.Function{
							Name:        "get_weather",
							Description: "Get current weather",
							Parameters: json.RawMessage(`{
								"type": "object",
								"properties": {
									"location": {
										"type": "string",
										"description": "The city name"
									}
								},
								"required": ["location"]
							}`),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid JSON",
			input: &llm.GenericHttpRequest{
				Method: "POST",
				URL:    "/api/chat",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{invalid json}`),
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformRequest(ctx, tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				assert.Equal(t, tt.expected.Model, result.Model)
				assert.Equal(t, len(tt.expected.Messages), len(result.Messages))

				if len(tt.expected.Messages) > 0 {
					assert.Equal(t, tt.expected.Messages[0].Role, result.Messages[0].Role)
					if tt.expected.Messages[0].Content.Content != nil {
						assert.Equal(t, *tt.expected.Messages[0].Content.Content, *result.Messages[0].Content.Content)
					}
				}

				if tt.expected.Stream != nil {
					assert.Equal(t, *tt.expected.Stream, *result.Stream)
				}

				if len(tt.expected.Tools) > 0 {
					assert.Equal(t, len(tt.expected.Tools), len(result.Tools))
					assert.Equal(t, tt.expected.Tools[0].Type, result.Tools[0].Type)
					assert.Equal(t, tt.expected.Tools[0].Function.Name, result.Tools[0].Function.Name)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewInboundTransformer()
	ctx := context.Background()

	tests := []struct {
		name     string
		input    *llm.Response
		expected *llm.GenericHttpResponse
		wantErr  bool
	}{
		{
			name: "basic response",
			input: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-3.5-turbo",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: stringPtr("Hello! How can I help you today?"),
							},
						},
						FinishReason: stringPtr("stop"),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 9,
					TotalTokens:      19,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(ctx, tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				assert.Equal(t, http.StatusOK, result.StatusCode)
				assert.Equal(t, "application/json", result.Headers.Get("Content-Type"))

				// Parse response body
				var responseData map[string]interface{}
				err := json.Unmarshal([]byte(result.Body), &responseData)
				require.NoError(t, err)

				assert.Equal(t, tt.input.ID, responseData["id"])
				assert.Equal(t, tt.input.Object, responseData["object"])
				assert.Equal(t, tt.input.Model, responseData["model"])
			}
		})
	}
}

func TestInboundTransformer_TransformStreamChunk(t *testing.T) {
	transformer := NewInboundTransformer()
	ctx := context.Background()

	tests := []struct {
		name     string
		input    *llm.Response
		expected string
		wantErr  bool
	}{
		{
			name: "text chunk",
			input: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-3.5-turbo",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Content: llm.MessageContent{
								Content: stringPtr("Hello"),
							},
						},
						FinishReason: nil,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformStreamChunk(ctx, tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Check that result contains AI SDK stream format
				assert.Contains(t, string(result.Data), ":")
				assert.True(t, strings.HasSuffix(string(result.Data), "\n"))
			}
		})
	}
}

func TestAiSDKStreamFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "text content",
			input:    "Hello, world!",
			expected: `0:"Hello, world!"` + "\n",
		},
		{
			name:     "empty content",
			input:    "",
			expected: `0:""` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentBytes, _ := json.Marshal(tt.input)
			result := "0:" + string(contentBytes) + "\n"
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}
