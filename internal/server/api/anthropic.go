package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/anthropic"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type AnthropicHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

type AnthropicHandlers struct {
	ChannelService         *biz.ChannelService
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewAnthropicHandlers(params AnthropicHandlersParams) *AnthropicHandlers {
	return &AnthropicHandlers{
		ChatCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionProcessor: chat.NewChatCompletionProcessor(
				params.ChannelService,
				params.RequestService,
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
	models := handlers.ChannelService.ListEnabledModels(c.Request.Context())

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
