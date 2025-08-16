package openai

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestOutboundTransformer_AzureConfigurations(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*OutboundTransformer)
		expectedURL    string
		expectedHeader string
		model          string
		stream         bool
	}{
		{
			name: "Standard OpenAI API",
			setupFunc: func(transformer *OutboundTransformer) {
				// Default configuration - no setup needed
			},
			expectedURL: "https://api.openai.com/v1/chat/completions",
			model:       "gpt-4",
			stream:      false,
		},
		{
			name: "Azure OpenAI - with deployment name",
			setupFunc: func(transformer *OutboundTransformer) {
				err := transformer.ConfigureForAzure("my-resource", "2024-06-01", "test-key")
				require.NoError(t, err)
				// Note: Current implementation uses model name as deployment
			},
			expectedURL: "https://my-resource.openai.azure.com/openai/deployments/gpt-4/chat/completions?api-version=2024-06-01",
			model:       "gpt-4",
			stream:      false,
		},
		{
			name: "Azure OpenAI - using model name as deployment",
			setupFunc: func(transformer *OutboundTransformer) {
				err := transformer.ConfigureForAzure("another-resource", "2024-02-01", "azure-key")
				require.NoError(t, err)
				// No deployment name set, should use model name
			},
			expectedURL: "https://another-resource.openai.azure.com/openai/deployments/gpt-3.5-turbo/chat/completions?api-version=2024-02-01",
			model:       "gpt-3.5-turbo",
			stream:      false,
		},
		{
			name: "Azure OpenAI - with special characters in deployment",
			setupFunc: func(transformer *OutboundTransformer) {
				err := transformer.ConfigureForAzure("test-resource", "2024-06-01", "key123")
				require.NoError(t, err)
				// Note: Current implementation uses model name as deployment
			},
			expectedURL: "https://test-resource.openai.azure.com/openai/deployments/gpt-4-turbo-preview/chat/completions?api-version=2024-06-01",
			model:       "gpt-4-turbo-preview",
			stream:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create transformer
			transformerInterface, err := NewOutboundTransformer("https://api.openai.com/v1", "test-api-key")
			require.NoError(t, err)

			transformer := transformerInterface.(*OutboundTransformer)

			// Apply platform-specific setup
			tt.setupFunc(transformer)

			// Create test request
			req := &llm.Request{
				Model: tt.model,
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

			// Verify Content-Type header
			assert.Equal(t, "application/json", httpReq.Headers.Get("Content-Type"))

			// Verify authentication based on platform
			if transformer.GetConfig().Type == PlatformAzure {
				require.NotNil(t, httpReq.Auth)
			} else {
				// Standard OpenAI uses Bearer token
				require.NotNil(t, httpReq.Auth)
				assert.Equal(t, "bearer", httpReq.Auth.Type)
				assert.Equal(t, "test-api-key", httpReq.Auth.APIKey)
				assert.Equal(t, "", httpReq.Headers.Get("Api-Key")) // No Api-Key header for standard OpenAI
			}
		})
	}
}

func TestOutboundTransformer_AzureConfigurationErrors(t *testing.T) {
	transformerInterface, err := NewOutboundTransformer("https://api.openai.com/v1", "test-api-key")
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	t.Run("Azure - Missing API version", func(t *testing.T) {
		err := transformer.ConfigureForAzure("my-resource", "", "key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API version is required")
	})

	t.Run("Azure - Missing API version in URL building", func(t *testing.T) {
		// This test should panic during SetConfig because API version is missing
		assert.Panics(t, func() {
			transformer.SetConfig(&Config{
				Type:    PlatformAzure,
				BaseURL: "https://test.openai.azure.com",
				APIKey:  "test-key", // Add API key to avoid that validation error
				// APIVersion is missing - this should cause validation to fail
			})
		})
	})
}

func TestOutboundTransformer_NewOutboundTransformerWithConfig(t *testing.T) {
	t.Run("Standard OpenAI configuration", func(t *testing.T) {
		config := &Config{
			Type:    PlatformOpenAI,
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "test-key",
		}

		transformerInterface, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		transformer := transformerInterface.(*OutboundTransformer)

		assert.Equal(t, "https://api.openai.com/v1", transformer.config.BaseURL)
		assert.Equal(t, PlatformOpenAI, transformer.config.Type)
		assert.Equal(t, "test-key", transformer.config.APIKey)
	})

	t.Run("Azure OpenAI configuration", func(t *testing.T) {
		config := &Config{
			Type:       PlatformAzure,
			BaseURL:    "https://my-azure-resource.openai.azure.com",
			APIVersion: "2024-06-01",
			APIKey:     "azure-key",
		}

		transformerInterface, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		transformer := transformerInterface.(*OutboundTransformer)

		assert.Equal(t, "https://my-azure-resource.openai.azure.com", transformer.config.BaseURL)
		assert.Equal(t, PlatformAzure, transformer.config.Type)
		assert.Equal(t, "2024-06-01", transformer.config.APIVersion)
		assert.Equal(t, "azure-key", transformer.config.APIKey)
	})

	t.Run("Custom base URL overrides default", func(t *testing.T) {
		config := &Config{
			Type:    PlatformOpenAI,
			BaseURL: "https://custom-openai-proxy.example.com",
			APIKey:  "test-key",
		}

		transformerInterface, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		transformer := transformerInterface.(*OutboundTransformer)

		assert.Equal(t, "https://custom-openai-proxy.example.com", transformer.config.BaseURL)
	})

	t.Run("Azure with custom base URL", func(t *testing.T) {
		config := &Config{
			Type:       PlatformAzure,
			BaseURL:    "https://custom-azure-endpoint.example.com",
			APIVersion: "2024-06-01",
			APIKey:     "azure-key",
		}

		transformerInterface, err := NewOutboundTransformerWithConfig(config)
		require.NoError(t, err)

		transformer := transformerInterface.(*OutboundTransformer)

		assert.Equal(t, "https://custom-azure-endpoint.example.com", transformer.config.BaseURL)
		assert.Equal(t, PlatformAzure, transformer.config.Type)
	})
}
