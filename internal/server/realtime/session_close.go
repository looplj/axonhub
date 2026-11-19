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
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/looplj/axonhub/internal/log"
)

func flushDownstreamClose(ctx context.Context, conn *websocket.Conn, code int, reason string, ack, readerDone <-chan struct{}) {
	ctx = context.WithoutCancel(ctx)
	started := time.Now()
	grace := 250 * time.Millisecond
	if reason == "backpressure" {
		grace = time.Second
	}
	deadline := started.Add(grace)
	log.Info(ctx, "realtime close initiated", log.Int("close_code", code), log.String("reason", reason), log.Int64("initiated_unix_ms", started.UnixMilli()))
	// WriteControl serializes behind any in-flight data write. That writer must
	// finish before a valid control frame can be appended to the TCP byte stream.
	if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), deadline); err != nil {
		var timeout net.Error
		if (errors.As(err, &timeout) && timeout.Timeout()) || !time.Now().Before(deadline) {
			log.Warn(ctx, "realtime close flush timed out", log.Int("close_code", code), log.String("stage", "write"))
		} else {
			log.Warn(ctx, "realtime close write failed", log.Int("close_code", code))
		}
		return
	}
	awaitCloseAcknowledgement(ctx, code, deadline, ack, readerDone)
}

func awaitCloseAcknowledgement(ctx context.Context, code int, deadline time.Time, ack, readerDone <-chan struct{}) {
	timer := time.NewTimer(max(time.Until(deadline), 0))
	defer timer.Stop()
	select {
	case <-ack:
		log.Info(ctx, "realtime close acknowledged", log.Int("close_code", code))
	case <-readerDone:
		// EOF or a protocol read error means the peer can no longer acknowledge.
		log.Info(ctx, "realtime close peer disconnected", log.Int("close_code", code))
	case <-timer.C:
		log.Warn(ctx, "realtime close flush timed out", log.Int("close_code", code), log.String("stage", "acknowledgement"))
	}
}
