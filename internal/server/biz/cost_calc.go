package biz

import (
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func unitsInMillionTokens(units int64) decimal.Decimal {
	if units <= 0 {
		return decimal.Zero
	}

	return decimal.NewFromInt(units).Div(decimal.NewFromInt(1_000_000))
}

func computeItemSubtotal(quantity int64, pricing objects.Pricing) (objects.CostItem, decimal.Decimal) {
	item := objects.CostItem{
		Quantity: quantity,
	}

	switch pricing.Mode {
	case objects.PricingModeFlatFee:
		if pricing.FlatFee != nil {
			item.Subtotal = *pricing.FlatFee

			return item, *pricing.FlatFee
		}

		return item, decimal.Zero
	case objects.PricingModeUsagePerUnit:
		if pricing.UsagePerUnit != nil {
			sub := pricing.UsagePerUnit.Mul(unitsInMillionTokens(quantity))
			item.Subtotal = sub

			return item, sub
		}

		return item, decimal.Zero
	case objects.PricingModeTiered:
		if pricing.UsageTiered != nil {
			var (
				total    decimal.Decimal
				prevUpTo int64
			)

			for _, tier := range pricing.UsageTiered.Tiers {
				var tierUnits int64

				if tier.UpTo != nil {
					if quantity <= *tier.UpTo {
						tierUnits = max(quantity-prevUpTo, 0)
					} else {
						tierUnits = max(*tier.UpTo-prevUpTo, 0)
					}
				} else {
					tierUnits = max(quantity-prevUpTo, 0)
				}

				if tierUnits <= 0 {
					if tier.UpTo != nil && quantity <= *tier.UpTo {
						break
					}

					prevUpTo = getUpToOrZero(tier.UpTo)

					continue
				}

				sub := tier.PricePerUnit.Mul(unitsInMillionTokens(tierUnits))
				total = total.Add(sub)
				item.TierBreakdown = append(item.TierBreakdown, objects.TierCost{
					UpTo:     tier.UpTo,
					Units:    tierUnits,
					Subtotal: sub,
				})
				prevUpTo = getUpToOrZero(tier.UpTo)

				if tier.UpTo != nil && quantity <= *tier.UpTo {
					break
				}
			}

			item.Subtotal = total

			return item, total
		}

		return item, decimal.Zero
	case objects.PricingModeVolume:
		if pricing.UsageTiered != nil {
			matchedIdx := -1

			for i, tier := range pricing.UsageTiered.Tiers {
				if tier.UpTo != nil && quantity <= *tier.UpTo {
					matchedIdx = i
					break
				}

				if tier.UpTo == nil {
					matchedIdx = i
					break
				}
			}

			if matchedIdx >= 0 {
				matchedTier := pricing.UsageTiered.Tiers[matchedIdx]
				sub := matchedTier.PricePerUnit.Mul(unitsInMillionTokens(quantity))
				item.Subtotal = sub
				item.TierBreakdown = []objects.TierCost{
					{
						UpTo:     matchedTier.UpTo,
						Units:    quantity,
						Subtotal: sub,
					},
				}

				return item, sub
			}

			return item, decimal.Zero
		}

		return item, decimal.Zero
	default:
		return item, decimal.Zero
	}
}

func getUpToOrZero(v *int64) int64 {
	if v == nil {
		return 0
	}

	return *v
}

// ComputeUsageCost calculates total cost and cost items breakdown for the given usage and model price.
// The now parameter is used for time-based schedule matching.
func ComputeUsageCost(usage *llm.Usage, price objects.ModelPrice, now time.Time) ([]objects.CostItem, decimal.Decimal) {
	return computeUsageCostWithItems(usage, effectivePriceItems(price, now))
}

// ComputeRealtimeUsageCost calculates a Realtime response using only its
// modality-specific price items. Ordinary usage billing always uses
// ComputeUsageCost, even when a model price also contains Realtime rates.
func ComputeRealtimeUsageCost(usage *llm.Usage, price objects.ModelPrice, now time.Time) ([]objects.CostItem, decimal.Decimal) {
	return computeRealtimeUsageCost(usage, effectivePriceItems(price, now))
}

func effectivePriceItems(price objects.ModelPrice, now time.Time) []objects.ModelPriceItem {
	if price.Schedule != nil {
		if override := findMatchingOverride(now, price.Schedule); override != nil {
			return override.Items
		}
	}

	return price.Items
}

// computeRealtimeUsageCost bills a realtime session by input/output modality.
// The four realtime codes are computed independently from the historical
// prompt_tokens/completion_tokens codes, which are untouched.
func computeRealtimeUsageCost(usage *llm.Usage, priceItems []objects.ModelPriceItem) ([]objects.CostItem, decimal.Decimal) {
	if usage == nil {
		return nil, decimal.Zero
	}

	var (
		items []objects.CostItem
		total decimal.Decimal
	)

	// Saturating text total = reported total - audio portion, clamped to the
	// reported total so text + audio can never exceed it. Missing details and
	// provider bugs are absorbed here rather than rejecting a confirmed usage.
	inputAudio := int64(0)
	outputAudio := int64(0)

	if usage.PromptTokensDetails != nil {
		inputAudio = realtimeAudioTokens(usage.PromptTokens, usage.PromptTokensDetails.AudioTokens)
	}

	if usage.CompletionTokensDetails != nil {
		outputAudio = realtimeAudioTokens(usage.CompletionTokens, usage.CompletionTokensDetails.AudioTokens)
	}

	inputText := max(usage.PromptTokens-inputAudio, 0)
	outputText := max(usage.CompletionTokens-outputAudio, 0)

	quantities := map[objects.PriceItemCode]int64{
		objects.PriceItemCodeRealtimeInputText:   inputText,
		objects.PriceItemCodeRealtimeInputAudio:  inputAudio,
		objects.PriceItemCodeRealtimeOutputText:  outputText,
		objects.PriceItemCodeRealtimeOutputAudio: outputAudio,
	}

	if details := usage.Realtime; details != nil {
		quantities[objects.PriceItemCodeRealtimeInputText] = max(details.InputTextTokens-details.CachedInputTextTokens, 0)
		quantities[objects.PriceItemCodeRealtimeInputAudio] = max(details.InputAudioTokens-details.CachedInputAudioTokens, 0)
		quantities[objects.PriceItemCodeRealtimeInputImage] = max(details.InputImageTokens-details.CachedInputImageTokens, 0)
		quantities[objects.PriceItemCodeRealtimeCachedInputText] = details.CachedInputTextTokens
		quantities[objects.PriceItemCodeRealtimeCachedInputAudio] = details.CachedInputAudioTokens
		quantities[objects.PriceItemCodeRealtimeCachedInputImage] = details.CachedInputImageTokens
		quantities[objects.PriceItemCodeRealtimeOutputText] = max(details.OutputTextTokens-details.UnbilledOutputTextTokens, 0)
		quantities[objects.PriceItemCodeRealtimeOutputAudio] = details.OutputAudioTokens
	}
	for _, it := range priceItems {
		quantity, ok := quantities[it.ItemCode]
		if !ok {
			// Real-time calls ignore legacy prompt/completion codes.
			continue
		}

		item, sub := objects.CostItem{Quantity: quantity}, decimal.Zero
		if quantity > 0 {
			item, sub = computeItemSubtotal(quantity, it.Pricing)
		}
		item.ItemCode = it.ItemCode
		items = append(items, item)
		total = total.Add(sub)
	}

	return items, total
}

// realtimeAudioTokens clamps the audio breakdown into [0, total]. A negative
// breakdown or one larger than the reported total would otherwise let the text
// quantity go negative or double-count audio, so the audio portion is bounded
// by the reported total.
func realtimeAudioTokens(total, audio int64) int64 {
	if audio <= 0 {
		return 0
	}

	return min(audio, total)
}

func computeUsageCostWithItems(usage *llm.Usage, priceItems []objects.ModelPriceItem) ([]objects.CostItem, decimal.Decimal) {
	var items []objects.CostItem

	total := decimal.Zero

	for _, it := range priceItems {
		if objects.IsRealtimePriceItemCode(it.ItemCode) {
			continue
		}

		var quantity int64

		switch it.ItemCode {
		case objects.PriceItemCodeUsage:
			// Exclude cached tokens from input token cost calculation
			// PromptTokens includes all tokens, so we subtract:
			// - CachedTokens (read from cache)
			// - WriteCachedTokens (write to cache)
			// These are billed separately at different rates
			quantity = usage.PromptTokens
			if usage.PromptTokensDetails != nil {
				quantity -= usage.PromptTokensDetails.CachedTokens
				quantity -= usage.PromptTokensDetails.WriteCachedTokens
			}
			// Clamp to non-negative to handle provider bugs or partial data
			if quantity < 0 {
				quantity = 0
			}
		case objects.PriceItemCodeCompletion:
			quantity = usage.CompletionTokens
		case objects.PriceItemCodePromptCachedToken:
			if usage.PromptTokensDetails != nil {
				quantity = usage.PromptTokensDetails.CachedTokens
			}
		case objects.PriceItemCodeWriteCachedTokens:
			// Handle write cached tokens with variant support
			if usage.PromptTokensDetails != nil {
				// Check if we have 5m/1h cache variants to calculate separately
				if usage.PromptTokensDetails.WriteCached5MinTokens > 0 || usage.PromptTokensDetails.WriteCached1HourTokens > 0 {
					// Process 5m variant if present
					if usage.PromptTokensDetails.WriteCached5MinTokens > 0 {
						pricing := it.FindPromptWriteCacheVariantPricing(objects.PromptWriteCacheVariantCode5Min)
						item, sub := computeItemSubtotal(usage.PromptTokensDetails.WriteCached5MinTokens, pricing)
						item.ItemCode = it.ItemCode
						item.PromptWriteCacheVariantCode = objects.PromptWriteCacheVariantCode5Min
						items = append(items, item)
						total = total.Add(sub)
					}

					// Process 1h variant if present
					if usage.PromptTokensDetails.WriteCached1HourTokens > 0 {
						pricing := it.FindPromptWriteCacheVariantPricing(objects.PromptWriteCacheVariantCode1Hour)
						item, sub := computeItemSubtotal(usage.PromptTokensDetails.WriteCached1HourTokens, pricing)
						item.ItemCode = it.ItemCode
						item.PromptWriteCacheVariantCode = objects.PromptWriteCacheVariantCode1Hour
						items = append(items, item)
						total = total.Add(sub)
					}

					// Skip the shared pricing calculation
					continue
				}

				// Fallback to shared WriteCachedTokens if no variants present
				quantity = usage.PromptTokensDetails.WriteCachedTokens
			}
		default:
			quantity = 0
		}

		item, sub := computeItemSubtotal(quantity, it.Pricing)
		item.ItemCode = it.ItemCode
		items = append(items, item)
		total = total.Add(sub)
	}

	return items, total
}

// findMatchingOverride finds the highest priority override that matches the current time.
// Returns nil if no override matches.
func findMatchingOverride(now time.Time, schedule *objects.PriceSchedule) *objects.PriceOverride {
	if schedule == nil || len(schedule.Overrides) == 0 {
		return nil
	}

	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		loc = time.UTC
	}

	localNow := now.In(loc)

	var matched *objects.PriceOverride

	for i := range schedule.Overrides {
		override := &schedule.Overrides[i]
		if matchesOverride(localNow, &override.When) {
			if matched == nil || override.Priority < matched.Priority {
				matched = override
			}
		}
	}

	return matched
}

// matchesOverride checks if the given time matches all conditions in the override.
func matchesOverride(now time.Time, when *objects.OverrideWhen) bool {
	if when == nil {
		return false
	}

	// Check daily time range if specified
	if when.DailyTime != nil {
		if !matchesDailyTime(now, when.DailyTime) {
			return false
		}
	}

	// Check weekdays if specified
	if len(when.Weekdays) > 0 {
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}

		if !slices.Contains(when.Weekdays, weekday) {
			return false
		}
	}

	// Check date range if specified
	if when.DateRange != nil {
		today := now.Format("2006-01-02")
		if today < when.DateRange.Start || today > when.DateRange.End {
			return false
		}
	}

	return true
}

// matchesDailyTime checks if the current time falls within a daily time range.
// Supports cross-midnight ranges (e.g., "22:00"-"06:00").
// Start and End are in "HH:mm" format.
func matchesDailyTime(now time.Time, dt *objects.DailyTimeRange) bool {
	if dt == nil {
		return false
	}

	current := now.Hour()*60 + now.Minute()
	start := parseHHMM(dt.Start)
	end := parseHHMM(dt.End)

	if start > end {
		// Cross-midnight range: e.g., 22:00-06:00
		return current >= start || current < end
	}

	return current >= start && current < end
}

// parseHHMM parses a "HH:mm" string into minutes since midnight.
func parseHHMM(s string) int {
	if len(s) != 5 || s[2] != ':' {
		return 0
	}

	hour := int(s[0]-'0')*10 + int(s[1]-'0')
	minute := int(s[3]-'0')*10 + int(s[4]-'0')

	return hour*60 + minute
}
