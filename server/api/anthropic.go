package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/llm/httpclient"
	anthropic "github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/server/biz"
)

type AnthropicResponseError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type AnthropicErrorHandler struct{}

func (e *AnthropicErrorHandler) HandlerError(c *gin.Context, err error) {
	c.JSON(500, &AnthropicResponseError{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "internal_error",
			Message: err.Error(),
		},
	})
}

func (e *AnthropicErrorHandler) HandleStreamError(c *gin.Context, err error) {
	c.SSEvent("", &AnthropicResponseError{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "internal_error",
			Message: err.Error(),
		},
	})
}

type AnthropicHandlersParams struct {
	fx.In

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     httpclient.HttpClient
}

type AnthropicHandlers struct {
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewAnthropicHandlers(params AnthropicHandlersParams) *AnthropicHandlers {
	return &AnthropicHandlers{
		ChatCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionProcessor: biz.NewChatCompletionProcessor(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				anthropic.NewInboundTransformer(),
			),
			ErrorHandler: &AnthropicErrorHandler{},
		},
	}
}

func (handlers *AnthropicHandlers) CreateMessage(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}
