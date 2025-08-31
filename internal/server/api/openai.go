package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type OpenAIHandlersParams struct {
	fx.In

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     *httpclient.HttpClient
}

type OpenAIHandlers struct {
	ChatCompletionHandlers *ChatCompletionSSEHandlers
}

func NewOpenAIHandlers(params OpenAIHandlersParams) *OpenAIHandlers {
	return &OpenAIHandlers{
		ChatCompletionHandlers: &ChatCompletionSSEHandlers{
			ChatCompletionProcessor: chat.NewChatCompletionProcessor(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				openai.NewInboundTransformer(),
			),
		},
	}
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}
