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

	"github.com/coder/websocket"
)

type webSocketReadTarget struct {
	messages chan []byte
	detached chan struct{}
}

type webSocketReceiver struct {
	conn *websocket.Conn
	stop chan struct{}
	done chan struct{}
	once sync.Once

	mu     sync.Mutex
	target *webSocketReadTarget

	// Written by receive before closing done, and read only after done is closed.
	err error
}

func newWebSocketReceiver(conn *websocket.Conn, onExit func()) *webSocketReceiver {
	reader := &webSocketReceiver{
		conn: conn,
		stop: make(chan struct{}),
		done: make(chan struct{}),
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

		r.mu.Lock()
		target := r.target
		r.mu.Unlock()

		if target == nil {
			r.err = fmt.Errorf("unexpected upstream WebSocket message with no active request")
			return
		}

		select {
		case target.messages <- message:
		case <-target.detached:
		case <-r.stop:
			r.err = net.ErrClosed
			return
		}
	}
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
	target := &webSocketReadTarget{messages: make(chan []byte), detached: make(chan struct{})}
	r.target = target
	return target, nil
}

func (r *webSocketReceiver) detach(target *webSocketReadTarget) {
	r.mu.Lock()
	r.target = nil
	close(target.detached)
	r.mu.Unlock()
}

func (r *webSocketReceiver) next(ctx context.Context, target *webSocketReadTarget) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.done:
		return nil, r.err
	case <-target.detached:
		return nil, fmt.Errorf("websocket response reader closed")
	case message := <-target.messages:
		return message, nil
	}
}

func (r *webSocketReceiver) close() {
	r.once.Do(func() {
		close(r.stop)
		_ = r.conn.CloseNow()
	})
}
