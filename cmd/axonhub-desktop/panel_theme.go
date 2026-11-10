package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// themeMode is the panel's appearance: the system's, or light or dark
// whatever the system shows.
type themeMode int

const (
	themeSystem themeMode = iota
	themeLight
	themeDark
)

// dark reports whether the panel draws dark, given the system's appearance.
func (m themeMode) dark(systemDark bool) bool {
	switch m {
	case themeLight:
		return false
	case themeDark:
		return true
	default:
		return systemDark
	}
}

func (m themeMode) String() string {
	switch m {
	case themeLight:
		return "light"
	case themeDark:
		return "dark"
	default:
		return "system"
	}
}

// panelPalette is the panel's colors, taken from the admin frontend's
// default theme (frontend/src/index.css): navy in the dark, warm paper in
// the light, and the logo's champagne gold as the accent.
type panelPalette struct {
	dark bool

	bg       ui.Color // the panel
	card     ui.Color // grouped sections
	raised   ui.Color // pills, chips, secondary buttons
	edge     ui.Color // the hairline around cards
	hairline ui.Color // dividers inside cards
	hover    ui.Color
	press    ui.Color

	text  ui.Color
	soft  ui.Color // secondary text that is still read
	muted ui.Color // labels and metrics
	wire  ui.Color // routing wires

	gold   ui.Color // the logo's accent nodes, selection, the primary action
	onGold ui.Color
	logo   ui.Color // the logo's arcs and side nodes

	success, successSoft ui.Color
	warning, warningSoft ui.Color
	danger, dangerSoft   ui.Color
	dangerText           ui.Color // banner title on dangerSoft

	primary, onPrimary ui.Color // the footer's Open admin
}

func panelPaletteFor(dark bool) panelPalette {
	if dark {
		gold := ui.Hex("#d4b98a")
		return panelPalette{
			dark:        true,
			bg:          ui.Hex("#070d1a"),
			card:        ui.Hex("#121b29"),
			raised:      ui.Hex("#1b2433"),
			edge:        ui.RGBA(255, 255, 255, 0.06),
			hairline:    ui.RGBA(255, 255, 255, 0.07),
			hover:       ui.RGBA(255, 255, 255, 0.05),
			press:       ui.RGBA(255, 255, 255, 0.09),
			text:        ui.Hex("#f8fafc"),
			soft:        ui.Hex("#cad5e2"),
			muted:       ui.Hex("#90a1b9"),
			wire:        ui.Hex("#90a1b9"),
			gold:        gold,
			onGold:      ui.Hex("#0f172b"),
			logo:        ui.Hex("#f8fafc"),
			success:     ui.Hex("#00bc7d"),
			successSoft: ui.RGBA(0, 188, 125, 0.14),
			warning:     ui.Hex("#fe9a00"),
			warningSoft: ui.RGBA(254, 154, 0, 0.14),
			danger:      ui.Hex("#fb575c"),
			dangerSoft:  ui.RGBA(251, 87, 92, 0.13),
			dangerText:  ui.Hex("#fda4a6"),
			primary:     gold,
			onPrimary:   ui.Hex("#0f172b"),
		}
	}
	return panelPalette{
		bg:          ui.Hex("#f0eeeb"),
		card:        ui.Hex("#fcfcf9"),
		raised:      ui.Hex("#e6e4df"),
		edge:        ui.RGBA(0, 0, 0, 0.06),
		hairline:    ui.Hex("#e6e4df"),
		hover:       ui.RGBA(0, 0, 0, 0.04),
		press:       ui.RGBA(0, 0, 0, 0.08),
		text:        ui.Hex("#0f172b"),
		soft:        ui.Hex("#45556c"),
		muted:       ui.Hex("#4b5666"),
		wire:        ui.Hex("#a19f98"),
		gold:        ui.Hex("#a8844e"),
		onGold:      ui.Hex("#fcfcf9"),
		logo:        ui.Hex("#0f172b"),
		success:     ui.Hex("#087a37"),
		successSoft: ui.RGBA(0, 156, 63, 0.12),
		warning:     ui.Hex("#b45309"),
		warningSoft: ui.RGBA(180, 83, 9, 0.12),
		danger:      ui.Hex("#c8262b"),
		dangerSoft:  ui.RGBA(238, 52, 57, 0.1),
		dangerText:  ui.Hex("#c8262b"),
		primary:     ui.Hex("#0f172b"),
		onPrimary:   ui.Hex("#fcfcf9"),
	}
}

// panelBackgroundColor fills the window before the panel first paints.
func panelBackgroundColor(mode themeMode) string {
	switch mode {
	case themeLight:
		return "#f0eeeb"
	case themeDark:
		return "#070d1a"
	default:
		return "light-dark(#f0eeeb, #070d1a)"
	}
}

// tone is the strong and the soft color of a status.
func (p panelPalette) tone(tone statusTone) (fg, bg ui.Color) {
	switch tone {
	case toneSuccess:
		return p.success, p.successSoft
	case toneActive:
		return p.warning, p.warningSoft
	case toneDanger:
		return p.danger, p.dangerSoft
	default:
		return p.soft, p.raised
	}
}

// usePanelTheme makes the window's theme the palette's, so widgets, the
// focus ring and tooltips follow it.
func usePanelTheme(c *ui.Context, p panelPalette) {
	t := *c.Theme()
	t.Background, t.Text, t.TextMuted = p.bg, p.text, p.muted
	t.Surface, t.SurfaceHover, t.SurfacePressed = p.card, p.raised, p.raised
	t.Border = p.hairline
	t.Accent, t.AccentText = p.gold, p.onGold
	t.InactiveAccent = p.gold
	t.Focus = p.gold
	t.Success, t.Warning, t.Danger = p.success, p.warning, p.danger
	c.SetTheme(&t)
}

// Motion shared by the panel, after mygo-music's: hovers fade in 120 ms,
// presses give a little on a snappy spring, rows come in from just above.
var (
	hoverFade = ui.Animation{Duration: 120 * time.Millisecond}
	rowEnter  = ui.ElementTransition{
		Enter:     &ui.Motion{Y: -6},
		Animation: &ui.Animation{Duration: 220 * time.Millisecond, Ease: ui.CubicBezier(0.2, 0.8, 0.2, 1)},
	}
)
