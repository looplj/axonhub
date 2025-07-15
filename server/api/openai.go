package api

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/server/biz"
)

type OpenAIHandlers struct {
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewOpenAIHandlers(channelService *biz.ChannelService) *OpenAIHandlers {
	return &OpenAIHandlers{
		ChatCompletionHandlers: NewChatCompletionHandlers(openai.NewTransformer(), channelService),
	}
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}
