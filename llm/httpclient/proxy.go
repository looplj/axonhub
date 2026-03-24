package httpclient

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

type ProxyType string

const (
	ProxyTypeDisabled    ProxyType = "disabled"
	ProxyTypeEnvironment ProxyType = "environment"
	ProxyTypeURL         ProxyType = "url"
)

type ProxyConfig struct {
	Type     ProxyType `json:"type"`
	URL      string    `json:"url,omitempty"`
	Username string    `json:"username,omitempty"`
	Password string    `json:"password,omitempty"`
}

// dialThroughProxy establishes a TCP connection through the configured proxy.
// Returns the underlying TCP connection that can be used for TLS handshake.
func dialThroughProxy(ctx context.Context, proxyConfig *ProxyConfig, network, addr string) (net.Conn, error) {
	if proxyConfig == nil || proxyConfig.Type == ProxyTypeDisabled {
		// Direct connection
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}

	if proxyConfig.Type == ProxyTypeEnvironment {
		return nil, fmt.Errorf("environment proxy not supported with TLS fingerprinting")
	}

	if proxyConfig.URL == "" {
		return nil, fmt.Errorf("proxy URL is required")
	}

	proxyURL, err := url.Parse(proxyConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	switch proxyURL.Scheme {
	case "socks5":
		return dialThroughSOCKS5(ctx, proxyURL, proxyConfig, network, addr)
	case "http", "https":
		return dialThroughHTTPConnect(ctx, proxyURL, proxyConfig, network, addr)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
	}
}

// dialThroughSOCKS5 establishes a connection through a SOCKS5 proxy.
func dialThroughSOCKS5(ctx context.Context, proxyURL *url.URL, proxyConfig *ProxyConfig, network, addr string) (net.Conn, error) {
	var auth *proxy.Auth
	if proxyConfig.Username != "" && proxyConfig.Password != "" {
		auth = &proxy.Auth{
			User:     proxyConfig.Username,
			Password: proxyConfig.Password,
		}
	}

	dialer, err := proxy.SOCKS5(network, proxyURL.Host, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
		return contextDialer.DialContext(ctx, network, addr)
	}

	return dialer.Dial(network, addr)
}

// dialThroughHTTPConnect establishes a connection through an HTTP/HTTPS CONNECT proxy.
func dialThroughHTTPConnect(ctx context.Context, proxyURL *url.URL, proxyConfig *ProxyConfig, network, addr string) (net.Conn, error) {
	var d net.Dialer
	proxyConn, err := d.DialContext(ctx, "tcp", proxyURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	if proxyConfig.Username != "" && proxyConfig.Password != "" {
		auth := proxyConfig.Username + ":" + proxyConfig.Password
		basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
		connectReq.Header.Set("Proxy-Authorization", basicAuth)
	}

	if err := connectReq.Write(proxyConn); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to write CONNECT request: %w", err)
	}

	br := bufio.NewReader(proxyConn)
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		proxyConn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed: %s", resp.Status)
	}

	return proxyConn, nil
}
