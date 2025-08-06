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

// func TestInboundTransformer_AggregateStreamChunks(t *testing.T) {
// 	transformer := NewInboundTransformer()

// 	tests := []struct {
// 		name     string
// 		chunks   []*llm.Response
// 		wantErr  bool
// 		validate func([]byte) bool
// 	}{
// 		{
// 			name:    "empty chunks",
// 			chunks:  []*llm.Response{},
// 			wantErr: false,
// 			validate: func(data []byte) bool {
// 				var resp llm.Response
// 				err := json.Unmarshal(data, &resp)
// 				return err == nil
// 			},
// 		},
// 		{
// 			name: "single chunk with content",
// 			chunks: []*llm.Response{
// 				{
// 					ID:      "chatcmpl-123",
// 					Object:  "chat.completion.chunk",
// 					Created: 1677652288,
// 					Model:   "gpt-4",
// 					Choices: []llm.Choice{
// 						{
// 							Index: 0,
// 							Delta: &llm.Message{
// 								Role: "assistant",
// 								Content: llm.MessageContent{
// 									Content: lo.ToPtr("Hello, world!"),
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 			wantErr: false,
// 			validate: func(data []byte) bool {
// 				var resp llm.Response
// 				err := json.Unmarshal(data, &resp)
// 				if err != nil {
// 					return false
// 				}
// 				return resp.Object == "chat.completion" &&
// 					len(resp.Choices) == 1 &&
// 					resp.Choices[0].Message != nil &&
// 					resp.Choices[0].Message.Content.Content != nil &&
// 					*resp.Choices[0].Message.Content.Content == "Hello, world!"
// 			},
// 		},
// 		{
// 			name: "multiple chunks with content",
// 			chunks: []*llm.Response{
// 				{
// 					ID:      "chatcmpl-123",
// 					Object:  "chat.completion.chunk",
// 					Created: 1677652288,
// 					Model:   "gpt-4",
// 					Choices: []llm.Choice{
// 						{
// 							Index: 0,
// 							Delta: &llm.Message{
// 								Role: "assistant",
// 								Content: llm.MessageContent{
// 									Content: lo.ToPtr("Hello, "),
// 								},
// 							},
// 						},
// 					},
// 				},
// 				{
// 					ID:      "chatcmpl-123",
// 					Object:  "chat.completion.chunk",
// 					Created: 1677652288,
// 					Model:   "gpt-4",
// 					Choices: []llm.Choice{
// 						{
// 							Index: 0,
// 							Delta: &llm.Message{
// 								Role: "assistant",
// 								Content: llm.MessageContent{
// 									Content: lo.ToPtr("world!"),
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 			wantErr: false,
// 			validate: func(data []byte) bool {
// 				var resp llm.Response
// 				err := json.Unmarshal(data, &resp)
// 				if err != nil {
// 					return false
// 				}
// 				return resp.Object == "chat.completion" &&
// 					len(resp.Choices) == 1 &&
// 					resp.Choices[0].Message != nil &&
// 					resp.Choices[0].Message.Content.Content != nil &&
// 					*resp.Choices[0].Message.Content.Content == "Hello, world!"
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result, err := transformer.AggregateStreamChunks(nil, tt.chunks)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("AggregateStreamChunks() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}

// 			if !tt.wantErr && !tt.validate(result) {
// 				t.Errorf("AggregateStreamChunks() validation failed")
// 			}
// 		})
// 	}
// }

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}
