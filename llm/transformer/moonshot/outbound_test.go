package moonshot

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestOutboundTransformer_TransformRequest_ResponseFormat(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.moonshot.cn/v1",
		APIKey:  "test-api-key",
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	tests := []struct {
		name                  string
		request               *llm.Request
		expectedType          string
		expectedJSONSchemaNil bool
	}{
		{
			name: "json_schema converted to json_object",
			request: &llm.Request{
				Model: "moonshot-v1-8k",
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
				Model: "moonshot-v1-8k",
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
				Model: "moonshot-v1-8k",
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

			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, http.MethodPost, got.Method)

			var oaiReq openai.Request

			err = json.Unmarshal(got.Body, &oaiReq)
			require.NoError(t, err)

			assert.NotNil(t, oaiReq.ResponseFormat)
			assert.Equal(t, tt.expectedType, oaiReq.ResponseFormat.Type)

			if tt.expectedJSONSchemaNil {
				assert.Nil(t, oaiReq.ResponseFormat.JSONSchema)
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest_URL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "base URL ending with /v1",
			baseURL:     "https://api.moonshot.cn/v1",
			expectedURL: "https://api.moonshot.cn/v1/chat/completions",
		},
		{
			name:        "base URL without /v1 suffix",
			baseURL:     "https://api.moonshot.cn",
			expectedURL: "https://api.moonshot.cn/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				BaseURL: tt.baseURL,
				APIKey:  "test-api-key",
			}

			transformer, err := NewOutboundTransformerWithConfig(config)
			require.NoError(t, err)

			request := &llm.Request{
				Model: "moonshot-v1-8k",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			}

			ctx := context.Background()
			got, err := transformer.TransformRequest(ctx, request)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, got.URL)
		})
	}
}

func TestOutboundTransformer_TransformRequest_Basic(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.moonshot.cn/v1",
		APIKey:  "test-api-key",
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	request := &llm.Request{
		Model: "moonshot-v1-8k",
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

	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "https://api.moonshot.cn/v1/chat/completions", got.URL)
	assert.Equal(t, "application/json", got.Headers.Get("Content-Type"))
	assert.Equal(t, "application/json", got.Headers.Get("Accept"))
	assert.NotNil(t, got.Auth)
	assert.Equal(t, "bearer", got.Auth.Type)
	assert.Equal(t, "test-api-key", got.Auth.APIKey)

	var oaiReq openai.Request

	err = json.Unmarshal(got.Body, &oaiReq)
	require.NoError(t, err)
	assert.Equal(t, "moonshot-v1-8k", oaiReq.Model)
	assert.Len(t, oaiReq.Messages, 1)
}

func TestOutboundTransformer_TransformRequest_Errors(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.moonshot.cn/v1",
		APIKey:  "test-api-key",
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	tests := []struct {
		name        string
		request     *llm.Request
		errContains string
	}{
		{
			name: "empty messages",
			request: &llm.Request{
				Model:    "moonshot-v1-8k",
				Messages: []llm.Message{},
			},
			errContains: "messages are required",
		},
		{
			name: "unsupported request type",
			request: &llm.Request{
				Model:       "moonshot-v1-8k",
				RequestType: llm.RequestTypeEmbedding,
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			errContains: "is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := transformer.TransformRequest(ctx, tt.request)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}
