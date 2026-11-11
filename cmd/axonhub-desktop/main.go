package main

//go:generate go tool mygo generate

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "time/tzdata"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	sdk "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/looplj/axonhub/conf"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/server"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
)

func main() {
	mygo.Use(updater.Plugin)
	// mygo build runs the binary with MYGO_GENERATE to write the TypeScript
	// client. App.Run returns immediately in that mode, so the server and
	// the desktop data directory stay untouched.
	if os.Getenv("MYGO_GENERATE") == "" {
		if err := startDesktop(); err != nil {
			fatal(err)
		}
	}
	if err := mygo.App.Run(); err != nil {
		fatal(err)
	}
}

// startDesktop starts the server on the configured database and opens the
// desktop session. A database that does not answer, or a session that
// cannot open on it, does not end the process: the panel opens on its
// settings page with the error, so the DSN can be fixed there.
func startDesktop() error {
	// The server lets AXONHUB_* override config.yml. Desktop settings edit
	// that file, so those variables are removed before anything reads it.
	clearAxonHubEnv()

	dir, err := ensureDataDir()
	if err != nil {
		return err
	}
	configPath, err := ensureConfig(dir)
	if err != nil {
		return err
	}
	conf.SetConfigFile(configPath)

	shell := newShell("", desktopSession{})
	shell.configPath = configPath
	shell.secretPath = ownerSecretPath(dir)
	shell.prefsPath = panelPrefsPath(dir)
	shell.prefs = readPanelPrefs(shell.prefsPath)
	useMenuBarPolicy(false)
	// Hiding the panel must not end the process. Quit still comes from
	// the tray menu and the application menu.
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { shell.quitting = true })
	mygo.App.WhenReady(shell.ready)

	if err := openDesktop(shell, dir, configPath); err != nil {
		log.Warn(context.Background(), "desktop starts in settings", log.Cause(err))
		shell.startOffline(err)
	}
	return nil
}

func openDesktop(shell *shell, dir, configPath string) error {
	cfg, err := readDBConfig(configPath)
	if err != nil {
		return err
	}
	if err := pingDB(context.Background(), cfg); err != nil {
		return fmt.Errorf("connect to the %s database: %w", dbKinds[dbKindIndex(cfg.Dialect)].Label, err)
	}

	started, stop, err := startServer()
	shell.serverStop = stop
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	session, err := openDesktopSession(ctx, started.baseURL, ownerSecretPath(dir), nil, started.account.ensure)
	if err != nil {
		return err
	}
	shell.origin = strings.TrimRight(started.baseURL, "/")
	if settings, err := readDesktopSettings(configPath); err == nil && settings.LAN {
		shell.lanPort = settings.Port
	}
	shell.session = session
	return nil
}

func applicationMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "AxonHub", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			updater.MenuItem(),
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	})
}

// desktopServer is the running server: where it listens, and the services
// that make the desktop account ready in its database.
type desktopServer struct {
	baseURL string
	account desktopAccount
}

type desktopStart struct {
	server desktopServer
	err    error
}

func startServer() (desktopServer, *serverStop, error) {
	stop := newServerStop()
	started := make(chan desktopStart, 1)
	go func() {
		defer recoverLog("desktop server")
		defer stop.finished()
		server.Run(
			fx.Invoke(stop.capture),
			fx.StartTimeout(60*time.Second),
			fx.StopTimeout(30*time.Second),
			fx.WithLogger(func() fxevent.Logger {
				return &fxevent.ConsoleLogger{W: os.Stderr}
			}),
			conf.Module,
			fx.Provide(metrics.NewProvider),
			fx.Invoke(func(lc fx.Lifecycle, srv *server.Server, provider *sdk.MeterProvider, client *ent.Client, requestSvc *biz.RequestService, projectSvc *biz.ProjectService) {
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						// Returning an error from OnStart makes fx exit the
						// process. Report it on started instead, so the panel
						// can open settings and the port can be changed.
						if !listenHostAllowed(srv.Config.Host) {
							started <- desktopStart{err: fmt.Errorf("desktop server must listen on a loopback address or %s, not %q", desktopLANHost, srv.Config.Host)}
							return nil
						}
						if srv.Config.Port <= 0 || srv.Config.Port > 65535 {
							started <- desktopStart{err: fmt.Errorf("desktop server port %d is invalid", srv.Config.Port)}
							return nil
						}
						if provider != nil {
							if err := metrics.SetupMetrics(provider, srv.Config.Name); err != nil {
								return err
							}
						}
						go func() {
							defer recoverLog("cancel stale processing")
							cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
							defer cleanupCancel()
							if err := requestSvc.ClearStaleProcessingOnStartup(cleanupCtx); err != nil {
								log.Warn(context.Background(), "failed to cancel stale processing records on startup", log.Cause(err))
							}
						}()
						listenErr := make(chan error, 1)
						go func() {
							defer recoverLog("http server")
							if err := srv.Run(); err != nil {
								log.Error(context.Background(), "server run error", log.Cause(err))
								listenErr <- err
							}
						}()
						go func() {
							defer recoverLog("antigravity version")
							antigravity.InitVersion(context.Background())
						}()
						// The panel always talks to the server over loopback,
						// also when it listens on every interface.
						addr := net.JoinHostPort(panelHost(srv.Config.Host), strconv.Itoa(srv.Config.Port))
						if err := waitUntilListening(ctx, addr, listenErr); err != nil {
							started <- desktopStart{err: fmt.Errorf("listen on %s: %w", addr, err)}
							return nil
						}
						started <- desktopStart{
							server: desktopServer{
								baseURL: "http://" + addr,
								account: desktopAccount{client: client, projects: projectSvc},
							},
						}
						return nil
					},
					OnStop: func(ctx context.Context) error {
						if provider != nil {
							if err := provider.Shutdown(ctx); err != nil {
								log.Error(context.Background(), "metrics shutdown error", log.Cause(err))
							}
						}
						if err := srv.Shutdown(ctx); err != nil {
							log.Error(context.Background(), "server shutdown error", log.Cause(err))
						}
						if err := client.Close(); err != nil {
							log.Error(context.Background(), "ent close error", log.Cause(err))
						}
						return nil
					},
				})
			}),
		)
	}()

	select {
	case result := <-started:
		return result.server, stop, result.err
	case <-time.After(60 * time.Second):
		return desktopServer{}, stop, fmt.Errorf("desktop server did not start")
	}
}

// serverStop shuts the embedded server down when the desktop app quits.
// server.Run blocks until fx is asked to stop; without that, process exit
// skips the server's OnStop hook and cuts off open requests.
type serverStop struct {
	mu       sync.Mutex
	shutdown fx.Shutdowner
	done     chan struct{}
	stopped  bool
}

func newServerStop() *serverStop {
	return &serverStop{done: make(chan struct{})}
}

func (s *serverStop) capture(shutdown fx.Shutdowner) {
	s.mu.Lock()
	s.shutdown = shutdown
	s.mu.Unlock()
}

func (s *serverStop) finished() {
	close(s.done)
}

// stop asks the embedded server to shut down and waits until it has, or
// until the server's own stop timeout has passed.
func (s *serverStop) stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	shutdown := s.shutdown
	if shutdown == nil {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	if err := shutdown.Shutdown(); err != nil {
		log.Error(context.Background(), "desktop server shutdown", log.Cause(err))
	}
	select {
	case <-s.done:
	case <-time.After(35 * time.Second):
		log.Error(context.Background(), "desktop server shutdown timed out")
	}
}

// clearAxonHubEnv removes AXONHUB_* from the environment. Viper treats those
// names as overrides of config.yml, which would ignore the desktop settings.
func clearAxonHubEnv() {
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(strings.ToUpper(key), "AXONHUB_") {
			continue
		}
		_ = os.Unsetenv(key)
	}
}

// waitUntilListening reports when addr accepts a connection, or the error
// from the listener if it fails first. A busy port therefore reaches the
// panel instead of ending the process.
func waitUntilListening(ctx context.Context, addr string, listenErr <-chan error) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var last error
	for {
		select {
		case err := <-listenErr:
			if err != nil {
				return err
			}
		default:
		}
		select {
		case err := <-listenErr:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			if last != nil {
				return last
			}
			return fmt.Errorf("timed out waiting for %s", addr)
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
			if err == nil {
				conn.Close()
				return nil
			}
			last = err
		}
	}
}

// lanHost reports whether host is the every-interface address the LAN
// setting writes.
func lanHost(host string) bool {
	return host == desktopLANHost
}

// listenHostAllowed accepts loopback, and every interface once the user
// turned LAN access on in settings. Any other host in config.yml is
// refused, so an edited file cannot expose the server by accident.
func listenHostAllowed(host string) bool {
	return loopbackHost(host) || lanHost(host)
}

// panelHost is the host the panel dials: loopback when the server
// listens on every interface, the configured loopback host otherwise.
func panelHost(host string) string {
	if lanHost(host) {
		return desktopHost
	}
	return host
}

// lanAddress is this machine's first private IPv4 address, or "" when it
// has none.
func lanAddress(addrs []net.Addr) string {
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ip := ipNet.IP.To4(); ip != nil && ip.IsPrivate() {
			return ip.String()
		}
	}
	return ""
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func recoverLog(name string) {
	recovered := recover()
	if recovered == nil {
		return
	}
	log.Error(context.Background(), "goroutine panicked",
		log.String("name", name),
		log.Any("panic", recovered),
		log.ByteString("stack", debug.Stack()),
	)
}

func fatal(err error) {
	log.Error(context.Background(), "desktop startup failed", log.Cause(err))
	fmt.Fprintf(os.Stderr, "AxonHub desktop: %v\n", err)
	os.Exit(1)
}
