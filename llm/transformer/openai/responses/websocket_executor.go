package responses

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

const WebSocketBetaHeaderValue = "responses_websockets=2026-02-06"

const (
	webSocketSessionHeader    = "Session_id"
	webSocketAccountIDHeader  = "Chatgpt-Account-Id"
	webSocketOriginatorHeader = "Originator"
	webSocketUserAgentHeader  = "User-Agent"
	webSocketOrgHeader        = "OpenAI-Organization"
	webSocketProjectHeader    = "OpenAI-Project"
	// OpenAI limits Responses WebSocket connections to 60 minutes. The
	// Codex store=false path depends on connection-local state: real upstream
	// probes show reconnecting loses previous_response_id recovery, and
	// prompt-cache hits appear only when the same WebSocket is reused. Keep
	// useful idle connections close to the upstream cap, but leave enough
	// headroom for a new turn to finish before that cap is reached.
	defaultWebSocketIdleTTL       = 50 * time.Minute
	defaultWebSocketMaxLifetime   = 50 * time.Minute
	defaultWebSocketMaxPoolSize   = 128
	defaultWebSocketMaxRetainedIn = 1 << 20
	minWebSocketCleanupDelay      = 10 * time.Millisecond
)

type WebSocketExecutor struct {
	inner  pipeline.Executor
	dialer *websocket.Dialer

	mu               sync.Mutex
	pool             map[webSocketPoolKey]*pooledWebSocketConn
	cleanupScheduled bool
	cleanupTimer     *time.Timer
	closed           bool
	idleTTL          time.Duration
	maxLifetime      time.Duration
	maxPoolSize      int
	maxRetainedInput int
}

func NewWebSocketExecutor(inner pipeline.Executor) *WebSocketExecutor {
	dialer := &websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}
	if hc, ok := inner.(*httpclient.HttpClient); ok {
		dialer.Proxy = hc.ProxyFunc()
		if native := hc.GetNativeClient(); native != nil {
			if transport, ok := native.Transport.(*http.Transport); ok && transport.TLSClientConfig != nil {
				dialer.TLSClientConfig = transport.TLSClientConfig.Clone()
			}
		}
	}

	return &WebSocketExecutor{
		inner:            inner,
		dialer:           dialer,
		pool:             map[webSocketPoolKey]*pooledWebSocketConn{},
		idleTTL:          defaultWebSocketIdleTTL,
		maxLifetime:      defaultWebSocketMaxLifetime,
		maxPoolSize:      defaultWebSocketMaxPoolSize,
		maxRetainedInput: defaultWebSocketMaxRetainedIn,
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
	if err := TopLevelWebSocketError(chunks); err != nil {
		return nil, err
	}

	body, _, err := AggregateStreamChunks(ctx, chunks)
	if err != nil {
		return nil, err
	}

	// Response-level terminal events (response.failed, response.cancelled,
	// response.incomplete) are valid Responses API payloads, not transport
	// failures. Keep them as HTTP 200 response objects so callers can inspect
	// body.status/error/incomplete_details instead of retrying or disabling the
	// channel as if the WebSocket transport failed.
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    body,
		Request: request,
	}, nil
}

// TopLevelWebSocketError returns an error when collected WebSocket stream events contain a top-level protocol error.
func TopLevelWebSocketError(chunks []*httpclient.StreamEvent) error {
	for _, chunk := range chunks {
		if chunk == nil || chunk.Type != string(StreamEventTypeError) {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal(chunk.Data, &event); err != nil {
			return fmt.Errorf("websocket error event")
		}
		if event.Code != "" && event.Message != "" {
			return fmt.Errorf("websocket error event: %s: %s", event.Code, event.Message)
		}
		if event.Message != "" {
			return fmt.Errorf("websocket error event: %s", event.Message)
		}
		if event.Code != "" {
			return fmt.Errorf("websocket error event: %s", event.Code)
		}

		return fmt.Errorf("websocket error event")
	}

	return nil
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

	payload, err := buildWebSocketCreatePayload(request.Body)
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

	lease, err := e.acquirePreparedLease(ctx, request, wsURL, headers, payload)
	if err != nil {
		return nil, err
	}

	if err := lease.conn.WriteJSON(payload); err != nil {
		lease.release(true)
		return nil, fmt.Errorf("failed to write response.create websocket event: %w", err)
	}

	stream := &webSocketStream{ctx: ctx, lease: lease, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = stream.Close()
		case <-stream.done:
		}
	}()

	return stream, nil
}

func (e *WebSocketExecutor) Inner() pipeline.Executor {
	if e == nil {
		return nil
	}
	return e.inner
}

func (e *WebSocketExecutor) Close() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	e.closed = true
	if e.cleanupTimer != nil {
		e.cleanupTimer.Stop()
		e.cleanupTimer = nil
	}
	e.cleanupScheduled = false
	idle := make([]*pooledWebSocketConn, 0, len(e.pool))
	for key, pc := range e.pool {
		if pc == nil {
			delete(e.pool, key)
			continue
		}
		if len(pc.inFlight) > 0 {
			continue
		}
		delete(e.pool, key)
		idle = append(idle, pc)
	}
	e.mu.Unlock()

	for _, pc := range idle {
		pc.close()
	}
	return nil
}

type webSocketPoolKey struct {
	URL        string
	SessionID  string
	Scope      string
	Auth       string
	AccountID  string
	Originator string
	UserAgent  string
	Org        string
	Project    string
	BetaHeader string
	Headers    string
}

type pooledWebSocketConn struct {
	key        webSocketPoolKey
	conn       *websocket.Conn
	inFlight   chan struct{}
	createdAt  time.Time
	lastUsedAt time.Time
	used       bool

	mu             sync.Mutex
	closed         bool
	lastInput      []json.RawMessage
	lastResponseID string
}

type webSocketLease struct {
	conn          *websocket.Conn
	owner         *WebSocketExecutor
	pooled        *pooledWebSocketConn
	reused        bool
	nextFullInput []json.RawMessage
	once          sync.Once
}

func (e *WebSocketExecutor) acquirePreparedLease(ctx context.Context, request *httpclient.Request, wsURL string, headers http.Header, payload map[string]any) (*webSocketLease, error) {
	originalPayload := clonePayloadMap(payload)
	lease, err := e.acquire(ctx, request, wsURL, headers)
	if err != nil {
		return nil, err
	}

	if err := prepareWebSocketPayloadForLease(payload, lease); err != nil {
		lease.release(true)
		var reconnectErr *webSocketReconnectRequiredError
		if !errors.As(err, &reconnectErr) {
			return nil, err
		}

		restorePayloadMap(payload, originalPayload)
		lease, err = e.acquire(ctx, request, wsURL, headers)
		if err != nil {
			return nil, err
		}
		if err := prepareWebSocketPayloadForLease(payload, lease); err != nil {
			lease.release(true)
			return nil, err
		}
	}

	return lease, nil
}

func clonePayloadMap(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	clone := make(map[string]any, len(payload))
	for key, value := range payload {
		clone[key] = value
	}
	return clone
}

func restorePayloadMap(payload, original map[string]any) {
	for key := range payload {
		delete(payload, key)
	}
	for key, value := range original {
		payload[key] = value
	}
}

func (e *WebSocketExecutor) acquire(ctx context.Context, request *httpclient.Request, wsURL string, headers http.Header) (*webSocketLease, error) {
	key, ok := e.poolKey(ctx, request, wsURL, headers)
	if !ok {
		conn, err := e.dial(ctx, request, wsURL, headers)
		if err != nil {
			return nil, err
		}
		return &webSocketLease{conn: conn}, nil
	}

	for {
		pc, err := e.getOrDialPooled(ctx, request, key, wsURL, headers)
		if err != nil {
			return nil, err
		}

		select {
		case pc.inFlight <- struct{}{}:
			closed, expired, used := pc.leaseState(time.Now(), e.maxLifetime)
			if closed || expired {
				<-pc.inFlight
				e.evict(pc)
				continue
			}
			return &webSocketLease{conn: pc.conn, owner: e, pooled: pc, reused: used}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (e *WebSocketExecutor) getOrDialPooled(ctx context.Context, request *httpclient.Request, key webSocketPoolKey, wsURL string, headers http.Header) (*pooledWebSocketConn, error) {
	now := time.Now()
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, fmt.Errorf("websocket executor is closed")
	}
	e.cleanupExpiredLocked(now)
	if pc := e.pool[key]; pc != nil && !pc.isClosed() {
		e.mu.Unlock()
		return pc, nil
	}
	e.mu.Unlock()

	conn, err := e.dial(ctx, request, wsURL, headers)
	if err != nil {
		return nil, err
	}

	pc := &pooledWebSocketConn{
		key:        key,
		conn:       conn,
		inFlight:   make(chan struct{}, 1),
		createdAt:  now,
		lastUsedAt: now,
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		_ = conn.Close()
		return nil, fmt.Errorf("websocket executor is closed")
	}
	if existing := e.pool[key]; existing != nil && !existing.isClosed() {
		e.mu.Unlock()
		_ = conn.Close()
		return existing, nil
	}
	// maxPoolSize is a hard cap on tracked pooled sessions (idle + in-flight).
	// If every tracked session is currently in-flight, this returns capacity
	// backpressure instead of opening unbounded overflow WebSockets. Same-session
	// concurrency does not hit this branch; it waits on that session's inFlight slot.
	if e.maxPoolSize > 0 && len(e.pool) >= e.maxPoolSize && !e.evictOldestIdleLocked() {
		e.mu.Unlock()
		_ = conn.Close()
		return nil, fmt.Errorf("websocket session pool is full")
	}
	e.pool[key] = pc
	e.scheduleCleanupLocked(now)
	e.mu.Unlock()

	return pc, nil
}

func (e *WebSocketExecutor) dial(ctx context.Context, request *httpclient.Request, wsURL string, headers http.Header) (*websocket.Conn, error) {
	dialer := e.dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return nil, newWebSocketDialError(request, resp, err)
	}
	return conn, nil
}

func (e *WebSocketExecutor) poolKey(ctx context.Context, request *httpclient.Request, wsURL string, headers http.Header) (webSocketPoolKey, bool) {
	sessionID := strings.TrimSpace(headers.Get(webSocketSessionHeader))
	if sessionID == "" {
		if value, ok := shared.GetSessionID(ctx); ok {
			sessionID = strings.TrimSpace(value)
		}
	}
	if sessionID == "" {
		return webSocketPoolKey{}, false
	}
	scope, ok := shared.GetSessionScope(ctx)
	if !ok || strings.TrimSpace(scope) == "" {
		return webSocketPoolKey{}, false
	}

	return webSocketPoolKey{
		URL:        wsURL,
		SessionID:  sessionID,
		Scope:      strings.TrimSpace(scope),
		Auth:       authPoolIdentity(request.Auth, headers),
		AccountID:  strings.TrimSpace(headers.Get(webSocketAccountIDHeader)),
		Originator: strings.TrimSpace(headers.Get(webSocketOriginatorHeader)),
		UserAgent:  strings.TrimSpace(headers.Get(webSocketUserAgentHeader)),
		Org:        strings.TrimSpace(headers.Get(webSocketOrgHeader)),
		Project:    strings.TrimSpace(headers.Get(webSocketProjectHeader)),
		BetaHeader: strings.TrimSpace(headers.Get("OpenAI-Beta")),
		Headers:    headerPoolIdentity(headers),
	}, true
}

func headerPoolIdentity(headers http.Header) string {
	if len(headers) == 0 {
		return ""
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		canonical := http.CanonicalHeaderKey(key)
		if _, excluded := webSocketPoolIdentityExcludedHeaders[canonical]; excluded {
			continue
		}
		keys = append(keys, canonical)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		values := slices.Clone(headers.Values(key))
		sort.Strings(values)
		builder.WriteString(key)
		builder.WriteByte(':')
		for _, value := range values {
			builder.WriteString(value)
			builder.WriteByte('\x00')
		}
		builder.WriteByte('\n')
	}

	return hashSecret(builder.String())
}

var webSocketPoolIdentityExcludedHeaders = canonicalHeaderSet(
	// Already represented as dedicated webSocketPoolKey fields.
	"Authorization",
	webSocketSessionHeader,
	webSocketAccountIDHeader,
	webSocketOriginatorHeader,
	webSocketUserAgentHeader,
	webSocketOrgHeader,
	webSocketProjectHeader,
	"OpenAI-Beta",

	// Protocol/request-shape headers do not affect the WebSocket session identity.
	"Accept",
	"Cache-Control",
	"Connection",

	// Per-request observability headers must not shard the session pool.
	"Baggage",
	"Traceparent",
	"Tracestate",
	"Sentry-Trace",
	"Uber-Trace-Id",
	"X-Amzn-Trace-Id",
	"X-B3-Flags",
	"X-B3-ParentSpanId",
	"X-B3-Sampled",
	"X-B3-SpanId",
	"X-B3-TraceId",
	"X-Cloud-Trace-Context",
	"X-Client-Request-Id",
	"X-Codex-Turn-Metadata",
	"X-Correlation-Id",
	"X-Datadog-Parent-Id",
	"X-Datadog-Sampling-Priority",
	"X-Datadog-Trace-Id",
	"X-Request-Id",
	"X-Request-Start",
	"X-Span-Id",
	"X-Trace-Id",
)

func canonicalHeaderSet(keys ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[http.CanonicalHeaderKey(key)] = struct{}{}
	}
	return set
}

func authPoolIdentity(auth *httpclient.AuthConfig, headers http.Header) string {
	if auth != nil {
		return auth.Type + ":" + auth.HeaderKey + ":" + hashSecret(auth.APIKey)
	}
	return hashSecret(headers.Get("Authorization"))
}

func hashSecret(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (e *WebSocketExecutor) evictOldestIdleLocked() bool {
	var oldestKey webSocketPoolKey
	var oldest *pooledWebSocketConn
	for key, pc := range e.pool {
		if pc == nil {
			delete(e.pool, key)
			continue
		}
		closed, lastUsedAt, _ := pc.poolState(time.Now(), e.maxLifetime)
		if closed {
			delete(e.pool, key)
			continue
		}
		if len(pc.inFlight) > 0 {
			continue
		}
		if oldest == nil || lastUsedAt.Before(oldest.poolLastUsedAt()) {
			oldestKey = key
			oldest = pc
		}
	}
	if oldest == nil {
		return false
	}
	delete(e.pool, oldestKey)
	oldest.close()
	return true
}

func (e *WebSocketExecutor) cleanupExpiredLocked(now time.Time) {
	for key, pc := range e.pool {
		if pc == nil {
			delete(e.pool, key)
			continue
		}
		closed, lastUsedAt, expired := pc.poolState(now, e.maxLifetime)
		if closed {
			delete(e.pool, key)
			continue
		}
		if len(pc.inFlight) > 0 {
			continue
		}
		if expired || (e.idleTTL > 0 && now.Sub(lastUsedAt) > e.idleTTL) {
			delete(e.pool, key)
			pc.close()
		}
	}
}

func (e *WebSocketExecutor) scheduleCleanup() {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.scheduleCleanupLocked(time.Now())
	e.mu.Unlock()
}

func (e *WebSocketExecutor) scheduleCleanupLocked(now time.Time) {
	if e.closed || e.cleanupScheduled || len(e.pool) == 0 || (e.idleTTL <= 0 && e.maxLifetime <= 0) {
		return
	}
	delay := e.nextCleanupDelayLocked(now)
	e.cleanupScheduled = true
	e.cleanupTimer = time.AfterFunc(delay, e.runScheduledCleanup)
}

func (e *WebSocketExecutor) runScheduledCleanup() {
	now := time.Now()
	e.mu.Lock()
	e.cleanupScheduled = false
	e.cleanupTimer = nil
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.cleanupExpiredLocked(now)
	e.scheduleCleanupLocked(now)
	e.mu.Unlock()
}

func (e *WebSocketExecutor) nextCleanupDelayLocked(now time.Time) time.Duration {
	var delay time.Duration
	for _, pc := range e.pool {
		if pc == nil || len(pc.inFlight) > 0 {
			continue
		}
		closed, lastUsedAt, expired := pc.poolState(now, e.maxLifetime)
		if closed || expired {
			return minWebSocketCleanupDelay
		}
		if e.idleTTL > 0 {
			remaining := e.idleTTL - now.Sub(lastUsedAt)
			if remaining <= 0 {
				return minWebSocketCleanupDelay
			}
			if delay == 0 || remaining < delay {
				delay = remaining
			}
		}
		if e.maxLifetime > 0 {
			remaining := e.maxLifetime - now.Sub(pc.createdAt)
			if remaining <= 0 {
				return minWebSocketCleanupDelay
			}
			if delay == 0 || remaining < delay {
				delay = remaining
			}
		}
	}
	if delay <= 0 {
		if e.idleTTL > 0 {
			return e.idleTTL
		}
		if e.maxLifetime > 0 {
			return e.maxLifetime
		}
		return minWebSocketCleanupDelay
	}
	if delay < minWebSocketCleanupDelay {
		return minWebSocketCleanupDelay
	}
	return delay
}

func (e *WebSocketExecutor) evict(pc *pooledWebSocketConn) {
	if pc == nil {
		return
	}
	e.mu.Lock()
	if e.pool[pc.key] == pc {
		delete(e.pool, pc.key)
	}
	e.mu.Unlock()
	pc.close()
}

func (pc *pooledWebSocketConn) leaseState(now time.Time, maxLifetime time.Duration) (closed bool, expired bool, used bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.closed, maxLifetime > 0 && now.Sub(pc.createdAt) > maxLifetime, pc.used
}

func (pc *pooledWebSocketConn) poolState(now time.Time, maxLifetime time.Duration) (closed bool, lastUsedAt time.Time, expired bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.closed, pc.lastUsedAt, maxLifetime > 0 && now.Sub(pc.createdAt) > maxLifetime
}

func (pc *pooledWebSocketConn) poolLastUsedAt() time.Time {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.lastUsedAt
}

func (pc *pooledWebSocketConn) isClosed() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.closed
}

func (pc *pooledWebSocketConn) markUsed() {
	pc.mu.Lock()
	pc.lastUsedAt = time.Now()
	pc.used = true
	pc.mu.Unlock()
}

func (pc *pooledWebSocketConn) close() {
	pc.mu.Lock()
	if pc.closed {
		pc.mu.Unlock()
		return
	}
	pc.closed = true
	pc.lastInput = nil
	pc.lastResponseID = ""
	pc.mu.Unlock()
	_ = pc.conn.Close()
}

func (pc *pooledWebSocketConn) previousTurn() ([]json.RawMessage, string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return cloneRawMessages(pc.lastInput), pc.lastResponseID
}

func (pc *pooledWebSocketConn) rememberTurn(input []json.RawMessage, responseID string, maxInputBytes int) bool {
	if strings.TrimSpace(responseID) == "" {
		return false
	}
	if len(input) == 0 {
		pc.mu.Lock()
		pc.lastInput = nil
		pc.lastResponseID = ""
		pc.mu.Unlock()
		return true
	}
	if maxInputBytes > 0 && rawMessagesSize(input) > maxInputBytes {
		return false
	}
	pc.mu.Lock()
	pc.lastInput = cloneRawMessages(input)
	pc.lastResponseID = strings.TrimSpace(responseID)
	pc.mu.Unlock()
	return true
}

func (l *webSocketLease) release(evict bool) {
	if l == nil {
		return
	}
	l.once.Do(func() {
		if l.pooled == nil {
			_ = l.conn.Close()
			return
		}
		if l.owner != nil {
			l.owner.releasePooled(l.pooled, evict)
		} else if evict {
			l.pooled.close()
		} else {
			l.pooled.markUsed()
		}
		select {
		case <-l.pooled.inFlight:
		default:
		}
	})
}

func (e *WebSocketExecutor) releasePooled(pc *pooledWebSocketConn, evict bool) {
	e.mu.Lock()
	closed := e.closed
	if (evict || closed) && e.pool[pc.key] == pc {
		delete(e.pool, pc.key)
	}
	e.mu.Unlock()

	if evict || closed {
		pc.close()
		return
	}

	pc.markUsed()
	e.scheduleCleanup()
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

type webSocketReconnectRequiredError struct{}

func (e *webSocketReconnectRequiredError) Error() string {
	return "websocket session context changed; reconnect and send full context"
}

func prepareWebSocketPayloadForLease(payload map[string]any, lease *webSocketLease) error {
	if lease == nil || lease.pooled == nil {
		return nil
	}

	fullInput, ok, err := payloadInputItems(payload)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	lease.nextFullInput = cloneRawMessages(fullInput)

	previousInput, previousResponseID := lease.pooled.previousTurn()
	if !lease.reused || previousResponseID == "" || len(previousInput) == 0 {
		return nil
	}
	if explicitPreviousResponseID(payload) != "" && explicitPreviousResponseID(payload) != previousResponseID {
		return &webSocketReconnectRequiredError{}
	}

	suffix, ok := inputSuffix(previousInput, fullInput)
	if !ok || len(suffix) == 0 || suffixStartsWithResponseOutput(suffix) {
		return &webSocketReconnectRequiredError{}
	}

	payload["input"] = suffix
	payload["previous_response_id"] = previousResponseID

	return nil
}

func explicitPreviousResponseID(payload map[string]any) string {
	value, ok := payload["previous_response_id"]
	if !ok {
		return ""
	}
	previousResponseID, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(previousResponseID)
}

func suffixStartsWithResponseOutput(suffix []json.RawMessage) bool {
	if len(suffix) == 0 {
		return false
	}

	var item struct {
		Type string `json:"type"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(suffix[0], &item); err != nil {
		return false
	}

	switch item.Type {
	case "reasoning", "function_call", "custom_tool_call":
		return true
	}
	return item.Role == "assistant"
}

func payloadInputItems(payload map[string]any) ([]json.RawMessage, bool, error) {
	inputValue, ok := payload["input"]
	if !ok || inputValue == nil {
		return nil, false, nil
	}

	inputRaw, err := json.Marshal(inputValue)
	if err != nil {
		return nil, false, fmt.Errorf("failed to encode websocket input payload: %w", err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(inputRaw, &items); err != nil {
		return nil, false, nil
	}

	return items, true, nil
}

func inputSuffix(previous, current []json.RawMessage) ([]json.RawMessage, bool) {
	if len(previous) >= len(current) {
		return nil, false
	}
	for i := range previous {
		if !jsonRawEqual(previous[i], current[i]) {
			return nil, false
		}
	}

	return cloneRawMessages(current[len(previous):]), true
}

func jsonRawEqual(a, b json.RawMessage) bool {
	var compactA bytes.Buffer
	if err := json.Compact(&compactA, a); err != nil {
		return bytes.Equal(a, b)
	}
	var compactB bytes.Buffer
	if err := json.Compact(&compactB, b); err != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(compactA.Bytes(), compactB.Bytes())
}

func cloneRawMessages(src []json.RawMessage) []json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	out := make([]json.RawMessage, len(src))
	for i := range src {
		out[i] = append(json.RawMessage(nil), src[i]...)
	}
	return out
}

func rawMessagesSize(messages []json.RawMessage) int {
	total := 0
	for _, msg := range messages {
		total += len(msg)
	}
	return total
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
	ctx      context.Context
	lease    *webSocketLease
	done     chan struct{}
	doneOnce sync.Once
	mu       sync.Mutex
	current  *httpclient.StreamEvent
	err      error
	closed   bool
	sawEvent bool
	terminal bool
}

func (s *webSocketStream) Next() bool {
	if s.contextCancelled() {
		return false
	}
	if s.isClosed() {
		return false
	}

	_, msg, err := s.lease.conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) || strings.Contains(err.Error(), "use of closed network connection") {
			if ctxErr := s.ctx.Err(); ctxErr != nil {
				s.setErr(ctxErr)
			} else if !s.hasSeenEvent() {
				s.setErr(fmt.Errorf("websocket closed before response event"))
			}
			s.finish(true)
			return false
		}
		s.setErr(err)
		s.finish(true)
		return false
	}

	typ := streamEventType(msg)
	s.setCurrent(&httpclient.StreamEvent{
		Type: typ,
		Data: normalizeWebSocketEvent(msg),
	})
	if isTerminalWebSocketEvent(typ) {
		s.markTerminal()
		// Only top-level `error` events are transport failures. Response-level
		// terminal events are yielded and then close the stream normally so the
		// non-streaming Do path can aggregate their response object.
		evict := terminalWebSocketEventEvicts(typ)
		if typ == "response.completed" {
			responseID := responseIDFromWebSocketEvent(msg)
			if responseID == "" {
				evict = true
			} else {
				// Retaining the full input is a local cache optimization for
				// future incremental sends. If the input exceeds the memory
				// budget, keep the healthy WebSocket connection and let the
				// next turn send full context instead of evicting the session.
				s.lease.rememberTurn(responseID)
			}
		}
		s.finish(evict)
	}

	return true
}

func (s *webSocketStream) Current() *httpclient.StreamEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

func (s *webSocketStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *webSocketStream) Close() error {
	s.finish(true)
	return nil
}

func (s *webSocketStream) contextCancelled() bool {
	select {
	case <-s.ctx.Done():
		s.setContextErr()
		s.finish(true)
		return true
	default:
		return false
	}
}

func (s *webSocketStream) setContextErr() {
	s.mu.Lock()
	if s.err == nil && !s.terminal {
		s.err = s.ctx.Err()
	}
	s.mu.Unlock()
}

func (s *webSocketStream) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err != nil || s.closed
}

func (s *webSocketStream) setCurrent(event *httpclient.StreamEvent) {
	s.mu.Lock()
	s.current = event
	s.sawEvent = true
	s.mu.Unlock()
}

func (s *webSocketStream) hasSeenEvent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sawEvent
}

func (s *webSocketStream) markTerminal() {
	s.mu.Lock()
	s.terminal = true
	s.mu.Unlock()
}

func (s *webSocketStream) setErr(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

func (s *webSocketStream) finish(evict bool) {
	s.doneOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.lease.release(evict)
		close(s.done)
	})
}

func (l *webSocketLease) rememberTurn(responseID string) bool {
	if l == nil || l.pooled == nil {
		return false
	}
	maxInputBytes := 0
	if l.owner != nil {
		maxInputBytes = l.owner.maxRetainedInput
	}
	return l.pooled.rememberTurn(l.nextFullInput, responseID, maxInputBytes)
}

func responseIDFromWebSocketEvent(raw []byte) string {
	var payload struct {
		Response struct {
			ID string `json:"id"`
		} `json:"response"`
	}
	_ = json.Unmarshal(raw, &payload)
	return payload.Response.ID
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
	case "response.completed", "response.failed", "response.cancelled", "response.incomplete", "error":
		return true
	default:
		return false
	}
}

func terminalWebSocketEventEvicts(eventType string) bool {
	switch eventType {
	case "response.failed", "response.cancelled", "response.incomplete", "error":
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
