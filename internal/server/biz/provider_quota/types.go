package provider_quota

import (
	"context"
	"errors"
	"time"

	"github.com/looplj/axonhub/internal/ent"
)

// QuotaChecker checks quota status for a provider.
type QuotaChecker interface {
	// CheckQuota makes a minimal API request to get quota information and returns parsed quota data
	CheckQuota(ctx context.Context, channel *ent.Channel) (QuotaData, error)
	// SupportsChannel returns true if this checker supports the channel
	SupportsChannel(channel *ent.Channel) bool
}

// ErrResetUnsupported is returned when a provider checker does not implement Resetter.
var ErrResetUnsupported = errors.New("provider quota reset is not supported")

// Reset is a provider-agnostic quota reset that can be consumed.
type Reset struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	Type      string     `json:"type,omitempty"`
	GrantedAt *time.Time `json:"grantedAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Title     string     `json:"title,omitempty"`
}

// ResetList describes a provider's optional reset capability and current resets.
type ResetList struct {
	Supported bool    `json:"supported"`
	Resets    []Reset `json:"resets"`
	Error     string  `json:"error,omitempty"`
}

// Resetter is an optional capability implemented by quota providers that can
// list and consume provider-managed quota resets.
type Resetter interface {
	ListResets(ctx context.Context, channel *ent.Channel) (ResetList, error)
	Reset(ctx context.Context, channel *ent.Channel) error
}

type QuotaLimitType string

const (
	QuotaLimitTypeImage             QuotaLimitType = "image"
	QuotaLimitTypeToken             QuotaLimitType = "token"
	QuotaLimitTypeSubscriptionCycle QuotaLimitType = "subscription_cycle"
)

// ApertisDefaultBaseURL is the default base URL for the Apertis API.
const ApertisDefaultBaseURL = "https://api.apertis.ai"

// ZenmuxDefaultBaseURL is the default base URL for the ZenMux API.
const ZenmuxDefaultBaseURL = "https://zenmux.ai"

type QuotaLimitStatus struct {
	Type        QuotaLimitType `json:"type"`
	Status      string         `json:"status"`
	UsageRatio  float64        `json:"usage_ratio"`
	Ready       bool           `json:"ready"`
	NextResetAt *time.Time     `json:"next_reset_at"`

	// Window identifies the limit window this status describes ("5h", "7d",
	// "weekly", ...). Providers report several limits of the same
	// QuotaLimitType, so this is what tells them apart in the UI.
	Window string `json:"window,omitempty"`

	// PeriodStart is the beginning of the window UsageRatio covers. Checkers
	// fill it whenever the window length is known (either reported by the
	// provider or fixed by the plan); it is what makes the usage-log cost
	// aggregation below possible.
	PeriodStart *time.Time `json:"period_start,omitempty"`

	// PeriodCost is the cost accumulated in [PeriodStart, now) on this channel
	// according to AxonHub usage logs. Filled by the quota service, not by the
	// checkers.
	PeriodCost *float64 `json:"period_cost,omitempty"`

	// PeriodQuota is the estimated total money quota of the period, derived
	// from PeriodCost and UsageRatio. See EstimatePeriodQuota.
	PeriodQuota *float64 `json:"period_quota,omitempty"`
}

// QuotaData is the unified quota data structure.
type QuotaData struct {
	Status       string             `json:"status"` // available, warning, exhausted, unknown
	ProviderType string             `json:"provider_type"`
	RawData      map[string]any     `json:"raw_data"`
	NextResetAt  *time.Time         `json:"next_reset_at"` // Next quota reset timestamp
	Ready        bool               `json:"ready"`         // True if status is available or warning
	Limits       []QuotaLimitStatus `json:"limits"`
	Resets       *ResetList         `json:"resets,omitempty"`
}

// WarningThresholdRatio is the usage ratio at which a channel transitions to "warning" status.
const WarningThresholdRatio = 0.8

// Well-known limit window identifiers. Providers name their windows
// differently; these are the normalized labels the UI renders.
const (
	QuotaWindow5h      = "5h"
	QuotaWindow7d      = "7d"
	QuotaWindow30d     = "30d"
	QuotaWindowDaily   = "daily"
	QuotaWindowWeekly  = "weekly"
	QuotaWindowMonthly = "monthly"
	QuotaWindowPrimary = "primary"
	QuotaWindowCycle   = "cycle"
)

// MinPeriodQuotaUsageRatio is the smallest usage ratio that yields a period
// quota estimate. Dividing the period cost by a tiny ratio amplifies both the
// pricing error and any usage that did not go through AxonHub into an absurd
// number, so below this threshold no estimate is reported at all.
const MinPeriodQuotaUsageRatio = 0.05

// EstimatePeriodQuota derives the total money quota of a limit period from the
// cost already spent in it: a period that cost `periodCost` while the provider
// reports `usageRatio` of the quota consumed is worth periodCost/usageRatio in
// total. It reports false when the inputs cannot support an estimate.
func EstimatePeriodQuota(periodCost float64, usageRatio float64) (float64, bool) {
	if periodCost <= 0 || usageRatio < MinPeriodQuotaUsageRatio {
		return 0, false
	}

	return periodCost / usageRatio, true
}

// FillPeriodQuota recomputes PeriodQuota from PeriodCost and UsageRatio,
// clearing it when no estimate can be made.
func (l *QuotaLimitStatus) FillPeriodQuota() {
	l.PeriodQuota = nil

	if l.PeriodCost == nil {
		return
	}

	if quota, ok := EstimatePeriodQuota(*l.PeriodCost, l.UsageRatio); ok {
		l.PeriodQuota = &quota
	}
}

// FillPeriodQuotas refreshes the period quota estimate of every limit.
func (d *QuotaData) FillPeriodQuotas() {
	for i := range d.Limits {
		d.Limits[i].FillPeriodQuota()
	}
}

// NormalizeQuotaWindowLabel maps a window length onto a well-known window
// label, returning "" when the length matches none of them.
func NormalizeQuotaWindowLabel(window time.Duration) string {
	switch window {
	case 5 * time.Hour:
		return QuotaWindow5h
	case 24 * time.Hour:
		return QuotaWindowDaily
	case 7 * 24 * time.Hour:
		return QuotaWindow7d
	case 30 * 24 * time.Hour:
		return QuotaWindow30d
	default:
		return ""
	}
}

// PeriodStartFromReset returns the start of the window that ends at
// nextResetAt and lasts for window. It returns nil when either input is
// missing, which is the signal that no period cost can be aggregated.
func PeriodStartFromReset(nextResetAt *time.Time, window time.Duration) *time.Time {
	if nextResetAt == nil || window <= 0 {
		return nil
	}

	start := nextResetAt.Add(-window)

	return &start
}

// PeriodStartFromMonthlyReset returns the start of the month-long window that
// ends at nextResetAt. The reset day is clamped to the previous month's last
// day because time.Time.AddDate normalizes invalid dates forward.
func PeriodStartFromMonthlyReset(nextResetAt *time.Time) *time.Time {
	if nextResetAt == nil {
		return nil
	}

	year, month, day := nextResetAt.Date()
	lastDayOfPreviousMonth := time.Date(year, month, 0, 0, 0, 0, 0, nextResetAt.Location()).Day()
	if day > lastDayOfPreviousMonth {
		day = lastDayOfPreviousMonth
	}

	start := time.Date(
		year,
		month-1,
		day,
		nextResetAt.Hour(),
		nextResetAt.Minute(),
		nextResetAt.Second(),
		nextResetAt.Nanosecond(),
		nextResetAt.Location(),
	)

	return &start
}

func RequestModality(isImageRequest bool) QuotaLimitType {
	if isImageRequest {
		return QuotaLimitTypeImage
	}
	return QuotaLimitTypeToken
}

func IsReadyStatus(status string) bool {
	return status == "available" || status == "warning"
}

func NewTokenLimitStatus(status string, usageRatio float64, nextResetAt *time.Time) QuotaLimitStatus {
	return QuotaLimitStatus{
		Type:        QuotaLimitTypeToken,
		Status:      status,
		UsageRatio:  usageRatio,
		Ready:       IsReadyStatus(status),
		NextResetAt: nextResetAt,
	}
}

// WithWindow labels a limit with its window identifier and derives the period
// start from the limit's reset time. A zero window length only sets the label,
// which is the right behavior for windows whose length the provider does not
// report.
func (l QuotaLimitStatus) WithWindow(name string, window time.Duration) QuotaLimitStatus {
	l.Window = name
	l.PeriodStart = PeriodStartFromReset(l.NextResetAt, window)

	return l
}
