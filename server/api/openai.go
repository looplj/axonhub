package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm/provider"
	"github.com/looplj/axonhub/llm/provider/openai"
	openaiTransformer "github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/types"
)

func NewOpenAIHandlers() *OpenAIHandlers {
	return &OpenAIHandlers{}
}

type OpenAIHandlers struct {
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {

	// Create provider registry
	registry := provider.NewRegistry()

	// Create and register OpenAI provider
	openaiProvider := openai.NewProvider(&types.ProviderConfig{
		Name:     "openai",
		Settings: map[string]interface{}{},
	})
	registry.RegisterProvider("openai", openaiProvider)
	registry.RegisterModelMapping("gpt-3.5-turbo", "openai")
	registry.RegisterModelMapping("gpt-4", "openai")
	registry.RegisterModelMapping("gpt-4-turbo", "openai")
	registry.RegisterModelMapping("gpt-4o-mini", "openai")

	// Create inbound transformer
	inboundTransformer := openaiTransformer.NewInboundTransformer()

	// Create processor with new architecture
	processor := NewChatCompletionProcessor(
		inboundTransformer,
		registry,
	)
	if err := processor.Process(c); err != nil {
		// Align with OpenAPI error response format
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}
