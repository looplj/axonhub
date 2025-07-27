package api

import (
	"io"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/server/biz"
)

type ChatCompletionErrorHandler interface {
	// HandlerError handles error in non-stream response, should return the proper error format to client.
	HandlerError(c *gin.Context, err error)

	// HandleStreamError handles error in stream response, should return the proper error format to client.
	HandleStreamError(c *gin.Context, err error)
}

func NewChatCompletionHandlers(processor *biz.ChatCompletionProcessor) *ChatCompletionSSEHandlers {
	return &ChatCompletionSSEHandlers{
		ChatCompletionProcessor: processor,
	}
}

type ChatCompletionSSEHandlers struct {
	ChatCompletionProcessor *biz.ChatCompletionProcessor
	ErrorHandler            ChatCompletionErrorHandler
}

func (handlers *ChatCompletionSSEHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	// Use ReadHTTPRequest to parse the request
	genericReq, err := llm.ReadHTTPRequest(c.Request)
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
				logger.Error(ctx, "Error closing stream", log.Cause(err))
			}
		}()

		// Set SSE headers
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")

		disconnected := c.Stream(func(w io.Writer) bool {
			if result.ChatCompletionStream.Next() {
				cur := result.ChatCompletionStream.Current()
				log.Debug(ctx, "stream event", log.Any("event", cur))
				c.SSEvent(cur.Type, cur.Data)
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
