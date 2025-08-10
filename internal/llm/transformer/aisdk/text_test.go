package aisdk

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewTextTransformer()
	ctx := t.Context()

	tests := []struct {
		name     string
		input    *httpclient.Request
		expected *llm.Request
		wantErr  bool
	}{
		{
			name: "basic text message",
			input: &httpclient.Request{
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
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
				Stream: lo.ToPtr(true),
			},
			wantErr: false,
		},
		{
			name: "message with tools",
			input: &httpclient.Request{
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
							Content: lo.ToPtr("What's the weather like?"),
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
			input: &httpclient.Request{
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
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				require.Equal(t, tt.expected.Model, result.Model)
				require.Equal(t, len(tt.expected.Messages), len(result.Messages))

				if len(tt.expected.Messages) > 0 {
					require.Equal(t, tt.expected.Messages[0].Role, result.Messages[0].Role)

					if tt.expected.Messages[0].Content.Content != nil {
						require.Equal(t, *tt.expected.Messages[0].Content.Content, *result.Messages[0].Content.Content)
					}
				}

				if tt.expected.Stream != nil {
					require.Equal(t, *tt.expected.Stream, *result.Stream)
				}

				if len(tt.expected.Tools) > 0 {
					require.Equal(t, len(tt.expected.Tools), len(result.Tools))
					require.Equal(t, tt.expected.Tools[0].Type, result.Tools[0].Type)
					require.Equal(t, tt.expected.Tools[0].Function.Name, result.Tools[0].Function.Name)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewTextTransformer()
	ctx := t.Context()

	tests := []struct {
		name     string
		input    *llm.Response
		expected *httpclient.Response
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
								Content: lo.ToPtr("Hello! How can I help you today?"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
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
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				require.Equal(t, http.StatusOK, result.StatusCode)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))

				// Parse response body
				var responseData map[string]interface{}

				err := json.Unmarshal(result.Body, &responseData)
				require.NoError(t, err)

				require.Equal(t, tt.input.ID, responseData["id"])
				require.Equal(t, tt.input.Object, responseData["object"])
				require.Equal(t, tt.input.Model, responseData["model"])
			}
		})
	}
}

func TestInboundTransformer_TransformStreamChunk(t *testing.T) {
	transformer := NewTextTransformer()
	ctx := t.Context()

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
								Content: lo.ToPtr("Hello"),
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
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Check that result contains AI SDK stream format
				require.Contains(t, string(result.Data), ":")
				require.True(t, strings.HasSuffix(string(result.Data), "\n"))
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
			require.Equal(t, tt.expected, result)
		})
	}
}
