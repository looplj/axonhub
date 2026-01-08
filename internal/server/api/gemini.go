package api

import (
	"net/http"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/gemini"
)

type GeminiHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	ModelService    *biz.ModelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

type GeminiHandlers struct {
	ChannelService         *biz.ChannelService
	ModelService           *biz.ModelService
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewGeminiHandlers(params GeminiHandlersParams) *GeminiHandlers {
	return &GeminiHandlers{
		ChatCompletionHandlers: NewChatCompletionHandlers(
			orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				gemini.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		),
		ChannelService: params.ChannelService,
		ModelService:   params.ModelService,
	}
}

func (handlers *GeminiHandlers) GenerateContent(c *gin.Context) {
	alt := c.Query("alt")
	switch alt {
	case "sse":
		handlers.ChatCompletionHandlers.WithStreamWriter(WriteGeminiSSEStream).ChatCompletion(c)
	default:
		handlers.ChatCompletionHandlers.WithStreamWriter(WriteGeminiStream).ChatCompletion(c)
	}
}

func WriteGeminiStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	ctx := c.Request.Context()
	clientDisconnected := false

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	c.Header("Content-Type", "application/json; charset=UTF-8")

	_, _ = c.Writer.Write([]byte("["))

	first := true

	for {
		select {
		case <-ctx.Done():
			clientDisconnected = true

			log.Warn(ctx, "Client disconnected, stop streaming")

			return
		default:
			if stream.Next() {
				cur := stream.Current()

				if !first {
					_, _ = c.Writer.Write([]byte(","))
				}

				_, _ = c.Writer.Write(cur.Data)
				first = false

				log.Debug(ctx, "write stream event", log.Any("event", cur))
				c.Writer.Flush()
			} else {
				if err := stream.Err(); err != nil {
					log.Error(ctx, "Error in stream", log.Cause(err))
				}

				_, _ = c.Writer.Write([]byte("]"))

				return
			}
		}
	}
}

// WriteGeminiSSEStream
// Gemini js sdk need more whitespace after data: to work properly.
// This prepends a space to each data payload to ensure compatibility.
func WriteGeminiSSEStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	ctx := c.Request.Context()
	clientDisconnected := false

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	c.Header("Content-Type", sse.ContentType)
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	for {
		select {
		case <-ctx.Done():
			clientDisconnected = true

			log.Warn(ctx, "Context done, stopping stream")

			return
		default:
			if stream.Next() {
				cur := stream.Current()
				c.SSEvent(cur.Type, prependSpace(cur.Data))
				log.Debug(ctx, "write stream event", log.Any("event", cur))
				c.Writer.Flush()
			} else {
				if stream.Err() != nil {
					log.Error(ctx, "Error in stream", log.Cause(stream.Err()))
					c.SSEvent("error", stream.Err())
				}

				c.Writer.Flush()

				return
			}
		}
	}
}

// prependSpace adds a leading space to the data payload for Gemini JS SDK compatibility.
func prependSpace(b []byte) []byte {
	result := make([]byte, len(b)+1)
	result[0] = ' '
	copy(result[1:], b)

	return result
}

// GeminiModel represents a model in the list models response.
type GeminiModel struct {
	Name        string `json:"name"`
	BaseModelID string `json:"baseModelId"`
	Version     string `json:"version"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

// ListModels returns all available Gemini models.
// This endpoint is compatible with Gemini's /v1/models API.
// It uses QueryAllChannelModels setting from system config to determine model source.
func (handlers *GeminiHandlers) ListModels(c *gin.Context) {
	ctx := c.Request.Context()

	models, err := handlers.ModelService.ListEnabledModels(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gemini.GeminiError{
			Error: gemini.ErrorDetail{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
				Status:  "internal_server_error",
			},
		})

		return
	}

	geminiModels := make([]GeminiModel, 0, len(models))
	for _, model := range models {
		geminiModels = append(geminiModels, GeminiModel{
			Name:        "models/" + model.ID,
			BaseModelID: model.ID,
			Version:     "001",
			DisplayName: model.DisplayName,
			Description: "",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   geminiModels,
	})
}
