package provider_quota

import (
	"testing"
	"time"
)

func TestIsBalanceLimit_UsesWindowLabel(t *testing.T) {
	tests := []struct {
		window string
		want   bool
	}{
		{window: QuotaWindowPayAsYouGo, want: true},
		{window: QuotaWindowCredits, want: true},
		{window: "payg", want: false},
		{window: "", want: false},
		{window: QuotaWindow5h, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.window, func(t *testing.T) {
			if got := IsBalanceLimit(QuotaLimitStatus{Window: tt.window}); got != tt.want {
				t.Fatalf("IsBalanceLimit(%q) = %t, want %t", tt.window, got, tt.want)
			}
		})
	}
}

func TestElapsedRatio_RequiresAnActiveWindow(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	start := now.Add(-30 * time.Minute)
	reset := now.Add(30 * time.Minute)

	tests := []struct {
		name      string
		start     *time.Time
		reset     *time.Time
		wantRatio float64
		wantOK    bool
	}{
		{name: "active", start: &start, reset: &reset, wantRatio: 0.5, wantOK: true},
		{name: "missing start", reset: &reset, wantOK: false},
		{name: "missing reset", start: &start, wantOK: false},
		{name: "start at now", start: &now, reset: &reset, wantOK: false},
		{name: "reset at now", start: &start, reset: &now, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRatio, gotOK := ElapsedRatio(QuotaLimitStatus{PeriodStart: tt.start, NextResetAt: tt.reset}, now)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %t, want %t", gotOK, tt.wantOK)
			}
			if gotOK && gotRatio != tt.wantRatio {
				t.Fatalf("ratio = %f, want %f", gotRatio, tt.wantRatio)
			}
		})
	}
}
