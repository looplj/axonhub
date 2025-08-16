package anthropic

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestOutboundTransformer_PlatformConfigurations(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*OutboundTransformer)
		expectedURL    string
		expectedHeader string
		model          string
		stream         bool
	}{
		{
			name: "Direct Anthropic API",
			setupFunc: func(transformer *OutboundTransformer) {
				// Default configuration - no setup needed
			},
			expectedURL:    "https://api.anthropic.com/v1/messages",
			expectedHeader: "2023-06-01",
			model:          "claude-3-sonnet-20240229",
			stream:         false,
		},
		{
			name: "AWS Bedrock - Non-streaming",
			setupFunc: func(transformer *OutboundTransformer) {
				transformer.ConfigureForBedrock("us-east-1")
			},
			expectedURL:    "https://bedrock-runtime.us-east-1.amazonaws.com/model/claude-3-sonnet-20240229/invoke",
			expectedHeader: "bedrock-2023-05-31",
			model:          "claude-3-sonnet-20240229",
			stream:         false,
		},
		{
			name: "AWS Bedrock - Streaming",
			setupFunc: func(transformer *OutboundTransformer) {
				transformer.ConfigureForBedrock("us-west-2")
			},
			expectedURL:    "https://bedrock-runtime.us-west-2.amazonaws.com/model/claude-3-sonnet-20240229/invoke-with-response-stream",
			expectedHeader: "bedrock-2023-05-31",
			model:          "claude-3-sonnet-20240229",
			stream:         true,
		},
		{
			name: "Google Vertex AI - Non-streaming",
			setupFunc: func(transformer *OutboundTransformer) {
				err := transformer.ConfigureForVertex("us-central1", "my-project-123")
				require.NoError(t, err)
			},
			expectedURL:    "https://us-central1-aiplatform.googleapis.com/v1/projects/my-project-123/locations/us-central1/publishers/anthropic/models/claude-3-sonnet-20240229:rawPredict",
			expectedHeader: "vertex-2023-10-16",
			model:          "claude-3-sonnet-20240229",
			stream:         false,
		},
		{
			name: "Google Vertex AI - Streaming",
			setupFunc: func(transformer *OutboundTransformer) {
				err := transformer.ConfigureForVertex("europe-west1", "my-project-456")
				require.NoError(t, err)
			},
			expectedURL:    "https://europe-west1-aiplatform.googleapis.com/v1/projects/my-project-456/locations/europe-west1/publishers/anthropic/models/claude-3-sonnet-20240229:streamRawPredict",
			expectedHeader: "vertex-2023-10-16",
			model:          "claude-3-sonnet-20240229",
			stream:         true,
		},
		{
			name: "Google Vertex AI - Global region",
			setupFunc: func(transformer *OutboundTransformer) {
				err := transformer.ConfigureForVertex("global", "my-project-789")
				require.NoError(t, err)
			},
			expectedURL:    "https://aiplatform.googleapis.com/v1/projects/my-project-789/locations/global/publishers/anthropic/models/claude-3-sonnet-20240229:rawPredict",
			expectedHeader: "vertex-2023-10-16",
			model:          "claude-3-sonnet-20240229",
			stream:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create transformer
			transformer, _ := NewOutboundTransformer("", "test-api-key")

			// Apply platform-specific setup
			tt.setupFunc(transformer.(*OutboundTransformer))

			// Create test request
			maxTokens := int64(1000)
			req := &llm.Request{
				Model:     tt.model,
				MaxTokens: &maxTokens,
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
				Stream: &tt.stream,
			}

			// Transform request
			httpReq, err := transformer.TransformRequest(context.Background(), req)
			require.NoError(t, err)
			require.NotNil(t, httpReq)

			// Verify URL
			assert.Equal(t, tt.expectedURL, httpReq.URL)

			// Verify Anthropic-Version header
			assert.Equal(t, tt.expectedHeader, httpReq.Headers.Get("Anthropic-Version"))

			// Verify Content-Type header
			assert.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))

			// Verify authentication is only set for direct API
			if transformer.(*OutboundTransformer).GetConfig().Type == PlatformDirect {
				require.NotNil(t, httpReq.Auth)
				assert.Equal(t, "api_key", httpReq.Auth.Type)
				assert.Equal(t, "test-api-key", httpReq.Auth.APIKey)
				assert.Equal(t, "X-API-Key", httpReq.Auth.HeaderKey)
			} else {
				assert.Nil(t, httpReq.Auth)
			}
		})
	}
}

func TestOutboundTransformer_PlatformConfigurationErrors(t *testing.T) {
	transformer, _ := NewOutboundTransformer("", "test-api-key")

	t.Run("Vertex AI - Missing region", func(t *testing.T) {
		err := transformer.(*OutboundTransformer).ConfigureForVertex("", "my-project")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "region is required")
	})

	t.Run("Vertex AI - Missing project ID", func(t *testing.T) {
		err := transformer.(*OutboundTransformer).ConfigureForVertex("us-central1", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project ID is required")
	})
}
