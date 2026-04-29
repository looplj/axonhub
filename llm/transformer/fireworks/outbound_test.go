package fireworks

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestStripReasoningFromMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []openai.Message
		validate func(*testing.T, []openai.Message)
	}{
		{
			name: "strips reasoning field from user message",
			messages: []openai.Message{
				{
					Role:      "user",
					Content:   openai.MessageContent{Content: lo.ToPtr("Hello")},
					Reasoning: lo.ToPtr("thinking about the question"),
				},
			},
			validate: func(t *testing.T, messages []openai.Message) {
				assert.Nil(t, messages[0].Reasoning, "Reasoning should be nil")
			},
		},
		{
			name: "strips reasoning_content field from assistant message",
			messages: []openai.Message{
				{
					Role:             "assistant",
					Content:          openai.MessageContent{Content: lo.ToPtr("Hello")},
					ReasoningContent: lo.ToPtr("reasoning content here"),
				},
			},
			validate: func(t *testing.T, messages []openai.Message) {
				assert.Nil(t, messages[0].ReasoningContent, "ReasoningContent should be nil")
			},
		},
		{
			name: "strips both reasoning and reasoning_content",
			messages: []openai.Message{
				{
					Role:             "assistant",
					Content:          openai.MessageContent{Content: lo.ToPtr("Hello")},
					Reasoning:        lo.ToPtr("reasoning"),
					ReasoningContent: lo.ToPtr("reasoning content"),
				},
			},
			validate: func(t *testing.T, messages []openai.Message) {
				assert.Nil(t, messages[0].Reasoning, "Reasoning should be nil")
				assert.Nil(t, messages[0].ReasoningContent, "ReasoningContent should be nil")
			},
		},
		{
			name: "handles nil request",
			validate: func(t *testing.T, messages []openai.Message) {
				assert.Nil(t, messages)
			},
		},
		{
			name:     "handles empty messages",
			messages: []openai.Message{},
			validate: func(t *testing.T, messages []openai.Message) {
				assert.Empty(t, messages)
			},
		},
		{
			name: "preserves content field",
			messages: []openai.Message{
				{
					Role:    "user",
					Content: openai.MessageContent{Content: lo.ToPtr("Hello world")},
				},
			},
			validate: func(t *testing.T, messages []openai.Message) {
				assert.NotNil(t, messages[0].Content.Content)
				assert.Equal(t, "Hello world", *messages[0].Content.Content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &openai.Request{
				Messages: tt.messages,
			}
			stripReasoningFromMessages(req)
			tt.validate(t, req.Messages)
		})
	}
}

func TestNewOutboundTransformerWithConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		errContains string
		validate    func(*testing.T, *OutboundTransformer)
	}{
		{
			name:        "nil config returns error",
			config:      nil,
			wantErr:     true,
			errContains: "config is nil",
		},
		{
			name: "nil APIKeyProvider returns error",
			config: &Config{
				BaseURL:        "https://api.fireworks.ai/inference/v1",
				APIKeyProvider: nil,
			},
			wantErr:     true,
			errContains: "API key provider is required",
		},
		{
			name: "empty baseURL uses default",
			config: &Config{
				BaseURL:        "",
				APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
			},
			wantErr: false,
			validate: func(t *testing.T, transformer *OutboundTransformer) {
				assert.Equal(t, DefaultBaseURL, transformer.BaseURL)
			},
		},
		{
			name: "custom baseURL is used",
			config: &Config{
				BaseURL:        "https://custom.fireworks.ai/v1",
				APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
			},
			wantErr: false,
			validate: func(t *testing.T, transformer *OutboundTransformer) {
				assert.Equal(t, "https://custom.fireworks.ai/v1", transformer.BaseURL)
			},
		},
		{
			name: "baseURL trailing slash is trimmed",
			config: &Config{
				BaseURL:        "https://api.fireworks.ai/inference/v1/",
				APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
			},
			wantErr: false,
			validate: func(t *testing.T, transformer *OutboundTransformer) {
				assert.Equal(t, "https://api.fireworks.ai/inference/v1", transformer.BaseURL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformerInterface, err := NewOutboundTransformerWithConfig(tt.config)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, transformerInterface)

			transformer := transformerInterface.(*OutboundTransformer)
			if tt.validate != nil {
				tt.validate(t, transformer)
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	createTransformer := func(baseURL, apiKey string) *OutboundTransformer {
		transformerInterface, err := NewOutboundTransformer(baseURL, apiKey)
		if err != nil {
			t.Fatalf("Failed to create transformer: %v", err)
		}

		return transformerInterface.(*OutboundTransformer)
	}

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
			transformer: createTransformer("https://api.fireworks.ai/inference/v1", "test-api-key"),
			request: &llm.Request{
				Model: "fireworks-ai/llama-3.1-70b-instruct",
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
					req.URL == "https://api.fireworks.ai/inference/v1/chat/completions" &&
					req.Headers.Get("Content-Type") == "application/json" &&
					req.Auth != nil &&
					req.Auth.Type == "bearer" &&
					req.Auth.APIKey == "test-api-key"
			},
		},
		{
			name:        "valid request with custom URL",
			transformer: createTransformer("https://custom.fireworks.ai/v1", "test-key"),
			request: &llm.Request{
				Model: "fireworks-ai/llama-3.1-70b-instruct",
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
				return req.URL == "https://custom.fireworks.ai/v1/chat/completions"
			},
		},
		{
			name:        "nil request",
			transformer: createTransformer("https://api.fireworks.ai/inference/v1", "test-key"),
			request:     nil,
			wantErr:     true,
			errContains: "chat completion request is nil",
		},
		{
			name:        "missing model",
			transformer: createTransformer("https://api.fireworks.ai/inference/v1", "test-key"),
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
			transformer: createTransformer("https://api.fireworks.ai/inference/v1", "test-key"),
			request: &llm.Request{
				Model: "fireworks-ai/llama-3.1-70b-instruct",
			},
			wantErr:     true,
			errContains: "messages are required",
		},
		{
			name:        "URL with trailing slash",
			transformer: createTransformer("https://api.fireworks.ai/inference/v1/", "test-key"),
			request: &llm.Request{
				Model: "fireworks-ai/llama-3.1-70b-instruct",
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
				return req.URL == "https://api.fireworks.ai/inference/v1/chat/completions"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.transformer.TransformRequest(context.Background(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformRequest() expected error but got none")
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("TransformRequest() error = %v, want error containing %v", err, tt.errContains)
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
				var unmarshaled openai.Request

				err := json.Unmarshal(result.Body, &unmarshaled)
				if err != nil {
					t.Errorf("TransformRequest() body is not valid JSON: %v", err)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest_StripsReasoningFields(t *testing.T) {
	transformerInterface, err := NewOutboundTransformer("https://api.fireworks.ai/inference/v1", "test-key")
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	transformer := transformerInterface.(*OutboundTransformer)

	httpReq, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model: "fireworks-ai/llama-3.1-70b-instruct",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello"),
				},
				Reasoning: lo.ToPtr("user reasoning"),
			},
			{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hi there"),
				},
				Reasoning:        lo.ToPtr("assistant reasoning"),
				ReasoningContent: lo.ToPtr("reasoning content"),
			},
		},
	})
	if err != nil {
		t.Fatalf("TransformRequest() unexpected error = %v", err)
	}

	var oaiReq openai.Request
	if err := json.Unmarshal(httpReq.Body, &oaiReq); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}

	// Verify reasoning fields are stripped
	assert.Len(t, oaiReq.Messages, 2)
	assert.Nil(t, oaiReq.Messages[0].Reasoning, "user message reasoning should be nil")
	assert.Nil(t, oaiReq.Messages[1].Reasoning, "assistant message reasoning should be nil")
	assert.Nil(t, oaiReq.Messages[1].ReasoningContent, "assistant message reasoning_content should be nil")

	// Verify content is preserved
	assert.NotNil(t, oaiReq.Messages[0].Content.Content)
	assert.Equal(t, "Hello", *oaiReq.Messages[0].Content.Content)
	assert.NotNil(t, oaiReq.Messages[1].Content.Content)
	assert.Equal(t, "Hi there", *oaiReq.Messages[1].Content.Content)
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	transformerInterface, err := NewOutboundTransformer("https://api.fireworks.ai/inference/v1", "test-key")
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	transformer := transformerInterface.(*OutboundTransformer)

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
					Model:   "fireworks-ai/llama-3.1-70b-instruct",
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
					resp.Model == "fireworks-ai/llama-3.1-70b-instruct" &&
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
			result, err := transformer.TransformResponse(context.Background(), tt.response)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformResponse() expected error but got none")
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("TransformResponse() error = %v, want error containing %v", err, tt.errContains)
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
	transformerInterface, err := NewOutboundTransformer("https://api.fireworks.ai/inference/v1", "initial-key")
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	transformer := transformerInterface.(*OutboundTransformer)

	newKey := "new-api-key"
	transformer.APIKeyProvider = auth.NewStaticKeyProvider(newKey)

	apiKey := transformer.APIKeyProvider.Get(context.Background())
	if apiKey != newKey {
		t.Errorf("SetAPIKey() failed, got %v, want %v", apiKey, newKey)
	}
}

func TestOutboundTransformer_SetBaseURL(t *testing.T) {
	transformerInterface, err := NewOutboundTransformer("initial-url", "test-key")
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	transformer := transformerInterface.(*OutboundTransformer)

	newURL := "https://new.api.com/v1"
	transformer.BaseURL = newURL

	if transformer.BaseURL != newURL {
		t.Errorf("SetBaseURL() failed, got %v, want %v", transformer.BaseURL, newURL)
	}
}

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}
