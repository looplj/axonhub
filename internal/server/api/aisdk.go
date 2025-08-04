package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/aisdk"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type AiSdkResponseError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type AiSdkErrorHandler struct{}

func (e *AiSdkErrorHandler) HandlerError(c *gin.Context, err error) {
	c.JSON(500, &AiSdkResponseError{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "internal_error",
			Message: err.Error(),
		},
	})
}

func (e *AiSdkErrorHandler) HandleStreamError(c *gin.Context, err error) {
	// For AI SDK streaming, we write the error directly to the response
	_, _ = c.Writer.Write([]byte("3:" + `"` + err.Error() + `"` + "\n"))
}

type AiSdkHandlersParams struct {
	fx.In

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     httpclient.HttpClient
}

type AiSDKHandlers struct {
	ChatCompletionProcessor *chat.ChatCompletionProcessor
	ErrorHandler            *AiSdkErrorHandler
}

func NewAiSDKHandlers(params AiSdkHandlersParams) *AiSDKHandlers {
	return &AiSDKHandlers{
		ChatCompletionProcessor: chat.NewChatCompletionProcessor(
			params.ChannelService,
			params.RequestService,
			params.HttpClient,
			aisdk.NewInboundTransformer(),
		),
		ErrorHandler: &AiSdkErrorHandler{},
	}
}

func (handlers *AiSDKHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	// Use ReadHTTPRequest to parse the request
	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		handlers.ErrorHandler.HandlerError(c, err)
		return
	}

	result, err := handlers.ChatCompletionProcessor.Process(ctx, genericReq)
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

		disconnected := c.Stream(func(w io.Writer) bool {
			if result.ChatCompletionStream.Next() {
				cur := result.ChatCompletionStream.Current()
				log.Debug(ctx, "stream event", log.Any("event", cur))

				// Write AI SDK format data directly to the response
				if cur.Data != nil {
					_, _ = w.Write(cur.Data)
					// _, _ = w.Write([]byte("\n\n"))
				}
				return true
			}

			return false
		})

		if disconnected {
			log.Warn(ctx, "Client disconnected")
		}

		err := result.ChatCompletionStream.Err()
		if err != nil {
			log.Error(ctx, "Error in stream", log.Cause(err))
			if !disconnected {
				handlers.ErrorHandler.HandleStreamError(c, err)
			}
		}
	}
}
