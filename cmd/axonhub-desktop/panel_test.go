package main

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestPanelAnchor(t *testing.T) {
	panel := mygo.Rectangle{Width: panelWidth, Height: panelHeight}
	tests := []struct {
		name         string
		anchor, work mygo.Rectangle
		panel        mygo.Rectangle
		x, y         int
		center       bool
	}{
		{
			name:   "below a menu bar icon",
			anchor: mygo.Rectangle{X: 1000, Y: 0, Width: 24, Height: 24},
			work:   mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 850},
			x:      812,
			y:      30,
		},
		{
			name:   "above a bottom taskbar icon",
			anchor: mygo.Rectangle{X: 1800, Y: 1040, Width: 40, Height: 40},
			work:   mygo.Rectangle{Width: 1920, Height: 1040},
			x:      1520,
			y:      394,
		},
		{
			name:   "clamped beside a vertical taskbar",
			anchor: mygo.Rectangle{Y: 400, Width: 40, Height: 40},
			work:   mygo.Rectangle{X: 48, Width: 1872, Height: 1080},
			x:      48,
			y:      0,
		},
		{
			name:   "stays on a secondary display",
			anchor: mygo.Rectangle{X: 3000, Width: 22, Height: 22},
			work:   mygo.Rectangle{X: 1920, Width: 1280, Height: 800},
			x:      2800,
			y:      28,
		},
		{
			name:   "clamped when the icon is tighter than the panel",
			anchor: mygo.Rectangle{X: 100, Width: 20, Height: 20},
			work:   mygo.Rectangle{Width: 800, Height: 100},
			panel:  mygo.Rectangle{Width: 320, Height: 120},
			x:      0,
			y:      0,
		},
		{
			name:   "missing icon bounds",
			anchor: mygo.Rectangle{},
			work:   mygo.Rectangle{Width: 1440, Height: 900},
			center: true,
		},
		{
			name:   "zero height anchor",
			anchor: mygo.Rectangle{X: 10, Y: 10, Width: 20},
			work:   mygo.Rectangle{Width: 800, Height: 600},
			center: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := panel
			if tt.panel.Width != 0 || tt.panel.Height != 0 {
				size = tt.panel
			}
			x, y, center := panelAnchor(tt.anchor, size, tt.work)
			if center != tt.center || x != tt.x || y != tt.y {
				t.Fatalf("panelAnchor() = (%d, %d, center=%v), want (%d, %d, center=%v)", x, y, center, tt.x, tt.y, tt.center)
			}
		})
	}
}

func TestOpenAIBaseURLUsesSessionOrigin(t *testing.T) {
	if got := openAIBaseURL("http://127.0.0.1:18090"); got != "http://127.0.0.1:18090/v1" {
		t.Fatalf("openAIBaseURL() = %q", got)
	}
	if got := openAIBaseURL("http://127.0.0.1:18090/"); got != "http://127.0.0.1:18090/v1" {
		t.Fatalf("openAIBaseURL() = %q", got)
	}
}

func TestDockPolicy(t *testing.T) {
	if dockPolicy(false) != mygo.ActivationPolicyAccessory {
		t.Fatal("hidden panel should be an accessory")
	}
	if dockPolicy(true) != mygo.ActivationPolicyRegular {
		t.Fatal("visible panel should be a regular app")
	}
}
