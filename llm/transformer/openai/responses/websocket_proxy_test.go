// 职责: 验证 HTTP 与 HTTPS 代理隧道中的上游 WebSocket 读取活动。
// 关联文件:
//
//	llm/transformer/openai/responses/websocket_dial.go
package responses

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestWebSocketDialProxyReadActivity(t *testing.T) {
	for _, secure := range []bool{false, true} {
		name := "http"
		if secure {
			name = "https"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if !assertCoderPeer(t, err) {
					return
				}
				defer conn.CloseNow()
				_, payload, err := conn.Read(ctx)
				if assertCoderPeer(t, err) {
					assertCoderPeer(t, conn.Write(ctx, websocket.MessageText, payload))
				}
			}))
			defer target.Close()
			proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodConnect {
					http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
					return
				}
				upstream, err := (&net.Dialer{}).DialContext(ctx, "tcp", r.Host)
				if !assertCoderPeer(t, err) {
					return
				}
				defer upstream.Close()
				client, _, err := w.(http.Hijacker).Hijack()
				if !assertCoderPeer(t, err) {
					return
				}
				defer client.Close()
				if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); !assertCoderPeer(t, err) {
					return
				}
				copied := make(chan struct{})
				go func() {
					defer close(copied)
					defer upstream.Close()
					_, _ = io.Copy(upstream, client)
				}()
				_, _ = io.Copy(client, upstream)
				_ = client.Close()
				<-copied
			}))
			if secure {
				proxy.StartTLS()
			} else {
				proxy.Start()
			}
			defer proxy.Close()
			executor := NewWebSocketExecutor(nil)
			defer executor.Close()
			executor.transport.TLSClientConfig = target.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			executor.transport.Proxy = http.ProxyURL(mustParseURL(t, proxy.URL))
			conn, observer, err := executor.dial(ctx, &httpclient.Request{}, "wss"+target.URL[len("https"):], nil)
			require.NoError(t, err)
			defer conn.CloseNow()
			require.NotNil(t, observer, "proxy tunnels must preserve the raw read activity observer")
			var reads atomic.Int32
			setWebSocketActivity(observer, func() { reads.Add(1) })
			require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte("synthetic")))
			_, payload, err := conn.Read(ctx)
			require.NoError(t, err)
			require.Equal(t, "synthetic", string(payload))
			require.Positive(t, reads.Load())
		})
	}
}
