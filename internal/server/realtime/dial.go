// 职责: 使用受信任地址、安全 DNS 与渠道代理建立实时上游连接。
// 关联文件:
//
//	llm/realtime/endpoint.go
//	llm/httpclient/client.go
//	internal/server/orchestrator/realtime.go
//
// 注意事项:
//
//	TLS 证书验证始终启用；直连解析后固定公网 IP，禁止回环和私网探测。

package realtime

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"

	"github.com/looplj/axonhub/llm/httpclient"
)

func Dial(ctx context.Context, target, key string, proxy *httpclient.ProxyConfig, timeout time.Duration, safetyIdentifier ...string) (*websocket.Conn, error) {
	d := &websocket.Dialer{HandshakeTimeout: timeout, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, NetDialContext: publicDial}
	// An explicitly configured proxy is trusted administrative infrastructure.
	// Only the fixed, validated provider target receives the upstream credential.
	if proxy == nil {
		proxy = &httpclient.ProxyConfig{Type: httpclient.ProxyTypeEnvironment}
	}
	if proxy != nil {
		switch proxy.Type {
		case httpclient.ProxyTypeEnvironment:
			targetURL, err := url.Parse(target)
			if err != nil {
				return nil, errors.New("invalid upstream URL")
			}
			targetURL.Scheme = "https"
			proxyURL, err := http.ProxyFromEnvironment(&http.Request{URL: targetURL})
			if err != nil {
				return nil, errors.New("invalid environment proxy")
			}
			if proxyURL != nil {
				d.Proxy = http.ProxyURL(proxyURL)
				d.NetDialContext = nil
			}
		case httpclient.ProxyTypeURL:
			u, err := url.Parse(proxy.URL)
			if err != nil || u.Host == "" {
				return nil, errors.New("invalid channel proxy")
			}
			if proxy.Username != "" {
				u.User = url.UserPassword(proxy.Username, proxy.Password)
			}
			d.Proxy = http.ProxyURL(u)
			d.NetDialContext = nil
		case "", httpclient.ProxyTypeDisabled:
		default:
			return nil, errors.New("unsupported channel proxy")
		}
	}
	headers := http.Header{"Authorization": []string{"Bearer " + key}}
	if len(safetyIdentifier) > 0 && safetyIdentifier[0] != "" {
		headers.Set("Openai-Safety-Identifier", safetyIdentifier[0])
	}
	conn, response, err := d.DialContext(ctx, target, headers)
	if response != nil && response.Body != nil {
		response.Body.Close()
	}
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		return nil, &HandshakeError{Status: status, Cause: err}
	}
	return conn, nil
}

func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("invalid realtime host")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("realtime DNS lookup failed")
	}
	for _, ip := range ips {
		if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() {
			return nil, errors.New("realtime host resolved to a forbidden address")
		}
	}
	var dialer net.Dialer
	dialer.Timeout = 10 * time.Second
	dialer.KeepAlive = 30 * time.Second
	return dialValidatedIPs(ctx, network, port, ips, dialer.DialContext)
}

func dialValidatedIPs(ctx context.Context, network, port string, ips []net.IPAddr, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	var lastErr error
	for _, ip := range ips {
		conn, err := dial(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	if lastErr == nil {
		return nil, errors.New("realtime DNS lookup returned no addresses")
	}
	return nil, lastErr
}

// Dialer separates connection establishment from routing and session processing.
type Dialer interface {
	Connect(context.Context, string, string, *httpclient.ProxyConfig, time.Duration, ...string) (*websocket.Conn, error)
}
type SecureDialer struct{}

func (SecureDialer) Connect(ctx context.Context, target, key string, proxy *httpclient.ProxyConfig, timeout time.Duration, safety ...string) (*websocket.Conn, error) {
	return Dial(ctx, target, key, proxy, timeout, safety...)
}

// HandshakeError retains the HTTP status and a safe cause, never provider headers/body or URLs.
type HandshakeError struct {
	Status int
	Cause  error
}

func (e *HandshakeError) Error() string {
	if e.Status == 0 {
		return "realtime upstream TLS, DNS or connection failed"
	}
	return fmt.Sprintf("realtime upstream WebSocket handshake failed (HTTP %d)", e.Status)
}

func (e *HandshakeError) Unwrap() error { return e.Cause }
