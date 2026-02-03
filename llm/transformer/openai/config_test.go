package openai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/auth"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "config cannot be nil",
		},
		{
			name: "valid OpenAI config",
			config: &Config{
				PlatformType:   PlatformOpenAI,
				APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
				BaseURL:        "https://api.openai.com/v1",
			},
			expectError: false,
		},
		{
			name: "OpenAI config missing API key provider",
			config: &Config{
				PlatformType: PlatformOpenAI,
				BaseURL:      "https://api.openai.com/v1",
			},
			expectError: true,
			errorMsg:    "API key provider is required",
		},
		{
			name: "OpenAI config missing base URL",
			config: &Config{
				PlatformType:   PlatformOpenAI,
				APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
			},
			expectError: true,
			errorMsg:    "base URL is required",
		},
		{
			name: "valid Azure config with base URL",
			config: &Config{
				PlatformType:   PlatformAzure,
				APIKeyProvider: auth.NewStaticKeyProvider("azure-api-key"),
				APIVersion:     "2024-06-01",
				BaseURL:        "https://my-resource.openai.azure.com",
			},
			expectError: false,
		},
		{
			name: "valid Azure config with custom base URL",
			config: &Config{
				PlatformType:   PlatformAzure,
				APIKeyProvider: auth.NewStaticKeyProvider("azure-api-key"),
				APIVersion:     "2024-06-01",
				BaseURL:        "https://custom-azure-endpoint.example.com",
			},
			expectError: false,
		},
		{
			name: "Azure config missing API key provider",
			config: &Config{
				PlatformType: PlatformAzure,
				APIVersion:   "2024-06-01",
				BaseURL:      "https://my-resource.openai.azure.com",
			},
			expectError: true,
			errorMsg:    "API key provider is required",
		},
		{
			name: "Azure config missing API version",
			config: &Config{
				PlatformType:   PlatformAzure,
				APIKeyProvider: auth.NewStaticKeyProvider("azure-api-key"),
				BaseURL:        "https://my-resource.openai.azure.com",
			},
			expectError: true,
			errorMsg:    "API version is required for Azure platform",
		},
		{
			name: "Azure config missing both base URL and resource name",
			config: &Config{
				PlatformType:   PlatformAzure,
				APIKeyProvider: auth.NewStaticKeyProvider("azure-api-key"),
				APIVersion:     "2024-06-01",
			},
			expectError: true,
			errorMsg:    "base URL is required",
		},
		{
			name: "unsupported platform type",
			config: &Config{
				PlatformType:   "invalid-platform", // Invalid platform type
				APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
				BaseURL:        "https://example.com",
			},
			expectError: true,
			errorMsg:    "unsupported platform type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.expectError {
				require.Error(t, err)

				if tt.errorMsg != "" {
					require.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewOutboundTransformerWithConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config - no error",
			config: &Config{
				PlatformType:   PlatformOpenAI,
				APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
				BaseURL:        "https://api.openai.com/v1",
			},
			expectError: false,
		},
		{
			name: "invalid config - missing API key provider",
			config: &Config{
				PlatformType: PlatformOpenAI,
				BaseURL:      "https://api.openai.com/v1",
				// Missing API key provider
			},
			expectError: true,
			errorMsg:    "API key provider is required",
		},
		{
			name: "invalid config - missing base URL",
			config: &Config{
				PlatformType:   PlatformOpenAI,
				APIKeyProvider: auth.NewStaticKeyProvider("test-api-key"),
				// Missing BaseURL
			},
			expectError: true,
			errorMsg:    "base URL is required",
		},
		{
			name: "Azure config missing API version",
			config: &Config{
				PlatformType:   PlatformAzure,
				APIKeyProvider: auth.NewStaticKeyProvider("azure-key"),
				BaseURL:        "https://my-resource.openai.azure.com",
				// Missing API version
			},
			expectError: true,
			errorMsg:    "API version is required for Azure platform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformerWithConfig(tt.config)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, transformer)

				if tt.errorMsg != "" {
					require.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, transformer)
			}
		})
	}
}

func TestSetConfig_Validation(t *testing.T) {
	transformerInterface, err := NewOutboundTransformerWithConfig(&Config{
		PlatformType:   PlatformOpenAI,
		BaseURL:        "https://api.openai.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("initial-key"),
	})
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	t.Run("valid config update", func(t *testing.T) {
		newConfig := &Config{
			PlatformType:   PlatformOpenAI,
			APIKeyProvider: auth.NewStaticKeyProvider("new-api-key"),
			BaseURL:        "https://api.openai.com/v1",
		}

		require.NotPanics(t, func() {
			transformer.SetConfig(newConfig)
		})

		require.Equal(t, newConfig, transformer.GetConfig())
	})

	t.Run("invalid config update should panic", func(t *testing.T) {
		invalidConfig := &Config{
			PlatformType: PlatformOpenAI,
			// Missing API key provider
		}

		require.Panics(t, func() {
			transformer.SetConfig(invalidConfig)
		})
	})

	t.Run("nil config gets defaults but still needs API key provider", func(t *testing.T) {
		// Setting nil config should panic because default config lacks API key provider
		require.Panics(t, func() {
			transformer.SetConfig(nil)
		})
	})
}

func TestSetAPIKey_Validation(t *testing.T) {
	transformerInterface, err := NewOutboundTransformerWithConfig(&Config{
		PlatformType:   PlatformOpenAI,
		APIKeyProvider: auth.NewStaticKeyProvider("initial-key"),
		BaseURL:        "https://api.openai.com/v1",
	})
	require.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	t.Run("valid API key update", func(t *testing.T) {
		require.NotPanics(t, func() {
			transformer.SetAPIKey("new-valid-key")
		})

		apiKey := transformer.GetConfig().APIKeyProvider.Get(context.Background())
		require.Equal(t, "new-valid-key", apiKey)
	})

	t.Run("empty API key should not panic - provider handles it", func(t *testing.T) {
		require.NotPanics(t, func() {
			transformer.SetAPIKey("")
		})
	})
}
