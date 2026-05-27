package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

const WebSocketBetaHeaderValue = "responses_websockets=2026-02-06"

type WebSocketExecutor struct {
	inner  pipeline.Executor
	dialer *websocket.Dialer
}

func NewWebSocketExecutor(inner pipeline.Executor) *WebSocketExecutor {
	dialer := &websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}
	if hc, ok := inner.(*httpclient.HttpClient); ok {
		dialer.Proxy = hc.ProxyFunc()
	}

	return &WebSocketExecutor{
		inner:  inner,
		dialer: dialer,
	}
}

func (e *WebSocketExecutor) Do(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	stream, err := e.DoStream(ctx, request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	chunks := make([]*httpclient.StreamEvent, 0, 16)
	for stream.Next() {
		ev := stream.Current()
		if ev == nil {
			continue
		}
		chunks = append(chunks, &httpclient.StreamEvent{
			Type:        ev.Type,
			LastEventID: ev.LastEventID,
			Data:        append([]byte(nil), ev.Data...),
		})
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}

	body, _, err := AggregateStreamChunks(ctx, chunks)
	if err != nil {
		return nil, err
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    body,
		Request: request,
	}, nil
}

func (e *WebSocketExecutor) DoStream(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if e == nil {
		return nil, fmt.Errorf("websocket executor is nil")
	}

	wsURL, err := toWebSocketURL(request.URL)
	if err != nil {
		return nil, err
	}

	headers := request.Headers.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	for k := range managedWebSocketHeaders() {
		headers.Del(k)
	}
	if request.Auth != nil {
		if err := applyWebSocketAuth(headers, request.Auth); err != nil {
			return nil, err
		}
	}
	headers.Set("OpenAI-Beta", WebSocketBetaHeaderValue)

	dialer := e.dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return nil, newWebSocketDialError(request, resp, err)
	}

	payload, err := buildWebSocketCreatePayload(request.Body)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := conn.WriteJSON(payload); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to write response.create websocket event: %w", err)
	}

	return &webSocketStream{conn: conn}, nil
}

func (e *WebSocketExecutor) Inner() pipeline.Executor {
	if e == nil {
		return nil
	}
	return e.inner
}

func toWebSocketURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid websocket request url: %w", err)
	}

	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "wss", "ws":
	default:
		return "", fmt.Errorf("unsupported websocket request scheme %q", u.Scheme)
	}

	return u.String(), nil
}

func buildWebSocketCreatePayload(body []byte) (map[string]any, error) {
	payload := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("failed to decode responses request body: %w", err)
		}
	}

	payload["type"] = "response.create"
	delete(payload, "stream")
	delete(payload, "background")

	return payload, nil
}

func applyWebSocketAuth(headers http.Header, auth *httpclient.AuthConfig) error {
	switch auth.Type {
	case httpclient.AuthTypeBearer:
		headers.Set("Authorization", "Bearer "+auth.APIKey)
	case httpclient.AuthTypeAPIKey:
		if auth.HeaderKey == "" {
			return fmt.Errorf("api key header is required")
		}
		headers.Set(auth.HeaderKey, auth.APIKey)
	default:
		return fmt.Errorf("unsupported auth type %q", auth.Type)
	}

	return nil
}

func managedWebSocketHeaders() map[string]struct{} {
	return map[string]struct{}{
		"Content-Length": {},
		"Content-Type":   {},
		"Host":           {},
	}
}

func newWebSocketDialError(request *httpclient.Request, resp *http.Response, err error) error {
	if resp == nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	return &httpclient.Error{
		Method:     request.Method,
		URL:        request.URL,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       body,
		Headers:    resp.Header,
	}
}

type webSocketStream struct {
	conn    *websocket.Conn
	current *httpclient.StreamEvent
	err     error
	closed  bool
}

func (s *webSocketStream) Next() bool {
	if s.err != nil || s.closed {
		return false
	}

	_, msg, err := s.conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) || strings.Contains(err.Error(), "use of closed network connection") {
			return false
		}
		s.err = err
		return false
	}

	typ := streamEventType(msg)
	s.current = &httpclient.StreamEvent{
		Type: typ,
		Data: normalizeWebSocketEvent(msg),
	}
	if isTerminalWebSocketEvent(typ) {
		s.closed = true
	}

	return true
}

func (s *webSocketStream) Current() *httpclient.StreamEvent {
	return s.current
}

func (s *webSocketStream) Err() error {
	return s.err
}

func (s *webSocketStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.conn.Close()
}

func streamEventType(raw []byte) string {
	var payload struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(raw, &payload)
	return payload.Type
}

func isTerminalWebSocketEvent(eventType string) bool {
	switch eventType {
	case "response.completed", "response.failed", "response.cancelled", "error":
		return true
	default:
		return false
	}
}

func normalizeWebSocketEvent(raw []byte) []byte {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return append([]byte(nil), raw...)
	}
	if payload["type"] != "error" {
		return append([]byte(nil), raw...)
	}
	errorValue, ok := payload["error"].(map[string]any)
	if !ok {
		return append([]byte(nil), raw...)
	}
	if value, ok := errorValue["code"]; ok {
		payload["code"] = value
	} else if value, ok := errorValue["type"]; ok {
		payload["code"] = value
	}
	for _, key := range []string{"message", "param"} {
		if value, ok := errorValue[key]; ok {
			payload[key] = value
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return append([]byte(nil), raw...)
	}
	return body
}
