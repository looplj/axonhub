package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/looplj/axonhub/llm"
)

func TestInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		request     *llm.GenericHttpRequest
		wantErr     bool
		errContains string
		validate    func(*llm.ChatCompletionRequest) bool
	}{
		{
			name: "valid request",
			request: &llm.GenericHttpRequest{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(llm.ChatCompletionRequest{
					Model: "gpt-4",
					Messages: []llm.ChatCompletionMessage{
						{
							Role: "user",
							Content: llm.ChatCompletionMessageContent{
								Content: stringPtr("Hello, world!"),
							},
						},
					},
				}),
			},
			wantErr: false,
			validate: func(req *llm.ChatCompletionRequest) bool {
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
			request: &llm.GenericHttpRequest{
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
			request: &llm.GenericHttpRequest{
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
			request: &llm.GenericHttpRequest{
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
			request: &llm.GenericHttpRequest{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(llm.ChatCompletionRequest{
					Messages: []llm.ChatCompletionMessage{
						{
							Role: "user",
							Content: llm.ChatCompletionMessageContent{
								Content: stringPtr("Hello, world!"),
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
			request: &llm.GenericHttpRequest{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(llm.ChatCompletionRequest{
					Model: "gpt-4",
				}),
			},
			wantErr:     true,
			errContains: "messages are required",
		},
		{
			name: "empty messages",
			request: &llm.GenericHttpRequest{
				Method: http.MethodPost,
				URL:    "/v1/chat/completions",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: mustMarshal(llm.ChatCompletionRequest{
					Model:    "gpt-4",
					Messages: []llm.ChatCompletionMessage{},
				}),
			},
			wantErr:     true,
			errContains: "messages are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformRequest(context.Background(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformRequest() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("TransformRequest() error = %v, want error containing %v", err, tt.errContains)
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
		response    *llm.ChatCompletionResponse
		wantErr     bool
		errContains string
		validate    func(*llm.GenericStreamEvent) bool
	}{
		{
			name: "streaming chunk with content",
			response: &llm.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.ChatCompletionChoice{
					{
						Index: 0,
						Delta: &llm.ChatCompletionMessage{
							Role: "assistant",
							Content: llm.ChatCompletionMessageContent{
								Content: stringPtr("Hello"),
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(event *llm.GenericStreamEvent) bool {
				if event.Type != "" {
					return false
				}
				
				// Unmarshal the data to verify it's a valid ChatCompletionResponse
				var chatResp llm.ChatCompletionResponse
				if err := json.Unmarshal(event.Data, &chatResp); err != nil {
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
			response: &llm.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.ChatCompletionChoice{
					{
						Index: 0,
						Delta: &llm.ChatCompletionMessage{
							Role: "assistant",
						},
						FinishReason: stringPtr("stop"),
					},
				},
			},
			wantErr: false,
			validate: func(event *llm.GenericStreamEvent) bool {
				if event.Type != "" {
					return false
				}
				
				// Unmarshal the data to verify it's a valid ChatCompletionResponse
				var chatResp llm.ChatCompletionResponse
				if err := json.Unmarshal(event.Data, &chatResp); err != nil {
					return false
				}
				
				return chatResp.ID == "chatcmpl-123" && 
					len(chatResp.Choices) > 0 && 
					chatResp.Choices[0].FinishReason != nil &&
					*chatResp.Choices[0].FinishReason == "stop"
			},
		},
		{
			name: "empty choices",
			response: &llm.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.ChatCompletionChoice{},
			},
			wantErr: false,
			validate: func(event *llm.GenericStreamEvent) bool {
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
			result, err := transformer.TransformStreamChunk(context.Background(), tt.response)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformStreamChunk() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("TransformStreamChunk() error = %v, want error containing %v", err, tt.errContains)
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

func TestInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		response    *llm.ChatCompletionResponse
		wantErr     bool
		errContains string
		validate    func(*llm.GenericHttpResponse) bool
	}{
		{
			name: "valid response",
			response: &llm.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []llm.ChatCompletionChoice{
					{
						Index: 0,
						Message: &llm.ChatCompletionMessage{
							Role: "assistant",
							Content: llm.ChatCompletionMessageContent{
								Content: stringPtr("Hello! How can I help you today?"),
							},
						},
						FinishReason: stringPtr("stop"),
					},
				},
			},
			wantErr: false,
			validate: func(resp *llm.GenericHttpResponse) bool {
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
				var chatResp llm.ChatCompletionResponse
				if err := json.Unmarshal(resp.Body, &chatResp); err != nil {
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
			result, err := transformer.TransformResponse(context.Background(), tt.response)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformResponse() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("TransformResponse() error = %v, want error containing %v", err, tt.errContains)
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

func TestInboundTransformer_Name(t *testing.T) {
	transformer := NewInboundTransformer().(*InboundTransformer)
	name := transformer.Name()
	if name == "" {
		t.Errorf("Name() returned empty string")
	}
}

func TestInboundTransformer_Priority(t *testing.T) {
	transformer := NewInboundTransformer().(*InboundTransformer)
	priority := transformer.Priority()
	if priority <= 0 {
		t.Errorf("Priority() = %v, want positive number", priority)
	}
}

// Helper functions
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 1; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}

func stringPtr(s string) *string {
	return &s
}