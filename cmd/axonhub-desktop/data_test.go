package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/looplj/axonhub/conf"
)

func TestDesktopDataDir(t *testing.T) {
	tests := []struct {
		goos, home, appData, want string
	}{
		{goos: "darwin", home: filepath.Join(string(filepath.Separator), "Users", "ada"), want: filepath.Join(string(filepath.Separator), "Users", "ada", "Library", "Application Support", "AxonHub")},
		{goos: "windows", appData: filepath.Join("C:", "Users", "ada", "AppData", "Roaming"), want: filepath.Join("C:", "Users", "ada", "AppData", "Roaming", "AxonHub")},
		{goos: "linux", home: filepath.Join(string(filepath.Separator), "home", "ada"), want: filepath.Join(string(filepath.Separator), "home", "ada", ".config", "axonhub-desktop")},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			got, err := desktopDataDir(tt.goos, tt.home, tt.appData)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("desktopDataDir() = %q, want %q", got, tt.want)
			}
		})
	}
	if _, err := desktopDataDir("darwin", "", ""); err == nil {
		t.Fatal("darwin without a home directory should fail")
	}
	if _, err := desktopDataDir("windows", "", ""); err == nil {
		t.Fatal("windows without AppData should fail")
	}
}

func TestConfigYAMLUsesLoopbackSQLiteAndLogFile(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { conf.SetConfigFile("") })

	path, err := ensureConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}

	original := configYAML(dir)
	if err := os.WriteFile(path, []byte(original+"\n# edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureConfig(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == original {
		t.Fatal("ensureConfig rewrote an existing config file")
	}

	conf.SetConfigFile(path)
	cfg, err := conf.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIServer.Host != desktopHost || cfg.APIServer.Port != desktopPort {
		t.Fatalf("server = %s:%d, want %s:%d", cfg.APIServer.Host, cfg.APIServer.Port, desktopHost, desktopPort)
	}
	if cfg.DB.Dialect != "sqlite3" {
		t.Fatalf("dialect = %q, want sqlite3", cfg.DB.Dialect)
	}
	wantDSN := sqliteDSN(filepath.Join(dir, "axonhub.db"))
	if cfg.DB.DSN != wantDSN {
		t.Fatalf("dsn = %q, want %q", cfg.DB.DSN, wantDSN)
	}
	if !strings.Contains(cfg.DB.DSN, "cache=shared") || !strings.Contains(cfg.DB.DSN, "_fk=1") || !strings.Contains(cfg.DB.DSN, "_pragma=journal_mode(WAL)") {
		t.Fatalf("dsn is missing the default SQLite parameters: %s", cfg.DB.DSN)
	}
	wantLog := filepath.Join(dir, "logs", "axonhub.log")
	if cfg.Log.Output != "file" || cfg.Log.File.Path != wantLog {
		t.Fatalf("log output = %q path = %q, want file %q", cfg.Log.Output, cfg.Log.File.Path, wantLog)
	}
}

func TestOwnerSecretIsCreatedOnceAndNotLogged(t *testing.T) {
	path := filepath.Join(t.TempDir(), ownerSecretName)
	first, err := ownerSecret(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 6 {
		t.Fatal("generated owner secret is shorter than the initialize API allows")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("owner secret mode = %o, want 600", info.Mode().Perm())
	}
	second, err := ownerSecret(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatal("owner secret was regenerated")
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(configYAML(t.TempDir()), first) || strings.Contains(string(stored), "password") {
		t.Fatal("owner secret leaked into config or was labeled in the secret file")
	}

	if _, err := ownerSecret(filepath.Join(t.TempDir(), "missing.secret"), false); err == nil {
		t.Fatal("missing secret was created when initialization was not requested")
	}
}
