package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeModeResolves(t *testing.T) {
	for _, systemDark := range []bool{false, true} {
		if themeSystem.dark(systemDark) != systemDark {
			t.Fatalf("system mode with system dark %v", systemDark)
		}
		if themeLight.dark(systemDark) || !themeDark.dark(systemDark) {
			t.Fatalf("forced modes must ignore system dark %v", systemDark)
		}
	}
}

func TestPanelPrefsPersist(t *testing.T) {
	path := panelPrefsPath(t.TempDir())
	if readPanelPrefs(path) != (panelPrefs{}) {
		t.Fatal("a missing file is the defaults")
	}
	for _, prefs := range []panelPrefs{
		{Theme: themeLight},
		{Theme: themeDark},
		{Language: "zh-Hans"},
		{},
	} {
		if err := writePanelPrefs(path, prefs); err != nil {
			t.Fatal(err)
		}
		if got := readPanelPrefs(path); got != prefs {
			t.Fatalf("read %+v, want %+v", got, prefs)
		}
	}
	if err := writePanelPrefs(path, panelPrefs{Language: "zh-CN"}); err != nil {
		t.Fatal(err)
	}
	if got := readPanelPrefs(path); got.Language != "zh-Hans" {
		t.Fatalf("zh-CN saved as %q", got.Language)
	}
	if err := os.WriteFile(path, []byte(`{"theme":"sepia","projectID":"  ","projectName":"x","language":"fr"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readPanelPrefs(path); got != (panelPrefs{}) {
		t.Fatalf("unknown theme and blank project = %+v, want defaults", got)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if readPanelPrefs(path) != (panelPrefs{}) {
		t.Fatal("a broken file is the defaults")
	}
	if err := writePanelPrefs(filepath.Join(path, "nested"), panelPrefs{Theme: themeDark}); err == nil {
		t.Fatal("a save under a file must fail")
	}
	if readPanelPrefs("") != (panelPrefs{}) || writePanelPrefs("", panelPrefs{Theme: themeDark}) != nil {
		t.Fatal("an empty path keeps the prefs in memory only")
	}
}

func TestSetThemeSaves(t *testing.T) {
	s := newShell("http://127.0.0.1:9", desktopSession{})
	s.prefsPath = panelPrefsPath(t.TempDir())
	s.setTheme(themeDark)
	if s.prefs.Theme != themeDark || readPanelPrefs(s.prefsPath).Theme != themeDark {
		t.Fatalf("prefs = %+v, saved = %+v", s.prefs, readPanelPrefs(s.prefsPath))
	}
}
