// 职责: 通过 HTTP Transport 建立上游 WebSocket 并保留原始读取活动观察器。
// 关联文件:
//
//	llm/transformer/openai/responses/websocket_executor.go
//	llm/httpclient/client.go
//
// 注意事项:
//
//	coder 不暴露底层连接，必须在 HTTP 握手获取连接时单独保存观察器。
//	上游消息保持无限读取，避免 coder 默认限制截断响应。
package responses

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/coder/websocket"

	"github.com/looplj/axonhub/llm/httpclient"
)

func (e *WebSocketExecutor) dial(ctx context.Context, request *httpclient.Request, wsURL string, headers http.Header) (*websocket.Conn, *webSocketActivityConn, error) {
	ctx, activity, stopIdle := httpclient.NewStreamIdleContext(ctx)
	defer stopIdle()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	transport := e.transport.Clone()
	defer transport.CloseIdleConnections()
	dialContext := transport.DialContext
	if dialContext == nil {
		dialContext = (&net.Dialer{}).DialContext
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &webSocketActivityConn{Conn: conn, activity: activity}, nil
	}

	var activityConn *webSocketActivityConn
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			conn := info.Conn
			for {
				tlsConn, ok := conn.(interface{ NetConn() net.Conn })
				if !ok {
					break
				}
				conn = tlsConn.NetConn()
			}
			activityConn, _ = conn.(*webSocketActivityConn)
		},
	})
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		HTTPHeader: headers,
	})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp != nil {
		request.ObserveResponseHeaders(ctx, resp.Header)
	}
	if err != nil {
		if cause := httpclient.StreamContextError(ctx); cause != nil {
			return nil, nil, fmt.Errorf("websocket dial failed: %w", cause)
		}
		return nil, nil, newWebSocketDialError(request, resp, err)
	}
	if cause := httpclient.StreamContextError(ctx); cause != nil {
		_ = conn.CloseNow()
		return nil, nil, fmt.Errorf("websocket dial failed: %w", cause)
	}
	conn.SetReadLimit(-1)
	setWebSocketActivity(activityConn, nil)
	return conn, activityConn, nil
}
