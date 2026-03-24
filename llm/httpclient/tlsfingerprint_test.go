package httpclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithTLSFingerprint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	client := NewHttpClientWithProxy(nil, WithTLSFingerprint(true), WithInsecureSkipVerify(true))

	req := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}

	resp, err := client.Do(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(resp.Body), "ok")

	// Verify HTTP/2 is disabled
	transport := client.GetNativeClient().Transport.(*http.Transport)
	require.False(t, transport.ForceAttemptHTTP2)
}

func TestTLSFingerprintWithProxy(t *testing.T) {
	// Create target HTTPS server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Create HTTP CONNECT proxy
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}

		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer clientConn.Close()

		// Connect to target
		targetConn, err := net.Dial("tcp", r.Host)
		if err != nil {
			clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer targetConn.Close()

		// Send 200 Connection Established
		clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

		// Tunnel data
		go io.Copy(targetConn, clientConn)
		io.Copy(clientConn, targetConn)
	}))
	defer proxy.Close()

	proxyConfig := &ProxyConfig{
		Type: ProxyTypeURL,
		URL:  proxy.URL,
	}

	client := NewHttpClientWithProxy(proxyConfig, WithTLSFingerprint(true), WithInsecureSkipVerify(true))

	req := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}

	resp, err := client.Do(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTLSFingerprintWithSOCKS5(t *testing.T) {
	// Create target HTTPS server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Create simple SOCKS5 proxy
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleSOCKS5(conn)
		}
	}()

	proxyConfig := &ProxyConfig{
		Type: ProxyTypeURL,
		URL:  "socks5://" + listener.Addr().String(),
	}

	client := NewHttpClientWithProxy(proxyConfig, WithTLSFingerprint(true), WithInsecureSkipVerify(true))

	req := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}

	resp, err := client.Do(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func handleSOCKS5(clientConn net.Conn) {
	defer clientConn.Close()

	buf := make([]byte, 256)

	// Read version and auth methods
	n, err := clientConn.Read(buf)
	if err != nil || n < 2 {
		return
	}

	// Send no auth required
	clientConn.Write([]byte{0x05, 0x00})

	// Read connect request
	n, err = clientConn.Read(buf)
	if err != nil || n < 7 {
		return
	}

	// Parse address
	var host string
	var port uint16
	addrType := buf[3]

	switch addrType {
	case 0x01: // IPv4
		host = net.IPv4(buf[4], buf[5], buf[6], buf[7]).String()
		port = uint16(buf[8])<<8 | uint16(buf[9])
	case 0x03: // Domain
		domainLen := int(buf[4])
		host = string(buf[5 : 5+domainLen])
		port = uint16(buf[5+domainLen])<<8 | uint16(buf[6+domainLen])
	default:
		clientConn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	// Connect to target
	targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		clientConn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer targetConn.Close()

	// Send success
	clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	// Tunnel data
	go io.Copy(targetConn, clientConn)
	io.Copy(clientConn, targetConn)
}
