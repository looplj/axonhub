package api

import (
	"encoding/json"
	"net/http"
	"time"

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

	ChannelService  *biz.ChannelService
	RequestService  *biz.RequestService
	TraceService    *biz.TraceService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

type AnthropicHandlers struct {
	ChannelService         *biz.ChannelService
	ChatCompletionHandlers *ChatCompletionSSEHandlers
}

func NewAnthropicHandlers(params AnthropicHandlersParams) *AnthropicHandlers {
	return &AnthropicHandlers{
		ChatCompletionHandlers: &ChatCompletionSSEHandlers{
			ChatCompletionProcessor: chat.NewChatCompletionProcessor(
				params.ChannelService,
				params.RequestService,
				params.TraceService,
				params.HttpClient,
				anthropic.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		},
		ChannelService: params.ChannelService,
	}
}

func (handlers *AnthropicHandlers) CreateMessage(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}

type AnthropicModel struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created"`
}

func (handlers *AnthropicHandlers) ListModels(c *gin.Context) {
	models := handlers.ChannelService.ListAllModels(c.Request.Context())

	anthropicModels := make([]AnthropicModel, 0, len(models))
	for _, model := range models {
		anthropicModels = append(anthropicModels, AnthropicModel{
			ID:          model.ID,
			DisplayName: model.DisplayName,
			CreatedAt:   model.CreatedAt,
		})
	}

	var firstID string
	if len(anthropicModels) > 0 {
		firstID = anthropicModels[0].ID
	}

	var lastID string
	if len(anthropicModels) > 0 {
		lastID = anthropicModels[len(anthropicModels)-1].ID
	}

	c.JSON(http.StatusOK, gin.H{
		"object":   "list",
		"data":     anthropicModels,
		"has_more": false,
		"first_id": firstID,
		"last_id":  lastID,
	})
}
