package api

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/chat"
)

type ChatCompletionErrorHandler interface {
	// HandlerError handles error in non-stream response, should return the proper error format to client.
	HandlerError(c *gin.Context, err error)
}

type ChatCompletionSSEHandlers struct {
	ChatCompletionProcessor *chat.ChatCompletionProcessor
	ErrorHandler            ChatCompletionErrorHandler
}

func NewChatCompletionHandlers(processor *chat.ChatCompletionProcessor) *ChatCompletionSSEHandlers {
	return &ChatCompletionSSEHandlers{
		ChatCompletionProcessor: processor,
	}
}

func (handlers *ChatCompletionSSEHandlers) ChatCompletion(c *gin.Context) {
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
			log.Debug(ctx, "Close chat stream")

			err := result.ChatCompletionStream.Close()
			if err != nil {
				logger.Error(ctx, "Error closing stream", log.Cause(err))
			}
		}()

		// Set SSE headers
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")

		writeSSEStream(c, result.ChatCompletionStream)
	}
}

func writeSSEStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
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

			log.Warn(ctx, "Client disconnected, stopping stream")

			return
		default:
			if stream.Next() {
				cur := stream.Current()
				c.SSEvent(cur.Type, cur.Data)
				log.Debug(ctx, "write stream event", log.Any("event", cur))
				c.Writer.Flush()
			} else {
				if stream.Err() != nil {
					log.Error(ctx, "Error in stream", log.Cause(stream.Err()))
					c.SSEvent("error", stream.Err())
				}

				return
			}
		}
	}
}
