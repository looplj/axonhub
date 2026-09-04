package zai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
)

func TestOutboundTransformer_TransformRequest_URL(t *testing.T) {
	// Helper function to create transformer
	createTransformer := func(baseURL, apiKey string) *OutboundTransformer {
		config := &Config{
			BaseURL:        baseURL,
			APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
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
		BaseURL:        "https://api.zai.com/v4",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
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
		BaseURL:        "https://api.zai.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
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

func TestOutboundTransformer_TransformRequest_ResponseFormat(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.zai.com/v4",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	tests := []struct {
		name                  string
		request               *llm.Request
		expectedType          string
		expectedJSONSchemaNil bool
	}{
		{
			name: "json_schema converted to json_object",
			request: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type:       "json_schema",
					JSONSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
				},
			},
			expectedType:          "json_object",
			expectedJSONSchemaNil: true,
		},
		{
			name: "json_object remains unchanged",
			request: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type: "json_object",
				},
			},
			expectedType:          "json_object",
			expectedJSONSchemaNil: true,
		},
		{
			name: "text remains unchanged",
			request: &llm.Request{
				Model: "gpt-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type: "text",
				},
			},
			expectedType:          "text",
			expectedJSONSchemaNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := transformer.TransformRequest(ctx, tt.request)

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, http.MethodPost, got.Method)

			var zaiReq Request

			err = json.Unmarshal(got.Body, &zaiReq)
			assert.NoError(t, err)

			assert.NotNil(t, zaiReq.ResponseFormat)
			assert.Equal(t, tt.expectedType, zaiReq.ResponseFormat.Type)

			if tt.expectedJSONSchemaNil {
				assert.Nil(t, zaiReq.ResponseFormat.JSONSchema)
			}
		})
	}
}

// TestOutboundTransformer_TransformRequest_URLOverrides covers the endpoint-level
// URL conventions that must match the generic OpenAI transformer: custom endpoint
// paths and the "##" raw-URL suffix. Custom chat endpoints on zhipu/zai family
// channels are routed to this transformer, so a bare base URL like
// https://open.bigmodel.cn/api/coding/paas/v4 must keep working.
func TestOutboundTransformer_TransformRequest_URLOverrides(t *testing.T) {
	newRequest := func() *llm.Request {
		return &llm.Request{
			Model: "glm-5.3-flash",
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello, world!"),
					},
				},
			},
		}
	}

	tests := []struct {
		name         string
		baseURL      string
		endpointPath string
		expectedURL  string
	}{
		{
			name:        "coding plan base URL without path appends /chat/completions only",
			baseURL:     "https://open.bigmodel.cn/api/coding/paas/v4",
			expectedURL: "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
		{
			name:        "base URL without version gets /v4 appended",
			baseURL:     "https://open.bigmodel.cn/api/coding/paas",
			expectedURL: "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
		{
			name:         "endpoint path replaces the default path and skips version normalization",
			baseURL:      "https://open.bigmodel.cn/api/coding/paas/v4",
			endpointPath: "/chat/completions",
			expectedURL:  "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
		{
			name:        "## suffix uses the base URL as raw request URL",
			baseURL:     "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions##",
			expectedURL: "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformerWithConfig(&Config{
				BaseURL:        tt.baseURL,
				EndpointPath:   tt.endpointPath,
				APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
			})
			assert.NoError(t, err)

			httpReq, err := transformer.TransformRequest(context.Background(), newRequest())
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedURL, httpReq.URL)
		})
	}
}

// TestOutboundTransformer_TransformRequest_StripsMetadata asserts the zai request
// body carries no metadata field (GLM coding plan rejects it with error 1210);
// user_id/request_id are extracted to their dedicated top-level fields instead.
func TestOutboundTransformer_TransformRequest_StripsMetadata(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://open.bigmodel.cn/api/coding/paas/v4",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	})
	assert.NoError(t, err)

	httpReq, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model: "glm-5.3-flash",
		Metadata: map[string]string{
			"user_id": "user-123",
		},
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello"),
				},
			},
		},
	})
	assert.NoError(t, err)

	var body map[string]any
	assert.NoError(t, json.Unmarshal(httpReq.Body, &body))
	assert.NotContains(t, body, "metadata")
	assert.Equal(t, "user-123", body["user_id"])
}

// TestOutboundTransformer_TransformRequest_UserIDLength asserts the GLM user_id
// constraint (6-128 chars, error 1214): Claude Code's long JSON-blob user_id is
// truncated, short values are dropped entirely.
func TestOutboundTransformer_TransformRequest_UserIDLength(t *testing.T) {
	transformer, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://open.bigmodel.cn/api/coding/paas/v4",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	})
	assert.NoError(t, err)

	buildRequest := func(userID string) *llm.Request {
		return &llm.Request{
			Model:    "glm-5.3-flash",
			Metadata: map[string]string{"user_id": userID},
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
				},
			},
		}
	}

	t.Run("long user_id is truncated to 128 chars", func(t *testing.T) {
		long := `{"device_id":"334251bf6b147d22d006e2e9cb990169a3ba5e77fecb52a03473e32eb1afe62e","account_uuid":"","session_id":"db0eef37-75a9-4288-8098-0557997cd879"}`
		require.Len(t, []rune(long), 150)

		httpReq, err := transformer.TransformRequest(context.Background(), buildRequest(long))
		assert.NoError(t, err)

		var body map[string]any
		assert.NoError(t, json.Unmarshal(httpReq.Body, &body))
		assert.Len(t, body["user_id"], 128)
	})

	t.Run("short user_id is omitted", func(t *testing.T) {
		httpReq, err := transformer.TransformRequest(context.Background(), buildRequest("abc"))
		assert.NoError(t, err)

		var body map[string]any
		assert.NoError(t, json.Unmarshal(httpReq.Body, &body))
		assert.NotContains(t, body, "user_id")
	})

	t.Run("normal user_id passes through", func(t *testing.T) {
		httpReq, err := transformer.TransformRequest(context.Background(), buildRequest("user-123456"))
		assert.NoError(t, err)

		var body map[string]any
		assert.NoError(t, json.Unmarshal(httpReq.Body, &body))
		assert.Equal(t, "user-123456", body["user_id"])
	})
}

func TestOutboundTransformer_TransformRequest_GLM53NoneMapsToLow(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.zai.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	request := &llm.Request{
		Model:           "glm-5.3-flash",
		ReasoningEffort: "none",
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

	var zaiReq Request
	err = json.Unmarshal(got.Body, &zaiReq)
	assert.NoError(t, err)
	require.NotNil(t, zaiReq.Thinking)
	assert.Equal(t, "enabled", zaiReq.Thinking.Type)
	assert.Equal(t, "low", zaiReq.ReasoningEffort)
}

func TestOutboundTransformer_TransformRequest_NonGLMStripsReasoningEffort(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.zai.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
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

	var zaiReq Request
	err = json.Unmarshal(got.Body, &zaiReq)
	assert.NoError(t, err)
	require.NotNil(t, zaiReq.Thinking)
	assert.Equal(t, "enabled", zaiReq.Thinking.Type)
	assert.Equal(t, "", zaiReq.ReasoningEffort)
}
