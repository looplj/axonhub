package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

type AnthropicHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	ModelService    *biz.ModelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

type AnthropicHandlers struct {
	ChannelService         *biz.ChannelService
	ModelService           *biz.ModelService
	SystemService          *biz.SystemService
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewAnthropicHandlers(params AnthropicHandlersParams) *AnthropicHandlers {
	return &AnthropicHandlers{
		ChatCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				anthropic.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		},
		ChannelService: params.ChannelService,
		ModelService:   params.ModelService,
		SystemService:  params.SystemService,
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

// ListModels returns all available models.
// It uses QueryAllChannelModels setting from system config to determine model source.
func (handlers *AnthropicHandlers) ListModels(c *gin.Context) {
	ctx := c.Request.Context()

	models, err := handlers.ModelService.ListEnabledModels(ctx)
	if err != nil {
		requestID, _ := contexts.GetRequestID(ctx)
		c.JSON(http.StatusInternalServerError, anthropic.AnthropicError{
			StatusCode: http.StatusInternalServerError,
			Type:       "internal_server_error",
			RequestID:  requestID,
			Error: anthropic.ErrorDetail{
				Type:    "internal_server_error",
				Message: err.Error(),
			},
		})

		return
	}

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
