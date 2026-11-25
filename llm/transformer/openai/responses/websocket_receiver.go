// 职责: 持续读取上游 WebSocket，并将响应帧交给持有连接的请求。
// 关联文件:
//
//	llm/transformer/openai/responses/websocket_executor.go
//	llm/transformer/openai/responses/websocket_lease_write.go
//
// 注意事项: 空闲连接必须继续读取以响应上游 ping；每个响应帧只交给收到它时的连接租约。

package responses

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	maxWebSocketResponseMessageBytes = 64 << 20
	maxQueuedWebSocketBytes          = 16 << 20
	// Charge small frames for the queue's slice headers and allocation overhead.
	webSocketFrameOverhead = 64
	// Reads cannot process control frames during backpressure. Keep the wait
	// short while allowing the consumer to drain an ordinary burst.
	webSocketQueueWait = time.Second
)

type webSocketReadTarget struct {
	// The receiver mutex protects the queue and byte count.
	messages    [][]byte
	queuedBytes int
	ready       chan struct{}
	space       chan struct{}
	detached    chan struct{}
}

type webSocketReceiver struct {
	conn     *websocket.Conn
	stopping chan struct{}
	done     chan struct{}
	once     sync.Once

	mu     sync.Mutex
	target *webSocketReadTarget

	// Written by receive before closing done, and read only after done is closed.
	err error
}

func newWebSocketReceiver(conn *websocket.Conn, onExit func()) *webSocketReceiver {
	reader := &webSocketReceiver{
		conn:     conn,
		stopping: make(chan struct{}),
		done:     make(chan struct{}),
	}

	go reader.receive(onExit)

	return reader
}

func (r *webSocketReceiver) receive(onExit func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.err = fmt.Errorf("websocket receiver panicked: %v", recovered)
			slog.Error("websocket receiver panicked", slog.Any("panic", recovered))
		}
		if errors.Is(r.err, websocket.ErrMessageTooBig) {
			r.err = fmt.Errorf("%w: %w", httpclient.ErrStreamEventTooLarge, r.err)
			slog.Warn("upstream WebSocket response message exceeded read limit", slog.Int("limit_bytes", maxWebSocketResponseMessageBytes))
		}
		close(r.done)
		r.close()
		if onExit != nil {
			onExit()
		}
	}()

	for {
		_, body, err := r.conn.Reader(context.Background())
		if err != nil {
			r.err = err
			return
		}

		// Reader handles control frames until the next data message starts.
		// Defer allocating its body while an oversized message is queued.
		target, err := r.targetForMessage()
		if err != nil {
			r.err = err
			return
		}
		message, err := io.ReadAll(body)
		if err != nil {
			r.err = err
			return
		}
		if err := r.enqueue(target, message); err != nil {
			r.err = err
			return
		}
	}
}

func (r *webSocketReceiver) targetForMessage() (*webSocketReadTarget, error) {
	r.mu.Lock()
	target := r.target
	oversizedQueued := target != nil && target.queuedBytes > maxQueuedWebSocketBytes
	r.mu.Unlock()
	if target == nil {
		return nil, fmt.Errorf("unexpected upstream WebSocket message with no active request")
	}
	if oversizedQueued {
		if err := r.waitForQueue(target, func() bool { return len(target.messages) == 0 }); err != nil {
			return nil, err
		}
		select {
		case <-target.detached:
			return nil, fmt.Errorf("upstream WebSocket message belongs to a released request")
		default:
		}
	}
	return target, nil
}

func (r *webSocketReceiver) enqueue(target *webSocketReadTarget, message []byte) error {
	size := cap(message) + webSocketFrameOverhead
	r.mu.Lock()
	if r.target != target {
		r.mu.Unlock()
		return fmt.Errorf("upstream WebSocket message belongs to a released request")
	}

	if size > maxQueuedWebSocketBytes {
		// Allow one oversized message in FIFO order. If another data message
		// arrives, targetForMessage waits before reading the new body.
		target.appendMessage(message, size)
		r.mu.Unlock()
		return nil
	}

	queued := target.tryEnqueue(message, size)
	r.mu.Unlock()
	if queued {
		return nil
	}

	return r.waitForQueue(target, func() bool { return target.tryEnqueue(message, size) })
}

// check is called with the receiver mutex held and only while target is active.
func (r *webSocketReceiver) waitForQueue(target *webSocketReadTarget, check func() bool) error {
	wait := time.NewTimer(webSocketQueueWait)
	defer wait.Stop()
	expired := false
	for {
		r.mu.Lock()
		complete := r.target != target || check()
		r.mu.Unlock()
		if complete {
			return nil
		}
		if expired {
			return fmt.Errorf("timed out waiting for upstream WebSocket response queue to drain")
		}

		select {
		case <-target.space:
		case <-target.detached:
			return nil
		case <-r.stopping:
			return net.ErrClosed
		case <-wait.C:
			expired = true
		}
	}
}

// tryEnqueue is called with the receiver mutex held.
func (t *webSocketReadTarget) tryEnqueue(message []byte, size int) bool {
	if size > maxQueuedWebSocketBytes-t.queuedBytes {
		return false
	}
	t.appendMessage(message, size)
	return true
}

// appendMessage is called with the receiver mutex held.
func (t *webSocketReadTarget) appendMessage(message []byte, size int) {
	t.messages = append(t.messages, message)
	t.queuedBytes += size
	select {
	case t.ready <- struct{}{}:
	default:
	}
}

func (r *webSocketReceiver) dequeue(target *webSocketReadTarget) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(target.messages) == 0 {
		return nil, false
	}
	message := target.messages[0]
	target.messages[0] = nil
	target.messages = target.messages[1:]
	target.queuedBytes -= cap(message) + webSocketFrameOverhead
	select {
	case target.space <- struct{}{}:
	default:
	}
	return message, true
}

func (r *webSocketReceiver) ended() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

func (r *webSocketReceiver) attach() (*webSocketReadTarget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ended() {
		return nil, r.err
	}
	target := &webSocketReadTarget{
		ready:    make(chan struct{}, 1),
		space:    make(chan struct{}, 1),
		detached: make(chan struct{}),
	}
	r.target = target
	return target, nil
}

func (r *webSocketReceiver) detach(target *webSocketReadTarget) {
	r.mu.Lock()
	r.target = nil
	target.messages = nil
	target.queuedBytes = 0
	close(target.detached)
	r.mu.Unlock()
}

func (r *webSocketReceiver) next(ctx context.Context, target *webSocketReadTarget) ([]byte, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if message, ok := r.dequeue(target); ok {
			return message, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-r.done:
			// The receiver may have queued a frame after the first dequeue.
			if message, ok := r.dequeue(target); ok {
				return message, nil
			}
			return nil, r.err
		case <-target.detached:
			return nil, fmt.Errorf("websocket response reader closed")
		case <-target.ready:
		}
	}
}

func (r *webSocketReceiver) close() {
	r.once.Do(func() {
		close(r.stopping)
		_ = r.conn.CloseNow()
	})
}
