package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
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

// SSEKeepAliveConfig controls downstream heartbeats for SSE-compatible APIs.
type SSEKeepAliveConfig struct {
	Enabled  bool
	Interval time.Duration
}

type sseHeartbeatFormat uint8

const (
	sseHeartbeatNone sseHeartbeatFormat = iota
	sseHeartbeatOpenAI
	sseHeartbeatAnthropic
)

type ChatCompletionHandlers struct {
	ChatCompletionOrchestrator *orchestrator.ChatCompletionOrchestrator
	StreamWriter               StreamWriter
	sseKeepAlive               SSEKeepAliveConfig
	sseHeartbeatFormat         sseHeartbeatFormat
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

	handlers.ChatCompletionWithRequest(c, genericReq)
}

func (handlers *ChatCompletionHandlers) ChatCompletionWithRequest(c *gin.Context, genericReq *httpclient.Request) {
	ctx := c.Request.Context()

	if genericReq == nil || len(genericReq.Body) == 0 {
		JSONError(c, http.StatusBadRequest, errors.New("Request body is empty"))
		return
	}

	// log.Debug(ctx, "Chat completion request", log.Any("request", genericReq))

	result, err := handlers.ChatCompletionOrchestrator.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing chat completion", log.Cause(err))

		httpErr := transformOrchestratorError(ctx, err, handlers.ChatCompletionOrchestrator)
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

		stream := newUpstreamErrorStream(ctx, result.ChatCompletionStream, handlers.ChatCompletionOrchestrator.SystemService)
		if handlers.StreamWriter != nil {
			handlers.StreamWriter(c, stream)
			return
		}

		writeSSEStream(c, stream, FormatStreamError, handlers.sseKeepAlive, handlers.sseHeartbeatFormat)
	}
}

// StreamErrorFormatter formats a stream error into a JSON-serializable object for SSE error events.
type StreamErrorFormatter func(ctx context.Context, err error) any

// maxStreamEventsAfterCancel bounds how many events the stream writers drain after
// the request context is canceled. Draining lets persistence wrappers observe a
// buffered terminal event, but streams are expected to end promptly on cancellation
// (see passThroughChannelStream.Next); the cap only guards against implementations
// that ignore it. Pass-through channel buffers hold 64 events, so 256 is generous.
const maxStreamEventsAfterCancel = 256

// WriteSSEStream writes stream events as Server-Sent Events (SSE) with default error formatting.
func WriteSSEStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	WriteSSEStreamWithErrorFormatter(c, stream, FormatStreamError)
}

// WriteSSEStreamWithErrorFormatter writes stream events as SSE with a custom error formatter.
func WriteSSEStreamWithErrorFormatter(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], formatErr StreamErrorFormatter) {
	writeSSEStream(c, stream, formatErr, SSEKeepAliveConfig{}, sseHeartbeatNone)
}

func writeSSEStream(
	c *gin.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	formatErr StreamErrorFormatter,
	keepAlive SSEKeepAliveConfig,
	heartbeatFormat sseHeartbeatFormat,
) {
	if !keepAlive.Enabled || keepAlive.Interval <= 0 || heartbeatFormat == sseHeartbeatNone {
		writeSSEStreamWithoutHeartbeat(c, stream, formatErr)
		return
	}

	writeSSEStreamWithHeartbeat(c, stream, formatErr, keepAlive.Interval, heartbeatFormat)
}

func writeSSEStreamWithoutHeartbeat(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent], formatErr StreamErrorFormatter) {
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
	setSSEHeaders(c)
	c.Writer.Flush()

	// Do not pre-check ctx.Done() before Next(). If the client disconnects right
	// after receiving the terminal event, a preferential ctx.Done() check can abort
	// before Next() drains EOF / the last buffered chunk, causing Close() to mark the
	// request canceled even though the stream completed. This relies on the stream
	// contract that Next() returns false promptly once cancellation is observed and
	// its buffer is drained; eventsAfterCancel bounds streams that violate it.
	eventsAfterCancel := 0
	terminalSeen := false

	for {
		if !stream.Next() {
			writeSSEStreamEnd(c, ctx, stream.Err(), formatErr, terminalSeen, &clientDisconnected)

			return
		}

		if ctx.Err() != nil {
			eventsAfterCancel++
			if eventsAfterCancel > maxStreamEventsAfterCancel {
				clientDisconnected = true

				log.Warn(ctx, "Stream still producing after cancellation, aborting drain",
					log.Int("events_after_cancel", eventsAfterCancel))

				return
			}
		}

		cur := stream.Current()
		if orchestrator.IsTerminalStreamEvent(cur) {
			terminalSeen = true
		}

		c.SSEvent(cur.Type, cur.Data)
		log.Debug(ctx, "write stream event", log.Any("event", cur))
		c.Writer.Flush()
	}
}

func writeSSEStreamWithHeartbeat(
	c *gin.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	formatErr StreamErrorFormatter,
	interval time.Duration,
	heartbeatFormat sseHeartbeatFormat,
) {
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

	setSSEHeaders(c)
	c.Writer.Flush()

	reader := newSSEStreamReader(ctx, stream)
	// The caller closes the stream after this function returns. Wait for the
	// reader first so Close cannot race with Next or Current.
	defer reader.Stop()

	timer := time.NewTimer(interval)
	defer timer.Stop()

	timerC := timer.C
	ctxDone := ctx.Done()
	eventsAfterCancel := 0
	terminalSeen := false
	heartbeatCount := 0

	for {
		select {
		case <-ctxDone:
			clientDisconnected = true
			ctxDone = nil
			stopTimer(timer)
			timerC = nil

		case result := <-reader.Results():
			if result.done {
				writeSSEStreamEnd(c, ctx, result.err, formatErr, terminalSeen, &clientDisconnected)
				return
			}

			if ctx.Err() != nil {
				eventsAfterCancel++
				if eventsAfterCancel > maxStreamEventsAfterCancel {
					clientDisconnected = true
					log.Warn(ctx, "Stream still producing after cancellation, aborting drain",
						log.Int("events_after_cancel", eventsAfterCancel))
					return
				}
			}

			cur := result.event
			if orchestrator.IsTerminalStreamEvent(cur) {
				terminalSeen = true
			}

			c.SSEvent(cur.Type, cur.Data)
			log.Debug(ctx, "write stream event", log.Any("event", cur))
			c.Writer.Flush()

			if timerC != nil {
				resetTimer(timer, interval)
			}

		case <-timerC:
			if err := writeSSEHeartbeat(c.Writer, heartbeatFormat); err != nil {
				clientDisconnected = true
				log.Warn(ctx, "Failed to write SSE heartbeat", log.Cause(err))
				return
			}

			heartbeatCount++
			log.Info(ctx, "SSE heartbeat sent",
				log.Int("heartbeat_count", heartbeatCount),
				log.String("heartbeat_format", sseHeartbeatFormatName(heartbeatFormat)),
				log.Duration("interval", interval),
			)

			c.Writer.Flush()
			timer.Reset(interval)
		}
	}
}

// writeSSEStreamEnd finalizes the SSE response once the stream is drained.
//
// terminalSeen reports whether a completion marker was actually written to the
// client. An upstream that ends at EOF without one produces no stream error (see
// the io.EOF branch in the SSE decoder), so without this check the response would
// end silently and the client would read a truncated generation as a successful
// completion. The orchestrator detects the same condition, but only in Close(),
// which runs after this writer returns and the body is already flushed — too late
// to tell the client anything.
func writeSSEStreamEnd(
	c *gin.Context,
	ctx context.Context,
	streamErr error,
	formatErr StreamErrorFormatter,
	terminalSeen bool,
	clientDisconnected *bool,
) {
	switch {
	case streamErr != nil:
		if errors.Is(streamErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			*clientDisconnected = true

			if !errors.Is(streamErr, context.Canceled) {
				log.Warn(ctx, "Stream error after client disconnected", log.Cause(streamErr))
			}
		} else {
			log.Error(ctx, "Error in stream", log.Cause(streamErr))
			c.SSEvent("error", formatErr(ctx, orchestrator.ClassifyUpstreamTransportError(streamErr)))
		}
	case errors.Is(ctx.Err(), context.Canceled):
		*clientDisconnected = true
	case !terminalSeen:
		log.Error(ctx, "Stream ended without terminal event, reporting incomplete stream to client",
			log.Cause(orchestrator.ErrStreamIncomplete))
		c.SSEvent("error", formatErr(ctx, orchestrator.ClassifyUpstreamTransportError(orchestrator.ErrStreamIncomplete)))
	}

	c.Writer.Flush()
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func resetTimer(timer *time.Timer, interval time.Duration) {
	stopTimer(timer)
	timer.Reset(interval)
}

func setSSEHeaders(c *gin.Context) {
	setSSEResponseHeaders(c.Writer.Header())
}

func setSSEResponseHeaders(header http.Header) {
	header.Set("Content-Type", sse.ContentType)
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
}

func writeSSEHeartbeat(writer io.Writer, format sseHeartbeatFormat) error {
	switch format {
	case sseHeartbeatOpenAI:
		_, err := io.WriteString(writer, ": keep-alive\n\n")
		return err
	case sseHeartbeatAnthropic:
		_, err := io.WriteString(writer, "event: ping\ndata: {\"type\":\"ping\"}\n\n")
		return err
	default:
		return errors.New("unsupported SSE heartbeat format")
	}
}

func sseHeartbeatFormatName(format sseHeartbeatFormat) string {
	switch format {
	case sseHeartbeatOpenAI:
		return "openai"
	case sseHeartbeatAnthropic:
		return "anthropic"
	default:
		return "unknown"
	}
}

// WriteBinaryStream writes raw bytes from stream events directly to the response body.
// The first chunk type is treated as the stream Content-Type when present.
func WriteBinaryStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	ctx := c.Request.Context()
	clientDisconnected := false
	headersWritten := false
	contentType := "application/octet-stream"

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	// Same as WriteSSEStream: do not pre-check ctx.Done() before Next(), so a
	// disconnect right after the terminal chunk does not skip drain / completion.
	// The drain after cancellation is bounded by eventsAfterCancel.
	eventsAfterCancel := 0

	for {
		if !stream.Next() {
			if err := stream.Err(); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
					clientDisconnected = true

					// Keep genuine upstream failures visible even when the client is gone.
					if !errors.Is(err, context.Canceled) {
						log.Warn(ctx, "Binary stream error after client disconnected", log.Cause(err))
					}
				} else {
					log.Error(ctx, "Error in binary stream", log.Cause(err))
					if !headersWritten {
						failure := orchestrator.ClassifyUpstreamTransportError(err)
						c.JSON(streamErrorStatus(failure), FormatStreamError(ctx, failure))

						return
					}
				}
			} else if errors.Is(ctx.Err(), context.Canceled) {
				clientDisconnected = true
			}

			c.Writer.Flush()

			return
		}

		if ctx.Err() != nil {
			eventsAfterCancel++
			if eventsAfterCancel > maxStreamEventsAfterCancel {
				clientDisconnected = true

				log.Warn(ctx, "Binary stream still producing after cancellation, aborting drain",
					log.Int("events_after_cancel", eventsAfterCancel))

				return
			}
		}

		cur := stream.Current()
		if cur != nil && cur.Type == httpclient.BinaryStreamDoneEventType {
			continue
		}

		if cur == nil || len(cur.Data) == 0 {
			continue
		}

		if !headersWritten {
			if ct := strings.TrimSpace(cur.Type); ct != "" {
				contentType = ct
			}

			c.Header("Content-Type", contentType)
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Access-Control-Allow-Origin", "*")
			headersWritten = true
		}

		if _, err := c.Writer.Write(cur.Data); err != nil {
			clientDisconnected = true
			log.Warn(ctx, "Failed to write binary stream chunk", log.Cause(err))

			return
		}

		c.Writer.Flush()
	}
}

func streamErrorStatus(err error) int {
	var quotaErr *orchestrator.QuotaExhaustedError
	if errors.As(err, &quotaErr) {
		return http.StatusServiceUnavailable
	}

	var respErr *llm.ResponseError
	if errors.As(err, &respErr) && respErr.StatusCode != 0 {
		return respErr.StatusCode
	}

	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) && httpErr.StatusCode != 0 {
		return httpErr.StatusCode
	}

	return http.StatusInternalServerError
}

// FormatStreamError formats a stream error into an OpenAI-compatible JSON error object.
// When the error carries no upstream request id, the gateway's own request id from ctx
// is used so clients can always correlate an error event with the request log.
func FormatStreamError(ctx context.Context, err error) any {
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
			"request_id": streamErrorRequestID(ctx, requestID),
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
		"request_id": streamErrorRequestID(ctx, requestID),
	}
}

// streamErrorRequestID returns the upstream request id when present, otherwise the
// gateway request id stored in ctx (the value echoed in the AH-Request-Id header).
func streamErrorRequestID(ctx context.Context, requestID string) string {
	if requestID != "" || ctx == nil {
		return requestID
	}

	if id, ok := contexts.GetRequestID(ctx); ok {
		return id
	}

	return ""
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
