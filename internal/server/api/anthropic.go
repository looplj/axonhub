package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type AnthropicErrorHandler struct{}

func (e *AnthropicErrorHandler) HandlerError(c *gin.Context, err error) {
	if aErr, ok := xerrors.As[*httpclient.Error](err); ok {
		c.JSON(aErr.StatusCode, json.RawMessage(aErr.Body))
		return
	}

	if aErr, ok := xerrors.As[*llm.ResponseError](err); ok {
		c.JSON(aErr.StatusCode, anthropic.AnthropicErr{
			StatusCode: aErr.StatusCode,
			RequestID:  aErr.Detail.RequestID,
			Message:    aErr.Error(),
		})

		return
	}

	c.JSON(500, anthropic.AnthropicErr{
		StatusCode: 0,
		RequestID:  "",
		Message:    "Internal server error",
	})
}

type AnthropicHandlersParams struct {
	fx.In

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     *httpclient.HttpClient
}

type AnthropicHandlers struct {
	ChatCompletionHandlers *ChatCompletionSSEHandlers
}

func NewAnthropicHandlers(params AnthropicHandlersParams) *AnthropicHandlers {
	return &AnthropicHandlers{
		ChatCompletionHandlers: &ChatCompletionSSEHandlers{
			ChatCompletionProcessor: chat.NewChatCompletionProcessor(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				anthropic.NewInboundTransformer(),
			),
		},
	}
}

func (handlers *AnthropicHandlers) CreateMessage(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}
