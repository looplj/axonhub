package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/llm/transformer/aisdk"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type PlaygroundResponseError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type PlaygroundErrorHandler struct{}

func (e *PlaygroundErrorHandler) HandlerError(c *gin.Context, err error) {
	c.JSON(500, &PlaygroundResponseError{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "internal_error",
			Message: err.Error(),
		},
	})
}

func (e *PlaygroundErrorHandler) HandleStreamError(c *gin.Context, err error) {
	// For playground streaming, we write the error directly to the response
	_, _ = c.Writer.Write([]byte("3:" + `"` + err.Error() + `"` + "\n"))
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
	ErrorHandler   *PlaygroundErrorHandler
}

func NewPlaygroundHandlers(params PlaygroundHandlersParams) *PlaygroundHandlers {
	return &PlaygroundHandlers{
		ChannelService: params.ChannelService,
		RequestService: params.RequestService,
		HttpClient:     params.HttpClient,
		ErrorHandler:   &PlaygroundErrorHandler{},
	}
}

// ChatCompletion handles playground chat completion requests with optional channel specification.
func (handlers *PlaygroundHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		handlers.ErrorHandler.HandlerError(c, err)
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
			handlers.ErrorHandler.HandlerError(c, err)
			return
		}

		processor = &chat.ChatCompletionProcessor{
			ChannelSelector: chat.NewSpecifiedChannelSelector(handlers.ChannelService, channelID),
			Inbound:         aisdk.NewTextTransformer(),
			RequestService:  handlers.RequestService,
			PipelineFactory: pipeline.NewFactory(handlers.HttpClient),
		}
	} else {
		// Use default processor with all available channels
		processor = chat.NewChatCompletionProcessor(
			handlers.ChannelService,
			handlers.RequestService,
			handlers.HttpClient,
			aisdk.NewTextTransformer(),
		)
	}

	result, err := processor.Process(ctx, genericReq)
	if err != nil {
		handlers.ErrorHandler.HandlerError(c, err)
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

		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("X-Vercel-AI-Data-Stream", "v1")
		c.Status(http.StatusOK)

		writeAITextStream(c, result.ChatCompletionStream)
	}
}

func writeAITextStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	ctx := c.Request.Context()
	clientDisconnected := false

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	clientGone := c.Writer.CloseNotify()

	for {
		select {
		case <-clientGone:
			clientDisconnected = true

			log.Warn(ctx, "Client disconnected")
			// continue to read the rest of the stream to collect stream.
		default:
			if stream.Next() {
				cur := stream.Current()
				_, _ = c.Writer.Write(cur.Data)
				log.Debug(ctx, "write stream event", log.Any("event", cur))
				c.Writer.Flush()
			} else {
				if err := stream.Err(); err != nil {
					log.Error(ctx, "Error in stream", log.Cause(err))
					_, _ = c.Writer.Write([]byte("3:" + `"` + err.Error() + `"` + "\n"))
				}

				return
			}
		}
	}
}
