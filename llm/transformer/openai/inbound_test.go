package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		request     *httpclient.Request
		wantErr     bool
		errContains string
		validate    func(*llm.Request) bool
	}{
		{
			name: "valid request",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(llm.Request{
					Model: "gpt-4",
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello, world!"),
							},
						},
					},
				}),
			},
			wantErr: false,
			validate: func(req *llm.Request) bool {
				return req.Model == "gpt-4" && len(req.Messages) == 1 &&
					req.Messages[0].Content.Content != nil && *req.Messages[0].Content.Content == "Hello, world!"
			},
		},
		{
			name:        "nil request",
			request:     nil,
			wantErr:     true,
			errContains: "http request is nil",
		},
		{
			name: "empty body",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte{},
			},
			wantErr:     true,
			errContains: "request body is empty",
		},
		{
			name: "unsupported content type",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"text/plain"},
				},
				Body: []byte("some text"),
			},
			wantErr:     true,
			errContains: "unsupported content type",
		},
		{
			name: "invalid JSON",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte("{invalid json}"),
			},
			wantErr:     true,
			errContains: "failed to decode openai request",
		},
		{
			name: "missing model",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(llm.Request{
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello, world!"),
							},
						},
					},
				}),
			},
			wantErr:     true,
			errContains: "model is required",
		},
		{
			name: "missing messages",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(llm.Request{
					Model: "gpt-4",
				}),
			},
			wantErr:     true,
			errContains: "messages are required",
		},
		{
			name: "empty messages",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(llm.Request{
					Model:    "gpt-4",
					Messages: []llm.Message{},
				}),
			},
			wantErr:     true,
			errContains: "messages are required",
		},
		{
			name: "request with reasoning budget",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(Request{
					Model: "o1",
					Messages: []Message{
						{
							Role: "user",
							Content: MessageContent{
								Content: lo.ToPtr("Test with reasoning budget"),
							},
						},
					},
					ReasoningBudget: lo.ToPtr(int64(8192)),
				}),
			},
			wantErr: false,
			validate: func(req *llm.Request) bool {
				return req != nil &&
					req.ReasoningBudget != nil &&
					*req.ReasoningBudget == 8192
			},
		},
		{
			name: "request with reasoning budget and effort",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(Request{
					Model: "o1",
					Messages: []Message{
						{
							Role: "user",
							Content: MessageContent{
								Content: lo.ToPtr("Test with reasoning"),
							},
						},
					},
					ReasoningEffort: "high",
					ReasoningBudget: lo.ToPtr(int64(16384)),
				}),
			},
			wantErr: false,
			validate: func(req *llm.Request) bool {
				return req != nil &&
					req.ReasoningEffort == "high" &&
					req.ReasoningBudget != nil &&
					*req.ReasoningBudget == 16384
			},
		},
		{
			name: "request with reasoning summary",
			request: &httpclient.Request{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(Request{
					Model: "o3",
					Messages: []Message{
						{
							Role: "user",
							Content: MessageContent{
								Content: lo.ToPtr("Test with reasoning summary"),
							},
						},
					},
					ReasoningEffort:  "medium",
					ReasoningSummary: lo.ToPtr("detailed"),
				}),
			},
			wantErr: false,
			validate: func(req *llm.Request) bool {
				return req != nil &&
					req.ReasoningEffort == "medium" &&
					req.ReasoningSummary != nil &&
					*req.ReasoningSummary == "detailed"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformRequest(t.Context(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformRequest() error = nil, wantErr %v", tt.wantErr)
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf(
						"TransformRequest() error = %v, want error containing %v",
						err,
						tt.errContains,
					)
				}

				return
			}

			if err != nil {
				t.Errorf("TransformRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Error("TransformRequest() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("TransformRequest() validation failed for result: %+v", result)
			}
		})
	}
}

func TestInboundTransformer_TransformStreamChunk(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		response    *llm.Response
		wantErr     bool
		errContains string
		validate    func(*httpclient.StreamEvent) bool
	}{
		{
			name: "streaming chunk with content",
			response: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello"),
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(event *httpclient.StreamEvent) bool {
				if event.Type != "" {
					return false
				}

				// Unmarshal the data to verify it's a valid ChatCompletionResponse
				var chatResp llm.Response

				err := json.Unmarshal(event.Data, &chatResp)
				if err != nil {
					return false
				}

				return chatResp.ID == "chatcmpl-123" &&
					len(chatResp.Choices) > 0 &&
					chatResp.Choices[0].Delta != nil &&
					chatResp.Choices[0].Delta.Content.Content != nil &&
					*chatResp.Choices[0].Delta.Content.Content == "Hello"
			},
		},
		{
			name: "final streaming chunk with finish_reason",
			response: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			wantErr: false,
			validate: func(event *httpclient.StreamEvent) bool {
				if event.Type != "" {
					return false
				}

				// Unmarshal the data to verify it's a valid ChatCompletionResponse
				var chatResp llm.Response

				err := json.Unmarshal(event.Data, &chatResp)
				if err != nil {
					return false
				}

				return chatResp.ID == "chatcmpl-123" &&
					len(chatResp.Choices) > 0 &&
					chatResp.Choices[0].FinishReason != nil &&
					*chatResp.Choices[0].FinishReason == "stop"
			},
		},
		{
			name: "streaming chunk with tool calls",
			response: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "get_user_city",
										Arguments: `{"user_id":"123"}`,
									},
								},
							},
						},
						FinishReason: lo.ToPtr("tool_calls"),
					},
				},
			},
			wantErr: false,
			validate: func(event *httpclient.StreamEvent) bool {
				if event.Type != "" {
					return false
				}

				// Unmarshal the data to verify it's a valid ChatCompletionResponse
				var chatResp llm.Response

				err := json.Unmarshal(event.Data, &chatResp)
				if err != nil {
					return false
				}

				return chatResp.ID == "chatcmpl-123" &&
					len(chatResp.Choices) > 0 &&
					chatResp.Choices[0].Delta != nil &&
					len(chatResp.Choices[0].Delta.ToolCalls) > 0 &&
					chatResp.Choices[0].Delta.ToolCalls[0].Function.Name == "get_user_city" &&
					chatResp.Choices[0].FinishReason != nil &&
					*chatResp.Choices[0].FinishReason == "tool_calls"
			},
		},
		{
			name: "empty choices",
			response: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{},
			},
			wantErr: false,
			validate: func(event *httpclient.StreamEvent) bool {
				return event.Type == ""
			},
		},
		{
			name:        "nil response",
			response:    nil,
			wantErr:     true,
			errContains: "chat completion response is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformStreamChunk(t.Context(), tt.response)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformStreamChunk() error = nil, wantErr %v", tt.wantErr)
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf(
						"TransformStreamChunk() error = %v, want error containing %v",
						err,
						tt.errContains,
					)
				}

				return
			}

			if err != nil {
				t.Errorf("TransformStreamChunk() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Error("TransformStreamChunk() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("TransformStreamChunk() validation failed for result: %+v", result)
			}
		})
	}
}

func TestInboundTransformer_TransformStream_SkipsPureReasoningSignatureChunk(t *testing.T) {
	transformer := NewInboundTransformer()
	signature := shared.GeminiThoughtSignaturePrefix + "stream_signature"

	stream, err := transformer.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID:      "chatcmpl-123",
			Object:  "chat.completion.chunk",
			Created: 1677652288,
			Model:   "gemini-3-pro",
			Choices: []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						ReasoningSignature: &signature,
					},
				},
			},
		},
		{
			ID:      "chatcmpl-123",
			Object:  "chat.completion.chunk",
			Created: 1677652289,
			Model:   "gemini-3-pro",
			Choices: []llm.Choice{
				{
					Index: 0,
					Delta: &llm.Message{
						Role: "assistant",
						ToolCalls: []llm.ToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: llm.FunctionCall{
									Name:      "get_weather",
									Arguments: `{"city":"Shanghai"}`,
								},
								Index: 0,
							},
						},
					},
					FinishReason: lo.ToPtr("tool_calls"),
				},
			},
		},
		{
			Object: "[DONE]",
		},
	}))
	require.NoError(t, err)

	var events []*httpclient.StreamEvent
	for stream.Next() {
		events = append(events, stream.Current())
	}

	require.NoError(t, stream.Err())
	require.Len(t, events, 2)

	var chunkResp Response
	require.NoError(t, json.Unmarshal(events[0].Data, &chunkResp))
	require.Len(t, chunkResp.Choices, 1)
	require.NotNil(t, chunkResp.Choices[0].Delta)
	require.Len(t, chunkResp.Choices[0].Delta.ToolCalls, 1)
	require.Nil(t, chunkResp.Choices[0].Delta.ToolCalls[0].ExtraContent)
	require.Equal(t, "[DONE]", string(events[1].Data))
}

func TestInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		response    *llm.Response
		wantErr     bool
		errContains string
		validate    func(*httpclient.Response) bool
	}{
		{
			name: "valid response",
			response: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
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
			},
			wantErr: false,
			validate: func(resp *httpclient.Response) bool {
				if resp.StatusCode != http.StatusOK {
					return false
				}

				if resp.Headers.Get("Content-Type") != "application/json" {
					return false
				}

				if len(resp.Body) == 0 {
					return false
				}

				// Try to unmarshal the response body
				var chatResp llm.Response

				err := json.Unmarshal(resp.Body, &chatResp)
				if err != nil {
					return false
				}

				return chatResp.ID == "chatcmpl-123" && chatResp.Model == "gpt-4"
			},
		},
		{
			name:        "nil response",
			response:    nil,
			wantErr:     true,
			errContains: "chat completion response is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(t.Context(), tt.response)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformResponse() error = nil, wantErr %v", tt.wantErr)
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf(
						"TransformResponse() error = %v, want error containing %v",
						err,
						tt.errContains,
					)
				}

				return
			}

			if err != nil {
				t.Errorf("TransformResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Error("TransformResponse() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("TransformResponse() validation failed for result: %+v", result)
			}
		})
	}
}

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}

func TestInboundTransformer_TransformError(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name          string
		err           error
		expectedError *httpclient.Error
	}{
		{
			name: "generic error",
			err: &llm.ResponseError{
				StatusCode: http.StatusInternalServerError,
				Detail: llm.ErrorDetail{
					Message: "Internal server error",
					Type:    "internal_error",
					Code:    "internal_server_error",
				},
			},
			expectedError: &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Body:       []byte(`{"error":{"code":"internal_server_error","message":"Internal server error","type":"internal_error"}}`),
			},
		},
		{
			name: "nil error",
			err:  nil,
			expectedError: &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Body:       []byte(`{"error":{"message":"An unexpected error occurred","type":"unexpected_error"}}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr := transformer.TransformError(context.Background(), tt.err)

			if httpErr.StatusCode != tt.expectedError.StatusCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedError.StatusCode, httpErr.StatusCode)
			}

			require.JSONEq(t, string(tt.expectedError.Body), string(httpErr.Body))
		})
	}
}

func TestMessageFromLLM_WithAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		llmMsg   llm.Message
		validate func(*testing.T, Message)
	}{
		{
			name: "message with annotations",
			llmMsg: llm.Message{
				Role:    "assistant",
				Content: llm.MessageContent{Content: lo.ToPtr("The meaning of life...")},
				Annotations: []llm.Annotation{
					{
						Type: "url_citation",
						URLCitation: &llm.URLCitation{
							URL:   "https://en.wikipedia.org/wiki/Meaning_of_life",
							Title: "Meaning of life - Wikipedia",
						},
					},
					{
						Type: "url_citation",
						URLCitation: &llm.URLCitation{
							URL:   "https://plato.stanford.edu/entries/life-meaning/",
							Title: "The Meaning of Life - Stanford Encyclopedia",
						},
					},
				},
			},
			validate: func(t *testing.T, msg Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Len(t, msg.Annotations, 2)
				require.Equal(t, "url_citation", msg.Annotations[0].Type)
				require.NotNil(t, msg.Annotations[0].URLCitation)
				require.Equal(t, "https://en.wikipedia.org/wiki/Meaning_of_life", msg.Annotations[0].URLCitation.URL)
				require.Equal(t, "Meaning of life - Wikipedia", msg.Annotations[0].URLCitation.Title)
			},
		},
		{
			name: "message without annotations",
			llmMsg: llm.Message{
				Role:    "assistant",
				Content: llm.MessageContent{Content: lo.ToPtr("Hello!")},
			},
			validate: func(t *testing.T, msg Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Nil(t, msg.Annotations)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MessageFromLLM(tt.llmMsg)
			tt.validate(t, result)
		})
	}
}

func TestInboundTransformer_TransformResponse_WithCitations(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name     string
		response *llm.Response
		validate func(*httpclient.Response) bool
	}{
		{
			name: "response with citations in metadata",
			response: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "llama-3.1-sonar-small-128k-online",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("The meaning of life is..."),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyCitations: []string{
						"https://www.theatlantic.com/family/archive/2021/10/meaning-life-macronutrients-purpose-search/620440/",
						"https://en.wikipedia.org/wiki/Meaning_of_life",
					},
				},
			},
			validate: func(resp *httpclient.Response) bool {
				if resp.StatusCode != http.StatusOK {
					return false
				}

				// Parse the response body
				var chatResp Response

				err := json.Unmarshal(resp.Body, &chatResp)
				if err != nil {
					return false
				}

				// Verify citations are present
				if len(chatResp.Citations) != 2 {
					return false
				}

				return chatResp.Citations[0] == "https://www.theatlantic.com/family/archive/2021/10/meaning-life-macronutrients-purpose-search/620440/" &&
					chatResp.Citations[1] == "https://en.wikipedia.org/wiki/Meaning_of_life"
			},
		},
		{
			name: "response without citations in metadata",
			response: &llm.Response{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello!"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validate: func(resp *httpclient.Response) bool {
				if resp.StatusCode != http.StatusOK {
					return false
				}

				// Parse the response body
				var chatResp Response

				err := json.Unmarshal(resp.Body, &chatResp)
				if err != nil {
					return false
				}

				// Citations should be nil/empty when not present in metadata
				return len(chatResp.Citations) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(t.Context(), tt.response)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.validate != nil {
				require.True(t, tt.validate(result), "Validation failed for result")
			}
		})
	}
}

func TestMessage_ToLLMMessage_WithGeminiThoughtSignature(t *testing.T) {
	msg := Message{
		Role: "assistant",
		ToolCalls: []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: FunctionCall{
					Name:      "get_weather",
					Arguments: `{"city":"Shanghai"}`,
				},
				Index: 0,
				ExtraContent: &ToolCallExtraContent{
					Google: &ToolCallGoogleExtraContent{
						ThoughtSignature: "base64_signature",
					},
				},
			},
		},
	}

	got := msg.ToLLMMessage()

	require.Len(t, got.ToolCalls, 1)
	require.NotNil(t, got.ReasoningSignature)
	require.Equal(t, "base64_signature", *got.ReasoningSignature)
}

func TestMessage_ToLLMMessage_WithAlreadyPrefixedGeminiThoughtSignature(t *testing.T) {
	msg := Message{
		Role: "assistant",
		ToolCalls: []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: FunctionCall{
					Name:      "get_weather",
					Arguments: `{"city":"Shanghai"}`,
				},
				Index: 0,
				ExtraContent: &ToolCallExtraContent{
					Google: &ToolCallGoogleExtraContent{
						ThoughtSignature: shared.GeminiThoughtSignaturePrefix + "base64_signature",
					},
				},
			},
		},
	}

	got := msg.ToLLMMessage()

	require.NotNil(t, got.ReasoningSignature)
	require.Equal(t, shared.GeminiThoughtSignaturePrefix+"base64_signature", *got.ReasoningSignature)
}

func TestToolCall_ToLLMToolCall_NormalizesGeminiThoughtSignature(t *testing.T) {
	tc := ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: FunctionCall{
			Name:      "get_weather",
			Arguments: `{"city":"Shanghai"}`,
		},
		Index: 0,
		ExtraContent: &ToolCallExtraContent{
			Google: &ToolCallGoogleExtraContent{
				ThoughtSignature: "base64_signature",
			},
		},
	}

	got := tc.ToLLMToolCall()

	require.NotNil(t, got.TransformerMetadata)
	require.Equal(
		t,
		"base64_signature",
		got.TransformerMetadata[TransformerMetadataKeyGoogleThoughtSignature],
	)
}

func TestToolCall_ToLLMToolCall_NormalizesGeminiThoughtSignatureFromExtraFields(t *testing.T) {
	tc := ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: FunctionCall{
			Name:      "get_weather",
			Arguments: `{"city":"Shanghai"}`,
		},
		Index: 0,
		ExtraFields: &ToolCallExtraFields{
			ExtraContent: &ToolCallExtraContent{
				Google: &ToolCallGoogleExtraContent{
					ThoughtSignature: "base64_signature",
				},
			},
		},
	}

	got := tc.ToLLMToolCall()

	require.NotNil(t, got.TransformerMetadata)
	require.Equal(
		t,
		"base64_signature",
		got.TransformerMetadata[TransformerMetadataKeyGoogleThoughtSignature],
	)
}

func TestInboundTransformer_TransformRequest_WithToolCallExtraFieldsThoughtSignature(t *testing.T) {
	transformer := NewInboundTransformer()

	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/chat/completions",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{
			"model":"gemini-2.5-flash",
			"messages":[
				{
					"role":"assistant",
					"tool_calls":[
						{
							"id":"call_1",
							"type":"function",
							"index":0,
							"function":{"name":"game-art_generate_image","arguments":"{}"},
							"extra_fields":{
								"extra_content":{
									"google":{"thought_signature":"raw_signature_from_extra_fields"}
								}
							}
						}
					]
				}
			]
		}`),
	}

	got, err := transformer.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Messages, 1)
	require.Len(t, got.Messages[0].ToolCalls, 1)
	require.NotNil(t, got.Messages[0].ReasoningSignature)
	require.Equal(
		t,
		"raw_signature_from_extra_fields",
		*got.Messages[0].ReasoningSignature,
	)

	metadataSignature, ok := got.Messages[0].ToolCalls[0].TransformerMetadata[TransformerMetadataKeyGoogleThoughtSignature].(string)
	require.True(t, ok)
	require.Equal(
		t,
		"raw_signature_from_extra_fields",
		metadataSignature,
	)
}

func TestMessageFromLLM_WithGeminiThoughtSignatureDoesNotInjectToolCallExtraContent(t *testing.T) {
	msg := llm.Message{
		Role:               "assistant",
		ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("base64_signature"), ""),
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "get_weather",
					Arguments: `{"city":"Shanghai"}`,
				},
				Index: 0,
			},
		},
	}

	got := MessageFromLLM(msg)

	require.Len(t, got.ToolCalls, 1)
	require.Nil(t, got.ToolCalls[0].ExtraContent)
}

func TestInboundTransformer_TransformResponse_WithGeminiToolCallThoughtSignatureDoesNotInjectExtraContent(t *testing.T) {
	transformer := NewInboundTransformer()

	resp, err := transformer.TransformResponse(t.Context(), &llm.Response{
		ID:      "chatcmpl-1",
		Object:  "chat.completion",
		Created: 123,
		Model:   "gemini-3-pro",
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role:               "assistant",
					ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("base64_signature"), ""),
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: llm.FunctionCall{
								Name:      "get_weather",
								Arguments: `{"city":"Shanghai"}`,
							},
							Index: 0,
						},
					},
				},
				FinishReason: lo.ToPtr("tool_calls"),
			},
		},
	})
	require.NoError(t, err)

	var oaiResp Response
	require.NoError(t, json.Unmarshal(resp.Body, &oaiResp))
	require.Len(t, oaiResp.Choices, 1)
	require.NotNil(t, oaiResp.Choices[0].Message)
	require.Len(t, oaiResp.Choices[0].Message.ToolCalls, 1)
	require.Nil(t, oaiResp.Choices[0].Message.ToolCalls[0].ExtraContent)
}

func TestInboundTransformer_TransformResponse_WithGeminiPrefixedToolCallMetadata(t *testing.T) {
	transformer := NewInboundTransformer()

	resp, err := transformer.TransformResponse(t.Context(), &llm.Response{
		ID:      "chatcmpl-1",
		Object:  "chat.completion",
		Created: 123,
		Model:   "gemini-3-pro",
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: llm.FunctionCall{
								Name:      "get_weather",
								Arguments: `{"city":"Shanghai"}`,
							},
							Index: 0,
							TransformerMetadata: map[string]any{
								TransformerMetadataKeyGoogleThoughtSignature: shared.GeminiThoughtSignaturePrefix + "base64_signature",
							},
						},
					},
				},
				FinishReason: lo.ToPtr("tool_calls"),
			},
		},
	})
	require.NoError(t, err)

	var oaiResp Response
	require.NoError(t, json.Unmarshal(resp.Body, &oaiResp))
	require.Len(t, oaiResp.Choices, 1)
	require.NotNil(t, oaiResp.Choices[0].Message)
	require.Len(t, oaiResp.Choices[0].Message.ToolCalls, 1)
	require.NotNil(t, oaiResp.Choices[0].Message.ToolCalls[0].ExtraContent)
	require.NotNil(t, oaiResp.Choices[0].Message.ToolCalls[0].ExtraContent.Google)
	require.Equal(
		t,
		shared.GeminiThoughtSignaturePrefix+"base64_signature",
		oaiResp.Choices[0].Message.ToolCalls[0].ExtraContent.Google.ThoughtSignature,
	)
}
