package api

import (
	"io"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm/provider"
	"github.com/looplj/axonhub/llm/provider/openai"
	openaiTransformer "github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/types"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/server/biz"
)

func NewOpenAIHandlers() *ChatCompletionHandlers {
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

	transformer := openaiTransformer.NewTransformer()
	return &ChatCompletionHandlers{
		ChatCompletionProcessor: biz.NewChatCompletionProcessor(
			transformer,
			registry,
		),
	}
}

type ChatCompletionHandlers struct {
	ChatCompletionProcessor *biz.ChatCompletionProcessor
	ErrorHandler            func(c *gin.Context, err error)
}

func (handlers *ChatCompletionHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()
	result, err := handlers.ChatCompletionProcessor.Process(ctx, c.Request)
	if err != nil {
		handlers.ErrorHandler(c, err)
		return
	}

	if result.ChatCompletion != nil {
		finalResp := result.ChatCompletion
		c.Data(finalResp.StatusCode, finalResp.Headers["Content-Type"][0], finalResp.Body)
		return
	}

	if result.ChatCompletionStream != nil {
		defer func() {
			err := result.ChatCompletionStream.Close()
			if err != nil {
				logger.Error(ctx, "Error closing stream", log.Cause(err))
			}
		}()

		// Set SSE headers
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")

		disconnected := c.Stream(func(w io.Writer) bool {
			if result.ChatCompletionStream.Next() {
				cur := result.ChatCompletionStream.Current()
				c.SSEvent("", cur.Body)
				return true
			}
			return false
		})
		if disconnected {
			logger.Debug(ctx, "Client disconnected")
		}
	}
}
