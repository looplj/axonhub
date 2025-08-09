package api

import (
	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
	"go.uber.org/fx"
)

type OpenAIErrorHandler struct{}

func (e *OpenAIErrorHandler) HandlerError(c *gin.Context, err error) {
	if aErr, ok := xerrors.As[*llm.ResponseError](err); ok {
		c.JSON(aErr.StatusCode, aErr)
		return
	}

	c.JSON(500, llm.ResponseError{
		StatusCode: 0,
		Detail: llm.ErrorDetail{
			Message: "Internal server error",
		},
	})
}

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
			ErrorHandler: &OpenAIErrorHandler{},
		},
	}
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}
