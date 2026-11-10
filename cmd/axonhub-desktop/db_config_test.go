package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDBConfigRoundTripKeepsOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path, err := ensureConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := readDBConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dialect != "sqlite3" || !strings.Contains(cfg.DSN, "axonhub.db") {
		t.Fatalf("generated db config = %+v", cfg)
	}

	mysql := dbConfig{Dialect: "mysql", DSN: "root:p@ss#1@tcp(127.0.0.1:3306)/axonhub?parseTime=True"}
	if err := writeDBConfig(path, mysql); err != nil {
		t.Fatal(err)
	}
	got, err := readDBConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != mysql {
		t.Fatalf("read back %+v, want %+v", got, mysql)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"host: " + desktopHost, "port: 18090", "output: file"} {
		if !strings.Contains(string(data), kept) {
			t.Fatalf("config lost %q:\n%s", kept, data)
		}
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, %v", info.Mode().Perm(), err)
	}
}

func TestWriteDesktopSettingsUpdatesPort(t *testing.T) {
	dir := t.TempDir()
	path, err := ensureConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := desktopSettings{
		Port: 19090,
		DB:   dbConfig{Dialect: "postgres", DSN: "postgres://u:p@h/axonhub"},
	}
	if err := writeDesktopSettings(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readDesktopSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "port: 19090") || strings.Contains(string(data), "port: \"19090\"") {
		t.Fatalf("port was not written as an integer:\n%s", data)
	}
	if !strings.Contains(string(data), "host: "+desktopHost) || !strings.Contains(string(data), "output: file") {
		t.Fatalf("other keys lost:\n%s", data)
	}
	if err := writeDesktopSettings(path, desktopSettings{Port: 0, DB: want.DB}); err == nil {
		t.Fatal("port 0 was written")
	}
}

func TestWriteDBConfigAddsMissingSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("# mine\nserver:\n  port: 1\ndb: off\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := dbConfig{Dialect: "postgres", DSN: "postgres://u:p@h/axonhub"}
	if err := writeDBConfig(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := readDBConfig(path); err != nil || got != want {
		t.Fatalf("read back %+v, %v", got, err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# mine") || !strings.Contains(string(data), "port: 1") {
		t.Fatalf("other content lost:\n%s", data)
	}
}

func TestDBKindIndex(t *testing.T) {
	for dialect, want := range map[string]int{"sqlite3": 0, "sqlite": 0, "": 0, "MySQL": 1, "tidb": 1, "pgx": 2, "postgresql": 2} {
		if got := dbKindIndex(dialect); got != want {
			t.Fatalf("dbKindIndex(%q) = %d, want %d", dialect, got, want)
		}
	}
}

func TestPingDB(t *testing.T) {
	ok := dbConfig{Dialect: "sqlite3", DSN: sqliteDSN(filepath.Join(t.TempDir(), "x.db"))}
	if err := pingDB(t.Context(), ok); err != nil {
		t.Fatal(err)
	}
	if err := pingDB(t.Context(), dbConfig{Dialect: "mysql", DSN: "  "}); err == nil {
		t.Fatal("an empty DSN must fail")
	}
}

func TestReadDesktopSettingsLoosePort(t *testing.T) {
	dir := t.TempDir()
	for body, want := range map[string]int{
		"server:\n  port: \"19090\"\n": 19090,
		"server:\n  port: 19090\n":     19090,
		"server:\n  port: nope\n":      desktopPort,
		"db:\n  dialect: mysql\n":      desktopPort,
	} {
		path := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := readDesktopSettings(path)
		if err != nil {
			t.Fatalf("%q: %v", body, err)
		}
		if got.Port != want {
			t.Fatalf("%q: port = %d, want %d", body, got.Port, want)
		}
	}
}
