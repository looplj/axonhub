package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
)

type OpenAIResponseError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type OpenAIErrorHandler struct{}

func (e *OpenAIErrorHandler) HandlerError(c *gin.Context, err error) {
	c.JSON(500, &OpenAIResponseError{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "internal_error",
			Message: err.Error(),
		},
	})
}

func (e *OpenAIErrorHandler) HandleStreamError(c *gin.Context, err error) {
	c.SSEvent("", &OpenAIResponseError{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "internal_error",
			Message: err.Error(),
		},
	})
}

type OpenAIHandlersParams struct {
	fx.In

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     httpclient.HttpClient
}

type OpenAIHandlers struct {
	ChatCompletionHandlers *ChatCompletionSSEHandlers
}

func NewOpenAIHandlers(params OpenAIHandlersParams) *OpenAIHandlers {
	return &OpenAIHandlers{
		ChatCompletionHandlers: &ChatCompletionSSEHandlers{
			ChatCompletionProcessor: biz.NewChatCompletionProcessor(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				openai.NewInboundTransformer(),
			),
			ErrorHandler: &OpenAIErrorHandler{},
		},
	}
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}
