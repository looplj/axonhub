package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const panelPrefsName = "panel-settings.json"

// panelPrefs is what settings saves for the panel. Language is "", to
// follow the system, or en or zh-Hans. Notify turns on background alerts
// for auto-disabled channels and exhausted quota; it is off unless the
// user chose it.
type panelPrefs struct {
	Theme    themeMode
	Language string
	Notify   bool
}

type panelPrefsFile struct {
	Theme    string `json:"theme,omitempty"`
	Language string `json:"language,omitempty"`
	Notify   bool   `json:"notify,omitempty"`
}

func panelPrefsPath(dir string) string {
	return filepath.Join(dir, panelPrefsName)
}

// parseLanguage keeps a language the catalogs ship. Anything else,
// including a blank, follows the system. zh-CN is the same Simplified
// catalog.
func parseLanguage(s string) string {
	switch strings.TrimSpace(s) {
	case "en":
		return "en"
	case "zh-Hans", "zh-CN", "zh":
		return "zh-Hans"
	default:
		return ""
	}
}

func parseThemeMode(s string) themeMode {
	switch strings.TrimSpace(s) {
	case "light":
		return themeLight
	case "dark":
		return themeDark
	default:
		return themeSystem
	}
}

// readPanelPrefs loads the saved prefs. A missing or unreadable file is
// the defaults: the system's appearance and language. A saved project
// choice from an older build is ignored.
func readPanelPrefs(path string) panelPrefs {
	if path == "" {
		return panelPrefs{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return panelPrefs{}
	}
	var file panelPrefsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return panelPrefs{}
	}
	return panelPrefs{Theme: parseThemeMode(file.Theme), Language: parseLanguage(file.Language), Notify: file.Notify}
}

// writePanelPrefs saves prefs. An empty path keeps them in memory only.
func writePanelPrefs(path string, prefs panelPrefs) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(panelPrefsFile{
		Theme:    prefs.Theme.String(),
		Language: parseLanguage(prefs.Language),
		Notify:   prefs.Notify,
	}, "", "  ")
	if err != nil {
		return errors.New("encode panel settings")
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return errors.New("save panel settings")
	}
	return nil
}
