package objects

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPriceSchedule_Validate(t *testing.T) {
	t.Run("valid schedule", func(t *testing.T) {
		s := &PriceSchedule{
			Timezone: "Asia/Shanghai",
			Overrides: []PriceOverride{
				{
					Name:     "Night Discount",
					Priority: 1,
					When: OverrideWhen{
						DailyTime: &DailyTimeRange{
							Start: "18:00",
							End:   "09:00",
						},
					},
					Items: []ModelPriceItem{
						{
							ItemCode: PriceItemCodeUsage,
							Pricing: Pricing{
								Mode:         PricingModeUsagePerUnit,
								UsagePerUnit: decPtr("0.01"),
							},
						},
					},
				},
			},
		}
		require.NoError(t, s.Validate())
	})

	t.Run("empty timezone", func(t *testing.T) {
		s := &PriceSchedule{
			Timezone: "",
			Overrides: []PriceOverride{
				{
					Name:     "Test",
					Priority: 1,
					When:     OverrideWhen{DailyTime: &DailyTimeRange{Start: "00:00", End: "23:59"}},
					Items: []ModelPriceItem{
						{
							ItemCode: PriceItemCodeUsage,
							Pricing:  Pricing{Mode: PricingModeUsagePerUnit, UsagePerUnit: decPtr("0.01")},
						},
					},
				},
			},
		}
		require.Error(t, s.Validate())
		require.Contains(t, s.Validate().Error(), "timezone is required")
	})

	t.Run("invalid timezone", func(t *testing.T) {
		s := &PriceSchedule{
			Timezone: "Invalid/Timezone",
			Overrides: []PriceOverride{
				{
					Name:     "Test",
					Priority: 1,
					When:     OverrideWhen{DailyTime: &DailyTimeRange{Start: "00:00", End: "23:59"}},
					Items: []ModelPriceItem{
						{
							ItemCode: PriceItemCodeUsage,
							Pricing:  Pricing{Mode: PricingModeUsagePerUnit, UsagePerUnit: decPtr("0.01")},
						},
					},
				},
			},
		}
		require.Error(t, s.Validate())
		require.Contains(t, s.Validate().Error(), "invalid timezone")
	})

	t.Run("empty overrides", func(t *testing.T) {
		s := &PriceSchedule{
			Timezone:  "UTC",
			Overrides: []PriceOverride{},
		}
		require.Error(t, s.Validate())
		require.Contains(t, s.Validate().Error(), "overrides is required")
	})
}

func TestOverrideWhen_Validate(t *testing.T) {
	t.Run("valid dailyTime", func(t *testing.T) {
		w := &OverrideWhen{DailyTime: &DailyTimeRange{Start: "09:00", End: "18:00"}}
		require.NoError(t, w.Validate())
	})

	t.Run("valid weekdays", func(t *testing.T) {
		w := &OverrideWhen{Weekdays: []int{1, 2, 3, 4, 5}}
		require.NoError(t, w.Validate())
	})

	t.Run("valid dateRange", func(t *testing.T) {
		w := &OverrideWhen{
			DateRange: &DateRange{Start: "2026-01-01", End: "2026-12-31"},
		}
		require.NoError(t, w.Validate())
	})

	t.Run("no conditions", func(t *testing.T) {
		w := &OverrideWhen{}
		require.Error(t, w.Validate())
		require.Contains(t, w.Validate().Error(), "at least one condition")
	})

	t.Run("invalid dailyTime format", func(t *testing.T) {
		w := &OverrideWhen{DailyTime: &DailyTimeRange{Start: "25:00", End: "18:00"}}
		require.Error(t, w.Validate())
	})

	t.Run("invalid dailyTime bad format", func(t *testing.T) {
		w := &OverrideWhen{DailyTime: &DailyTimeRange{Start: "abc", End: "18:00"}}
		require.Error(t, w.Validate())
	})

	t.Run("invalid weekday", func(t *testing.T) {
		w := &OverrideWhen{Weekdays: []int{0, 8}}
		require.Error(t, w.Validate())
	})

	t.Run("invalid dateRange start", func(t *testing.T) {
		w := &OverrideWhen{
			DateRange: &DateRange{Start: "invalid", End: "2026-12-31"},
		}
		require.Error(t, w.Validate())
	})

	t.Run("invalid dateRange end before start", func(t *testing.T) {
		w := &OverrideWhen{
			DateRange: &DateRange{Start: "2026-12-31", End: "2026-01-01"},
		}
		require.Error(t, w.Validate())
	})

	t.Run("multiple conditions", func(t *testing.T) {
		w := &OverrideWhen{
			DailyTime: &DailyTimeRange{Start: "18:00", End: "09:00"},
			Weekdays:  []int{6, 7},
		}
		require.NoError(t, w.Validate())
	})
}

func TestModelPrice_Validate_WithSchedule(t *testing.T) {
	t.Run("nil schedule is valid", func(t *testing.T) {
		p := &ModelPrice{
			Items: []ModelPriceItem{
				{
					ItemCode: PriceItemCodeUsage,
					Pricing:  Pricing{Mode: PricingModeUsagePerUnit, UsagePerUnit: decPtr("0.01")},
				},
			},
		}
		require.NoError(t, p.Validate())
	})

	t.Run("valid schedule", func(t *testing.T) {
		p := &ModelPrice{
			Items: []ModelPriceItem{
				{
					ItemCode: PriceItemCodeUsage,
					Pricing:  Pricing{Mode: PricingModeUsagePerUnit, UsagePerUnit: decPtr("0.01")},
				},
			},
			Schedule: &PriceSchedule{
				Timezone: "UTC",
				Overrides: []PriceOverride{
					{
						Name:     "Test",
						Priority: 1,
						When:     OverrideWhen{DailyTime: &DailyTimeRange{Start: "00:00", End: "23:59"}},
						Items: []ModelPriceItem{
							{
								ItemCode: PriceItemCodeUsage,
								Pricing:  Pricing{Mode: PricingModeUsagePerUnit, UsagePerUnit: decPtr("0.005")},
							},
						},
					},
				},
			},
		}
		require.NoError(t, p.Validate())
	})
}

func TestModelPrice_Equals_WithSchedule(t *testing.T) {
	t.Run("both nil schedules", func(t *testing.T) {
		a := ModelPrice{Items: []ModelPriceItem{{ItemCode: PriceItemCodeUsage}}}
		b := ModelPrice{Items: []ModelPriceItem{{ItemCode: PriceItemCodeUsage}}}
		require.True(t, a.Equals(b))
	})

	t.Run("one nil one non-nil", func(t *testing.T) {
		a := ModelPrice{Items: []ModelPriceItem{{ItemCode: PriceItemCodeUsage}}}
		b := ModelPrice{
			Items: []ModelPriceItem{{ItemCode: PriceItemCodeUsage}},
			Schedule: &PriceSchedule{
				Timezone:  "UTC",
				Overrides: []PriceOverride{},
			},
		}
		require.False(t, a.Equals(b))
	})

	t.Run("equal schedules", func(t *testing.T) {
		schedule := &PriceSchedule{
			Timezone: "UTC",
			Overrides: []PriceOverride{
				{
					Name:     "Test",
					Priority: 1,
					When:     OverrideWhen{DailyTime: &DailyTimeRange{Start: "00:00", End: "23:59"}},
					Items:    []ModelPriceItem{{ItemCode: PriceItemCodeUsage}},
				},
			},
		}
		a := ModelPrice{Items: []ModelPriceItem{{ItemCode: PriceItemCodeUsage}}, Schedule: schedule}
		b := ModelPrice{Items: []ModelPriceItem{{ItemCode: PriceItemCodeUsage}}, Schedule: schedule}
		require.True(t, a.Equals(b))
	})
}

func decPtr(s string) *decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return &d
}
