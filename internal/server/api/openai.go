package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/openai"
	"github.com/looplj/axonhub/internal/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
)

type OpenAIHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

type OpenAIHandlers struct {
	ChannelService             *biz.ChannelService
	ChatCompletionHandlers     *ChatCompletionHandlers
	ResponseCompletionHandlers *ChatCompletionHandlers
	EmbeddingHandlers          *ChatCompletionHandlers
}

func NewOpenAIHandlers(params OpenAIHandlersParams) *OpenAIHandlers {
	return &OpenAIHandlers{
		ChatCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				openai.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		},
		ResponseCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				responses.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		},
		EmbeddingHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				openai.NewEmbeddingInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		},
		ChannelService: params.ChannelService,
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
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ListModels returns all available models from enabled channels.
// This endpoint is compatible with OpenAI's /v1/models API.
func (handlers *OpenAIHandlers) ListModels(c *gin.Context) {
	models := handlers.ChannelService.ListEnabledModels(c.Request.Context())

	openaiModels := make([]OpenAIModel, 0, len(models))
	for _, model := range models {
		openaiModels = append(openaiModels, OpenAIModel{
			ID:      model.ID,
			Created: model.Created,
			OwnedBy: model.OwnedBy,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   openaiModels,
	})
}
