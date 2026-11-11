package main

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
	"github.com/looplj/axonhub/internal/log"
)

// panelPage is what the panel shows.
type panelPage int

const (
	pageOverview panelPage = iota
	pageSettings
)

// settingsForm is the settings page's state. It is touched on the main
// thread only; a test or save runs its ping off it and hands the result
// back through the window. seq only increases, including across visits, so
// a ping from an earlier form cannot match the one on screen.
type settingsForm struct {
	kind    int
	dsn     string
	port    string
	lan     bool
	showDSN bool
	busy    bool
	status  string
	tone    statusTone
	seq     uint64
}

const (
	formConnecting  = "Connecting…"
	formConnected   = "Connected"
	formSaved       = "Saved. Restarting…"
	formDSNEmpty    = "DSN is empty"
	formPortInvalid = "Port must be from 1 to 65535"
)

// startOffline opens the panel on settings with why the server did not
// start. The panel loads nothing until the app relaunches.
func (s *shell) startOffline(err error) {
	s.offline = true
	s.startErr = err.Error()
	s.openSettings()
}

// openSettings fills the form from config.yml and drops any ping still
// running from the previous visit.
func (s *shell) openSettings() {
	seq := s.form.seq + 1
	s.form = settingsForm{seq: seq}
	if cfg, err := readDesktopSettings(s.configPath); err == nil {
		s.form.kind = dbKindIndex(cfg.DB.Dialect)
		s.form.dsn = cfg.DB.DSN
		s.form.port = strconv.Itoa(cfg.Port)
		s.form.lan = cfg.LAN
	} else {
		s.form.status, s.form.tone = err.Error(), toneDanger
	}
	s.page = pageSettings
}

func (s *shell) closeSettings() {
	if s.offline {
		return
	}
	s.form.seq++
	s.page = pageOverview
}

// formConfig is the database the form describes. An empty SQLite DSN is
// the default file in the data directory.
func (s *shell) formConfig() dbConfig {
	kind := dbKinds[s.form.kind]
	dsn := strings.TrimSpace(s.form.dsn)
	if dsn == "" && kind.Dialect == "sqlite3" && s.configPath != "" {
		dsn = sqliteDSN(filepath.Join(filepath.Dir(s.configPath), "axonhub.db"))
	}
	return dbConfig{Dialect: kind.Dialect, DSN: dsn}
}

// testDatabase pings the form's database. save also writes config.yml
// and relaunches once the ping succeeds; a failed ping saves nothing.
func (s *shell) testDatabase(save bool) {
	if s.form.busy {
		return
	}
	port, err := parseListenPort(s.form.port)
	if err != nil {
		s.form.status, s.form.tone = err.Error(), toneDanger
		return
	}
	cfg := s.formConfig()
	lan := s.form.lan
	s.form.seq++
	seq := s.form.seq
	s.form.busy = true
	s.form.status, s.form.tone = formConnecting, toneMuted
	go func() {
		defer recoverLog("desktop settings ping")
		err := pingDB(context.Background(), cfg)
		s.win.Update(func() {
			if seq != s.form.seq {
				return
			}
			s.form.busy = false
			if err != nil {
				s.form.status, s.form.tone = err.Error(), toneDanger
				return
			}
			if !save {
				s.form.status, s.form.tone = formConnected, toneSuccess
				return
			}
			if err := writeDesktopSettings(s.configPath, desktopSettings{Port: port, LAN: lan, DB: cfg}); err != nil {
				log.Warn(context.Background(), "desktop config not saved", log.Cause(err))
				s.form.status, s.form.tone = err.Error(), toneDanger
				return
			}
			s.form.status, s.form.tone = formSaved, toneSuccess
			mygo.App.Relaunch()
		})
	}()
}

// formStatusText is a known settings status in the language the app shows.
// A database error stays as the server or driver wrote it.
func formStatusText(status string) string {
	switch status {
	case formConnecting:
		return locales.Connecting()
	case formConnected:
		return locales.Connected()
	case formSaved:
		return locales.SavedRestarting()
	case formDSNEmpty:
		return locales.DsnEmpty()
	case formPortInvalid:
		return locales.PortInvalid()
	default:
		return status
	}
}

// formEdited drops a ping result that no longer matches the form.
func (s *shell) formEdited() {
	s.form.seq++
	s.form.busy = false
	s.form.status = ""
}

func (s *shell) settingsView(c *ui.Context, p panelPalette) {
	ui.Row(c).Shrink(0).Height(40).Padding(0, 2, 0, 0).Gap(4).AlignItems(ui.Center).Children(func() {
		if !s.offline {
			if iconButton(c, p, iconBack, locales.Back()).Clicked() {
				s.closeSettings()
			}
		} else {
			ui.Box(c).Size(6, 1)
			logo := ui.Box(c).Size(24, 24).Shrink(0)
			logo.Draw(func(pt *ui.Painter, r ui.Rect) { paintLogo(pt, r, p.logo, p.gold) })
			ui.Box(c).Size(4, 1)
		}
		ui.Text(c, locales.Settings()).FontSize(15).FontWeight(600).LetterSpacing(-0.15).SingleLine()
	})

	ui.Scroll(c).Grow(1).MinHeight(0).Gap(8).Children(func() {
		if s.offline {
			s.startBanner(c, p)
		}
		s.languageCard(c, p)
		s.appearanceCard(c, p)
		if !s.offline {
			s.notifyCard(c, p)
		}
		s.serverCard(c, p)
		s.databaseCard(c, p)
		s.updatesCard(c, p)
	})

	ui.Row(c).Shrink(0).Padding(8, 0, 0, 0).Gap(8).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(p.hairline).Children(func() {
		if s.offline {
			if textButton(c, p, locales.Quit(), ui.Transparent, p.soft).Clicked() {
				mygo.App.Quit()
			}
		} else if textButton(c, p, locales.Cancel(), ui.Transparent, p.soft).Clicked() {
			s.closeSettings()
		}
		ui.Spacer(c)
		save := textButton(c, p, locales.SaveRestart(), p.primary, p.onPrimary).FontWeight(600).Disabled(s.form.busy)
		if save.Clicked() {
			s.testDatabase(true)
		}
	})
}

// startBanner says why AxonHub did not start. The error stays on screen
// until a save relaunches the app.
func (s *shell) startBanner(c *ui.Context, p panelPalette) {
	ui.Row(c.Key("start-banner")).Shrink(0).Padding(10, 12).Gap(10).Radius(10).
		AlignItems(ui.Start).Background(p.dangerSoft).Children(func() {
		ui.Icon(c, iconAlert).Size(15, 15).TextColor(p.danger).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, locales.CouldntStart()).FontSize(12.5).FontWeight(600).TextColor(p.dangerText)
			ui.Text(c, s.startErr).FontSize(11.5).TextColor(p.soft).MaxLines(4)
		})
	})
}

func (s *shell) databaseCard(c *ui.Context, p panelPalette) {
	kind := dbKinds[s.form.kind]
	card(c, p).Padding(12).Gap(10).Children(func() {
		sectionLabel(c, p, locales.Database())
		labels := make([]string, len(dbKinds))
		for i, k := range dbKinds {
			labels[i] = k.Label
		}
		if segmented(c, p, &s.form.kind, labels...) {
			s.formEdited()
		}

		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, locales.Dsn()).FontSize(12).TextColor(p.muted)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				input := ui.TextInput(c, &s.form.dsn).Grow(1).MinWidth(0).Font("monospace").FontSize(11.5).
					Placeholder(kind.Example).Label(locales.Dsn())
				if !s.form.showDSN && kind.Dialect != "sqlite3" {
					input.Password()
				}
				if input.Changed() {
					s.formEdited()
				}
				if input.Submitted() {
					s.testDatabase(false)
				}
				if kind.Dialect != "sqlite3" {
					icon, label := iconEye, locales.ShowDsn()
					if s.form.showDSN {
						icon, label = iconEyeOff, locales.HideDsn()
					}
					if iconButton(c, p, icon, label).Clicked() {
						s.form.showDSN = !s.form.showDSN
					}
				}
			})
			hint := locales.DsnExample(kind.Example)
			if kind.Dialect == "sqlite3" {
				hint = locales.SqliteHint()
			}
			ui.Text(c, hint).FontSize(11).TextColor(p.muted).MaxLines(3)
		})

		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			test := textButton(c, p, locales.TestConnection(), p.raised, p.text).Height(28).Disabled(s.form.busy)
			if test.Clicked() {
				s.testDatabase(false)
			}
			if s.form.status != "" {
				color := p.muted
				if s.form.tone != toneMuted {
					color, _ = p.tone(s.form.tone)
				}
				ui.Text(c, formStatusText(s.form.status)).FontSize(11.5).TextColor(color).MaxLines(3).Grow(1).MinWidth(0)
			}
		})
		ui.Text(c, locales.SaveNote()).
			FontSize(11).TextColor(p.muted)
	})
}

func (s *shell) serverCard(c *ui.Context, p panelPalette) {
	card(c, p).Padding(12).Gap(10).Children(func() {
		sectionLabel(c, p, locales.Server())
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, locales.Port()).FontSize(12).TextColor(p.muted)
			input := ui.TextInput(c, &s.form.port).Font("monospace").FontSize(12.5).
				Placeholder(strconv.Itoa(desktopPort)).Label(locales.Port())
			if input.Changed() {
				s.formEdited()
			}
			ui.Text(c, locales.PortHint()).FontSize(11).TextColor(p.muted).MaxLines(3)
		})
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, locales.LanAccess()).FontSize(12.5).FontWeight(500)
				ui.Text(c, locales.LanNote()).FontSize(11).TextColor(p.muted)
			})
			if ui.Switch(c.Key("lan"), &s.form.lan).Label(locales.LanAccess()).Changed() {
				s.formEdited()
			}
		})
	})
}

func (s *shell) appearanceCard(c *ui.Context, p panelPalette) {
	card(c, p).Padding(12).Gap(10).Children(func() {
		sectionLabel(c, p, locales.Appearance())
		mode := int(s.prefs.Theme)
		if segmented(c, p, &mode, locales.System(), locales.Light(), locales.Dark()) {
			s.setTheme(themeMode(mode))
		}
	})
}

func (s *shell) notifyCard(c *ui.Context, p panelPalette) {
	card(c, p).Padding(12).Gap(10).Children(func() {
		sectionLabel(c, p, locales.Notifications())
		on := s.prefs.Notify
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, locales.NotifyAlerts()).FontSize(12.5).FontWeight(500)
				ui.Text(c, locales.NotifyNote()).FontSize(11).TextColor(p.muted)
			})
			if ui.Switch(c.Key("notify"), &on).Label(locales.NotifyAlerts()).Changed() {
				s.setNotify(on)
			}
		})
	})
}

// segmented is a segmented control with a capsule that slides to the
// choice, after mygo-music's.
func segmented(c *ui.Context, p panelPalette, selected *int, labels ...string) bool {
	const height, inset, radius = 28, 2, 7
	width := float32(0)
	for _, l := range labels {
		w, _ := c.MeasureText(0, ui.Span{Text: l, Size: 12.5})
		width = max(width, w+28)
	}
	capsule := p.card
	if p.dark {
		capsule = p.raised.MixOklab(p.text, 0.12)
	}
	before := *selected
	parts := ui.SegmentedBase(c, selected, len(labels))
	at := parts.Track.AnimateTo("at", float32(*selected), ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.08}})
	parts.Track.Height(height).Padding(inset).Radius(radius).Background(p.raised).AlignSelf(ui.Start).
		Draw(func(pt *ui.Painter, r ui.Rect) {
			pill := ui.Rect{X: r.X + inset + at*width, Y: r.Y + inset, W: width, H: r.H - 2*inset}
			if !p.dark {
				pt.Shadow(pill, radius-1, 0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.12))
			}
			pt.Fill(pill, capsule, radius-1)
		}).Children(func() {
		for i, l := range labels {
			seg := parts.Segment(i).Width(width).Height(height - 2*inset).Center().Radius(radius - 1)
			color := p.soft
			if i == *selected {
				color = p.text
			}
			seg.Children(func() { ui.Text(c, l).FontSize(12.5).FontWeight(500).TextColor(color).SingleLine() })
		}
	})
	return *selected != before
}
