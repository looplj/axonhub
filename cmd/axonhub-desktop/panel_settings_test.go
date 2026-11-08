package main

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func settingsShell(t *testing.T) *shell {
	t.Helper()
	path, err := ensureConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := newShell("http://127.0.0.1:9", desktopSession{})
	s.configPath = path
	return s
}

func TestOpenSettingsReadsConfig(t *testing.T) {
	s := settingsShell(t)
	if err := writeDBConfig(s.configPath, dbConfig{Dialect: "tidb", DSN: "root@tcp(db:4000)/axonhub"}); err != nil {
		t.Fatal(err)
	}
	s.form.seq = 4
	s.openSettings()
	if s.page != pageSettings || s.form.kind != 1 || s.form.dsn != "root@tcp(db:4000)/axonhub" || s.form.port != strconv.Itoa(desktopPort) || s.form.status != "" || s.form.seq != 5 {
		t.Fatalf("form = %+v, page = %v", s.form, s.page)
	}
	s.closeSettings()
	if s.page != pageOverview || s.form.seq != 6 {
		t.Fatalf("leaving settings must invalidate a pending save, page = %v seq = %d", s.page, s.form.seq)
	}
	s.openSettings()
	if s.form.seq != 7 {
		t.Fatalf("reopening settings reset seq to %d", s.form.seq)
	}
}

func TestFormConfig(t *testing.T) {
	s := settingsShell(t)
	s.form = settingsForm{kind: 1, dsn: "  u:p@tcp(h:3306)/axonhub  "}
	if got := s.formConfig(); got != (dbConfig{Dialect: "mysql", DSN: "u:p@tcp(h:3306)/axonhub"}) {
		t.Fatalf("mysql = %+v", got)
	}
	s.form = settingsForm{kind: 0}
	want := sqliteDSN(filepath.Join(filepath.Dir(s.configPath), "axonhub.db"))
	if got := s.formConfig(); got.Dialect != "sqlite3" || got.DSN != want {
		t.Fatalf("empty sqlite = %+v, want dsn %q", got, want)
	}
	s.form = settingsForm{kind: 2}
	if got := s.formConfig(); got.DSN != "" {
		t.Fatalf("empty postgres must stay empty for the ping to reject, got %+v", got)
	}
}

func TestStartOfflineStaysOnSettings(t *testing.T) {
	s := settingsShell(t)
	s.startOffline(errors.New("connect to the MySQL database: dial tcp 127.0.0.1:3306: connection refused"))
	if !s.offline || s.page != pageSettings || !strings.Contains(s.startErr, "connection refused") {
		t.Fatalf("offline shell = %+v", s)
	}
	s.closeSettings()
	if s.page != pageSettings {
		t.Fatal("an offline panel has no overview to go back to")
	}
	s.openNext("gid://axonhub/Project/1", adminNextPath)
	if s.loadErr != "" {
		t.Fatalf("an offline panel must not try to open admin, loadErr = %q", s.loadErr)
	}
}

func TestFormEditedDropsPendingResult(t *testing.T) {
	s := settingsShell(t)
	s.form = settingsForm{busy: true, status: "Connecting…", seq: 3}
	s.formEdited()
	if s.form.busy || s.form.status != "" || s.form.seq != 4 {
		t.Fatalf("form = %+v", s.form)
	}
}

func TestParseListenPort(t *testing.T) {
	if got, err := parseListenPort(" 19090 "); err != nil || got != 19090 {
		t.Fatalf("port = %d, %v", got, err)
	}
	if got, err := parseListenPort(""); err != nil || got != desktopPort {
		t.Fatalf("blank port = %d, %v", got, err)
	}
	if _, err := parseListenPort("0"); err == nil || err.Error() != formPortInvalid {
		t.Fatal(err)
	}
	if _, err := parseListenPort("nope"); err == nil {
		t.Fatal("a non-numeric port was accepted")
	}
}
