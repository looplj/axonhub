package provider_quota

import (
	"testing"
	"time"
)

func TestEvaluateQuotaRouting_DecisionTable(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-30 * time.Minute)
	windowReset := now.Add(30 * time.Minute)

	tests := []struct {
		name          string
		limits        []QuotaLimitStatus
		overallStatus string
		limitType     QuotaLimitType
		wantState     RoutingState
		wantReason    string
	}{
		{
			name: "open window under pace",
			limits: []QuotaLimitStatus{{
				Type:        QuotaLimitTypeToken,
				Status:      "available",
				UsageRatio:  0.4,
				Window:      QuotaWindow5h,
				PeriodStart: &windowStart,
				NextResetAt: &windowReset,
			}},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingOpen,
		},
		{
			name: "window over pace and balance positive",
			limits: []QuotaLimitStatus{
				{
					Type:              QuotaLimitTypeToken,
					Status:            "available",
					UsageRatio:        0.8,
					Window:            QuotaWindow5h,
					PeriodStart:       &windowStart,
					NextResetAt:       &windowReset,
					AvailabilityGroup: "funding",
				},
				{
					Type:              QuotaLimitTypeToken,
					Status:            "available",
					Window:            QuotaWindowPayAsYouGo,
					AvailabilityGroup: "funding",
				},
			},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingStickyOnly,
			wantReason:    "window_pressure",
		},
		{
			name: "window exhausted and payg available in same group",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1, Window: QuotaWindow5h, AvailabilityGroup: "funding"},
				{Type: QuotaLimitTypeToken, Status: "available", Window: QuotaWindowPayAsYouGo, AvailabilityGroup: "funding"},
			},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingStickyOnly,
			wantReason:    "window_exhausted_balance_fallback",
		},
		{
			name: "window exhausted and another window available in same group",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1, Window: QuotaWindow5h, AvailabilityGroup: "funding"},
				{Type: QuotaLimitTypeToken, Status: "available", UsageRatio: 0.2, Window: QuotaWindow7d, AvailabilityGroup: "funding"},
			},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingOpen,
		},
		{
			name: "window group exhausted and separate balance group available",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1, Window: QuotaWindow5h, AvailabilityGroup: "window"},
				{Type: QuotaLimitTypeToken, Status: "available", Window: QuotaWindowCredits, AvailabilityGroup: "balance"},
			},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingStickyOnly,
			wantReason:    "window_exhausted_balance_fallback",
		},
		{
			name: "window exhausted and balance exhausted",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1, Window: QuotaWindow5h, AvailabilityGroup: "funding"},
				{Type: QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1, Window: QuotaWindowPayAsYouGo, AvailabilityGroup: "funding"},
			},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingExhausted,
		},
		{
			name: "balance-only available",
			limits: []QuotaLimitStatus{{
				Type:   QuotaLimitTypeToken,
				Status: "available",
				Window: QuotaWindowCredits,
			}},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingOpen,
		},
		{
			name: "balance-only exhausted",
			limits: []QuotaLimitStatus{{
				Type:       QuotaLimitTypeToken,
				Status:     "exhausted",
				UsageRatio: 1,
				Window:     QuotaWindowCredits,
			}},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingExhausted,
		},
		{
			name: "window without time information",
			limits: []QuotaLimitStatus{{
				Type:       QuotaLimitTypeToken,
				Status:     "available",
				UsageRatio: 0.9,
				Window:     QuotaWindow5h,
			}},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingOpen,
		},
		{
			name: "stale window is not balance",
			limits: []QuotaLimitStatus{{
				Type:   QuotaLimitTypeToken,
				Status: "exhausted",
				Window: QuotaWindow5h,
			}},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingExhausted,
		},
		{
			name: "ratio one is exhausted",
			limits: []QuotaLimitStatus{{
				Type:        QuotaLimitTypeToken,
				Status:      "available",
				UsageRatio:  1,
				Window:      QuotaWindow5h,
				PeriodStart: &windowStart,
				NextResetAt: &windowReset,
			}},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingExhausted,
		},
		{
			name: "multiple windows worst wins",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeToken, Status: "available", UsageRatio: 0.2, Window: QuotaWindow5h},
				{Type: QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1, Window: QuotaWindow7d},
			},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingOpen,
		},
		{
			name: "channel exhausted short-circuits limits",
			limits: []QuotaLimitStatus{{
				Type:   QuotaLimitTypeToken,
				Status: "available",
				Window: QuotaWindow5h,
			}},
			overallStatus: "exhausted",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingExhausted,
		},
		{
			name: "limit type isolation without shared group",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeImage, Status: "exhausted", UsageRatio: 1, Window: QuotaWindow5h},
				{Type: QuotaLimitTypeToken, Status: "available", Window: QuotaWindow5h},
			},
			overallStatus: "available",
			limitType:     QuotaLimitTypeImage,
			wantState:     RoutingExhausted,
		},
		{
			name: "cross type or group inclusion",
			limits: []QuotaLimitStatus{
				{Type: QuotaLimitTypeToken, Status: "exhausted", UsageRatio: 1, Window: QuotaWindow5h, AvailabilityGroup: "funding"},
				{Type: QuotaLimitTypeSubscriptionCycle, Status: "available", Window: QuotaWindowCycle, AvailabilityGroup: "funding"},
			},
			overallStatus: "available",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingOpen,
		},
		{
			name:          "unknown and no data is neutral",
			overallStatus: "unknown",
			limitType:     QuotaLimitTypeToken,
			wantState:     RoutingUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotReason := EvaluateQuotaRouting(tt.limits, tt.overallStatus, tt.limitType, now)
			if gotState != tt.wantState {
				t.Fatalf("state = %q, want %q (reason %q)", gotState, tt.wantState, gotReason)
			}
			if gotReason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", gotReason, tt.wantReason)
			}
		})
	}
}
