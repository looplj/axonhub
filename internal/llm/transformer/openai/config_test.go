package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
				Type:    PlatformOpenAI,
				APIKey:  "test-api-key",
				BaseURL: "https://api.openai.com/v1",
			},
			expectError: false,
		},
		{
			name: "OpenAI config missing API key",
			config: &Config{
				Type:    PlatformOpenAI,
				BaseURL: "https://api.openai.com/v1",
			},
			expectError: true,
			errorMsg:    "API key is required",
		},
		{
			name: "OpenAI config missing base URL",
			config: &Config{
				Type:   PlatformOpenAI,
				APIKey: "test-api-key",
			},
			expectError: true,
			errorMsg:    "base URL is required",
		},
		{
			name: "valid Azure config with base URL",
			config: &Config{
				Type:       PlatformAzure,
				APIKey:     "azure-api-key",
				APIVersion: "2024-06-01",
				BaseURL:    "https://my-resource.openai.azure.com",
			},
			expectError: false,
		},
		{
			name: "valid Azure config with custom base URL",
			config: &Config{
				Type:       PlatformAzure,
				APIKey:     "azure-api-key",
				APIVersion: "2024-06-01",
				BaseURL:    "https://custom-azure-endpoint.example.com",
			},
			expectError: false,
		},
		{
			name: "Azure config missing API key",
			config: &Config{
				Type:       PlatformAzure,
				APIVersion: "2024-06-01",
				BaseURL:    "https://my-resource.openai.azure.com",
			},
			expectError: true,
			errorMsg:    "API key is required",
		},
		{
			name: "Azure config missing API version",
			config: &Config{
				Type:    PlatformAzure,
				APIKey:  "azure-api-key",
				BaseURL: "https://my-resource.openai.azure.com",
			},
			expectError: true,
			errorMsg:    "API version is required for Azure platform",
		},
		{
			name: "Azure config missing both base URL and resource name",
			config: &Config{
				Type:       PlatformAzure,
				APIKey:     "azure-api-key",
				APIVersion: "2024-06-01",
			},
			expectError: true,
			errorMsg:    "base URL is required",
		},
		{
			name: "unsupported platform type",
			config: &Config{
				Type:    "invalid-platform", // Invalid platform type
				APIKey:  "test-api-key",
				BaseURL: "https://example.com",
			},
			expectError: true,
			errorMsg:    "unsupported platform type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)

				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
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
				Type:    PlatformOpenAI,
				APIKey:  "test-api-key",
				BaseURL: "https://api.openai.com/v1",
			},
			expectError: false,
		},
		{
			name: "invalid config - missing API key",
			config: &Config{
				Type:    PlatformOpenAI,
				BaseURL: "https://api.openai.com/v1",
				// Missing API key
			},
			expectError: true,
			errorMsg:    "API key is required",
		},
		{
			name: "invalid config - missing base URL",
			config: &Config{
				Type:   PlatformOpenAI,
				APIKey: "test-api-key",
				// Missing BaseURL
			},
			expectError: true,
			errorMsg:    "base URL is required",
		},
		{
			name: "Azure config missing API version",
			config: &Config{
				Type:    PlatformAzure,
				APIKey:  "azure-key",
				BaseURL: "https://my-resource.openai.azure.com",
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
				assert.Error(t, err)
				assert.Nil(t, transformer)

				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, transformer)
			}
		})
	}
}

func TestConfigureForAzure_Validation(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		apiVersion   string
		apiKey       string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "valid Azure configuration",
			resourceName: "my-resource",
			apiVersion:   "2024-06-01",
			apiKey:       "azure-api-key",
			expectError:  false,
		},
		{
			name:         "missing API version",
			resourceName: "my-resource",
			apiVersion:   "",
			apiKey:       "azure-api-key",
			expectError:  true,
			errorMsg:     "API version is required",
		},
		{
			name:         "missing API key",
			resourceName: "my-resource",
			apiVersion:   "2024-06-01",
			apiKey:       "",
			expectError:  true,
			errorMsg:     "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a basic transformer first
			transformerInterface, initErr := NewOutboundTransformerWithConfig(&Config{
				Type:    PlatformOpenAI,
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "initial-key",
			})
			assert.NoError(t, initErr)

			transformer := transformerInterface.(*OutboundTransformer)

			err := transformer.ConfigureForAzure(tt.resourceName, tt.apiVersion, tt.apiKey)

			if tt.expectError {
				assert.Error(t, err)

				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify the configuration was applied correctly
				config := transformer.GetConfig()
				assert.Equal(t, PlatformAzure, config.Type)
				// Note: ResourceName field removed from Config struct
				assert.Equal(t, tt.apiVersion, config.APIVersion)
				assert.Equal(t, tt.apiKey, config.APIKey)
			}
		})
	}
}

func TestSetConfig_Validation(t *testing.T) {
	transformerInterface, err := NewOutboundTransformerWithConfig(&Config{
		Type:    PlatformOpenAI,
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "initial-key",
	})
	assert.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	t.Run("valid config update", func(t *testing.T) {
		newConfig := &Config{
			Type:    PlatformOpenAI,
			APIKey:  "new-api-key",
			BaseURL: "https://api.openai.com/v1",
		}

		assert.NotPanics(t, func() {
			transformer.SetConfig(newConfig)
		})

		assert.Equal(t, newConfig, transformer.GetConfig())
	})

	t.Run("invalid config update should panic", func(t *testing.T) {
		invalidConfig := &Config{
			Type: PlatformOpenAI,
			// Missing API key
		}

		assert.Panics(t, func() {
			transformer.SetConfig(invalidConfig)
		})
	})

	t.Run("nil config gets defaults but still needs API key", func(t *testing.T) {
		// Setting nil config should panic because default config lacks API key
		assert.Panics(t, func() {
			transformer.SetConfig(nil)
		})
	})
}

func TestSetAPIKey_Validation(t *testing.T) {
	transformerInterface, err := NewOutboundTransformerWithConfig(&Config{
		Type:    PlatformOpenAI,
		APIKey:  "initial-key",
		BaseURL: "https://api.openai.com/v1",
	})
	assert.NoError(t, err)

	transformer := transformerInterface.(*OutboundTransformer)

	t.Run("valid API key update", func(t *testing.T) {
		assert.NotPanics(t, func() {
			transformer.SetAPIKey("new-valid-key")
		})

		assert.Equal(t, "new-valid-key", transformer.GetConfig().APIKey)
	})

	t.Run("empty API key should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			transformer.SetAPIKey("")
		})
	})
}
