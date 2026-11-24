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
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	maxQueuedWebSocketBytes = 16 << 20
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
		close(r.done)
		r.close()
		if onExit != nil {
			onExit()
		}
	}()

	for {
		_, message, err := r.conn.Read(context.Background())
		if err != nil {
			r.err = err
			return
		}

		if err := r.enqueue(message); err != nil {
			r.err = err
			return
		}
	}
}

func (r *webSocketReceiver) enqueue(message []byte) error {
	size := cap(message) + webSocketFrameOverhead
	if size > maxQueuedWebSocketBytes {
		return fmt.Errorf("upstream WebSocket frame exceeds response queue byte limit")
	}

	r.mu.Lock()
	target := r.target
	if target == nil {
		r.mu.Unlock()
		return fmt.Errorf("unexpected upstream WebSocket message with no active request")
	}
	queued := target.tryEnqueue(message, size)
	r.mu.Unlock()
	if queued {
		return nil
	}

	wait := time.NewTimer(webSocketQueueWait)
	defer wait.Stop()
	for {
		select {
		case <-target.space:
			r.mu.Lock()
			active := r.target == target
			queued = active && target.tryEnqueue(message, size)
			r.mu.Unlock()
			if !active || queued {
				return nil
			}
		case <-target.detached:
			return nil
		case <-r.stopping:
			return net.ErrClosed
		case <-wait.C:
			return fmt.Errorf("timed out waiting for space in upstream WebSocket response queue")
		}
	}
}

// tryEnqueue is called with the receiver mutex held.
func (t *webSocketReadTarget) tryEnqueue(message []byte, size int) bool {
	if size > maxQueuedWebSocketBytes-t.queuedBytes {
		return false
	}
	t.messages = append(t.messages, message)
	t.queuedBytes += size
	select {
	case t.ready <- struct{}{}:
	default:
	}
	return true
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
