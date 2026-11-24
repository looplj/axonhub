// 职责: 验证 HTTP 与 HTTPS 代理隧道中的上游 WebSocket 读取活动。
// 关联文件:
//
//	llm/transformer/openai/responses/websocket_dial.go
//
// 注意事项:
//
//	代理只接受 CONNECT，用于覆盖迁移前 ws:// 与 wss:// 的隧道行为。
package responses

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestWebSocketDialProxyReadActivity(t *testing.T) {
	const proxyUser, proxyPassword = "proxy-user", "proxy-pass"
	proxyAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(proxyUser+":"+proxyPassword))
	for _, test := range []struct {
		name        string
		secureProxy bool
		securePeer  bool
	}{
		{name: "http/ws"},
		{name: "http/wss", securePeer: true},
		{name: "https/ws", secureProxy: true},
		{name: "https/wss", secureProxy: true, securePeer: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer upstream-token" || r.Header.Get("Proxy-Authorization") != "" {
					http.Error(w, "unexpected upstream authorization", http.StatusUnauthorized)
					return
				}
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
			if test.securePeer {
				target.StartTLS()
			} else {
				target.Start()
			}
			defer target.Close()
			proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodConnect {
					http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
					return
				}
				if r.Header.Get("Proxy-Authorization") != proxyAuthorization || r.Header.Get("Authorization") != "" {
					http.Error(w, "unexpected proxy authorization", http.StatusProxyAuthRequired)
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
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("proxy relay panic: %v", r)
						}
					}()
					defer close(copied)
					defer upstream.Close()
					_, _ = io.Copy(upstream, client)
				}()
				_, _ = io.Copy(client, upstream)
				_ = client.Close()
				<-copied
			}))
			if test.secureProxy {
				proxy.StartTLS()
			} else {
				proxy.Start()
			}
			defer proxy.Close()
			executor := NewWebSocketExecutor(nil)
			defer executor.Close()
			switch {
			case test.securePeer:
				executor.transport.TLSClientConfig = target.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			case test.secureProxy:
				executor.transport.TLSClientConfig = proxy.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			}
			proxyURL := mustParseURL(t, proxy.URL)
			proxyURL.User = url.UserPassword(proxyUser, proxyPassword)
			executor.transport.Proxy = http.ProxyURL(proxyURL)
			wsURL, err := toWebSocketURL(target.URL)
			require.NoError(t, err)
			headers := http.Header{"Authorization": {"Bearer upstream-token"}}
			conn, observer, err := executor.dial(ctx, &httpclient.Request{}, wsURL, headers)
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

func TestWebSocketDialPlainProxyCancellationClosesTunnel(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	closed := make(chan error, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		conn, buffered, err := w.(http.Hijacker).Hijack()
		if !assertCoderPeer(t, err) {
			return
		}
		defer conn.Close()
		deadline, _ := ctx.Deadline()
		if !assertCoderPeer(t, conn.SetReadDeadline(deadline)) {
			return
		}
		cancel()
		_, err = buffered.ReadByte()
		closed <- err
	}))
	defer proxy.Close()
	executor := NewWebSocketExecutor(nil)
	defer executor.Close()
	executor.transport.Proxy = http.ProxyURL(mustParseURL(t, proxy.URL))
	conn, _, err := executor.dial(ctx, &httpclient.Request{}, "ws://example.test", nil)
	require.Nil(t, conn)
	require.ErrorIs(t, err, context.Canceled)
	select {
	case err := <-closed:
		require.ErrorIs(t, err, io.EOF)
	case <-time.After(5 * time.Second):
		t.Fatal("proxy connection remained open after cancellation")
	}
}
