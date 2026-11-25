// 职责: 通过 HTTP Transport 建立上游 WebSocket 并保留原始读取活动观察器。
// 关联文件:
//
//	llm/transformer/openai/responses/websocket_executor.go
//	llm/httpclient/client.go
//
// 注意事项:
//
//	coder 不暴露底层连接，必须在 HTTP 握手获取连接时单独保存观察器。
//	上游单条消息的读取上限独立于响应队列的缓冲预算。
//	ws:// 经过 HTTP(S) 代理时使用 CONNECT，兼容只接受隧道的代理。
package responses

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
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
	if err := configurePlainWebSocketProxy(ctx, transport, wsURL, headers); err != nil {
		return nil, nil, newWebSocketDialError(request, nil, err)
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
	conn.SetReadLimit(maxWebSocketResponseMessageBytes)
	setWebSocketActivity(activityConn, nil)
	return conn, activityConn, nil
}

func configurePlainWebSocketProxy(ctx context.Context, transport *http.Transport, wsURL string, headers http.Header) error {
	target, plain := strings.CutPrefix(wsURL, "ws://")
	if !plain || transport.Proxy == nil {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+target, nil)
	if err != nil {
		return err
	}
	request.Header = headers.Clone()
	proxyURL, err := transport.Proxy(request)
	if err != nil {
		return err
	}
	if proxyURL == nil || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") {
		transport.Proxy = http.ProxyURL(proxyURL)
		return nil
	}

	dial := transport.DialContext
	transport.Proxy = nil
	transport.DialContext = func(_ context.Context, network, targetAddr string) (net.Conn, error) {
		// Transport can detach DialContext cancellation to allow connection reuse.
		// This tunnel belongs to a single WebSocket handshake.
		connectHeaders := transport.ProxyConnectHeader
		if get := transport.GetProxyConnectHeader; get != nil {
			h, err := get(ctx, proxyURL, targetAddr)
			if err != nil {
				return nil, err
			}
			connectHeaders = h
		}
		conn, err := dialWebSocketProxyTunnel(ctx, dial, transport.TLSClientConfig, proxyURL, network, targetAddr, connectHeaders)
		if err != nil {
			return nil, fmt.Errorf("websocket proxy CONNECT: %w", err)
		}
		return conn, nil
	}
	return nil
}

func dialWebSocketProxyTunnel(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error), tlsConfig *tls.Config, proxyURL *url.URL, network, targetAddr string, headers http.Header) (net.Conn, error) {
	port := proxyURL.Port()
	if port == "" {
		port = "80"
		if proxyURL.Scheme == "https" {
			port = "443"
		}
	}
	rawConn, err := dial(ctx, network, net.JoinHostPort(proxyURL.Hostname(), port))
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	established := false
	defer func() {
		if !established {
			stop()
			_ = rawConn.Close()
		}
	}()
	conn := rawConn
	if proxyURL.Scheme == "https" {
		config := tlsConfig.Clone()
		if config == nil {
			config = &tls.Config{}
		}
		if config.ServerName == "" {
			config.ServerName = proxyURL.Hostname()
		}
		config.NextProtos = []string{"http/1.1"}
		tlsConn := tls.Client(rawConn, config)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		conn = tlsConn
	}

	request := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: targetAddr},
		Host:   targetAddr,
		Header: headers.Clone(),
	}
	if user := proxyURL.User; user != nil {
		if request.Header == nil {
			request.Header = make(http.Header)
		}
		password, _ := user.Password()
		credentials := base64.StdEncoding.EncodeToString([]byte(user.Username() + ":" + password))
		request.Header.Set("Proxy-Authorization", "Basic "+credentials)
	}
	if err := request.Write(conn); err != nil {
		return nil, err
	}
	// The upstream waits for the upgrade request before writing any bytes.
	response, err := http.ReadResponse(bufio.NewReader(io.LimitReader(conn, 1<<20)), request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy returned %s", response.Status)
	}
	if !stop() {
		return nil, ctx.Err()
	}
	established = true
	return conn, nil
}
