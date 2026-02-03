package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
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
	QuotaService    *biz.QuotaService
	HttpClient      *httpclient.HttpClient
	Client          *ent.Client
}

type OpenAIHandlers struct {
	ChannelService             *biz.ChannelService
	ModelService               *biz.ModelService
	SystemService              *biz.SystemService
	ChatCompletionHandlers     *ChatCompletionHandlers
	ResponseCompletionHandlers *ChatCompletionHandlers
	EmbeddingHandlers          *ChatCompletionHandlers
	ImageGenerationHandlers    *ChatCompletionHandlers
	ImageEditHandlers          *ChatCompletionHandlers
	ImageVariationHandlers     *ChatCompletionHandlers
	EntClient                  *ent.Client
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
				params.QuotaService,
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
				params.QuotaService,
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
				params.QuotaService,
			),
		},
		ImageGenerationHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				openai.NewImageGenerationInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
			),
		},
		ImageEditHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				openai.NewImageEditInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
			),
		},
		ImageVariationHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				openai.NewImageVariationInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
			),
		},
		EntClient:      params.Client,
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

func (handlers *OpenAIHandlers) CreateImage(c *gin.Context) {
	handlers.ImageGenerationHandlers.ChatCompletion(c)
}

func (handlers *OpenAIHandlers) CreateImageEdit(c *gin.Context) {
	handlers.ImageEditHandlers.ChatCompletion(c)
}

func (handlers *OpenAIHandlers) CreateImageVariation(c *gin.Context) {
	handlers.ImageVariationHandlers.ChatCompletion(c)
}

type Capabilities struct {
	Vision    bool `json:"vision"`
	ToolCall  bool `json:"tool_call"`
	Reasoning bool `json:"reasoning"`
}

type Pricing struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Unit       string  `json:"unit"`
	Currency   string  `json:"currency"`
}

type OpenAIModel struct {
	ID              string        `json:"id"`
	Object          string        `json:"object"`
	Created         int64         `json:"created"`
	OwnedBy         string        `json:"owned_by"`
	Name            string        `json:"name,omitempty"`
	Description     string        `json:"description,omitempty"`
	ContextLength   int           `json:"context_length,omitempty"`
	MaxOutputTokens int           `json:"max_output_tokens,omitempty"`
	Capabilities    *Capabilities `json:"capabilities,omitempty"`
	Pricing         *Pricing      `json:"pricing,omitempty"`
	Icon            string        `json:"icon,omitempty"`
	Type            string        `json:"type,omitempty"`
}

// ListModels returns all available models.

// convertModelToOpenAIExtended transforms an ent.Model to OpenAIModel with extended metadata fields.
// It safely handles nil ModelCard, Cost, and Limit fields.
func convertModelToOpenAIExtended(m *ent.Model) OpenAIModel {
	result := OpenAIModel{
		ID:      m.ModelID,
		Object:  "model",
		Created: m.CreatedAt.Unix(),
		OwnedBy: m.Developer,
		Name:    m.Name,
		Icon:    m.Icon,
		Type:    string(m.Type),
	}

	if m.Remark != nil {
		result.Description = *m.Remark
	}

	if m.ModelCard != nil {
		// Capabilities
		caps := Capabilities{
			Vision:    m.ModelCard.Vision,
			ToolCall:  m.ModelCard.ToolCall,
			Reasoning: m.ModelCard.Reasoning.Supported,
		}
		result.Capabilities = &caps
		// Limits
		result.ContextLength = m.ModelCard.Limit.Context
		result.MaxOutputTokens = m.ModelCard.Limit.Output
		// Pricing
		pricing := Pricing{
			Input:      m.ModelCard.Cost.Input,
			Output:     m.ModelCard.Cost.Output,
			CacheRead:  m.ModelCard.Cost.CacheRead,
			CacheWrite: m.ModelCard.Cost.CacheWrite,
			Unit:       "per_1m_tokens",
			Currency:   "USD",
		}
		result.Pricing = &pricing
	}
	return result
}

// ListModels returns all available models.
// This endpoint is compatible with OpenAI's /v1/models API.
// It uses QueryAllChannelModels setting from system config to determine model source.
func (handlers *OpenAIHandlers) ListModels(c *gin.Context) {
	ctx := c.Request.Context()

	requestID, _ := contexts.GetRequestID(ctx)

	// Check for extended query parameter
	extended := c.Query("extended") == "true"

	if extended {
		// Query full model data from database with extended metadata
		models, err := handlers.EntClient.Model.Query().
			Where(model.StatusEQ(model.StatusEnabled)).
			All(ctx)
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
		for _, m := range models {
			openaiModels = append(openaiModels, convertModelToOpenAIExtended(m))
		}

		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   openaiModels,
		})
		return
	}

	// Original behavior for backward compatibility
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
