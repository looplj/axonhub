package main

import (
	"net"
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

func TestWriteDesktopSettingsTurnsLANOnAndOff(t *testing.T) {
	path, err := ensureConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	settings, err := readDesktopSettings(path)
	if err != nil || settings.LAN {
		t.Fatalf("a fresh config has LAN on: %+v, %v", settings, err)
	}
	settings.LAN = true
	if err := writeDesktopSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "host: "+desktopLANHost) {
		t.Fatalf("LAN host not written:\n%s", data)
	}
	if got, err := readDesktopSettings(path); err != nil || !got.LAN {
		t.Fatalf("LAN not read back: %+v, %v", got, err)
	}
	settings.LAN = false
	if err := writeDesktopSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "host: "+desktopHost) {
		t.Fatalf("loopback host not restored:\n%s", data)
	}
	// Changing only the database keeps the host the user chose.
	settings.LAN = true
	if err := writeDesktopSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	if err := writeDBConfig(path, settings.DB); err != nil {
		t.Fatal(err)
	}
	if got, _ := readDesktopSettings(path); !got.LAN {
		t.Fatal("writing the database reset LAN access")
	}
}

func TestListenHostAllowsLoopbackAndTheLANHostOnly(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "localhost", desktopLANHost} {
		if !listenHostAllowed(host) {
			t.Errorf("%q refused", host)
		}
	}
	for _, host := range []string{"", "192.168.1.5", "::", "example.com"} {
		if listenHostAllowed(host) {
			t.Errorf("%q allowed", host)
		}
	}
	if panelHost(desktopLANHost) != desktopHost || panelHost("localhost") != "localhost" {
		t.Fatal("the panel does not dial loopback")
	}
}

func TestLANAddressPicksAPrivateIPv4(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1")},
		&net.IPNet{IP: net.ParseIP("fe80::1")},
		&net.IPNet{IP: net.ParseIP("8.8.8.8")},
		&net.IPNet{IP: net.ParseIP("192.168.1.20")},
		&net.IPNet{IP: net.ParseIP("10.0.0.2")},
	}
	if got := lanAddress(addrs); got != "192.168.1.20" {
		t.Fatalf("lanAddress = %q", got)
	}
	if got := lanAddress(addrs[:3]); got != "" {
		t.Fatalf("no private address, got %q", got)
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
