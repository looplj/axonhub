package claw

import (
	"testing"
	"time"
)

func TestHeartbeatSettingsInActiveHours(t *testing.T) {
	utc := time.UTC

	tests := []struct {
		name   string
		cfg    HeartbeatSettings
		hour   int
		minute int
		want   bool
	}{
		{
			name: "within hours",
			cfg:  HeartbeatSettings{ActiveStart: "08:00", ActiveEnd: "23:00", Timezone: "UTC"},
			hour: 12, minute: 0, want: true,
		},
		{
			name: "before start",
			cfg:  HeartbeatSettings{ActiveStart: "08:00", ActiveEnd: "23:00", Timezone: "UTC"},
			hour: 7, minute: 30, want: false,
		},
		{
			name: "at start",
			cfg:  HeartbeatSettings{ActiveStart: "08:00", ActiveEnd: "23:00", Timezone: "UTC"},
			hour: 8, minute: 0, want: true,
		},
		{
			name: "at end",
			cfg:  HeartbeatSettings{ActiveStart: "08:00", ActiveEnd: "23:00", Timezone: "UTC"},
			hour: 23, minute: 0, want: false,
		},
		{
			name: "no config (always active)",
			cfg:  HeartbeatSettings{},
			hour: 3, minute: 0, want: true,
		},
		{
			name: "wrap midnight - in range",
			cfg:  HeartbeatSettings{ActiveStart: "22:00", ActiveEnd: "06:00", Timezone: "UTC"},
			hour: 23, minute: 30, want: true,
		},
		{
			name: "wrap midnight - in range early morning",
			cfg:  HeartbeatSettings{ActiveStart: "22:00", ActiveEnd: "06:00", Timezone: "UTC"},
			hour: 3, minute: 0, want: true,
		},
		{
			name: "wrap midnight - out of range",
			cfg:  HeartbeatSettings{ActiveStart: "22:00", ActiveEnd: "06:00", Timezone: "UTC"},
			hour: 12, minute: 0, want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 3, 14, tt.hour, tt.minute, 0, 0, utc)

			got := tt.cfg.InActiveHours(now)
			if got != tt.want {
				t.Errorf("InActiveHours() at %02d:%02d = %v, want %v", tt.hour, tt.minute, got, tt.want)
			}
		})
	}
}
