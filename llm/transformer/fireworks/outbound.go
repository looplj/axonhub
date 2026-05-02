package fireworks

import (
	"fmt"
	"strings"

	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// DefaultBaseURL is the default Fireworks API base URL.
const DefaultBaseURL = "https://api.fireworks.ai/inference/v1"

// Config holds all configuration for the Fireworks outbound transformer.
type Config struct {
	// BaseURL is the base URL for the Fireworks API.
	BaseURL string `json:"base_url,omitempty"`
	// APIKeyProvider provides API keys for authentication.
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

// NewOutboundTransformer creates a new Fireworks OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new Fireworks OutboundTransformer with unified configuration.
// Fireworks API is OpenAI-compatible and supports reasoning_content field.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, fmt.Errorf("invalid Fireworks transformer configuration: config is nil")
	}

	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("invalid Fireworks transformer configuration: API key provider is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
		ReasoningField: openai.ReasoningFieldContent,
	}

	return openai.NewOutboundTransformerWithConfig(oaiConfig)
}
