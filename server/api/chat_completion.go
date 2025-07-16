package api

import (
	"io"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/server/biz"
)

func NewChatCompletionHandlers(processor *biz.ChatCompletionProcessor) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionProcessor: processor,
	}
}

type ChatCompletionHandlers struct {
	ChatCompletionProcessor *biz.ChatCompletionProcessor
	ErrorHandler            func(c *gin.Context, err error)
}

func (handlers *ChatCompletionHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	// Use ReadHTTPRequest to parse the request
	genericReq, err := llm.ReadHTTPRequest(c.Request)
	if err != nil {
		handlers.ErrorHandler(c, err)
		return
	}

	result, err := handlers.ChatCompletionProcessor.Process(ctx, genericReq)
	if err != nil {
		handlers.ErrorHandler(c, err)
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
				c.SSEvent("", cur.Body)
				return true
			}
			return false
		})
		if disconnected {
			logger.Debug(ctx, "Client disconnected")
		}
	}
}
