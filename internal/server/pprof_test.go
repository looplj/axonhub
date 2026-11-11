package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/fx/fxtest"
)

func TestValidatePprofAddr(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:6060", "[::1]:6060", "127.0.0.2:0"} {
		if err := validatePprofAddr(addr); err != nil {
			t.Errorf("validatePprofAddr(%q) = %v, want nil", addr, err)
		}
	}
	for _, addr := range []string{"", ":6060", "0.0.0.0:6060", "[::]:6060", "localhost:6060", "192.168.1.2:6060", "127.0.0.1"} {
		if err := validatePprofAddr(addr); err == nil {
			t.Errorf("validatePprofAddr(%q) = nil, want error", addr)
		}
	}
}

func TestPprofHandlerAccess(t *testing.T) {
	h := newPprofHandler()
	tests := []struct {
		name   string
		remote string
		host   string
		want   int
	}{
		{"ipv4 loopback", "127.0.0.1:50000", "127.0.0.1:6060", http.StatusOK},
		{"localhost name", "127.0.0.1:50000", "localhost:6060", http.StatusOK},
		{"ipv6 loopback", "[::1]:50000", "[::1]:6060", http.StatusOK},
		{"remote caller", "192.168.1.9:50000", "127.0.0.1:6060", http.StatusForbidden},
		{"rebound host", "127.0.0.1:50000", "evil.example.com:6060", http.StatusForbidden},
		{"localhost subdomain", "127.0.0.1:50000", "a.localhost:6060", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
			req.RemoteAddr = tt.remote
			req.Host = tt.host
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestPprofServerServesHeapProfile(t *testing.T) {
	p := &pprofServer{}
	if err := p.start(Pprof{Enabled: true, Addr: "127.0.0.1:0"}); err != nil {
		t.Fatalf("start() error = %v", err)
	}
	t.Cleanup(func() { _ = p.shutdown(context.Background()) })

	resp, err := http.Get("http://" + p.addr + "/debug/pprof/heap?debug=1")
	if err != nil {
		t.Fatalf("GET heap: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "heap profile:") || !strings.Contains(string(body), "# HeapAlloc") {
		t.Fatalf("unexpected heap profile body: %.200s", body)
	}
}

func TestPprofServerRejectsNonLoopbackAddr(t *testing.T) {
	p := &pprofServer{}
	if err := p.start(Pprof{Enabled: true, Addr: "0.0.0.0:0"}); err == nil {
		_ = p.shutdown(context.Background())
		t.Fatal("start() with wildcard addr = nil, want error")
	}
}

func TestRegisterPprofLifecycle(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	disabled := fxtest.NewLifecycle(t)
	registerPprof(disabled, Config{Pprof: Pprof{Enabled: false, Addr: addr}})
	disabled.RequireStart()
	if conn, err := net.Dial("tcp", addr); err == nil {
		_ = conn.Close()
		t.Fatal("disabled pprof should not listen")
	}
	disabled.RequireStop()

	enabled := fxtest.NewLifecycle(t)
	registerPprof(enabled, Config{Pprof: Pprof{Enabled: true, Addr: addr}})
	enabled.RequireStart()
	resp, err := http.Get("http://" + addr + "/debug/pprof/goroutine?debug=1")
	if err != nil {
		t.Fatalf("GET goroutine: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	enabled.RequireStop()
	if conn, err := net.Dial("tcp", addr); err == nil {
		_ = conn.Close()
		t.Fatal("pprof should stop listening after lifecycle stop")
	}
}
