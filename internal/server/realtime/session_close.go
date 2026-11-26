// 职责: 有界写出分类 Close 帧并等待对端确认，再允许会话关闭底层 TCP。
// 关联文件:
//
//	internal/server/realtime/session.go
//	internal/log/global.go
//
// 注意事项: 背压排队等待保持原有一秒；此处的一秒只用于关闭握手，不延长无限期停读的连接。

package realtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/coder/websocket"

	"github.com/looplj/axonhub/internal/log"
)

func flushDownstreamClose(ctx context.Context, conn *websocket.Conn, code int, reason string, killDownstream context.CancelFunc, readerDone <-chan struct{}) {
	ctx = context.WithoutCancel(ctx)
	started := time.Now()
	grace := 250 * time.Millisecond
	if reason == "backpressure" {
		grace = time.Second
	}
	log.Info(ctx, "realtime close initiated", log.Int("close_code", code), log.String("reason", reason), log.Int64("initiated_unix_ms", started.UnixMilli()))
	// If the downstream reader already exited (EOF, peer close, or protocol/size
	// limit error already answered by coder/websocket), do not enter conn.Close's
	// 5s handshake wait without an active cancellable reader.
	select {
	case <-readerDone:
		log.Info(ctx, "realtime close peer disconnected", log.Int("close_code", code))
		return
	default:
	}
	// conn.Close serializes its Close frame behind any in-flight data write and
	// awaits the peer Close ACK while the downstream reader drains input frames.
	var closeErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				log.Error(ctx, "realtime close handshake panicked", log.Int("close_code", code), log.String("panic_type", fmt.Sprintf("%T", r)))
				closeErr = errors.New("realtime close handshake panicked")
			}
		}()
		closeErr = conn.Close(websocket.StatusCode(code), reason)
	}()
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
		switch {
		case closeErr == nil || websocket.CloseStatus(closeErr) != -1:
			log.Info(ctx, "realtime close acknowledged", log.Int("close_code", code))
		case errors.Is(closeErr, io.EOF) || errors.Is(closeErr, net.ErrClosed):
			log.Info(ctx, "realtime close peer disconnected", log.Int("close_code", code))
		default:
			log.Warn(ctx, "realtime close write failed", log.Int("close_code", code))
		}
	case <-timer.C:
		log.Warn(ctx, "realtime close flush timed out", log.Int("close_code", code), log.String("stage", "acknowledgement"))
		// Canceling downstreamCtx aborts any active Read/Write and closes the socket
		// so conn.Close cannot outlive the bounded grace window.
		killDownstream()
		select {
		case <-done:
		case <-readerDone:
		case <-time.After(100 * time.Millisecond):
		}
	}
}
