package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
)

// Pprof configures the Go runtime profiling listener. It is served on its own
// loopback-only listener, never on the public API port.
type Pprof struct {
	Enabled bool   `conf:"enabled" yaml:"enabled" json:"enabled"`
	Addr    string `conf:"addr" yaml:"addr" json:"addr"`
}

// validatePprofAddr requires an explicit loopback IP: profiles expose
// goroutine stacks and the process command line, so a wildcard or LAN bind
// would publish them to the network.
func validatePprofAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid server.pprof.addr %q: %w", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("server.pprof.addr %q must bind a loopback IP such as 127.0.0.1 or [::1]", addr)
	}
	return nil
}

func newPprofHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemote(r.RemoteAddr) || !isLoopbackHost(r.Host) {
			http.Error(w, "pprof is only served to loopback callers addressing localhost or a loopback IP", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func isLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// isLoopbackHost rejects requests whose Host names anything but this machine,
// which blocks DNS-rebinding pages from reaching the loopback listener.
func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type pprofServer struct {
	server *http.Server
	addr   string
}

func (p *pprofServer) start(cfg Pprof) error {
	if err := validatePprofAddr(cfg.Addr); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen server.pprof.addr %s: %w", cfg.Addr, err)
	}

	p.addr = ln.Addr().String()
	// No WriteTimeout: CPU profiles and traces stream for ?seconds=.
	p.server = &http.Server{
		Handler:           newPprofHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Info(context.Background(), "pprof server listening", log.String("addr", p.addr))

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(context.Background(), "pprof server panicked", log.Any("panic", recovered))
			}
		}()
		if err := p.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(context.Background(), "pprof server stopped", log.Cause(err))
		}
	}()

	return nil
}

func (p *pprofServer) shutdown(ctx context.Context) error {
	if p.server == nil {
		return nil
	}
	return p.server.Shutdown(ctx)
}

func registerPprof(lc fx.Lifecycle, cfg Config) {
	if !cfg.Pprof.Enabled {
		return
	}
	p := &pprofServer{}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { return p.start(cfg.Pprof) },
		OnStop:  p.shutdown,
	})
}
