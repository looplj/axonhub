package zai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm"
)

func TestOutboundTransformer_TransformRequest_URL(t *testing.T) {
	// Helper function to create transformer
	createTransformer := func(baseURL, apiKey string) *OutboundTransformer {
		config := &Config{
			BaseURL: baseURL,
			APIKey:  apiKey,
		}

		transformerInterface, err := NewOutboundTransformerWithConfig(config)
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
		expectedURL string
	}{
		{
			name:        "base URL ending with /v4",
			transformer: createTransformer("https://api.zai.com/v4", "test-api-key"),
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
			wantErr:     false,
			expectedURL: "https://api.zai.com/v4/chat/completions",
		},
		{
			name:        "base URL without /v4 suffix",
			transformer: createTransformer("https://api.zai.com", "test-api-key"),
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
			wantErr:     false,
			expectedURL: "https://api.zai.com/v4/chat/completions",
		},
		{
			name:        "base URL with trailing slash but no /v4",
			transformer: createTransformer("https://api.zai.com/", "test-api-key"),
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
			wantErr:     false,
			expectedURL: "https://api.zai.com/v4/chat/completions",
		},
		{
			name:        "base URL with trailing slash and /v4",
			transformer: createTransformer("https://api.zai.com/v4/", "test-api-key"),
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
			wantErr:     false,
			expectedURL: "https://api.zai.com/v4/chat/completions",
		},
		{
			name:        "base URL with path but not /v4",
			transformer: createTransformer("https://api.zai.com/v1", "test-api-key"),
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
			wantErr:     false,
			expectedURL: "https://api.zai.com/v1/v4/chat/completions",
		},
		{
			name:        "nil request",
			transformer: createTransformer("https://api.zai.com/v4", "test-api-key"),
			request:     nil,
			wantErr:     true,
			errContains: "chat completion request is nil",
		},
		{
			name:        "empty model",
			transformer: createTransformer("https://api.zai.com/v4", "test-api-key"),
			request: &llm.Request{
				Model: "",
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
			name:        "empty messages",
			transformer: createTransformer("https://api.zai.com/v4", "test-api-key"),
			request: &llm.Request{
				Model:    "gpt-4",
				Messages: []llm.Message{},
			},
			wantErr:     true,
			errContains: "messages are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := tt.transformer.TransformRequest(ctx, tt.request)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, http.MethodPost, got.Method)
			assert.Equal(t, tt.expectedURL, got.URL)
			assert.Equal(t, "application/json", got.Headers.Get("Content-Type"))
			assert.Equal(t, "application/json", got.Headers.Get("Accept"))
			assert.NotNil(t, got.Auth)
			assert.Equal(t, "bearer", got.Auth.Type)
			assert.Equal(t, "test-api-key", got.Auth.APIKey)

			// Verify the request body contains Zai-specific fields
			var zaiReq Request

			err = json.Unmarshal(got.Body, &zaiReq)
			assert.NoError(t, err)
			assert.Equal(t, tt.request.Model, zaiReq.Model)
			assert.Equal(t, len(tt.request.Messages), len(zaiReq.Messages))
			assert.Nil(t, zaiReq.Metadata)
		})
	}
}

func TestOutboundTransformer_TransformRequest_WithMetadata(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.zai.com/v4",
		APIKey:  "test-api-key",
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	request := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello, world!"),
				},
			},
		},
		Metadata: map[string]string{
			"user_id":    "test-user-123",
			"request_id": "test-request-456",
		},
	}

	ctx := context.Background()
	got, err := transformer.TransformRequest(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "https://api.zai.com/v4/chat/completions", got.URL)

	// Verify the request body contains metadata fields
	var zaiReq Request

	err = json.Unmarshal(got.Body, &zaiReq)
	assert.NoError(t, err)
	assert.Equal(t, "test-user-123", zaiReq.UserID)
	assert.Equal(t, "test-request-456", zaiReq.RequestID)
	assert.Nil(t, zaiReq.Metadata)
}

func TestOutboundTransformer_TransformRequest_WithThinking(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.zai.com",
		APIKey:  "test-api-key",
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	request := &llm.Request{
		Model:           "gpt-4",
		ReasoningEffort: "high",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello, world!"),
				},
			},
		},
	}

	ctx := context.Background()
	got, err := transformer.TransformRequest(ctx, request)

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "https://api.zai.com/v4/chat/completions", got.URL)

	// Verify the request body contains thinking field
	var zaiReq Request

	err = json.Unmarshal(got.Body, &zaiReq)
	assert.NoError(t, err)
	assert.NotNil(t, zaiReq.Thinking)
	assert.Equal(t, "enabled", zaiReq.Thinking.Type)
}
