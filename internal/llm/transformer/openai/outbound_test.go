package openai

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	tests := []struct {
		name        string
		transformer *OutboundTransformer
		request     *llm.Request
		wantErr     bool
		errContains string
		validate    func(*httpclient.Request) bool
	}{
		{
			name:        "valid request with default URL",
			transformer: NewOutboundTransformer("", "test-api-key").(*OutboundTransformer),
			request: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				return req.Method == http.MethodPost &&
					req.URL == "https://api.openai.com/v1/chat/completions" &&
					req.Headers.Get("Content-Type") == "application/json" &&
					req.Auth != nil &&
					req.Auth.Type == "bearer" &&
					req.Auth.APIKey == "test-api-key"
			},
		},
		{
			name:        "valid request with custom URL",
			transformer: NewOutboundTransformer("https://custom.api.com/v1", "test-key").(*OutboundTransformer),
			request: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				return req.URL == "https://custom.api.com/v1/chat/completions"
			},
		},
		{
			name:        "valid request without API key",
			transformer: NewOutboundTransformer("https://api.openai.com/v1", "").(*OutboundTransformer),
			request: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				return req.Auth == nil
			},
		},
		{
			name:        "nil request",
			transformer: NewOutboundTransformer("", "test-key").(*OutboundTransformer),
			request:     nil,
			wantErr:     true,
			errContains: "chat completion request is nil",
		},
		{
			name:        "missing model",
			transformer: NewOutboundTransformer("", "test-key").(*OutboundTransformer),
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr:     true,
			errContains: "model is required",
		},
		{
			name:        "missing messages",
			transformer: NewOutboundTransformer("", "test-key").(*OutboundTransformer),
			request: &llm.Request{
				Model: "gpt-4",
			},
			wantErr:     true,
			errContains: "messages are required",
		},
		{
			name:        "URL with trailing slash",
			transformer: NewOutboundTransformer("https://api.openai.com/v1/", "test-key").(*OutboundTransformer),
			request: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				return req.URL == "https://api.openai.com/v1/chat/completions"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.transformer.TransformRequest(t.Context(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformRequest() expected error but got none")
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
				t.Errorf("TransformRequest() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("TransformRequest() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("TransformRequest() validation failed for result: %+v", result)
			}

			// Validate that body can be unmarshaled back to original request
			if len(result.Body) > 0 {
				var unmarshaled llm.Request

				err := json.Unmarshal(result.Body, &unmarshaled)
				if err != nil {
					t.Errorf("TransformRequest() body is not valid JSON: %v", err)
				}
			}
		})
	}
}

func TestOutboundTransformer_AggregateStreamChunks(t *testing.T) {
	transformer := NewOutboundTransformer("", "test-key").(*OutboundTransformer)

	tests := []struct {
		name        string
		chunks      []*httpclient.StreamEvent
		wantErr     bool
		errContains string
		validate    func([]byte) bool
	}{
		{
			name:   "empty chunks",
			chunks: []*httpclient.StreamEvent{},
			validate: func(respBytes []byte) bool {
				var resp llm.Response
				err := json.Unmarshal(respBytes, &resp)
				return err == nil
			},
		},
		{
			name: "valid OpenAI streaming chunks",
			chunks: []*httpclient.StreamEvent{
				{
					Data: []byte(
						`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
					),
				},
				{
					Data: []byte(
						`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":" world"}}]}`,
					),
				},
				{
					Data: []byte(
						`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
					),
				},
			},
			validate: func(respBytes []byte) bool {
				var resp llm.Response
				err := json.Unmarshal(respBytes, &resp)
				if err != nil {
					return false
				}
				if len(resp.Choices) == 0 {
					return false
				}
				// Check if content is aggregated correctly
				if *resp.Choices[0].Message.Content.Content != "Hello world" {
					return false
				}
				// Check if object type is changed to chat.completion
				if resp.Object != "chat.completion" {
					return false
				}
				return true
			},
		},
		{
			name: "invalid JSON chunk",
			chunks: []*httpclient.StreamEvent{
				{
					Data: []byte(
						`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
					),
				},
				{
					Data: []byte(`invalid json`),
				},
				{
					Data: []byte(
						`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":" world"}}]}`,
					),
				},
			},
			validate: func(respBytes []byte) bool {
				var resp llm.Response
				err := json.Unmarshal(respBytes, &resp)
				if err != nil {
					return false
				}
				if len(resp.Choices) == 0 {
					return false
				}
				// Should still aggregate valid chunks, skipping invalid ones
				return *resp.Choices[0].Message.Content.Content == "Hello world"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := transformer.AggregateStreamChunks(t.Context(), tt.chunks)

			if tt.wantErr {
				if err == nil {
					t.Errorf("AggregateStreamChunks() expected error, got nil")
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf(
						"AggregateStreamChunks() error = %v, want error containing %v",
						err,
						tt.errContains,
					)
				}

				return
			}

			if err != nil {
				t.Errorf("AggregateStreamChunks() unexpected error = %v", err)
				return
			}

			if tt.validate != nil && !tt.validate(resp) {
				t.Errorf("AggregateStreamChunks() validation failed for response: %+v", resp)
			}
		})
	}
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewOutboundTransformer("", "test-key").(*OutboundTransformer)

	tests := []struct {
		name        string
		response    *httpclient.Response
		wantErr     bool
		errContains string
		validate    func(*llm.Response) bool
	}{
		{
			name: "valid response",
			response: &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body: mustMarshal(llm.Response{
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
				}),
			},
			wantErr: false,
			validate: func(resp *llm.Response) bool {
				return resp.ID == "chatcmpl-123" &&
					resp.Model == "gpt-4" &&
					len(resp.Choices) == 1 &&
					resp.Choices[0].Message.Content.Content != nil &&
					*resp.Choices[0].Message.Content.Content == "Hello! How can I help you today?"
			},
		},
		{
			name:        "nil response",
			response:    nil,
			wantErr:     true,
			errContains: "http response is nil",
		},
		{
			name: "HTTP error response",
			response: &httpclient.Response{
				StatusCode: http.StatusBadRequest,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte(`{"error": "Bad request"}`),
			},
			wantErr:     true,
			errContains: "HTTP error 400",
		},
		{
			name: "HTTP error with error object",
			response: &httpclient.Response{
				StatusCode: http.StatusUnauthorized,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte(`{"error": "Unauthorized"}`),
				Error: &httpclient.ResponseError{
					Code:    "HTTP_401",
					Message: "Unauthorized access",
					Type:    "http_error",
				},
			},
			wantErr:     true,
			errContains: "Unauthorized access",
		},
		{
			name: "empty response body",
			response: &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte{},
			},
			wantErr:     true,
			errContains: "response body is empty",
		},
		{
			name: "invalid JSON response",
			response: &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte("invalid json"),
			},
			wantErr:     true,
			errContains: "failed to unmarshal chat completion response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(t.Context(), tt.response)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformResponse() expected error but got none")
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
				t.Errorf("TransformResponse() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("TransformResponse() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("TransformResponse() validation failed for result: %+v", result)
			}
		})
	}
}

func TestOutboundTransformer_SetAPIKey(t *testing.T) {
	transformer := NewOutboundTransformer("", "initial-key").(*OutboundTransformer)

	newKey := "new-api-key"
	transformer.SetAPIKey(newKey)

	if transformer.apiKey != newKey {
		t.Errorf("SetAPIKey() failed, got %v, want %v", transformer.apiKey, newKey)
	}
}

func TestOutboundTransformer_SetBaseURL(t *testing.T) {
	transformer := NewOutboundTransformer("initial-url", "test-key").(*OutboundTransformer)

	newURL := "https://new.api.com/v1"
	transformer.SetBaseURL(newURL)

	if transformer.baseURL != newURL {
		t.Errorf("SetBaseURL() failed, got %v, want %v", transformer.baseURL, newURL)
	}
}

func TestNewOutboundTransformer(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		wantURL string
	}{
		{
			name:    "empty base URL uses default",
			baseURL: "",
			apiKey:  "test-key",
			wantURL: "https://api.openai.com/v1",
		},
		{
			name:    "custom base URL",
			baseURL: "https://custom.api.com/v1",
			apiKey:  "test-key",
			wantURL: "https://custom.api.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer := NewOutboundTransformer(tt.baseURL, tt.apiKey).(*OutboundTransformer)

			if transformer.baseURL != tt.wantURL {
				t.Errorf(
					"NewOutboundTransformer() baseURL = %v, want %v",
					transformer.baseURL,
					tt.wantURL,
				)
			}

			if transformer.apiKey != tt.apiKey {
				t.Errorf(
					"NewOutboundTransformer() apiKey = %v, want %v",
					transformer.apiKey,
					tt.apiKey,
				)
			}
		})
	}
}
