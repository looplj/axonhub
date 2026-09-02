package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

const (
	responseCreateWebSocketEventType   = "response.create"
	responsesWebSocketMaxMessageSize   = 1 << 20
	responsesWebSocketIdleTimeout      = 5 * time.Minute
	responsesWebSocketPingInterval     = responsesWebSocketIdleTimeout / 2
	responsesWebSocketPingWriteTimeout = 10 * time.Second
	responsesWebSocketWriteTimeout     = 10 * time.Second
)

type responsesWebSocketProcessFunc func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error)

type responsesWebSocketErrorFunc func(context.Context, error) *httpclient.Error

// serveResponsesWebSocket implements the downstream Responses WebSocket mode.
// Each response.create message is processed synchronously, which preserves the
// protocol's single in-flight response per connection rule.
func serveResponsesWebSocket(
	c *gin.Context,
	requestTimeout time.Duration,
	process responsesWebSocketProcessFunc,
	transformError responsesWebSocketErrorFunc,
) {
	if !websocket.IsWebSocketUpgrade(c.Request) {
		JSONError(c, http.StatusBadRequest, errors.New("websocket upgrade required"))
		return
	}

	ctx := shared.WithResponsesWebSocket(c.Request.Context())
	if _, ok := shared.GetSessionID(ctx); !ok {
		ctx = shared.WithSessionID(ctx, "responses-ws-"+uuid.NewString())
	}
	c.Request = c.Request.WithContext(ctx)

	responseHeaders := c.Writer.Header().Clone()
	upgrader := new(websocket.Upgrader)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, responseHeaders)
	if err != nil {
		log.Warn(ctx, "Failed to upgrade Responses WebSocket", log.Cause(err))
		return
	}
	defer conn.Close()

	conn.SetReadLimit(responsesWebSocketMaxMessageSize)
	if err := conn.SetReadDeadline(time.Now().Add(responsesWebSocketIdleTimeout)); err != nil {
		log.Warn(ctx, "Failed to set Responses WebSocket read deadline", log.Cause(err))
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(responsesWebSocketIdleTimeout))
	})

	pingDone := make(chan struct{})
	defer close(pingDone)
	go runResponsesWebSocketPings(ctx, conn, pingDone)

	session := new(responsesWebSocketSession)
	for {
		messageType, message, readErr := conn.ReadMessage()
		if readErr != nil {
			if !websocket.IsCloseError(readErr, websocket.CloseNormalClosure, websocket.CloseGoingAway) &&
				!errors.Is(readErr, io.EOF) && ctx.Err() == nil {
				log.Warn(ctx, "Failed to read Responses WebSocket message", log.Cause(readErr))
			}
			return
		}
		if err := conn.SetReadDeadline(time.Now().Add(responsesWebSocketIdleTimeout)); err != nil {
			log.Warn(ctx, "Failed to refresh Responses WebSocket read deadline", log.Cause(err))
			return
		}

		if messageType != websocket.TextMessage {
			if err := writeResponsesWebSocketError(conn, invalidResponsesWebSocketRequest("response.create must be sent as a text message", "")); err != nil {
				return
			}
			continue
		}

		request, warmup, requestErr := session.prepareRequest(c.Request, message)
		if requestErr != nil {
			if err := writeResponsesWebSocketError(conn, requestErr); err != nil {
				return
			}
			continue
		}
		if warmup != nil {
			if err := writeResponsesWebSocketWarmup(conn, warmup); err != nil {
				return
			}
			continue
		}

		requestCtx := ctx
		cancel := func() {}
		if requestTimeout > 0 {
			requestCtx, cancel = context.WithTimeout(ctx, requestTimeout)
		}
		if request.RawRequest != nil {
			request.RawRequest = request.RawRequest.WithContext(requestCtx)
		}

		result, processErr := process(requestCtx, request)
		if processErr != nil {
			httpErr := transformResponsesWebSocketError(requestCtx, processErr, transformError)
			cancel()
			if err := writeResponsesWebSocketError(conn, httpErr); err != nil {
				return
			}
			continue
		}

		writeErr := writeResponsesWebSocketResult(requestCtx, conn, result, transformError)
		cancel()
		if writeErr != nil {
			if !errors.Is(writeErr, context.Canceled) && ctx.Err() == nil {
				log.Warn(ctx, "Failed to write Responses WebSocket result", log.Cause(writeErr))
			}
			return
		}
	}
}

func runResponsesWebSocketPings(ctx context.Context, conn *websocket.Conn, done <-chan struct{}) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(ctx, "Panic while sending Responses WebSocket pings", log.Any("panic", recovered))
		}
	}()

	ticker := time.NewTicker(responsesWebSocketPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			deadline := time.Now().Add(responsesWebSocketPingWriteTimeout)
			if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				return
			}
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

type responsesWebSocketSession struct {
	warmupID      string
	warmupPayload map[string]json.RawMessage
}

type responsesWebSocketWarmup struct {
	ID                 string
	Model              string
	PreviousResponseID string
}

func (s *responsesWebSocketSession) prepareRequest(
	rawRequest *http.Request,
	message []byte,
) (*httpclient.Request, *responsesWebSocketWarmup, *httpclient.Error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(message, &payload); err != nil || payload == nil {
		return nil, nil, invalidResponsesWebSocketRequest("invalid response.create JSON payload", "")
	}

	var eventType string
	if rawType, ok := payload["type"]; !ok || json.Unmarshal(rawType, &eventType) != nil || eventType != responseCreateWebSocketEventType {
		return nil, nil, invalidResponsesWebSocketRequest("expected a response.create event", "type")
	}

	generate := true
	if rawGenerate, ok := payload["generate"]; ok {
		if err := json.Unmarshal(rawGenerate, &generate); err != nil {
			return nil, nil, invalidResponsesWebSocketRequest("generate must be a boolean", "generate")
		}
	}
	displayControls := make(map[string]json.RawMessage, 2)
	for _, key := range []string{"generate", "background"} {
		if value, ok := payload[key]; ok {
			displayControls[key] = append(json.RawMessage(nil), value...)
		}
	}

	delete(payload, "type")
	delete(payload, "generate")
	delete(payload, "background")

	previousResponseID := responseWebSocketStringField(payload, "previous_response_id")
	if s.warmupID != "" && previousResponseID == s.warmupID {
		var err error
		payload, err = mergeResponsesWebSocketWarmup(s.warmupPayload, payload)
		if err != nil {
			return nil, nil, invalidResponsesWebSocketRequest(err.Error(), "input")
		}
		s.warmupID = ""
		s.warmupPayload = nil
	}

	if !generate {
		model := responseWebSocketStringField(payload, "model")
		if model == "" {
			return nil, nil, invalidResponsesWebSocketRequest("model is required", "model")
		}

		id := "resp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		s.warmupID = id
		s.warmupPayload = cloneResponsesWebSocketPayload(payload)

		return nil, &responsesWebSocketWarmup{
			ID:                 id,
			Model:              model,
			PreviousResponseID: responseWebSocketStringField(payload, "previous_response_id"),
		}, nil
	}

	payload["stream"] = json.RawMessage("true")

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, invalidResponsesWebSocketRequest("failed to encode response.create payload", "")
	}

	request := rawRequest.Clone(rawRequest.Context())
	request.Method = http.MethodPost
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	request.Header = responseWebSocketRequestHeaders(rawRequest.Header)

	genericRequest, err := httpclient.ReadHTTPRequest(request)
	if err != nil {
		return nil, nil, invalidResponsesWebSocketRequest(fmt.Sprintf("failed to read response.create payload: %v", err), "")
	}

	// Body is the normalized HTTP payload consumed by the orchestrator. Keep a
	// separate JSON representation of the WebSocket frame for request history and
	// curl previews so the protocol event type is not lost and the implicit stream
	// flag is not presented as client input.
	displayPayload := cloneResponsesWebSocketPayload(payload)
	delete(displayPayload, "stream")
	maps.Copy(displayPayload, displayControls)
	displayPayload["type"] = json.RawMessage(`"response.create"`)
	displayBody, err := json.Marshal(displayPayload)
	if err != nil {
		return nil, nil, invalidResponsesWebSocketRequest("failed to encode response.create display payload", "")
	}
	genericRequest.JSONBody = displayBody

	return genericRequest, nil, nil
}

func mergeResponsesWebSocketWarmup(
	base map[string]json.RawMessage,
	continuation map[string]json.RawMessage,
) (map[string]json.RawMessage, error) {
	merged := cloneResponsesWebSocketPayload(base)
	for key, value := range continuation {
		if key == "input" || key == "previous_response_id" {
			continue
		}
		merged[key] = append(json.RawMessage(nil), value...)
	}

	baseInput, baseHasInput := base["input"]
	continuationInput, continuationHasInput := continuation["input"]
	switch {
	case baseHasInput && continuationHasInput:
		items, err := appendResponsesWebSocketInput(baseInput, continuationInput)
		if err != nil {
			return nil, err
		}
		merged["input"] = items
	case continuationHasInput:
		merged["input"] = append(json.RawMessage(nil), continuationInput...)
	}

	if previous, ok := base["previous_response_id"]; ok {
		merged["previous_response_id"] = append(json.RawMessage(nil), previous...)
	} else {
		delete(merged, "previous_response_id")
	}

	return merged, nil
}

func appendResponsesWebSocketInput(first, second json.RawMessage) (json.RawMessage, error) {
	firstItems, err := responsesWebSocketInputItems(first)
	if err != nil {
		return nil, err
	}
	secondItems, err := responsesWebSocketInputItems(second)
	if err != nil {
		return nil, err
	}

	items := append(firstItems, secondItems...)
	return json.Marshal(items)
}

func responsesWebSocketInputItems(input json.RawMessage) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err == nil {
		return items, nil
	}

	var text string
	if err := json.Unmarshal(input, &text); err != nil {
		return nil, fmt.Errorf("input must be a string or an array")
	}

	item, err := json.Marshal(gin.H{
		"type":    "message",
		"role":    "user",
		"content": text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode warmup input: %w", err)
	}

	return []json.RawMessage{item}, nil
}

func cloneResponsesWebSocketPayload(payload map[string]json.RawMessage) map[string]json.RawMessage {
	cloned := make(map[string]json.RawMessage, len(payload))
	for key, value := range payload {
		cloned[key] = append(json.RawMessage(nil), value...)
	}
	return cloned
}

func responseWebSocketStringField(payload map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(payload[key], &value)
	return value
}

func writeResponsesWebSocketWarmup(conn *websocket.Conn, warmup *responsesWebSocketWarmup) error {
	if warmup == nil {
		return nil
	}

	createdAt := time.Now().Unix()
	response := gin.H{
		"id":         warmup.ID,
		"object":     "response",
		"created_at": createdAt,
		"model":      warmup.Model,
		"status":     "in_progress",
		"output":     []any{},
	}
	if warmup.PreviousResponseID != "" {
		response["previous_response_id"] = warmup.PreviousResponseID
	}

	if err := writeResponsesWebSocketJSON(conn, gin.H{
		"type":            "response.created",
		"sequence_number": 0,
		"response":        response,
	}); err != nil {
		return err
	}
	if err := writeResponsesWebSocketJSON(conn, gin.H{
		"type":            "response.in_progress",
		"sequence_number": 1,
		"response":        response,
	}); err != nil {
		return err
	}

	response["status"] = "completed"
	return writeResponsesWebSocketJSON(conn, gin.H{
		"type":            "response.completed",
		"sequence_number": 2,
		"response":        response,
	})
}

func responseWebSocketRequestHeaders(headers http.Header) http.Header {
	result := headers.Clone()
	for key := range result {
		canonical := http.CanonicalHeaderKey(key)
		if canonical == "Connection" || canonical == "Upgrade" || strings.HasPrefix(canonical, "Sec-Websocket-") {
			result.Del(key)
		}
	}
	result.Del("Content-Encoding")
	result.Del("Content-Length")
	result.Set("Content-Type", "application/json")

	return result
}

func writeResponsesWebSocketResult(
	ctx context.Context,
	conn *websocket.Conn,
	result orchestrator.ChatCompletionResult,
	transformError responsesWebSocketErrorFunc,
) error {
	if result.ChatCompletionStream != nil {
		stream := result.ChatCompletionStream
		defer stream.Close()

		terminalSeen := false
		for stream.Next() {
			event := stream.Current()
			if event == nil || len(event.Data) == 0 {
				continue
			}
			if orchestrator.IsTerminalStreamEvent(event) {
				terminalSeen = true
			}
			if err := writeResponsesWebSocketMessage(conn, websocket.TextMessage, event.Data); err != nil {
				return err
			}
		}

		if err := stream.Err(); err != nil && !terminalSeen {
			return writeResponsesWebSocketError(conn, transformResponsesWebSocketError(ctx, err, transformError))
		}

		return nil
	}

	if result.ChatCompletion != nil {
		var response json.RawMessage
		if !json.Valid(result.ChatCompletion.Body) {
			return writeResponsesWebSocketError(conn, invalidResponsesWebSocketRequest("invalid Responses result", ""))
		}
		response = result.ChatCompletion.Body

		return writeResponsesWebSocketJSON(conn, gin.H{
			"type":            "response.completed",
			"sequence_number": 0,
			"response":        response,
		})
	}

	return writeResponsesWebSocketError(conn, invalidResponsesWebSocketRequest("Responses request returned no result", ""))
}

func transformResponsesWebSocketError(ctx context.Context, err error, transformError responsesWebSocketErrorFunc) *httpclient.Error {
	if transformError != nil {
		if transformed := transformError(ctx, err); transformed != nil {
			return transformed
		}
	}

	return &httpclient.Error{
		Method:     "",
		URL:        "",
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       []byte(`{"error":{"message":"internal server error","type":"server_error"}}`),
		Headers:    nil,
	}
}

func invalidResponsesWebSocketRequest(message, param string) *httpclient.Error {
	detail := gin.H{
		"message": message,
		"type":    "invalid_request_error",
		"code":    "invalid_request_error",
	}
	if param != "" {
		detail["param"] = param
	}

	body, _ := json.Marshal(gin.H{"error": detail})
	return &httpclient.Error{
		Method:     "",
		URL:        "",
		StatusCode: http.StatusBadRequest,
		Status:     http.StatusText(http.StatusBadRequest),
		Body:       body,
		Headers:    nil,
	}
}

func writeResponsesWebSocketError(conn *websocket.Conn, httpErr *httpclient.Error) error {
	status := http.StatusInternalServerError
	if httpErr != nil && httpErr.StatusCode != 0 {
		status = httpErr.StatusCode
	}

	detail := map[string]any{
		"message": http.StatusText(status),
		"type":    "server_error",
	}
	if httpErr != nil && len(httpErr.Body) > 0 {
		var envelope struct {
			Error map[string]any `json:"error"`
		}
		if err := json.Unmarshal(httpErr.Body, &envelope); err == nil && len(envelope.Error) > 0 {
			detail = envelope.Error
		}
	}

	return writeResponsesWebSocketJSON(conn, gin.H{
		"type":   "error",
		"status": status,
		"error":  detail,
	})
}

func writeResponsesWebSocketMessage(conn *websocket.Conn, messageType int, data []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(responsesWebSocketWriteTimeout)); err != nil {
		return err
	}

	return conn.WriteMessage(messageType, data)
}

func writeResponsesWebSocketJSON(conn *websocket.Conn, value any) error {
	if err := conn.SetWriteDeadline(time.Now().Add(responsesWebSocketWriteTimeout)); err != nil {
		return err
	}

	return conn.WriteJSON(value)
}
