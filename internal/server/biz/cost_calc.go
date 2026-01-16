package biz

import (
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
		Mode:     pricing.Mode,
		Quantity: quantity,
	}

	switch pricing.Mode {
	case objects.PricingModeFlatFee:
		if pricing.FlatFee != nil {
			item.FlatFee = pricing.FlatFee
			item.Subtotal = *pricing.FlatFee

			return item, *pricing.FlatFee
		}

		return item, decimal.Zero
	case objects.PricingModeUsagePerUnit:
		if pricing.UsagePerUnit != nil {
			item.UnitPrice = pricing.UsagePerUnit
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
						tierUnits = max64(quantity-prevUpTo, 0)
					} else {
						tierUnits = max64(*tier.UpTo-prevUpTo, 0)
					}
				} else {
					tierUnits = max64(quantity-prevUpTo, 0)
				}

				if tierUnits <= 0 {
					if tier.UpTo != nil && quantity <= *tier.UpTo {
						// no more units beyond current quantity
						break
					}

					prevUpTo = getUpToOrZero(tier.UpTo)

					continue
				}

				sub := tier.PricePerUnit.Mul(unitsInMillionTokens(tierUnits))
				total = total.Add(sub)
				item.TierBreakdown = append(item.TierBreakdown, objects.TierCost{
					UpTo:         tier.UpTo,
					Units:        tierUnits,
					PricePerUnit: tier.PricePerUnit,
					Subtotal:     sub,
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

func max64(a, b int64) int64 {
	if a > b {
		return a
	}

	return b
}

// ComputeUsageCost calculates total cost and cost items breakdown for the given usage and model price.
func ComputeUsageCost(usage *llm.Usage, price objects.ModelPrice) ([]objects.CostItem, decimal.Decimal) {
	var items []objects.CostItem

	total := decimal.Zero

	for _, it := range price.Items {
		var quantity int64

		switch it.ItemCode {
		case objects.PriceItemCodeUsage:
			quantity = usage.PromptTokens
		case objects.PriceItemCodeCompletion:
			quantity = usage.CompletionTokens
		case objects.PriceItemCodePromptCachedToken:
			if usage.PromptTokensDetails != nil {
				quantity = usage.PromptTokensDetails.CachedTokens
			}
		case objects.PriceItemCodeWriteCachedTokens:
			if usage.PromptTokensDetails != nil {
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
