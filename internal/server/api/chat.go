package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

const (
	errTypeQuotaExhausted = "quota_exhausted"
	errCodeQuotaExhausted = "quota_exhausted"
)

// StreamWriter is a function type for writing stream events to the response.
type StreamWriter func(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent])

type ChatCompletionHandlers struct {
	ChatCompletionOrchestrator *orchestrator.ChatCompletionOrchestrator
	StreamWriter               StreamWriter
}

func NewChatCompletionHandlers(orchestrator *orchestrator.ChatCompletionOrchestrator) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionOrchestrator: orchestrator,
		StreamWriter:               WriteSSEStream,
	}
}

// WithStreamWriter returns a new ChatCompletionHandlers with the specified stream writer.
func (handlers *ChatCompletionHandlers) WithStreamWriter(writer StreamWriter) *ChatCompletionHandlers {
	return &ChatCompletionHandlers{
		ChatCompletionOrchestrator: handlers.ChatCompletionOrchestrator,
		StreamWriter:               writer,
	}
}

func (handlers *ChatCompletionHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	// Use ReadHTTPRequest to parse the request
	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		httpErr := handlers.ChatCompletionOrchestrator.Inbound.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))

		return
	}

	if len(genericReq.Body) == 0 {
		JSONError(c, http.StatusBadRequest, errors.New("Request body is empty"))
		return
	}

	// log.Debug(ctx, "Chat completion request", log.Any("request", genericReq))

	result, err := handlers.ChatCompletionOrchestrator.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing chat completion", log.Cause(err))

		err = wrapQuotaExhaustedAsResponseError(err)

		httpErr := handlers.ChatCompletionOrchestrator.Inbound.TransformError(ctx, err)
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

// StreamErrorFormatter formats a stream error into a JSON-serializable object for SSE error events.
type StreamErrorFormatter func(ctx context.Context, err error) any

// WriteSSEStream writes stream events as Server-Sent Events (SSE) with default error formatting.
func WriteSSEStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteSSEStreamWithErrorFormatter(c, stream, FormatStreamError)
}

// WriteSSEStreamWithErrorFormatter writes stream events as SSE with a custom error formatter.
func WriteSSEStreamWithErrorFormatter(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], formatErr StreamErrorFormatter) {
	ctx := c.Request.Context()
	clientDisconnected := false

	if formatErr == nil {
		formatErr = FormatStreamError
	}

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	// Set SSE headers
	c.Header("Content-Type", sse.ContentType)
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			clientDisconnected = true

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
					c.SSEvent("error", formatErr(ctx, stream.Err()))
				}

				c.Writer.Flush()

				return
			}
		}
	}
}

// FormatStreamError formats a stream error into an OpenAI-compatible JSON error object.
func FormatStreamError(_ context.Context, err error) any {
	errType := "server_error"
	errCode := ""
	requestID := ""

	var quotaErr *orchestrator.QuotaExhaustedError
	if errors.As(err, &quotaErr) {
		return gin.H{
			"error": gin.H{
				"message": quotaErr.Error(),
				"type":    errTypeQuotaExhausted,
				"code":    errCodeQuotaExhausted,
			},
		}
	}

	var respErr *llm.ResponseError
	if errors.As(err, &respErr) {
		if respErr.Detail.Type != "" {
			errType = respErr.Detail.Type
		}

		errCode = respErr.Detail.Code
		requestID = respErr.Detail.RequestID

		return gin.H{
			"error": gin.H{
				"message": respErr.Detail.Message,
				"type":    errType,
				"code":    errCode,
			},
			"request_id": requestID,
		}
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && len(httpErr.Body) > 0 {
		if t := gjson.GetBytes(httpErr.Body, "error.type"); t.Exists() && t.Type == gjson.String && t.String() != "" {
			errType = t.String()
		}

		if c := gjson.GetBytes(httpErr.Body, "error.code"); c.Exists() && c.Type == gjson.String && c.String() != "" {
			errCode = c.String()
		}

		if rid := gjson.GetBytes(httpErr.Body, "request_id"); rid.Exists() && rid.Type == gjson.String && rid.String() != "" {
			requestID = rid.String()
		}
	}

	return gin.H{
		"error": gin.H{
			"message": orchestrator.ExtractErrorMessage(err),
			"type":    errType,
			"code":    errCode,
		},
		"request_id": requestID,
	}
}

func wrapQuotaExhaustedAsResponseError(err error) error {
	if err == nil {
		return nil
	}

	var quotaErr *orchestrator.QuotaExhaustedError
	if errors.As(err, &quotaErr) {
		return &llm.ResponseError{
			StatusCode: http.StatusServiceUnavailable,
			Detail: llm.ErrorDetail{
				Message: quotaErr.Error(),
				Type:    errTypeQuotaExhausted,
				Code:    errCodeQuotaExhausted,
			},
		}
	}

	return err
}
