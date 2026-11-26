// 职责: 使用本地 Mock WebSocket 和真实 Nginx 容器验证 Upgrade 与双向转发配置。
// 关联文件:
//
//	docs/en/guides/realtime.md
//	docs/zh/guides/realtime.md
//
// 注意事项:
//
//	不调用真实 Provider；只使用随机本地监听端口和合成凭证，退出时清理容器。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var authenticated atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/realtime" || r.URL.Query().Get("model") != "synthetic" || r.Header.Get("Authorization") != "Bearer synthetic-proxy-check" {
			http.Error(w, "invalid proxy forwarding", http.StatusBadRequest)
			return
		}
		authenticated.Store(true)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		kind, data, err := conn.Read(r.Context())
		if err == nil {
			_ = conn.Write(r.Context(), kind, data)
		}
	}))
	defer upstream.Close()
	root, err := os.MkdirTemp("/var/tmp", "axonhub-realtime-proxy-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	// Bind a random local port for Nginx without exposing it outside this VM.
	reserve := httptest.NewUnstartedServer(http.NotFoundHandler())
	address := reserve.Listener.Addr().String()
	reserve.Listener.Close()
	conf := fmt.Sprintf("events {}\nhttp { map $http_upgrade $realtime_connection { default upgrade; '' close; } server { listen %s; location /v1/realtime { proxy_pass %s; proxy_http_version 1.1; proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection $realtime_connection; proxy_set_header Authorization $http_authorization; proxy_set_header Host $host; proxy_read_timeout 20m; proxy_send_timeout 20m; proxy_buffering off; } } }\n", address, upstream.URL)
	path := filepath.Join(root, "nginx.conf")
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		return err
	}
	mount := path + ":/etc/nginx/nginx.conf:ro"
	syntax := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "host", "-v", mount, "nginx:alpine", "nginx", "-t")
	if output, err := syntax.CombinedOutput(); err != nil {
		return fmt.Errorf("Nginx syntax check failed: %s", output)
	}
	fmt.Println("PASS Nginx configuration syntax")
	name := fmt.Sprintf("axonhub-realtime-proxy-%d", os.Getpid())
	start := exec.CommandContext(ctx, "docker", "run", "--rm", "-d", "--name", name, "--network", "host", "-v", mount, "nginx:alpine")
	if output, err := start.CombinedOutput(); err != nil {
		return fmt.Errorf("Nginx start failed: %s", output)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Nginx cleanup failed: %v\n", err)
		}
	}()
	var conn *websocket.Conn
	for range 50 {
		dialCtx, dialCancel := context.WithTimeout(ctx, 2*time.Second)
		var response *http.Response
		conn, response, err = websocket.Dial(dialCtx, "ws://"+address+"/v1/realtime?model=synthetic", &websocket.DialOptions{
			HTTPHeader: http.Header{"Authorization": []string{"Bearer synthetic-proxy-check"}},
		})
		dialCancel()
		if err == nil {
			break
		}
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return errors.New("Nginx WebSocket handshake failed")
	}
	defer func() { _ = conn.CloseNow() }()
	payload := `{"type":"proxy.probe","event_id":"synthetic"}`
	writeCtx, cancelWrite := context.WithTimeout(ctx, 3*time.Second)
	defer cancelWrite()
	if err := conn.Write(writeCtx, websocket.MessageText, []byte(payload)); err != nil {
		return err
	}
	readCtx, cancelRead := context.WithTimeout(ctx, 3*time.Second)
	defer cancelRead()
	_, data, err := conn.Read(readCtx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) != payload || !authenticated.Load() {
		return errors.New("proxy did not preserve headers and bidirectional frames")
	}
	fmt.Println("PASS HTTP 101 Upgrade, Authorization forwarding, query preservation and bidirectional frames")
	return nil
}
