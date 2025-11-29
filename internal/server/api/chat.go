package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/chat"
)

// StreamWriter is a function type for writing stream events to the response.
type StreamWriter func(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent])

type ChatCompletionHandlers struct {
	ChatCompletionProcessor *chat.ChatCompletionProcessor
	StreamWriter            StreamWriter
}

func NewChatCompletionHandlers(processor *chat.ChatCompletionProcessor) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionProcessor: processor,
		StreamWriter:            WriteSSEStream,
	}
}

// WithStreamWriter returns a new ChatCompletionHandlers with the specified stream writer.
func (handlers *ChatCompletionHandlers) WithStreamWriter(writer StreamWriter) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionProcessor: handlers.ChatCompletionProcessor,
		StreamWriter:            writer,
	}
}

func (handlers *ChatCompletionHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	// Use ReadHTTPRequest to parse the request
	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		httpErr := handlers.ChatCompletionProcessor.Inbound.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))

		return
	}

	if len(genericReq.Body) == 0 {
		c.JSON(http.StatusBadRequest, objects.ErrorResponse{
			Error: objects.Error{
				Type:    http.StatusText(http.StatusBadRequest),
				Message: "Request body is empty",
			},
		})

		return
	}

	// log.Debug(ctx, "Chat completion request", log.Any("request", genericReq))

	result, err := handlers.ChatCompletionProcessor.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing chat completion", log.Cause(err))

		httpErr := handlers.ChatCompletionProcessor.Inbound.TransformError(ctx, err)
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
			log.Debug(ctx, "Close chat stream")

			err := result.ChatCompletionStream.Close()
			if err != nil {
				logger.Error(ctx, "Error closing stream", log.Cause(err))
			}
		}()

		c.Header("Access-Control-Allow-Origin", "*")

		streamWriter := handlers.StreamWriter
		if streamWriter == nil {
			streamWriter = WriteSSEStream
		}

		streamWriter(c, result.ChatCompletionStream)
	}
}

// WriteSSEStream writes stream events as Server-Sent Events (SSE).
func WriteSSEStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	ctx := c.Request.Context()
	clientDisconnected := false

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	clientGone := c.Writer.CloseNotify()

	for {
		select {
		case <-clientGone:
			clientDisconnected = true

			log.Warn(ctx, "Client disconnected, stopping stream")

			return

		case <-ctx.Done():
			log.Warn(ctx, "Context done, stopping stream")

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
