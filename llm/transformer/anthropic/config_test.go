package anthropic

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestOutboundTransformer_PlatformConfigurations(t *testing.T) {
	tests := []struct {
		name           string
		baseURL        string
		expectedURL    string
		expectedHeader string
		model          string
		stream         bool
	}{
		{
			name:           "Direct Anthropic API",
			baseURL:        "https://api.anthropic.com",
			expectedURL:    "https://api.anthropic.com/v1/messages",
			expectedHeader: "2023-06-01",
			model:          "claude-3-sonnet-20240229",
			stream:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create transformer with proper baseURL
			transformer, _ := NewOutboundTransformer(tt.baseURL, "test-api-key")

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
			require.Equal(t, tt.expectedURL, httpReq.URL)

			// Verify Anthropic-Version header
			require.Equal(t, tt.expectedHeader, httpReq.Headers.Get("Anthropic-Version"))

			// Verify Content-Type header
			require.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))

			// Verify authentication is only set for direct API
			if transformer.(*OutboundTransformer).GetConfig().Type == PlatformDirect {
				require.NotNil(t, httpReq.Auth)
				require.Equal(t, "api_key", httpReq.Auth.Type)
				require.Equal(t, "test-api-key", httpReq.Auth.APIKey)
				require.Equal(t, "X-API-Key", httpReq.Auth.HeaderKey)
			} else {
				require.Nil(t, httpReq.Auth)
			}
		})
	}
}
