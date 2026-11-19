// 职责: 覆盖 Realtime 四个价格码及价格完整性校验。
// 关联文件:
//
//	internal/objects/price.go
package objects

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func realtimePriceItems(rates map[PriceItemCode]string) []ModelPriceItem {
	items := make([]ModelPriceItem, 0, len(rates))

	for _, code := range RealtimePriceItemCodes() {
		rate, ok := rates[code]
		if !ok {
			continue
		}

		items = append(items, ModelPriceItem{
			ItemCode: code,
			Pricing: Pricing{
				Mode:         PricingModeUsagePerUnit,
				UsagePerUnit: decPtr(rate),
			},
		})
	}

	return items
}

func validRealtimeRates() map[PriceItemCode]string {
	return map[PriceItemCode]string{
		PriceItemCodeRealtimeInputText:   "1.10",
		PriceItemCodeRealtimeInputAudio:  "2.20",
		PriceItemCodeRealtimeOutputText:  "3.30",
		PriceItemCodeRealtimeOutputAudio: "4.40",
	}
}

func TestRealtimePriceItemCodes(t *testing.T) {
	codes := RealtimePriceItemCodes()
	require.Equal(t, []PriceItemCode{
		"realtime_input_text_tokens",
		"realtime_input_audio_tokens",
		"realtime_output_text_tokens",
		"realtime_output_audio_tokens",
	}, codes)

	for _, code := range codes {
		require.True(t, IsRealtimePriceItemCode(code))
	}

	require.False(t, IsRealtimePriceItemCode(PriceItemCodeUsage))
	require.False(t, IsRealtimePriceItemCode(PriceItemCodeCompletion))
}

func TestValidateRealtimePrice(t *testing.T) {
	t.Run("accepts all four positive items", func(t *testing.T) {
		price := ModelPrice{Items: realtimePriceItems(validRealtimeRates())}
		require.NoError(t, ValidateRealtimePrice(price))
	})

	t.Run("accepts unrelated legacy items alongside the four", func(t *testing.T) {
		items := append(realtimePriceItems(validRealtimeRates()), ModelPriceItem{
			ItemCode: PriceItemCodeUsage,
			Pricing:  Pricing{Mode: PricingModeUsagePerUnit, UsagePerUnit: decPtr("0.01")},
		})
		require.NoError(t, ValidateRealtimePrice(ModelPrice{Items: items}))
	})

	t.Run("rejects a missing realtime item", func(t *testing.T) {
		rates := validRealtimeRates()
		delete(rates, PriceItemCodeRealtimeOutputAudio)

		err := ValidateRealtimePrice(ModelPrice{Items: realtimePriceItems(rates)})
		require.Error(t, err)
		require.Contains(t, err.Error(), "realtime_output_audio_tokens")
	})

	t.Run("rejects duplicate realtime item", func(t *testing.T) {
		items := realtimePriceItems(validRealtimeRates())
		items = append(items, ModelPriceItem{
			ItemCode: PriceItemCodeRealtimeInputText,
			Pricing:  Pricing{Mode: PricingModeUsagePerUnit, UsagePerUnit: decPtr("1.10")},
		})

		err := ValidateRealtimePrice(ModelPrice{Items: items})
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate")
	})

	t.Run("rejects zero price", func(t *testing.T) {
		rates := validRealtimeRates()
		rates[PriceItemCodeRealtimeInputAudio] = "0"

		err := ValidateRealtimePrice(ModelPrice{Items: realtimePriceItems(rates)})
		require.Error(t, err)
		require.Contains(t, err.Error(), "positive")
	})

	t.Run("rejects negative price", func(t *testing.T) {
		rates := validRealtimeRates()
		rates[PriceItemCodeRealtimeOutputText] = "-1"

		err := ValidateRealtimePrice(ModelPrice{Items: realtimePriceItems(rates)})
		require.Error(t, err)
		require.Contains(t, err.Error(), "positive")
	})

	t.Run("rejects illegal pricing", func(t *testing.T) {
		items := realtimePriceItems(validRealtimeRates())
		items[0].Pricing = Pricing{Mode: PricingModeUsagePerUnit}

		err := ValidateRealtimePrice(ModelPrice{Items: items})
		require.Error(t, err)
		require.Contains(t, err.Error(), "usagePerUnit is required")
	})

	t.Run("rejects invalid schedule", func(t *testing.T) {
		price := ModelPrice{
			Items: realtimePriceItems(validRealtimeRates()),
			Schedule: &PriceSchedule{
				Timezone: "Invalid/Zone",
				Overrides: []PriceOverride{{
					Name:     "bad",
					Priority: 1,
					When:     OverrideWhen{DailyTime: &DailyTimeRange{Start: "00:00", End: "23:59"}},
					Items:    realtimePriceItems(validRealtimeRates()),
				}},
			},
		}

		err := ValidateRealtimePrice(price)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid timezone")
	})

	t.Run("rejects a schedule override missing a realtime item", func(t *testing.T) {
		rates := validRealtimeRates()
		delete(rates, PriceItemCodeRealtimeInputText)

		price := ModelPrice{
			Items: realtimePriceItems(validRealtimeRates()),
			Schedule: &PriceSchedule{
				Timezone: "UTC",
				Overrides: []PriceOverride{{
					Name:     "partial",
					Priority: 1,
					When:     OverrideWhen{DailyTime: &DailyTimeRange{Start: "00:00", End: "23:59"}},
					Items:    realtimePriceItems(rates),
				}},
			},
		}

		err := ValidateRealtimePrice(price)
		require.Error(t, err)
		require.Contains(t, err.Error(), "realtime_input_text_tokens")
	})

	t.Run("rejects a non-positive tier", func(t *testing.T) {
		items := realtimePriceItems(validRealtimeRates())
		items[1].Pricing = Pricing{
			Mode: PricingModeTiered,
			UsageTiered: &TieredPricing{Tiers: []PriceTier{
				{UpTo: nil, PricePerUnit: decimal.Zero},
			}},
		}

		err := ValidateRealtimePrice(ModelPrice{Items: items})
		require.Error(t, err)
		require.Contains(t, err.Error(), "positive")
	})
}
