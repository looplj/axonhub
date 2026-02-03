package deepseek

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
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestOutboundTransformer_TransformRequest_ResponseFormat(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.deepseek.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
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
				Model: "deepseek-chat",
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
				Model: "deepseek-chat",
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
				Model: "deepseek-chat",
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

			var dsReq Request

			err = json.Unmarshal(got.Body, &dsReq)
			require.NoError(t, err)

			assert.NotNil(t, dsReq.ResponseFormat)
			assert.Equal(t, tt.expectedType, dsReq.ResponseFormat.Type)

			if tt.expectedJSONSchemaNil {
				assert.Nil(t, dsReq.ResponseFormat.JSONSchema)
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest_Thinking(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.deepseek.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	tests := []struct {
		name            string
		reasoningEffort string
		expectThinking  bool
	}{
		{
			name:            "reasoning effort high enables thinking",
			reasoningEffort: "high",
			expectThinking:  true,
		},
		{
			name:            "reasoning effort medium enables thinking",
			reasoningEffort: "medium",
			expectThinking:  true,
		},
		{
			name:            "reasoning effort none disables thinking",
			reasoningEffort: "none",
			expectThinking:  false,
		},
		{
			name:            "empty reasoning effort disables thinking",
			reasoningEffort: "",
			expectThinking:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &llm.Request{
				Model:           "deepseek-reasoner",
				ReasoningEffort: tt.reasoningEffort,
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
			assert.NotNil(t, got)

			var dsReq Request

			err = json.Unmarshal(got.Body, &dsReq)
			require.NoError(t, err)

			if tt.expectThinking {
				assert.NotNil(t, dsReq.Thinking)
				assert.Equal(t, "enabled", dsReq.Thinking.Type)
			} else {
				assert.Nil(t, dsReq.Thinking)
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
			baseURL:     "https://api.deepseek.com/v1",
			expectedURL: "https://api.deepseek.com/v1/chat/completions",
		},
		{
			name:        "base URL without /v1 suffix",
			baseURL:     "https://api.deepseek.com",
			expectedURL: "https://api.deepseek.com/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				BaseURL:        tt.baseURL,
				APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
			}

			transformer, err := NewOutboundTransformerWithConfig(config)
			require.NoError(t, err)

			request := &llm.Request{
				Model: "deepseek-chat",
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

// Verify Request struct embeds openai.Request correctly.
func TestRequest_EmbeddedOpenAIRequest(t *testing.T) {
	dsReq := Request{
		Request: openai.Request{
			Model: "deepseek-chat",
		},
		Thinking: &Thinking{
			Type: "enabled",
		},
	}

	data, err := json.Marshal(dsReq)
	require.NoError(t, err)

	var parsed map[string]any

	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "deepseek-chat", parsed["model"])
	thinking, ok := parsed["thinking"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "enabled", thinking["type"])
}
