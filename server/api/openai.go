package api

import (
	"github.com/gin-gonic/gin"
)

type OpenAIHandlers struct {
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewOpenAIHandlers(chatCompletionHandlers *ChatCompletionHandlers) *OpenAIHandlers {
	return &OpenAIHandlers{
		ChatCompletionHandlers: chatCompletionHandlers,
	}
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}
