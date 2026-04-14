package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type OpenAIHandlersParams struct {
	fx.In

	VideoService                *biz.VideoService
	ChannelService              *biz.ChannelService
	ModelService                *biz.ModelService
	DefaultSelector             *orchestrator.DefaultSelector
	RequestService              *biz.RequestService
	SystemService               *biz.SystemService
	UsageLogService             *biz.UsageLogService
	PromptService               *biz.PromptService
	PromptProtectionRuleService *biz.PromptProtectionRuleService
	QuotaService                *biz.QuotaService
	HttpClient                  *httpclient.HttpClient
	LiveStreamRegistry          *biz.LiveStreamRegistry
	Client                      *ent.Client
}

type OpenAIHandlers struct {
	ChannelService             *biz.ChannelService
	ModelService               *biz.ModelService
	SystemService              *biz.SystemService
	VideoService               *biz.VideoService
	ChatCompletionHandlers     *ChatCompletionHandlers
	ResponseCompletionHandlers *ChatCompletionHandlers
	CompactHandlers            *ChatCompletionHandlers
	EmbeddingHandlers          *ChatCompletionHandlers
	ImageGenerationHandlers    *ChatCompletionHandlers
	ImageEditHandlers          *ChatCompletionHandlers
	ImageVariationHandlers     *ChatCompletionHandlers
	VideoHandlers              *ChatCompletionHandlers
	VideoInboundTransformer    *openai.VideoInboundTransformer
	EntClient                  *ent.Client
}

func NewOpenAIHandlers(params OpenAIHandlersParams) *OpenAIHandlers {
	videoInbound := openai.NewVideoInboundTransformer()

	return &OpenAIHandlers{
		ChatCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				openai.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
			),
		},
		ResponseCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				responses.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
			),
		},
		CompactHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				responses.NewCompactInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
			),
		},
		EmbeddingHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				openai.NewEmbeddingInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
			),
		},
		ImageGenerationHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				openai.NewImageGenerationInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
			),
		},
		ImageEditHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				openai.NewImageEditInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
			),
		},
		ImageVariationHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				openai.NewImageVariationInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
			),
		},
		VideoHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				videoInbound,
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
			),
		},
		VideoInboundTransformer: videoInbound,
		VideoService:            params.VideoService,
		EntClient:               params.Client,
		ChannelService:          params.ChannelService,
		ModelService:            params.ModelService,
		SystemService:           params.SystemService,
	}
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}

func (handlers *OpenAIHandlers) CreateResponse(c *gin.Context) {
	handlers.ResponseCompletionHandlers.ChatCompletion(c)
}

func (handlers *OpenAIHandlers) CompactResponse(c *gin.Context) {
	handlers.CompactHandlers.ChatCompletion(c)
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

func (handlers *OpenAIHandlers) CreateVideo(c *gin.Context) {
	ctx := c.Request.Context()

	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		httpErr := handlers.VideoHandlers.ChatCompletionOrchestrator.Inbound.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))
		return
	}

	if len(genericReq.Body) == 0 {
		JSONError(c, http.StatusBadRequest, errors.New("Request body is empty"))
		return
	}

	result, err := handlers.VideoHandlers.ChatCompletionOrchestrator.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing openai video create", log.Cause(err))

		httpErr := handlers.VideoHandlers.ChatCompletionOrchestrator.Inbound.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))
		return
	}

	if result.ChatCompletion == nil {
		JSONError(c, http.StatusInternalServerError, biz.ErrInternal)
		return
	}

	resp := result.ChatCompletion
	contentType := "application/json"
	if ct := resp.Headers.Get("Content-Type"); ct != "" {
		contentType = ct
	}
	c.Data(resp.StatusCode, contentType, resp.Body)
}

func (handlers *OpenAIHandlers) GetVideo(c *gin.Context) {
	ctx := c.Request.Context()

	externalID := c.Param("id")
	if externalID == "" {
		JSONError(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	resp, err := handlers.VideoService.GetTaskByExternalID(ctx, externalID)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	resp.Object = "video"
	resp.APIFormat = llm.APIFormatOpenAIVideo
	resp.Choices = []llm.Choice{}

	httpResp, err := handlers.VideoInboundTransformer.TransformResponse(ctx, resp)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	contentType := "application/json"
	if ct := httpResp.Headers.Get("Content-Type"); ct != "" {
		contentType = ct
	}
	c.Data(httpResp.StatusCode, contentType, httpResp.Body)
}

func (handlers *OpenAIHandlers) DeleteVideo(c *gin.Context) {
	ctx := c.Request.Context()

	externalID := c.Param("id")
	if externalID == "" {
		JSONError(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	if err := handlers.VideoService.DeleteTaskByExternalID(ctx, externalID); err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusNoContent)
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

const (
	openAIModelObjectType         = "model"
	openAIErrorCodeInternalServer = "internal_server_error"
	openAIErrorCodeModelNotFound  = "model_not_found"
	openAIErrorTypeServer         = "server_error"
	openAIErrorTypeInvalidRequest = "invalid_request_error"
	openAIErrorParamModel         = "model"
)

func parseOpenAIModelInclude(includeParam string, defaultIncludeAll bool) (map[string]bool, bool) {
	var (
		include      map[string]bool
		needFullData bool
	)

	if includeParam == "" {
		return nil, defaultIncludeAll
	}

	if includeParam == "all" {
		return nil, true
	}

	fields := strings.Split(includeParam, ",")
	include = make(map[string]bool)
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			include[field] = true
		}
	}

	extendedFields := []string{"name", "description", "context_length", "max_output_tokens", "capabilities", "pricing", "icon", "type"}
	for _, field := range extendedFields {
		if include[field] {
			needFullData = true
			break
		}
	}

	return include, needFullData
}

func convertModelFacadeToOpenAIModel(m biz.ModelFacade) OpenAIModel {
	return OpenAIModel{
		ID:      m.ID,
		Object:  openAIModelObjectType,
		Created: m.Created,
		OwnedBy: m.OwnedBy,
	}
}

// convertModelToOpenAIExtended transforms an ent.Model to OpenAIModel with extended metadata fields.
// It safely handles nil ModelCard, Cost, and Limit fields.
// The include set specifies which optional fields to populate. If nil or empty, all fields are populated.
// Supported field names: name, description, context_length, max_output_tokens, capabilities, pricing, icon, type
func convertModelToOpenAIExtended(m *ent.Model, include map[string]bool) OpenAIModel {
	result := OpenAIModel{
		ID:      m.ModelID,
		Object:  openAIModelObjectType,
		Created: m.CreatedAt.Unix(),
		OwnedBy: m.Developer,
	}

	// Helper function to check if a field should be included
	shouldInclude := func(field string) bool {
		if include == nil {
			return true // all fields included
		}
		return include[field]
	}

	// Always include basic fields (ID, Object, Created, OwnedBy) - they're set above

	// Optional fields
	if shouldInclude("name") {
		result.Name = m.Name
	}
	if shouldInclude("icon") {
		result.Icon = m.Icon
	}
	if shouldInclude("type") {
		result.Type = string(m.Type)
	}
	if shouldInclude("description") {
		if m.Remark != nil {
			result.Description = *m.Remark
		}
	}

	if m.ModelCard != nil {
		// Capabilities, ContextLength, MaxOutputTokens, Pricing come from ModelCard
		if shouldInclude("capabilities") {
			caps := Capabilities{
				Vision:    m.ModelCard.Vision,
				ToolCall:  m.ModelCard.ToolCall,
				Reasoning: m.ModelCard.Reasoning.Supported,
			}
			result.Capabilities = &caps
		}
		if shouldInclude("context_length") {
			result.ContextLength = m.ModelCard.Limit.Context
		}
		if shouldInclude("max_output_tokens") {
			result.MaxOutputTokens = m.ModelCard.Limit.Output
		}
		if shouldInclude("pricing") {
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
	}
	return result
}

func (handlers *OpenAIHandlers) writeOpenAIInternalError(c *gin.Context, requestID string, err error) {
	c.JSON(http.StatusInternalServerError, openai.OpenAIError{
		StatusCode: http.StatusInternalServerError,
		Detail: llm.ErrorDetail{
			Code:      openAIErrorCodeInternalServer,
			Message:   err.Error(),
			Type:      openAIErrorTypeServer,
			RequestID: requestID,
		},
	})
}

func (handlers *OpenAIHandlers) writeOpenAIModelNotFoundError(c *gin.Context, requestID, modelID string) {
	message := "The model does not exist or you do not have access to it."
	if modelID != "" {
		message = fmt.Sprintf("The model `%s` does not exist or you do not have access to it.", modelID)
	}

	c.JSON(http.StatusNotFound, openai.OpenAIError{
		StatusCode: http.StatusNotFound,
		Detail: llm.ErrorDetail{
			Code:      openAIErrorCodeModelNotFound,
			Message:   message,
			Type:      openAIErrorTypeInvalidRequest,
			Param:     openAIErrorParamModel,
			RequestID: requestID,
		},
	})
}

// RetrieveModel returns a single available model.
// This endpoint is compatible with OpenAI's /v1/models/{model} API.
func (handlers *OpenAIHandlers) RetrieveModel(c *gin.Context) {
	ctx := c.Request.Context()

	requestID, _ := contexts.GetRequestID(ctx)
	modelID := strings.TrimPrefix(c.Param("model"), "/")
	if modelID == "" {
		handlers.writeOpenAIModelNotFoundError(c, requestID, "")
		return
	}

	include, needFullData := parseOpenAIModelInclude(c.Query("include"), false)

	models, err := handlers.ModelService.ListEnabledModels(ctx)
	if err != nil {
		handlers.writeOpenAIInternalError(c, requestID, err)
		return
	}

	visibleModel, found := lo.Find(models, func(m biz.ModelFacade) bool {
		return m.ID == modelID
	})
	if !found {
		handlers.writeOpenAIModelNotFoundError(c, requestID, modelID)
		return
	}

	if !needFullData {
		c.JSON(http.StatusOK, convertModelFacadeToOpenAIModel(visibleModel))
		return
	}

	configuredModel, err := handlers.EntClient.Model.Query().
		Where(
			model.ModelID(modelID),
			model.StatusEQ(model.StatusEnabled),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusOK, convertModelFacadeToOpenAIModel(visibleModel))
			return
		}

		handlers.writeOpenAIInternalError(c, requestID, err)
		return
	}

	c.JSON(http.StatusOK, convertModelToOpenAIExtended(configuredModel, include))
}

// ListModels returns all available models.
// This endpoint is compatible with OpenAI's /v1/models API.
// It uses QueryAllChannelModels setting from system config to determine model source.
func (handlers *OpenAIHandlers) ListModels(c *gin.Context) {
	ctx := c.Request.Context()

	requestID, _ := contexts.GetRequestID(ctx)

	include, needFullData := parseOpenAIModelInclude(c.Query("include"), handlers.SystemService.ModelSettingsOrDefault(ctx).DefaultModelAPIIncludeAll)

	var openaiModels []OpenAIModel
	if needFullData {
		// Query full model data from database with extended metadata
		models, err := handlers.EntClient.Model.Query().
			Where(model.StatusEQ(model.StatusEnabled)).
			All(ctx)
		if err != nil {
			handlers.writeOpenAIInternalError(c, requestID, err)
			return
		}

		openaiModels = make([]OpenAIModel, 0, len(models))
		for _, m := range models {
			openaiModels = append(openaiModels, convertModelToOpenAIExtended(m, include))
		}
	} else {
		// Basic mode: only return basic fields (backward compatible)
		models, err := handlers.ModelService.ListEnabledModels(ctx)
		if err != nil {
			handlers.writeOpenAIInternalError(c, requestID, err)
			return
		}

		openaiModels = make([]OpenAIModel, 0, len(models))
		for _, m := range models {
			openaiModels = append(openaiModels, convertModelFacadeToOpenAIModel(m))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   openaiModels,
	})
}
