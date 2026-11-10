package main

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
)

// The update plugin, which tests replace: its functions panic unless the
// plugin is used.
var (
	checkForUpdates    = updater.CheckForUpdates
	updateChecks       = updater.AutomaticChecks
	setUpdateChecks    = updater.SetAutomaticChecks
	updateDownloads    = updater.AutomaticDownloads
	setUpdateDownloads = updater.SetAutomaticDownloads
	lastUpdateCheck    = updater.LastCheck
	availableVersion   = updater.AvailableVersion
	updateMenuItem     = updater.MenuItem
)

// relativePast says how long before now t was, in the language the app shows.
func relativePast(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	locale := i18n.Locale()
	opts := i18n.RelativeTimeOptions{Auto: true}
	switch {
	case d < time.Minute:
		return i18n.FormatRelativeTime(locale, 0, i18n.RelativeSecond, opts)
	case d < time.Hour:
		return i18n.FormatRelativeTime(locale, -float64(int(d/time.Minute)), i18n.RelativeMinute, opts)
	case d < 24*time.Hour:
		return i18n.FormatRelativeTime(locale, -float64(int(d/time.Hour)), i18n.RelativeHour, opts)
	default:
		days := int(d / (24 * time.Hour))
		if days < 1 {
			days = 1
		}
		return i18n.FormatRelativeTime(locale, -float64(days), i18n.RelativeDay, opts)
	}
}

func appVersion() string {
	if v := strings.TrimSpace(mygo.App.Version()); v != "" {
		return v
	}
	return panelBlank
}

// languageChoices are the settings picker: follow the system, or a language
// the catalogs ship. chosen is the one SetLocale kept, or follow the system.
func languageChoices(current string) (codes, names []string, chosen string) {
	codes = []string{"", "en", "zh-Hans"}
	names = []string{locales.FollowSystem(), i18n.Autonym("en"), i18n.Autonym("zh-Hans")}
	chosen = names[0]
	for i, code := range codes {
		if code != "" && code == current {
			chosen = names[i]
		}
	}
	return codes, names, chosen
}

// trayMenu is the status-item menu. It is built when it opens, so a language
// change or a newly found update is in the next menu.
func (s *shell) trayMenu() *mygo.Menu {
	items := []*mygo.MenuItem{
		{Label: locales.OpenPanel(), Click: func(*mygo.MenuItem, *mygo.Window) { s.show() }},
		{Label: locales.CopyApiBaseUrl(), Click: func(*mygo.MenuItem, *mygo.Window) {
			mygo.Clipboard.WriteText(openAIBaseURL(s.origin))
		}},
		mygo.Separator(),
	}
	if v := availableVersion(); v != "" {
		items = append(items, &mygo.MenuItem{
			Label: locales.UpdateTo(v),
			Click: func(*mygo.MenuItem, *mygo.Window) { checkForUpdates() },
		})
	} else {
		items = append(items, updateMenuItem())
	}
	items = append(items, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleQuit})
	return mygo.NewMenu(items)
}

// refreshTrayMenu replaces the menu that stays installed. macOS and Windows
// build a new one on the right-click that opens it.
func (s *shell) refreshTrayMenu() {
	if s.tray == nil || !trayMenuStays() {
		return
	}
	s.tray.SetMenu(s.trayMenu())
}

func trayMenuStays() bool { return runtime.GOOS == "linux" }

// setLanguage shows code from now on and keeps it. "" follows the system.
func (s *shell) setLanguage(code string) {
	code = parseLanguage(code)
	if code == s.prefs.Language {
		return
	}
	s.prefs.Language = code
	s.savePrefs()
	mygo.App.SetLocale(code)
}

// applyLanguage shows the language settings saved, once the app can.
// MYGO_LOCALE (`mygo dev -lang`) wins for that run.
func (s *shell) applyLanguage() {
	if strings.TrimSpace(os.Getenv("MYGO_LOCALE")) != "" {
		return
	}
	mygo.App.SetLocale(s.prefs.Language)
}

func (s *shell) watchChrome() {
	mygo.App.SetMenu(applicationMenu())
	mygo.App.OnLocaleChanged(func(string) {
		mygo.App.SetMenu(applicationMenu())
		s.refreshTrayMenu()
	})
	updater.OnChange(func() {
		if s.win != nil {
			s.win.Invalidate()
		}
		s.refreshTrayMenu()
	})
}

func (s *shell) updatesCard(c *ui.Context, p panelPalette) {
	card(c, p).Padding(12).Gap(10).Children(func() {
		sectionLabel(c, p, locales.Updates())
		checks := updateChecks()
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, locales.CheckAutomatically()).FontSize(12.5).FontWeight(500)
				desc := locales.CheckDaily()
				if t := lastUpdateCheck(); !t.IsZero() {
					desc = locales.CheckDailyLast(relativePast(t))
				}
				ui.Text(c, desc).FontSize(11).TextColor(p.muted)
			})
			if ui.Switch(c.Key("update-checks"), &checks).Label(locales.CheckAutomatically()).Changed() {
				setUpdateChecks(checks)
			}
		})
		downloads := updateDownloads()
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, locales.InstallAutomatically()).FontSize(12.5).FontWeight(500)
				ui.Text(c, locales.InstallNote()).FontSize(11).TextColor(p.muted)
			})
			sw := ui.Switch(c.Key("update-downloads"), &downloads).Label(locales.InstallAutomatically())
			if !checks {
				sw = sw.Disabled(true)
			}
			if sw.Changed() && checks {
				setUpdateDownloads(downloads)
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.CurrentVersion(appVersion())).FontSize(12).TextColor(p.soft).SingleLine().Grow(1).MinWidth(0)
			if textButton(c, p, locales.CheckForUpdates(), p.raised, p.text).Height(28).Clicked() {
				checkForUpdates()
			}
		})
		if v := availableVersion(); v != "" {
			ui.Text(c, locales.UpdateReady(v)).FontSize(11.5).FontWeight(500).TextColor(p.success)
		}
	})
}

func (s *shell) languageCard(c *ui.Context, p panelPalette) {
	card(c, p).Padding(12).Gap(10).Children(func() {
		sectionLabel(c, p, locales.Language())
		codes, names, chosen := languageChoices(s.prefs.Language)
		if ui.Select(c.Key("language"), &chosen, names).Label(locales.Language()).Width(180).Changed() {
			code := codes[0]
			for i, name := range names {
				if name == chosen {
					code = codes[i]
				}
			}
			s.setLanguage(code)
		}
	})
}
