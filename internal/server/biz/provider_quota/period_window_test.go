package provider_quota

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestClaudeCodeQuotaChecker_LimitsCarryTheirPeriod(t *testing.T) {
	t.Parallel()

	fiveHourReset := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	sevenDayReset := time.Now().Add(3 * 24 * time.Hour).Truncate(time.Second)

	headers := http.Header{}
	headers.Set("Anthropic-Ratelimit-Unified-Status", "allowed")
	headers.Set("Anthropic-Ratelimit-Unified-5h-Utilization", "0.5")
	headers.Set("Anthropic-Ratelimit-Unified-5h-Reset", strconv.FormatInt(fiveHourReset.Unix(), 10))
	headers.Set("Anthropic-Ratelimit-Unified-7d-Utilization", "0.25")
	headers.Set("Anthropic-Ratelimit-Unified-7d-Reset", strconv.FormatInt(sevenDayReset.Unix(), 10))

	checker := &ClaudeCodeQuotaChecker{}

	quota, err := checker.parseResponse(headers)
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)

	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.NotNil(t, quota.Limits[0].PeriodStart)
	require.Equal(t, fiveHourReset.Add(-5*time.Hour).Unix(), quota.Limits[0].PeriodStart.Unix())

	require.Equal(t, QuotaWindow7d, quota.Limits[1].Window)
	require.NotNil(t, quota.Limits[1].PeriodStart)
	require.Equal(t, sevenDayReset.Add(-7*24*time.Hour).Unix(), quota.Limits[1].PeriodStart.Unix())
}

func TestClaudeCodeQuotaChecker_NoPeriodWithoutResetHeader(t *testing.T) {
	t.Parallel()

	headers := http.Header{}
	headers.Set("Anthropic-Ratelimit-Unified-Status", "allowed")
	headers.Set("Anthropic-Ratelimit-Unified-5h-Utilization", "0.5")

	checker := &ClaudeCodeQuotaChecker{}

	quota, err := checker.parseResponse(headers)
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)
	require.Nil(t, quota.Limits[0].PeriodStart)
	require.Nil(t, quota.Limits[1].PeriodStart)
}

func TestCodexQuotaChecker_LimitPeriodUsesReportedWindowLength(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	body := []byte(`{
		"plan_type": "pro",
		"rate_limit": {
			"allowed": true,
			"primary_window": {
				"used_percent": 40,
				"reset_at": ` + strconv.FormatInt(resetAt.Unix(), 10) + `,
				"limit_window_seconds": 18000
			}
		}
	}`)

	checker := &CodexQuotaChecker{}

	quota, err := checker.parseResponse(body)
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.InDelta(t, 0.4, quota.Limits[0].UsageRatio, 1e-9)
	require.Equal(t, QuotaWindowPrimary, quota.Limits[0].Window)
	require.NotNil(t, quota.Limits[0].PeriodStart)
	require.Equal(t, resetAt.Add(-5*time.Hour).Unix(), quota.Limits[0].PeriodStart.Unix())
}

func TestCodexQuotaChecker_NoPeriodWithoutWindowLength(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	body := []byte(`{
		"plan_type": "pro",
		"rate_limit": {
			"allowed": true,
			"primary_window": {
				"used_percent": 40,
				"reset_at": ` + strconv.FormatInt(resetAt.Unix(), 10) + `
			}
		}
	}`)

	checker := &CodexQuotaChecker{}

	quota, err := checker.parseResponse(body)
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Nil(t, quota.Limits[0].PeriodStart)
}

func TestClineLimitStatuses_CarryWindowPeriods(t *testing.T) {
	t.Parallel()

	now := time.Now()
	reset := now.Add(time.Hour)
	start := reset.Add(-5 * time.Hour)
	ratio := 0.6

	limits := clineLimitStatuses([]clineWindow{
		{
			key:           "last5h",
			duration:      5 * time.Hour,
			usageRatio:    &ratio,
			nextResetAt:   &reset,
			windowStartAt: &start,
			state:         clineWindowStateActive,
			active:        true,
		},
	}, true)

	require.Len(t, limits, 1)
	require.Equal(t, QuotaWindow5h, limits[0].Window)
	require.NotNil(t, limits[0].PeriodStart)
	require.Equal(t, start, *limits[0].PeriodStart)
}

func TestOpenCodeGoQuotaChecker_AllWindowsCarryTheirPeriod(t *testing.T) {
	t.Parallel()

	rollingReset := time.Date(2026, 8, 14, 15, 0, 0, 0, time.UTC)
	weeklyReset := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	// A monthly cycle resetting on the 1st: the period started a calendar month
	// earlier, not 30 days earlier.
	monthlyReset := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	body := []byte(`{"usage":{
		"rolling":{"percent":12,"resetsAt":"` + rollingReset.Format(time.RFC3339) + `"},
		"weekly":{"percent":30,"resetsAt":"` + weeklyReset.Format(time.RFC3339) + `"},
		"monthly":{"percent":40,"resetsAt":"` + monthlyReset.Format(time.RFC3339) + `"}
	}}`)

	checker := &OpenCodeGoQuotaChecker{now: func() time.Time { return rollingReset.Add(-time.Hour) }}

	quota, err := checker.parseResponse(body)
	require.NoError(t, err)
	require.Len(t, quota.Limits, 3)

	require.Equal(t, QuotaWindow5h, quota.Limits[0].Window)
	require.NotNil(t, quota.Limits[0].PeriodStart)
	require.Equal(t, rollingReset.Add(-5*time.Hour), quota.Limits[0].PeriodStart.UTC())

	require.Equal(t, QuotaWindowWeekly, quota.Limits[1].Window)
	require.NotNil(t, quota.Limits[1].PeriodStart)
	require.Equal(t, weeklyReset.Add(-7*24*time.Hour), quota.Limits[1].PeriodStart.UTC())

	require.Equal(t, QuotaWindowMonthly, quota.Limits[2].Window)
	require.NotNil(t, quota.Limits[2].PeriodStart)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), quota.Limits[2].PeriodStart.UTC())
}

func TestApertisQuotaChecker_SubscriptionCycleCarriesReportedStart(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"object": "billing_credits",
		"is_subscriber": true,
		"subscription": {
			"plan_type": "lite",
			"status": "active",
			"cycle_quota_limit": 600,
			"cycle_quota_used": 300,
			"cycle_quota_remaining": 300,
			"cycle_start": "2026-03-16T10:02:35Z",
			"cycle_end": "2026-04-16T10:02:35Z",
			"payg_fallback_enabled": false
		}
	}`)

	checker := &ApertisQuotaChecker{}

	quota, err := checker.parseResponse(body)
	require.NoError(t, err)

	var cycle *QuotaLimitStatus

	for i := range quota.Limits {
		if quota.Limits[i].Type == QuotaLimitTypeSubscriptionCycle {
			cycle = &quota.Limits[i]
		}
	}

	require.NotNil(t, cycle)
	require.Equal(t, QuotaWindowCycle, cycle.Window)
	require.NotNil(t, cycle.PeriodStart)
	require.Equal(t, time.Date(2026, 3, 16, 10, 2, 35, 0, time.UTC), cycle.PeriodStart.UTC())
}

func TestGithubCopilotQuotaChecker_MonthlyPeriodStepsBackACalendarMonth(t *testing.T) {
	t.Parallel()

	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			// A reset on March 1 means the period opened on February 1, which a
			// fixed 30 day window would miss.
			body := `{
				"copilot_plan": "individual",
				"quota_reset_date_utc": "2026-03-01T00:00:00Z",
				"quota_snapshots": {"premium_interactions": {"percent_remaining": 40, "entitlement": 300, "remaining": 120}}
			}`

			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}),
	})

	checker := NewGithubCopilotQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Credentials: objects.ChannelCredentials{APIKey: "test-api-key"},
	})
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaWindowMonthly, quota.Limits[0].Window)
	require.NotNil(t, quota.Limits[0].PeriodStart)
	require.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), quota.Limits[0].PeriodStart.UTC())
}

func TestWaferQuotaChecker_UsesReportedBillingWindow(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"plan_tier": "pro",
		"window_start": "2026-08-01T00:00:00Z",
		"window_end": "2026-09-01T00:00:00Z",
		"included_request_limit": 1000,
		"remaining_included_requests": 400,
		"current_period_used_percent": 60
	}`)

	checker := &WaferQuotaChecker{}

	quota, err := checker.parseResponse(body)
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, QuotaWindowCycle, quota.Limits[0].Window)
	require.NotNil(t, quota.Limits[0].PeriodStart)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), quota.Limits[0].PeriodStart.UTC())
	require.InDelta(t, 0.6, quota.Limits[0].UsageRatio, 1e-9)
}
