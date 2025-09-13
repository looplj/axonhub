package aisdk

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestDataStreamTransformer_TransformRequest(t *testing.T) {
	transformer := NewDataStreamTransformer()
	ctx := context.Background()

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
					"model": "gpt-4",
					"stream": true
				}`),
			},
			expected: &llm.Request{
				Model: "gpt-4",
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
					]
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
						},
					},
				},
				Stream: lo.ToPtr(true),
			},
			wantErr: false,
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

				if len(tt.expected.Tools) > 0 {
					require.Equal(t, len(tt.expected.Tools), len(result.Tools))
					require.Equal(t, tt.expected.Tools[0].Type, result.Tools[0].Type)
					require.Equal(t, tt.expected.Tools[0].Function.Name, result.Tools[0].Function.Name)
				}
			}
		})
	}
}

func TestDataStreamTransformer_TransformResponse(t *testing.T) {
	transformer := NewDataStreamTransformer()
	ctx := context.Background()

	// Data stream protocol should not support non-streaming responses
	resp := &llm.Response{
		ID:     "test-id",
		Object: "chat.completion",
		Model:  "gpt-4",
	}

	result, err := transformer.TransformResponse(ctx, resp)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "data stream protocol only supports streaming responses")
}

func TestDataStreamTransformer_TransformStreamChunk(t *testing.T) {
	transformer := NewDataStreamTransformer()
	ctx := context.Background()

	tests := []struct {
		name          string
		input         *llm.Response
		expectedParts []string
		wantErr       bool
	}{
		{
			name: "text content chunk",
			input: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello"),
							},
						},
					},
				},
			},
			expectedParts: []string{
				`"type":"text-start"`,
				`"type":"text-delta"`,
				`"delta":"Hello"`,
				`"type":"text-end"`,
			},
			wantErr: false,
		},
		{
			name: "tool call start chunk",
			input: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							ToolCalls: []llm.ToolCall{
								{
									ID: "call_123",
									Function: llm.FunctionCall{
										Name:      "get_weather",
										Arguments: "",
									},
								},
							},
						},
					},
				},
			},
			expectedParts: []string{
				`"type":"tool-input-start"`,
				`"toolCallId":"call_123"`,
				`"toolName":"get_weather"`,
			},
			wantErr: false,
		},
		{
			name: "tool call arguments chunk",
			input: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							ToolCalls: []llm.ToolCall{
								{
									ID: "call_123",
									Function: llm.FunctionCall{
										Arguments: `{"location":"San Francisco"}`,
									},
								},
							},
						},
					},
				},
			},
			expectedParts: []string{
				`"type":"tool-input-delta"`,
				`"toolCallId":"call_123"`,
				`"inputTextDelta":"{\"location\":\"San Francisco\"}"`,
			},
			wantErr: false,
		},
		{
			name: "complete tool call",
			input: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							ToolCalls: []llm.ToolCall{
								{
									ID: "call_123",
									Function: llm.FunctionCall{
										Name:      "get_weather",
										Arguments: `{"location":"San Francisco"}`,
									},
								},
							},
						},
					},
				},
			},
			expectedParts: []string{
				`"type":"tool-input-available"`,
				`"toolCallId":"call_123"`,
				`"toolName":"get_weather"`,
				`"input":{"location":"San Francisco"}`,
			},
			wantErr: false,
		},
		{
			name: "finish chunk",
			input: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index:        0,
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			expectedParts: []string{
				`"type":"finish-step"`,
				`"type":"finish"`,
				`data: [DONE]`,
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

				resultStr := string(result.Data)

				// Check that result follows SSE format
				require.True(t, strings.HasPrefix(resultStr, "data: ") || strings.Contains(resultStr, "\ndata: "))
				require.True(t, strings.HasSuffix(resultStr, "\n"))

				// Check for expected parts
				for _, expectedPart := range tt.expectedParts {
					require.Contains(t, resultStr, expectedPart, "Expected part not found: %s", expectedPart)
				}
			}
		})
	}
}

func TestDataStreamTransformer_APIFormat(t *testing.T) {
	transformer := NewDataStreamTransformer()
	require.Equal(t, llm.APIFormatAiSDKDataStream, transformer.APIFormat())
}

func TestSetDataStreamHeaders(t *testing.T) {
	headers := make(http.Header)
	SetDataStreamHeaders(headers)

	require.Equal(t, "v1", headers.Get("X-Vercel-Ai-Ui-Message-Stream"))
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
}

func TestStreamPartTypes(t *testing.T) {
	tests := []struct {
		name     string
		part     interface{}
		expected string
	}{
		{
			name: "message start part",
			part: UIMessagePart{
				Type: "start",
				ID:   "msg_123",
			},
			expected: `{"type":"start","id":"msg_123"}`,
		},
		{
			name: "text start part",
			part: UIMessagePart{
				Type: "text-start",
				ID:   "text_123",
			},
			expected: `{"type":"text-start","id":"text_123"}`,
		},
		{
			name: "text delta part",
			part: UIMessagePart{
				Type:  "text-delta",
				ID:    "text_123",
				Delta: "Hello",
			},
			expected: `{"type":"text-delta","id":"text_123","delta":"Hello"}`,
		},
		{
			name: "text end part",
			part: UIMessagePart{
				Type: "text-end",
				ID:   "text_123",
			},
			expected: `{"type":"text-end","id":"text_123"}`,
		},
		{
			name: "tool input start part",
			part: UIMessagePart{
				Type:       "tool-input-start",
				ToolCallID: "call_123",
				ToolName:   "get_weather",
			},
			expected: `{"type":"tool-input-start","toolCallId":"call_123","toolName":"get_weather"}`,
		},
		{
			name: "tool input delta part",
			part: UIMessagePart{
				Type:           "tool-input-delta",
				ToolCallID:     "call_123",
				InputTextDelta: "San Francisco",
			},
			expected: `{"type":"tool-input-delta","toolCallId":"call_123","inputTextDelta":"San Francisco"}`,
		},
		{
			name: "tool input available part",
			part: UIMessagePart{
				Type:       "tool-input-available",
				ToolCallID: "call_123",
				ToolName:   "get_weather",
				Input:      map[string]string{"location": "San Francisco"},
			},
			expected: `{"type":"tool-input-available","toolCallId":"call_123","toolName":"get_weather","input":{"location":"San Francisco"}}`,
		},
		{
			name: "finish step part",
			part: UIMessagePart{
				Type: "finish-step",
			},
			expected: `{"type":"finish-step"}`,
		},
		{
			name: "finish message part",
			part: UIMessagePart{
				Type: "finish",
			},
			expected: `{"type":"finish"}`,
		},
		{
			name: "error part",
			part: UIMessagePart{
				Type:      "error",
				ErrorText: "Something went wrong",
			},
			expected: `{"type":"error","errorText":"Something went wrong"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.part)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(jsonBytes))
		})
	}
}

func TestGenerateTextID(t *testing.T) {
	id1 := generateTextID()
	id2 := generateTextID()

	// IDs should be different
	require.NotEqual(t, id1, id2)

	// IDs should start with "msg_"
	require.True(t, strings.HasPrefix(id1, "msg_"))
	require.True(t, strings.HasPrefix(id2, "msg_"))

	// IDs should not contain hyphens (UUID hyphens are removed)
	require.False(t, strings.Contains(id1, "-"))
	require.False(t, strings.Contains(id2, "-"))
}

func TestDataStreamTransformer_TransformError(t *testing.T) {
	transformer := NewDataStreamTransformer()
	ctx := context.Background()

	tests := []struct {
		name           string
		input          error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "nil error",
			input:          nil,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"message":"internal server error","type":"internal_server_error"}`,
		},
		{
			name: "http client error",
			input: &httpclient.Error{
				StatusCode: http.StatusBadRequest,
				Status:     "Bad Request",
				Body:       []byte(`{"message":"bad request","type":"invalid_request"}`),
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"message":"bad request","type":"invalid_request"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.TransformError(ctx, tt.input)
			require.NotNil(t, result)
			require.Equal(t, tt.expectedStatus, result.StatusCode)
			require.JSONEq(t, tt.expectedBody, string(result.Body))
		})
	}
}
