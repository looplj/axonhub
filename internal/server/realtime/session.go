// 职责: 管理有界、可取消的实时 WebSocket 双向传输与关闭。
// 关联文件:
//
//	internal/server/api/realtime.go
//	internal/server/lifecycle/lifecycle.go
//
// 注意事项:
//
//	每个连接只有一个数据写入协程；队列满时关闭会话，不丢弃音频或无限缓存。
//	钩子只能返回经过脱敏的错误；音视频消息不得记录到日志或追踪。
//	关闭握手有界等待写出与对端确认；等待期间只排空输入，不再转换或转发媒体。

// Package realtime manages bounded realtime WebSocket sessions.
package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/log"
	qwenrt "github.com/looplj/axonhub/llm/realtime"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

var ErrResponseCancelNotActive = errors.New("No active response to cancel")

var ErrQuotaExceeded = errors.New("realtime quota exceeded")

var ErrAccountingUnavailable = errors.New("realtime accounting unavailable")

var (
	ErrInvalidEvent      = errors.New("unsupported or invalid realtime event")
	ErrBackpressure      = errors.New("realtime send queue exhausted")
	ErrIdleTimeout       = errors.New("realtime session idle timeout")
	ErrClientRateLimited = errors.New("realtime client event rate exceeded")
)

var base64CloseCause = regexp.MustCompile(`[A-Za-z0-9+/_-]{24,}={0,2}`)

const MaxMessageBytes = 256 << 10
const defaultClientEventsPerSecond = 1024

// One configuration is in flight and at most four more are waiting.
const maxQueuedUpdates = 4

var errUpdateQueueFull = errors.New("session.update queue is full; wait for session.updated before retrying")

type Options struct {
	IdleTimeout, MaxDuration, WriteTimeout, PingInterval time.Duration
	QueueSize                                            int
	ClientEventsPerSecond                                int
}

func (o Options) defaults() Options {
	if o.IdleTimeout <= 0 {
		o.IdleTimeout = 2 * time.Minute
	}
	if o.MaxDuration <= 0 {
		o.MaxDuration = 15 * time.Minute
	}
	if o.WriteTimeout <= 0 {
		o.WriteTimeout = 5 * time.Second
	}
	if o.PingInterval <= 0 {
		o.PingInterval = 20 * time.Second
	}
	if o.QueueSize <= 0 {
		o.QueueSize = 8
	}
	if o.ClientEventsPerSecond <= 0 {
		o.ClientEventsPerSecond = defaultClientEventsPerSecond
	}
	return o
}

func rejectDuplicateJSONKeys(data []byte) error {
	if !json.Valid(data) {
		return ErrInvalidEvent
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return ErrInvalidEvent
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return ErrInvalidEvent
			}
			key, ok := keyToken.(string)
			if !ok {
				return ErrInvalidEvent
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w: duplicate JSON field %q", ErrInvalidEvent, key)
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return ErrInvalidEvent
		}
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return ErrInvalidEvent
		}
	default:
		return ErrInvalidEvent
	}
	return nil
}

type Hooks struct {
	// BindContext runs before workers start so their operations share the session deadline and cancellation.
	BindContext func(context.Context)
	Client      func([]byte) ([]byte, error)
	Server      func([]byte) ([]byte, error)
	// ServerFollowup optionally emits another message after a converted upstream event.
	ServerFollowup  func([]byte) ([]byte, error)
	UpstreamWritten func([]byte)
	ServerDone      func([]byte) bool
	// Check runs periodically, including while server VAD can generate autonomously.
	Check func(context.Context) error
}

// sendQueue is shared by both directions. A full queue waits for a bounded
// interval, then fails the session rather than dropping media or growing memory.
type sendQueue struct {
	frames chan []byte
	wait   time.Duration
}

func newSendQueue(size int, wait time.Duration) *sendQueue {
	return &sendQueue{frames: make(chan []byte, size), wait: min(time.Second, wait)}
}

func (q *sendQueue) enqueue(ctx context.Context, data []byte) error {
	if data == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.frames <- data:
		return nil
	default:
	}
	timer := time.NewTimer(q.wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.frames <- data:
		return nil
	case <-timer.C:
		return ErrBackpressure
	}
}

// Run owns both connections, joins every worker and closes the peer on any failure.
// Client validation errors are reported without forwarding the rejected event.
//
//nolint:maintidx // Keep session orchestration together so cancellation and close ordering stay visible.
func Run(parent context.Context, downstream, upstream *websocket.Conn, options Options, hooks Hooks) error {
	o := options.defaults()
	ctx, cancel := context.WithTimeout(parent, o.MaxDuration)
	defer cancel()
	defer downstream.Close()
	defer upstream.Close()
	if hooks.BindContext != nil {
		hooks.BindContext(ctx)
	}
	toClient := newSendQueue(o.QueueSize, o.WriteTimeout)
	toProvider := newSendQueue(o.QueueSize, o.WriteTimeout)
	// Serialize client admission and replay. The upstream reader replays after
	// forwarding the acknowledgement or rejection, through the normal writer.
	var updateMu sync.Mutex
	var updates [][]byte
	submitClient := func(data []byte) error {
		if err := rejectDuplicateJSONKeys(data); err != nil {
			return err
		}
		updateMu.Lock()
		defer updateMu.Unlock()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		isUpdate := gjson.GetBytes(data, "type").String() == "session.update"
		isCancel := gjson.GetBytes(data, "type").String() == "response.cancel"
		if len(updates) == 0 || isCancel {
			wire, err := hooks.Client(data)
			if !isUpdate || !errors.Is(err, ErrUpdatePending) {
				if err != nil {
					return err
				}
				return toProvider.enqueue(ctx, wire)
			}
		}
		if len(updates) > 0 && !isUpdate {
			return ErrUpdatePending
		}
		if len(updates) == maxQueuedUpdates {
			return errUpdateQueueFull
		}
		updates = append(updates, data)
		return nil
	}
	replayUpdates := func() error {
		updateMu.Lock()
		defer updateMu.Unlock()
		for len(updates) > 0 && ctx.Err() == nil {
			original := updates[0]
			wire, err := hooks.Client(original)
			if errors.Is(err, ErrUpdatePending) {
				break
			}
			updates[0] = nil
			updates = updates[1:]
			if err == nil {
				return toProvider.enqueue(ctx, wire)
			}
			if errors.Is(err, ErrAccountingUnavailable) || errors.Is(err, ErrBackpressure) {
				return err
			}
			if err := toClient.enqueue(ctx, clientError(original, err)); err != nil {
				return err
			}
		}
		return ctx.Err()
	}
	var last atomic.Int64
	last.Store(time.Now().UnixNano())
	touchActivity := func() { last.Store(time.Now().UnixNano()) }
	errs := make(chan error, 4)
	var wg sync.WaitGroup
	var closing atomic.Bool
	closeACK := make(chan struct{})
	var closeOnce sync.Once
	downstreamReaderDone := make(chan struct{})
	tailWritten := make(chan struct{})
	// The session owner sends the classified reply. Gorilla's default handler
	// would send a close before that owner can serialize shutdown.
	downstream.SetCloseHandler(func(int, string) error {
		closeOnce.Do(func() { close(closeACK) })
		return nil
	})
	launch := func(side string, fn func() error) {
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error(ctx, "realtime worker panicked", log.Any("panic", fmt.Sprintf("%T", r)))
					select {
					case errs <- errors.New("realtime worker failed"):
					default:
					}
					cancel()
				}
			}()
			err := fn()
			if err != nil {
				err = &SessionError{Side: side, Cause: err}
			}
			select {
			case errs <- err:
			default:
			}
			if !isOrderlyUpstreamClose(err) {
				cancel()
			}
		})
	}
	read := func(conn *websocket.Conn, q *sendQueue, transform func([]byte) ([]byte, error), client bool) func() error {
		return func() error {
			conn.SetReadLimit(MaxMessageBytes)
			conn.SetPingHandler(func(payload string) error {
				touchActivity()
				return conn.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(o.WriteTimeout))
			})
			conn.SetPongHandler(func(string) error { touchActivity(); return nil })
			window := time.Now()
			events, bytes := 0, 0
			for {
				kind, data, err := conn.ReadMessage()
				if err != nil {
					if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
						return ErrIdleTimeout
					}
					return err
				}
				if client && (closing.Load() || ctx.Err() != nil) {
					continue // Drain unread input through the peer's Close ACK; avoid TCP RST.
				}
				touchActivity()
				if client {
					if time.Since(window) >= time.Second {
						window = time.Now()
						events = 0
						bytes = 0
					}
					events++
					bytes += len(data)
					if events > o.ClientEventsPerSecond {
						log.Warn(ctx, "realtime client event rate exceeded", log.Int("events", events), log.Int("bytes", bytes), log.Int("limit", o.ClientEventsPerSecond))
						return ErrClientRateLimited
					}
					if bytes > 2<<20 {
						return errors.New("realtime client data rate exceeded")
					}
				}
				if kind != websocket.TextMessage {
					if !client {
						return ErrInvalidEvent
					}
					if err := toClient.enqueue(ctx, clientError(data, err)); err != nil {
						if ctx.Err() != nil {
							continue
						}
						return err
					}
					continue
				}
				if client && transform != nil {
					if err := submitClient(data); err != nil && ctx.Err() == nil {
						if errors.Is(err, ErrAccountingUnavailable) || errors.Is(err, ErrBackpressure) {
							return err
						}
						if err := toClient.enqueue(ctx, clientError(data, err)); err != nil && ctx.Err() == nil {
							return err
						}
					}
					continue
				}
				if transform != nil {
					data, err = transform(data)
					if err != nil {
						return err
					}
				}
				if err := q.enqueue(ctx, data); err != nil {
					if client && ctx.Err() != nil {
						continue
					}
					return err
				}
				if !client && data != nil && hooks.ServerDone != nil && hooks.ServerDone(data) {
					return &websocket.CloseError{Code: websocket.CloseNormalClosure}
				}
				if !client && data != nil && hooks.ServerFollowup != nil {
					followup, err := hooks.ServerFollowup(data)
					if err != nil {
						return err
					}
					if err := q.enqueue(ctx, followup); err != nil {
						return err
					}
				}
				if !client && hooks.Client != nil && data != nil {
					switch gjson.GetBytes(data, "type").String() {
					case "session.updated", "error":
						if err := replayUpdates(); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	write := func(conn *websocket.Conn, q <-chan []byte) func() error {
		return func() error {
			ticker := time.NewTicker(o.PingInterval)
			defer ticker.Stop()
			for {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case data := <-q:
					if data == nil {
						close(tailWritten)
						<-ctx.Done()
						return ctx.Err()
					}
					if err := conn.SetWriteDeadline(time.Now().Add(o.WriteTimeout)); err != nil {
						return err
					}
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						return err
					}
					if conn == upstream && hooks.UpstreamWritten != nil {
						hooks.UpstreamWritten(data)
					}
				case <-ticker.C:
					if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(o.WriteTimeout)); err != nil {
						return err
					}
				}
			}
		}
	}
	readDownstream := read(downstream, toProvider, hooks.Client, true)
	launch("downstream", func() error { defer close(downstreamReaderDone); return readDownstream() })
	launch("upstream", read(upstream, toClient, hooks.Server, false))
	downstreamWriterDone := make(chan struct{})
	writeDownstream := write(downstream, toClient.frames)
	launch("downstream", func() error {
		defer close(downstreamWriterDone)
		return writeDownstream()
	})
	launch("upstream", write(upstream, toProvider.frames))
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var result error
loop:
	for {
		select {
		case result = <-errs:
			break loop
		case <-ctx.Done():
			result = ctx.Err()
			select {
			case err := <-errs:
				result = err
			default:
			}
			break loop
		case <-tick.C:
			if time.Since(time.Unix(0, last.Load())) > o.IdleTimeout {
				result = ErrIdleTimeout
				break loop
			}
			if hooks.Check != nil {
				if err := hooks.Check(ctx); err != nil {
					result = err
					break loop
				}
			}
		}
	}
	closing.Store(true)
	if isOrderlyUpstreamClose(result) {
		if err := drainUpstreamTail(ctx, toClient.frames, tailWritten, min(o.WriteTimeout, time.Second)); err != nil {
			result = err
			// Preserve a transport failure that canceled the drain.
			if errors.Is(err, context.Canceled) {
				select {
				case result = <-errs:
				default:
				}
			}
		}
	}
	code, reason := CloseStatus(result)
	cancel()
	upstream.Close() // Stop provider work while the downstream completes its close handshake.
	if code == websocket.CloseInternalServerErr || reason == "upstream_unavailable" {
		// Let the sole writer exit before sending the terminal error.
		// Reserve the existing Close handshake for slow consumers.
		timer := time.NewTimer(time.Second)
		select {
		case <-downstreamWriterDone:
			message := "The gateway could not continue this Realtime session."
			if reason == "upstream_unavailable" {
				message = "The upstream Realtime connection ended unexpectedly. Retry the session."
			}
			frame, _ := json.Marshal(map[string]any{"type": "error", "error": map[string]string{
				"type": "server_error", "code": reason, "message": message,
			}})
			if err := downstream.SetWriteDeadline(time.Now().Add(time.Second)); err == nil {
				_ = downstream.WriteMessage(websocket.TextMessage, frame)
			}
		case <-timer.C:
			log.Warn(ctx, "realtime terminal error frame timed out")
		}
		timer.Stop()
	}
	if IsAbnormal(result) {
		cause := result
		if side, ok := errors.AsType[*SessionError](result); ok {
			cause = side.Cause
		}
		// Provider close text and arbitrary nested errors can contain wire data.
		log.Warn(ctx, "realtime session terminated", log.String("reason", reason), log.String("cause", safeCloseCause(cause)))
	}
	flushDownstreamClose(ctx, downstream, code, reason, closeACK, downstreamReaderDone)
	downstream.Close()
	wg.Wait()
	clear(updates)
	return result
}

func clientError(data []byte, cause ...error) []byte {
	var original struct {
		ID string `json:"event_id"`
	}
	_ = json.Unmarshal(data, &original)
	if len(original.ID) > 256 {
		original.ID = ""
	}
	code, message := "unsupported_event", "Unsupported or invalid realtime event; see the gateway Realtime capability documentation."
	if len(cause) > 0 && errors.Is(cause[0], ErrQuotaExceeded) {
		code = "quota_exceeded"
		message = "Realtime quota exceeded; stop sending input"
	}
	if len(cause) > 0 {
		for _, safe := range []error{openairt.ErrModelSwitch, openairt.ErrSessionType, openairt.ErrLegacySessionFields, openairt.ErrTranslationSessionFields, qwenrt.ErrConflictingTurnDetection, qwenrt.ErrInvalidNestedTurnDetection} {
			if errors.Is(cause[0], safe) {
				message = safe.Error()
				break
			}
		}
		var field *qwenrt.UnsupportedFieldError
		switch {
		case errors.As(cause[0], &field):
			message = field.Error()
		case errors.Is(cause[0], qwenrt.ErrUnsupportedField):
			message = qwenrt.ErrUnsupportedField.Error()
		case errors.Is(cause[0], ErrResponseCancelNotActive):
			code, message = "response_cancel_not_active", ErrResponseCancelNotActive.Error()
		case errors.Is(cause[0], ErrUpdatePending):
			message = ErrUpdatePending.Error()
		case errors.Is(cause[0], errUpdateQueueFull):
			code, message = "session_update_queue_full", errUpdateQueueFull.Error()
		}
		if errors.Is(cause[0], openairt.ErrModelSwitch) {
			code = "model_switch_not_allowed"
		}
	}
	result, _ := json.Marshal(map[string]any{"type": "error", "event_id": "event_" + uuid.NewString(), "error": map[string]any{"type": "invalid_request_error", "code": code, "event_id": original.ID, "message": message}})
	return result
}

// drainUpstreamTail runs after the upstream reader has queued its final frame.
func drainUpstreamTail(ctx context.Context, frames chan<- []byte, written <-chan struct{}, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	// Only the downstream queue accepts a nil marker. enqueue filters nil events.
	select {
	case frames <- nil:
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ErrBackpressure
	}
	select {
	case <-written:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ErrBackpressure
	}
}

func isOrderlyUpstreamClose(err error) bool {
	var side *SessionError
	var closed *websocket.CloseError
	return errors.As(err, &side) && side.Side == "upstream" && errors.As(err, &closed) &&
		(closed.Code == websocket.CloseNormalClosure || closed.Code == websocket.CloseGoingAway)
}

// SessionError classifies the disconnected side without exposing provider payloads.
type SessionError struct {
	Side  string
	Cause error
}

func (e *SessionError) Error() string { return "realtime " + e.Side + " session ended" }
func (e *SessionError) Unwrap() error { return e.Cause }

func IsAbnormal(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, ErrIdleTimeout) {
		return false
	}
	var closed *websocket.CloseError
	if errors.As(err, &closed) && (closed.Code == websocket.CloseNormalClosure || closed.Code == websocket.CloseGoingAway) {
		return false
	}
	return true
}

// CloseStatus returns only gateway-generated reasons suitable for clients and audit.
func CloseStatus(err error) (int, string) {
	if errors.Is(err, ErrIdleTimeout) {
		return websocket.CloseNormalClosure, "idle_timeout"
	}
	if !IsAbnormal(err) {
		return websocket.CloseNormalClosure, "normal_closure"
	}
	if errors.Is(err, ErrBackpressure) {
		return websocket.CloseTryAgainLater, "backpressure"
	}
	if errors.Is(err, ErrClientRateLimited) {
		return websocket.ClosePolicyViolation, "client_rate_limited"
	}
	var side *SessionError
	var closeErr *websocket.CloseError
	var networkErr net.Error
	if errors.As(err, &side) && side.Side == "downstream" &&
		(errors.As(err, &closeErr) || errors.As(err, &networkErr) || errors.Is(err, io.EOF)) {
		return websocket.CloseGoingAway, "client_closed"
	}
	if errors.As(err, &side) && side.Side == "upstream" {
		if errors.As(err, &closeErr) {
			switch closeErr.Code {
			case websocket.CloseAbnormalClosure, websocket.CloseInternalServerErr, websocket.CloseServiceRestart, websocket.CloseTryAgainLater:
				return websocket.CloseTryAgainLater, "upstream_unavailable"
			}
		} else if errors.As(err, &networkErr) || errors.Is(err, io.EOF) {
			return websocket.CloseTryAgainLater, "upstream_unavailable"
		}
	}
	return websocket.CloseInternalServerErr, "internal_error"
}

func safeCloseCause(err error) string {
	if err == nil {
		return ""
	}
	if closed, ok := errors.AsType[*websocket.CloseError](err); ok {
		return fmt.Sprintf("websocket close code %d", closed.Code)
	}
	for _, safe := range []error{ErrInvalidEvent, ErrAccountingUnavailable, ErrQuotaExceeded, ErrBackpressure, ErrClientRateLimited, io.EOF, context.DeadlineExceeded} {
		if errors.Is(err, safe) {
			return safe.Error()
		}
	}
	switch err.Error() {
	case "unexpected configuration acknowledgement", "duplicate upstream session",
		"response before configuration or concurrent responses", "response.done without response.created",
		"realtime response usage missing or invalid", "realtime usage persistence failed",
		"realtime configuration acknowledgement timed out", "translation configuration acknowledgement timed out":
		return err.Error()
	}
	message := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, err.Error())
	message = base64CloseCause.ReplaceAllString(message, "[redacted-base64]")
	runes := []rune(message)
	if len(runes) > 200 {
		return string(runes[:199]) + "…"
	}
	return message
}
