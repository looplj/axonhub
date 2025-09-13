package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/aisdk"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type AiSdkHandlersParams struct {
	fx.In

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     *httpclient.HttpClient
}

type AiSDKHandlers struct {
	StreamChatCompletionProcessor *chat.ChatCompletionProcessor
	SSEChatCompletionHandler      *ChatCompletionSSEHandlers
}

func NewAiSDKHandlers(params AiSdkHandlersParams) *AiSDKHandlers {
	return &AiSDKHandlers{
		StreamChatCompletionProcessor: chat.NewChatCompletionProcessor(
			params.ChannelService,
			params.RequestService,
			params.HttpClient,
			aisdk.NewTextTransformer(),
		),
		SSEChatCompletionHandler: &ChatCompletionSSEHandlers{
			ChatCompletionProcessor: chat.NewChatCompletionProcessor(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				aisdk.NewDataStreamTransformer(),
			),
		},
	}
}

func (handlers *AiSDKHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	if aisdk.IsDataStream(c.Request.Header) {
		handlers.SSEChatCompletionHandler.ChatCompletion(c)
		return
	}

	// Use ReadHTTPRequest to parse the request
	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		log.Error(ctx, "Error reading HTTP request", log.Cause(err))
		// Create a default transformer for error handling
		transformer := aisdk.NewTransformer(c.Request.Header)
		httpErr := transformer.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))

		return
	}

	result, err := handlers.StreamChatCompletionProcessor.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing chat completion", log.Cause(err))
		httpErr := handlers.StreamChatCompletionProcessor.Inbound.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))

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

		// Set text stream headers
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

			log.Warn(ctx, "Client disconnected, stop streaming")

			return
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
