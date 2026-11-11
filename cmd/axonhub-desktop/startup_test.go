package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/conf"
)

func TestClearAxonHubEnvKeepsConfigFile(t *testing.T) {
	restore := snapshotEnvPrefix("AXONHUB_")
	t.Cleanup(restore)
	t.Cleanup(func() { os.Unsetenv("OTHER_APP_TOKEN") })

	if err := os.Setenv("AXONHUB_DB_DSN", "postgres://env/axonhub"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("AXONHUB_DB_DIALECT", "postgres"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("axonhub_server_port", "1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("OTHER_APP_TOKEN", "keep"); err != nil {
		t.Fatal(err)
	}

	clearAxonHubEnv()
	for _, key := range []string{"AXONHUB_DB_DSN", "AXONHUB_DB_DIALECT", "axonhub_server_port"} {
		if os.Getenv(key) != "" {
			t.Fatalf("%s is still set", key)
		}
	}
	if os.Getenv("OTHER_APP_TOKEN") != "keep" {
		t.Fatal("an unrelated variable was cleared")
	}

	dir := t.TempDir()
	path, err := ensureConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	conf.SetConfigFile(path)
	t.Cleanup(func() { conf.SetConfigFile("") })
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DB.Dialect != "sqlite3" || !strings.Contains(cfg.DB.DSN, "axonhub.db") {
		t.Fatalf("db = dialect %q dsn %q", cfg.DB.Dialect, cfg.DB.DSN)
	}
	if cfg.APIServer.Port != desktopPort {
		t.Fatalf("port = %d, want %d", cfg.APIServer.Port, desktopPort)
	}
}

func snapshotEnvPrefix(prefix string) func() {
	prefix = strings.ToUpper(prefix)
	var saved [][2]string
	for _, entry := range os.Environ() {
		key, val, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(strings.ToUpper(key), prefix) {
			continue
		}
		saved = append(saved, [2]string{key, val})
	}
	return func() {
		for _, entry := range os.Environ() {
			key, _, ok := strings.Cut(entry, "=")
			if ok && strings.HasPrefix(strings.ToUpper(key), prefix) {
				_ = os.Unsetenv(key)
			}
		}
		for _, kv := range saved {
			_ = os.Setenv(kv[0], kv[1])
		}
	}
}

type fakeShutdown struct {
	calls int
	done  chan struct{}
}

func (f *fakeShutdown) Shutdown(...fx.ShutdownOption) error {
	f.calls++
	close(f.done)
	return nil
}

func TestServerStopWaitsForTheServer(t *testing.T) {
	fake := &fakeShutdown{done: make(chan struct{})}
	stop := newServerStop()
	stop.done = fake.done
	stop.shutdown = fake

	finished := make(chan struct{})
	go func() {
		stop.stop()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("stop did not return after the server finished")
	}
	if fake.calls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", fake.calls)
	}
	stop.stop()
	if fake.calls != 1 {
		t.Fatal("a second stop shut the server down again")
	}
}

func TestServerStopWithoutAServer(t *testing.T) {
	var stop *serverStop
	stop.stop()
	newServerStop().stop()
}
