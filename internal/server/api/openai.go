package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type OpenAIHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	ModelService    *biz.ModelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	PromptService   *biz.PromptService
	HttpClient      *httpclient.HttpClient
}

type OpenAIHandlers struct {
	ChannelService             *biz.ChannelService
	ModelService               *biz.ModelService
	SystemService              *biz.SystemService
	ChatCompletionHandlers     *ChatCompletionHandlers
	ResponseCompletionHandlers *ChatCompletionHandlers
	EmbeddingHandlers          *ChatCompletionHandlers
}

func NewOpenAIHandlers(params OpenAIHandlersParams) *OpenAIHandlers {
	return &OpenAIHandlers{
		ChatCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				openai.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
			),
		},
		ResponseCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				responses.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
			),
		},
		EmbeddingHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				openai.NewEmbeddingInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
			),
		},
		ChannelService: params.ChannelService,
		ModelService:   params.ModelService,
		SystemService:  params.SystemService,
	}
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}

func (handlers *OpenAIHandlers) CreateResponse(c *gin.Context) {
	handlers.ResponseCompletionHandlers.ChatCompletion(c)
}

func (handlers *OpenAIHandlers) CreateEmbedding(c *gin.Context) {
	handlers.EmbeddingHandlers.ChatCompletion(c)
}

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ListModels returns all available models.
// This endpoint is compatible with OpenAI's /v1/models API.
// It uses QueryAllChannelModels setting from system config to determine model source.
func (handlers *OpenAIHandlers) ListModels(c *gin.Context) {
	ctx := c.Request.Context()

	requestID, _ := contexts.GetRequestID(ctx)

	models, err := handlers.ModelService.ListEnabledModels(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, openai.OpenAIError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Code:      "internal_server_error",
				Message:   err.Error(),
				Type:      "server_error",
				RequestID: requestID,
			},
		})

		return
	}

	openaiModels := make([]OpenAIModel, 0, len(models))
	for _, model := range models {
		openaiModels = append(openaiModels, OpenAIModel{
			ID:      model.ID,
			Object:  "model",
			Created: model.Created,
			OwnedBy: model.OwnedBy,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   openaiModels,
	})
}
