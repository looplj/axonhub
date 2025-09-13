package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/aisdk"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type PlaygroundResponseError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type PlaygroundHandlersParams struct {
	fx.In

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     *httpclient.HttpClient
}

type PlaygroundHandlers struct {
	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     *httpclient.HttpClient
}

func NewPlaygroundHandlers(params PlaygroundHandlersParams) *PlaygroundHandlers {
	return &PlaygroundHandlers{
		ChannelService: params.ChannelService,
		RequestService: params.RequestService,
		HttpClient:     params.HttpClient,
	}
}

// ChatCompletion handles playground chat completion requests with optional channel specification.
func (handlers *PlaygroundHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		log.Error(ctx, "Error reading HTTP request", log.Cause(err))
		c.JSON(http.StatusBadRequest, PlaygroundResponseError{
			Error: struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "bad_request",
				Message: err.Error(),
			},
		})

		return
	}

	channelIDStr := c.Query("channel_id")
	if channelIDStr == "" {
		channelIDStr = c.GetHeader("X-Channel-ID")
	}

	log.Debug(ctx, "Received request", log.Any("request", genericReq), log.String("channel_id", channelIDStr))

	var processor *chat.ChatCompletionProcessor

	if channelIDStr != "" {
		channelID, err := objects.ParseGUID(channelIDStr)
		if err != nil {
			log.Error(ctx, "Error parsing channel ID", log.Cause(err))
			c.JSON(http.StatusBadRequest, PlaygroundResponseError{
				Error: struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}{
					Code:    "bad_request",
					Message: err.Error(),
				},
			})

			return
		}

		processor = chat.NewChatCompletionProcessorWithSelector(
			chat.NewSpecifiedChannelSelector(handlers.ChannelService, channelID),
			handlers.RequestService,
			handlers.HttpClient,
			aisdk.NewDataStreamTransformer(),
		)
	} else {
		// Use default processor with all available channels
		processor = chat.NewChatCompletionProcessor(
			handlers.ChannelService,
			handlers.RequestService,
			handlers.HttpClient,
			aisdk.NewDataStreamTransformer(),
		)
	}

	result, err := processor.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing chat completion", log.Cause(err))
		c.JSON(http.StatusInternalServerError, PlaygroundResponseError{
			Error: struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "internal_error",
				Message: err.Error(),
			},
		})

		return
	}

	if result.ChatCompletion != nil {
		resp := result.ChatCompletion

		contentType := "application/json"
		if ct := resp.Headers.Get("Content-Type"); ct != "" {
			contentType = ct
		}

		c.Data(resp.StatusCode, contentType, resp.Body)

		return
	}

	if result.ChatCompletionStream != nil {
		defer func() {
			err := result.ChatCompletionStream.Close()
			if err != nil {
				log.Error(ctx, "Error closing stream", log.Cause(err))
			}
		}()

		// Set AI SDK data stream headers

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("X-Vercel-AI-Data-Stream", "v1")
		c.Status(http.StatusOK)

		writeSSEStream(c, result.ChatCompletionStream)
	}
}
