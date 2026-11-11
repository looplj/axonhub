package main

import (
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
)

const (
	barsTime     = 900 * time.Millisecond
	skeletonTime = 1600 * time.Millisecond
)

// Icons on a 16-point grid, stroked in the color of the text.
func strokeIcon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	iconRefresh  = strokeIcon(`<path d="M13 8a5 5 0 1 1-1.6-3.7"/><path d="M13 2.5v2.8h-2.8"/>`)
	iconKey      = strokeIcon(`<circle cx="5.5" cy="10.5" r="3"/><path d="M7.7 8.3 13 3M11 5l1.5 1.5"/>`)
	iconAlert    = strokeIcon(`<circle cx="8" cy="8" r="6.25"/><path d="M8 4.8v3.6M8 11h0"/>`)
	iconWarning  = strokeIcon(`<path d="M8 2.5l6 11H2z"/><path d="M8 7v3"/>`)
	iconSettings = strokeIcon(`<path d="M2.5 4.5h6M11.5 4.5h2M2.5 11.5h2M7.5 11.5h6"/><circle cx="10" cy="4.5" r="1.5"/><circle cx="6" cy="11.5" r="1.5"/>`)
	iconBack     = strokeIcon(`<path d="M10 3.5 5.5 8l4.5 4.5"/>`)
	iconEye      = strokeIcon(`<path d="M1.75 8S4 3.75 8 3.75 14.25 8 14.25 8 12 12.25 8 12.25 1.75 8 1.75 8z"/><circle cx="8" cy="8" r="2"/>`)
	iconEyeOff   = strokeIcon(`<path d="M6.6 3.9A6.4 6.4 0 0 1 8 3.75C12 3.75 14.25 8 14.25 8a10.6 10.6 0 0 1-1.6 2.2M4.3 4.9A10.4 10.4 0 0 0 1.75 8S4 12.25 8 12.25a6 6 0 0 0 3.1-.85"/><path d="M2 2l12 12"/>`)
)

func (s *shell) view(c *ui.Context) {
	if c.Shortcut(0, ui.KeyEscape) {
		if s.page == pageSettings && !s.offline {
			s.closeSettings()
		} else {
			s.hide()
		}
	}
	p := panelPaletteFor(s.prefs.Theme.dark(c.Theme().Dark))
	usePanelTheme(c, p)
	ui.Column(c).Fill().Padding(12).Gap(8).Background(p.bg).Children(func() {
		if s.page == pageSettings {
			s.settingsView(c, p)
			return
		}
		s.header(c, p)
		s.tabBar(c, p)
		switch s.tab {
		case tabKeys:
			s.keysPage(c, p)
		case tabChannels:
			s.channelsPage(c, p)
		case tabModels:
			s.modelsPage(c, p)
		default:
			s.trafficPage(c, p)
		}
		if s.loadErr != "" && s.loadErr != panelRefreshError {
			s.notice(c, p, loadErrText(s.loadErr))
		}
		s.footer(c, p)
	})
}

func (s *shell) trafficPage(c *ui.Context, p panelPalette) {
	if s.loadErr == panelRefreshError {
		s.refreshBanner(c, p)
	}
	s.statsCard(c, p)
	s.flowCard(c, p)
	s.listHeader(c, p)
	s.requestList(c, p)
}

func (s *shell) header(c *ui.Context, p panelPalette) {
	ui.Row(c).Shrink(0).Height(40).Padding(0, 2, 0, 6).Gap(8).AlignItems(ui.Center).Children(func() {
		logo := ui.Box(c).Size(24, 24).Shrink(0)
		logo.Draw(func(pt *ui.Painter, r ui.Rect) {
			paintLogo(pt, r, p.logo, p.gold)
		})
		ui.Text(c, "AxonHub").FontSize(15).FontWeight(600).LetterSpacing(-0.15).SingleLine()
		if s.loadErr == panelRefreshError && s.hasSnapshot {
			statusPill(c, p, locales.DataStale(), toneActive)
		} else {
			statusPill(c, p, locales.Running(), toneSuccess)
		}
		ui.Spacer(c)
		if iconButton(c, p, iconRefresh, locales.Refresh()).Clicked() {
			s.reloadTab()
		}
		if iconButton(c, p, iconSettings, locales.Settings()).Clicked() {
			s.openSettings()
		}
	})
}

// refreshBanner says the last load failed. The panel keeps the last
// snapshot under it, and the timer keeps retrying.
func (s *shell) refreshBanner(c *ui.Context, p panelPalette) {
	detail := locales.Retrying()
	if s.hasSnapshot {
		detail = locales.ShowingLast()
	}
	ui.Row(c.Key("refresh-banner")).Shrink(0).Padding(8, 6, 8, 12).Gap(10).Radius(10).
		AlignItems(ui.Center).Background(p.dangerSoft).Transition(rowEnter).Children(func() {
		ui.Icon(c, iconAlert).Size(15, 15).TextColor(p.danger).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, locales.CouldntRefresh()).FontSize(12.5).FontWeight(600).TextColor(p.dangerText).SingleLine()
			ui.Text(c, detail).FontSize(11.5).TextColor(p.soft).SingleLine()
		})
		if textButton(c, p, locales.RetryNow(), p.danger.Alpha(0.2), p.text).Clicked() {
			s.loadPanelAsync()
		}
	})
}

func (s *shell) statsCard(c *ui.Context, p panelPalette) {
	stats := panelStats(s.snapshot)
	card(c, p).Padding(10, 4).Children(func() {
		ui.Row(c).Children(func() {
			for i, stat := range stats {
				cell := ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).PaddingX(10)
				if i > 0 {
					cell.BorderWidth(0, 0, 0, 1).BorderColor(p.hairline)
				}
				color := p.text
				if s.hasSnapshot && stat.Tone == toneDanger {
					color = p.danger
				} else if !s.hasSnapshot || stat.Value == "0" {
					color = p.muted
				}
				cell.Children(func() {
					sectionLabel(c, p, stat.Label)
					ui.Text(c, stat.Value).FontSize(17).FontWeight(600).LetterSpacing(-0.15).
						FontFeatures("tnum").TextColor(color).SingleLine()
				})
			}
		})
	})
}

func (s *shell) listHeader(c *ui.Context, p panelPalette) {
	ui.Row(c).Shrink(0).Padding(4, 6, 0, 6).Gap(6).AlignItems(ui.Center).Children(func() {
		sectionLabel(c, p, locales.RecentRequests())
		if n := len(s.snapshot.Requests); n > 0 {
			ui.Row(c).Height(16).PaddingX(6).Radius(999).AlignItems(ui.Center).Background(p.raised).Children(func() {
				ui.Text(c, formatCompact(int64(n))).FontSize(10.5).FontFeatures("tnum").TextColor(p.soft)
			})
		}
	})
}

func (s *shell) requestList(c *ui.Context, p panelPalette) {
	ui.Scroll(c).Grow(1).MinHeight(0).Gap(2).Children(func() {
		switch {
		case len(s.snapshot.Requests) > 0:
			lo.ForEach(s.snapshot.Requests, func(request panelRequest, index int) {
				key := any(request.ID)
				if request.ID == "" {
					key = index
				}
				s.requestRow(c.Key(key), p, request)
				if s.expandedID != "" && request.ID == s.expandedID {
					s.requestDetail(c.Key("detail"), p, request)
				}
			})
		case s.hasSnapshot:
			s.firstRun(c, p)
		case s.loadErr == "":
			skeletonRows(c, p)
		}
	})
}

// requestRow is the status glyph, caller and channel over the model, then
// the result on the right. The snapshot has no created time, so the row
// does not show one. A click opens the request's attempts under it.
func (s *shell) requestRow(c *ui.Context, p panelPalette, request panelRequest) {
	selected := s.selectedID != "" && request.ID == s.selectedID
	result := presentRequest(request)
	row := ui.ButtonBase(c).Justify(ui.Start).AlignItems(ui.Center).Gap(10).MinHeight(46).
		Padding(6, 10, 6, 8).Radius(10).Cursor(ui.CursorPointer).Transition(rowEnter)
	bg := ui.Transparent
	switch {
	case selected:
		bg = p.gold.Alpha(0.14)
	case row.Pressed():
		bg = p.press
	case row.Hovered():
		bg = p.hover
	}
	row.Background(row.AnimateColor("bg", bg, hoverFade))
	if row.Clicked() {
		s.chooseRequest(request)
	}
	row.Children(func() {
		statusGlyph(c, p, result.Tone)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Baseline).Children(func() {
				ui.Text(c, panelLabel(request.Caller)).FontSize(13).FontWeight(500).SingleLine().Shrink(1)
				ui.Text(c, "→ "+result.Channel).FontSize(11.5).TextColor(p.muted).SingleLine().Shrink(1)
			})
			if request.ModelID != "" {
				ui.Text(c, request.ModelID).Font("monospace").FontSize(11).TextColor(p.muted).SingleLine()
			}
		})
		ui.Column(c).Shrink(0).Gap(1).AlignItems(ui.End).Children(func() {
			primary := p.text
			switch result.Tone {
			case toneActive:
				primary = p.warning
			case toneDanger:
				primary = p.danger
			}
			if result.Primary != "" {
				ui.Text(c, result.Primary).FontSize(12).FontWeight(500).FontFeatures("tnum").TextColor(primary).SingleLine()
			}
			if result.Secondary != "" {
				ui.Text(c, result.Secondary).FontSize(11).FontFeatures("tnum").TextColor(p.muted).SingleLine()
			}
		})
	})
}

// firstRun replaces the empty list with the three steps that bring the
// first request in.
func (s *shell) firstRun(c *ui.Context, p panelPalette) {
	ui.Column(c).Grow(1).Justify(ui.Center).Gap(14).Padding(4, 14).Children(func() {
		ui.Text(c, locales.NoRequests()).FontSize(14).FontWeight(600)
		steps := []string{
			locales.StepBaseUrl(),
			locales.StepApiKey(),
			locales.StepChannel(),
		}
		for i, step := range steps {
			ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
				ui.Row(c).Size(20, 20).Shrink(0).Center().Radius(999).Background(p.gold.Alpha(0.18)).Children(func() {
					ui.Textf(c, "%d", i+1).FontSize(11).FontWeight(600).TextColor(p.gold)
				})
				ui.Text(c, step).FontSize(12.5).TextColor(p.soft).Grow(1).MinWidth(0)
			})
		}
	})
}

// skeletonRows hold the list's place until the first snapshot arrives.
func skeletonRows(c *ui.Context, p panelPalette) {
	list := ui.Column(c).Gap(2)
	pulse := list.Loop("pulse", skeletonTime, ui.Bounce(ui.EaseInOut))
	list.Opacity(0.45 + 0.55*pulse).Children(func() {
		for i := range 4 {
			ui.Row(c.Key(i)).Height(46).Padding(6, 10, 6, 8).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(14, 14).Radius(999).Background(p.raised)
				ui.Column(c).Grow(1).Gap(6).Children(func() {
					ui.Box(c).Height(9).Width(float32(140 - 18*i)).Radius(4).Background(p.raised)
					ui.Box(c).Height(7).Width(90).Radius(4).Background(p.raised)
				})
				ui.Box(c).Size(54, 9).Radius(4).Background(p.raised)
			})
		}
	})
}

func (s *shell) notice(c *ui.Context, p panelPalette, message string) {
	ui.Row(c.Key("notice")).Shrink(0).PaddingX(6).Gap(8).AlignItems(ui.Center).Transition(rowEnter).Children(func() {
		ui.Icon(c, iconWarning).Size(13, 13).TextColor(p.warning).Shrink(0)
		ui.Text(c, message).FontSize(11.5).TextColor(p.warning).SingleLine().Shrink(1)
	})
}

func connectRow(c *ui.Context, p panelPalette, label string, divided bool, children func()) {
	row := ui.Row(c).Height(40).Padding(0, 6, 0, 12).Gap(10).AlignItems(ui.Center)
	if divided {
		row.BorderWidth(1, 0, 0, 0).BorderColor(p.hairline)
	}
	row.Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(p.muted).Shrink(0).SingleLine()
		children()
	})
}

// copyButton reads Copied for a moment after it copies.
func (s *shell) copyButton(c *ui.Context, p panelPalette, what string, emphasize bool) ui.Element {
	label := locales.Copy()
	if s.copied == what {
		label = locales.Copied()
	}
	bg, fg := p.raised, p.text
	if emphasize {
		bg, fg = p.gold, p.onGold
	}
	return textButton(c.Key(what), p, label, bg, fg).Height(26).PaddingX(10).Radius(7)
}

func (s *shell) footer(c *ui.Context, p panelPalette) {
	ui.Row(c).Shrink(0).Padding(8, 0, 0, 0).Gap(8).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(p.hairline).Children(func() {
		if textButton(c, p, locales.OpenAdmin(), p.primary, p.onPrimary).FontWeight(600).Clicked() {
			s.openAdmin()
		}
		if s.tab == tabTraffic {
			ui.Text(c, locales.RefreshesEvery("5")).FontSize(11).TextColor(p.muted).SingleLine().Shrink(1)
		}
		ui.Spacer(c)
		if textButton(c, p, locales.Quit(), ui.Transparent, p.soft).Clicked() {
			mygo.App.Quit()
		}
	})
}

// card groups a section: a raised fill edged by a hairline, with a soft
// shadow in the light.
func card(c *ui.Context, p panelPalette) ui.Element {
	el := ui.Column(c).Shrink(0).Radius(12).Background(p.card).InsetShadow(0, 0, 0, 1, p.edge)
	if !p.dark {
		el.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.04))
	}
	return el
}

func sectionLabel(c *ui.Context, p panelPalette, label string) {
	text := ui.Text(c, strings.ToUpper(label)).FontSize(10.5).FontWeight(500).TextColor(p.muted).SingleLine()
	// Tracking suits a short Latin label. Chinese reads tighter without it.
	if !strings.ContainsFunc(label, func(r rune) bool { return r > unicode.MaxASCII }) {
		text.LetterSpacing(0.7)
	}
}

func statusPill(c *ui.Context, p panelPalette, label string, tone statusTone) {
	fg, bg := p.tone(tone)
	ui.Row(c).Shrink(0).Height(22).PaddingX(8).Gap(6).Radius(999).AlignItems(ui.Center).Background(bg).Children(func() {
		dot(c, fg, 6)
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(fg).SingleLine()
	})
}

func chip(c *ui.Context, p panelPalette, label string, tone statusTone) {
	fg, bg := p.tone(tone)
	ui.Row(c).Shrink(0).Height(20).PaddingX(8).Radius(999).AlignItems(ui.Center).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(fg).SingleLine()
	})
}

// endpointPill is a caller or a channel on the flow. A zero dot color
// leaves the dot out. A selected pill is edged in gold. A placeholder is
// outlined, with no fill.
func endpointPill(c *ui.Context, p panelPalette, label string, dotColor ui.Color, selected, placeholder bool) {
	pill := ui.Row(c).Shrink(1).MinWidth(0).MaxWidth(132).Height(26).PaddingX(10).Gap(6).Radius(999).AlignItems(ui.Center)
	text := p.text
	switch {
	case placeholder:
		pill.InsetShadow(0, 0, 0, 1, p.muted.Alpha(0.35))
		text = p.muted
	case selected:
		pill.Background(p.gold.Alpha(0.14)).InsetShadow(0, 0, 0, 1, p.gold.Alpha(0.6))
	default:
		pill.Background(p.bg).InsetShadow(0, 0, 0, 1, p.edge)
	}
	pill.Children(func() {
		if dotColor != (ui.Color{}) {
			dot(c, dotColor, 7)
		}
		ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(text).SingleLine().Shrink(1)
	})
}

// statusGlyph leads a row: equalizer bars while in progress, a check once
// completed, a cross when failed or canceled.
func statusGlyph(c *ui.Context, p panelPalette, tone statusTone) {
	glyph := ui.Box(c).Size(22, 22).Shrink(0)
	phase := float32(0)
	if tone == toneActive {
		phase = glyph.Loop("bars", barsTime, ui.Linear)
	}
	fg, bg := p.tone(tone)
	glyph.Draw(func(pt *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		switch tone {
		case toneActive:
			paintBars(pt, ui.Rect{X: cx - 7, Y: cy - 6, W: 14, H: 12}, phase, fg)
		case toneSuccess, toneDanger:
			var disc ui.Path
			disc.Circle(cx, cy, 6.5)
			pt.FillPath(&disc, bg)
			var mark ui.Path
			if tone == toneSuccess {
				mark.MoveTo(cx-2.6, cy+0.2).LineTo(cx-0.8, cy+2).LineTo(cx+2.6, cy-1.6)
			} else {
				mark.MoveTo(cx-2, cy-2).LineTo(cx+2, cy+2).MoveTo(cx+2, cy-2).LineTo(cx-2, cy+2)
			}
			mark.Cap(ui.RoundCap).Join(ui.RoundJoin)
			pt.StrokePath(&mark, 1.5, fg)
		default:
			var disc ui.Path
			disc.Circle(cx, cy, 3)
			pt.FillPath(&disc, p.muted)
		}
	})
}

// paintBars draws three rounded bars rising and falling out of step, as
// mygo-music's equalizer does for the playing track.
func paintBars(pt *ui.Painter, r ui.Rect, phase float32, color ui.Color) {
	const bars = 3
	width := r.W / (bars + (bars-1)*0.5)
	gap := width * 0.5
	for i := range bars {
		wave := 0.5 - 0.5*math.Cos(2*math.Pi*float64(phase+float32(i)*0.31))
		h := r.H * (0.35 + 0.65*float32(wave))
		x := r.X + float32(i)*(width+gap)
		pt.Fill(ui.Rect{X: x, Y: r.Y + r.H - h, W: width, H: h}, color, width/2)
	}
}

// iconButton is a borderless icon button that a soft square marks under
// the pointer and that gives a little as it is pressed.
func iconButton(c *ui.Context, p panelPalette, icon *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c).Size(28, 28).Center().Radius(7).Label(label).Tooltip(label).TextColor(p.soft)
	bg := ui.Transparent
	if b.Hovered() {
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, hoverFade))
	pressPose(b, 0.88)
	b.Children(func() { ui.Icon(c, icon).Size(15, 15) })
	return b
}

// textButton is a filled button with a label, lighter under the pointer.
func textButton(c *ui.Context, p panelPalette, label string, bg, fg ui.Color) ui.Element {
	b := ui.ButtonBase(c).Height(30).PaddingX(12).Radius(8).TextColor(fg)
	fill := bg
	if b.Hovered() {
		if bg == ui.Transparent {
			fill = p.hover
		} else {
			fill = bg.MixOklab(p.text, 0.1)
		}
	}
	b.Background(b.AnimateColor("bg", fill, hoverFade))
	pressPose(b, 0.94)
	b.Children(func() {
		ui.Text(c, label).FontSize(12.5).FontWeight(500).SingleLine()
	})
	return b
}

func pressPose(b ui.Element, scale float32) {
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = scale
	}
	b.AnimatePose(pose, ui.Snappy)
}

func dot(c *ui.Context, color ui.Color, size float32) {
	ui.Box(c).Size(size, size).Shrink(0).Radius(999).Background(color)
}

// paintLogo draws the synapse mark of frontend/src/components/axonhub-logo.tsx
// on its 64-point grid: four arcs through a shared hub, the side nodes in
// ink and the top and bottom nodes in gold.
func paintLogo(pt *ui.Painter, r ui.Rect, ink, gold ui.Color) {
	size := min(r.W, r.H)
	k := size / 64
	ox, oy := r.X+(r.W-size)/2, r.Y+(r.H-size)/2
	at := func(x, y float32) (float32, float32) { return ox + x*k, oy + y*k }

	var arcs ui.Path
	arcs.MoveTo(at(32, 7)).LineTo(at(32, 15))
	cubic(&arcs, at, 32, 24, 24, 32, 15, 32)
	arcs.LineTo(at(7, 32))
	arcs.MoveTo(at(32, 15))
	cubic(&arcs, at, 32, 24, 40, 32, 49, 32)
	arcs.LineTo(at(57, 32))
	arcs.MoveTo(at(32, 57)).LineTo(at(32, 49))
	cubic(&arcs, at, 32, 40, 24, 32, 15, 32)
	arcs.MoveTo(at(32, 49))
	cubic(&arcs, at, 32, 40, 40, 32, 49, 32)
	arcs.Cap(ui.RoundCap).Join(ui.RoundJoin)
	pt.StrokePath(&arcs, 3.5*k, ink)

	node := func(x, y float32, color ui.Color) {
		var n ui.Path
		cx, cy := at(x, y)
		n.Circle(cx, cy, 5.5*k)
		pt.FillPath(&n, color)
	}
	node(7, 32, ink)
	node(57, 32, ink)
	node(32, 7, gold)
	node(32, 57, gold)
}

func cubic(path *ui.Path, at func(x, y float32) (float32, float32), c1x, c1y, c2x, c2y, x, y float32) {
	ax, ay := at(c1x, c1y)
	bx, by := at(c2x, c2y)
	ex, ey := at(x, y)
	path.CubeTo(ax, ay, bx, by, ex, ey)
}
