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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/looplj/axonhub/internal/log"
)

var ErrQuotaExceeded = errors.New("realtime quota exceeded")

var ErrAccountingUnavailable = errors.New("realtime accounting unavailable")

var (
	ErrInvalidEvent = errors.New("unsupported or invalid realtime event")
	ErrBackpressure = errors.New("realtime send queue exhausted")
)

const MaxMessageBytes = 256 << 10

type Options struct {
	IdleTimeout, MaxDuration, WriteTimeout, PingInterval time.Duration
	QueueSize                                            int
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
	return o
}

type Hooks struct {
	// BindContext runs before workers start so their operations share the session deadline and cancellation.
	BindContext func(context.Context)
	Client      func([]byte) ([]byte, error)
	Server      func([]byte) ([]byte, error)
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
	var last atomic.Int64
	last.Store(time.Now().UnixNano())
	errs := make(chan error, 4)
	var wg sync.WaitGroup
	var closing atomic.Bool
	closeACK := make(chan struct{})
	var closeOnce sync.Once
	downstreamReaderDone := make(chan struct{})
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
			cancel()
		})
	}
	read := func(conn *websocket.Conn, q *sendQueue, transform func([]byte) ([]byte, error), client bool) func() error {
		return func() error {
			conn.SetReadLimit(MaxMessageBytes)
			if err := conn.SetReadDeadline(time.Now().Add(o.IdleTimeout)); err != nil {
				return err
			}
			conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(o.IdleTimeout)) })
			window := time.Now()
			events, bytes := 0, 0
			for {
				kind, data, err := conn.ReadMessage()
				if err != nil {
					return err
				}
				if client && (closing.Load() || ctx.Err() != nil) {
					continue // Drain unread input through the peer's Close ACK; avoid TCP RST.
				}
				last.Store(time.Now().UnixNano())
				if client {
					if time.Since(window) >= time.Second {
						window = time.Now()
						events = 0
						bytes = 0
					}
					events++
					bytes += len(data)
					if events > 256 || bytes > 2<<20 {
						return errors.New("realtime client data rate exceeded")
					}
				}
				if err := conn.SetReadDeadline(time.Now().Add(o.IdleTimeout)); err != nil {
					return err
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
				if transform != nil {
					original := data
					data, err = transform(data)
					if err != nil {
						if client && ctx.Err() != nil {
							continue
						}
						if !client || errors.Is(err, ErrAccountingUnavailable) || errors.Is(err, ErrBackpressure) {
							return err
						}
						if err := toClient.enqueue(ctx, clientError(original, err)); err != nil {
							return err
						}
						continue
					}
				}
				if err := q.enqueue(ctx, data); err != nil {
					if client && ctx.Err() != nil {
						continue
					}
					return err
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
					if err := conn.SetWriteDeadline(time.Now().Add(o.WriteTimeout)); err != nil {
						return err
					}
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						return err
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
	launch("downstream", write(downstream, toClient.frames))
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
				result = errors.New("realtime session idle timeout")
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
	code := websocket.CloseInternalServerErr
	reason := "internal_error"
	if !IsAbnormal(result) {
		code = websocket.CloseNormalClosure
		reason = "normal_closure"
	}
	if errors.Is(result, ErrBackpressure) {
		code = websocket.CloseTryAgainLater
		reason = "backpressure"
	}
	closing.Store(true)
	cancel()
	upstream.Close() // Stop provider work while the downstream completes its close handshake.
	flushDownstreamClose(ctx, downstream, code, reason, closeACK, downstreamReaderDone)
	downstream.Close()
	wg.Wait()
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
	result, _ := json.Marshal(map[string]any{"type": "error", "event_id": "event_" + uuid.NewString(), "error": map[string]any{"type": "invalid_request_error", "code": code, "event_id": original.ID, "message": message}})
	return result
}

// SessionError classifies the disconnected side without exposing provider payloads.
type SessionError struct {
	Side  string
	Cause error
}

func (e *SessionError) Error() string { return "realtime " + e.Side + " session ended" }
func (e *SessionError) Unwrap() error { return e.Cause }

func IsAbnormal(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var closed *websocket.CloseError
	if errors.As(err, &closed) && (closed.Code == websocket.CloseNormalClosure || closed.Code == websocket.CloseGoingAway) {
		return false
	}
	return true
}
