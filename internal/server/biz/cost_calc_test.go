package biz

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestComputeUsageCost_WithCachedTokens(t *testing.T) {
	// Test that cached tokens are excluded from input token cost calculation
	usage := &llm.Usage{
		PromptTokens:     1000, // Includes 300 cached tokens
		CompletionTokens: 500,
		TotalTokens:      1500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens: 300, // Read from cache
		},
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"), // $0.03 per 1M tokens
				},
			},
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.06"), // $0.06 per 1M tokens
				},
			},
			{
				ItemCode: objects.PriceItemCodePromptCachedToken,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.015"), // $0.015 per 1M tokens (50% discount)
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	// Expected cost:
	// - Input tokens (billable): (700 / 1_000_000) * 0.03 = 0.000021
	// - Cached tokens: (300 / 1_000_000) * 0.015 = 0.0000045
	// - Completion tokens: (500 / 1_000_000) * 0.06 = 0.00003
	// Total: 0.000021 + 0.0000045 + 0.00003 = 0.0000555
	expectedTotal := 0.0000555
	require.InDelta(t, expectedTotal, total.InexactFloat64(), 0.0000001)

	// Verify we have 3 cost items
	require.Len(t, items, 3)

	// Find each cost item
	var inputItem, cachedItem, completionItem *objects.CostItem

	for i := range items {
		switch items[i].ItemCode {
		case objects.PriceItemCodeUsage:
			inputItem = &items[i]
		case objects.PriceItemCodePromptCachedToken:
			cachedItem = &items[i]
		case objects.PriceItemCodeCompletion:
			completionItem = &items[i]
		}
	}

	require.NotNil(t, inputItem, "input cost item should exist")
	require.NotNil(t, cachedItem, "cached cost item should exist")
	require.NotNil(t, completionItem, "completion cost item should exist")

	// Verify input tokens quantity excludes cached tokens
	require.Equal(t, int64(700), inputItem.Quantity, "input quantity should be 700 (1000 - 300 cached)")
	require.InDelta(t, 0.000021, inputItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify cached tokens quantity
	require.Equal(t, int64(300), cachedItem.Quantity, "cached quantity should be 300")
	require.InDelta(t, 0.0000045, cachedItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify completion tokens quantity
	require.Equal(t, int64(500), completionItem.Quantity, "completion quantity should be 500")
	require.InDelta(t, 0.00003, completionItem.Subtotal.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_WithoutCachedTokens(t *testing.T) {
	// Test that when there are no cached tokens, all prompt tokens are billable
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		// No PromptTokensDetails, so no cached tokens
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.06"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	// Expected cost:
	// - Input tokens: (1000 / 1_000_000) * 0.03 = 0.00003
	// - Completion tokens: (500 / 1_000_000) * 0.06 = 0.00003
	// Total: 0.00006
	expectedTotal := 0.00006
	require.InDelta(t, expectedTotal, total.InexactFloat64(), 0.0000001)

	require.Len(t, items, 2)

	// Verify input tokens use full prompt tokens
	var inputItem *objects.CostItem

	for i := range items {
		if items[i].ItemCode == objects.PriceItemCodeUsage {
			inputItem = &items[i]
			break
		}
	}

	require.NotNil(t, inputItem)
	require.Equal(t, int64(1000), inputItem.Quantity, "input quantity should be 1000 when no cached tokens")
	require.InDelta(t, 0.00003, inputItem.Subtotal.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_WithZeroCachedTokens(t *testing.T) {
	// Test that when cached tokens is 0, all prompt tokens are billable
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens: 0, // Explicitly 0
		},
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.06"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	expectedTotal := 0.00006
	require.InDelta(t, expectedTotal, total.InexactFloat64(), 0.0000001)

	var inputItem *objects.CostItem

	for i := range items {
		if items[i].ItemCode == objects.PriceItemCodeUsage {
			inputItem = &items[i]
			break
		}
	}

	require.NotNil(t, inputItem)
	require.Equal(t, int64(1000), inputItem.Quantity, "input quantity should be 1000 when cached tokens is 0")
}

func TestComputeUsageCost_WithWriteCachedTokens(t *testing.T) {
	// Test that write cached tokens are excluded from input token cost calculation
	usage := &llm.Usage{
		PromptTokens:     1000, // Includes 200 write cached tokens
		CompletionTokens: 500,
		TotalTokens:      1500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			WriteCachedTokens: 200, // Write to cache
		},
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.06"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeWriteCachedTokens,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.0375"), // 25% more than input
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	// Expected cost:
	// - Input tokens (billable): (800 / 1_000_000) * 0.03 = 0.000024
	// - Write cached tokens: (200 / 1_000_000) * 0.0375 = 0.0000075
	// - Completion tokens: (500 / 1_000_000) * 0.06 = 0.00003
	// Total: 0.000024 + 0.0000075 + 0.00003 = 0.0000615
	expectedTotal := 0.0000615
	require.InDelta(t, expectedTotal, total.InexactFloat64(), 0.0000001)

	require.Len(t, items, 3)

	var inputItem, writeCachedItem, completionItem *objects.CostItem

	for i := range items {
		switch items[i].ItemCode {
		case objects.PriceItemCodeUsage:
			inputItem = &items[i]
		case objects.PriceItemCodeWriteCachedTokens:
			writeCachedItem = &items[i]
		case objects.PriceItemCodeCompletion:
			completionItem = &items[i]
		}
	}

	require.NotNil(t, inputItem)
	require.NotNil(t, writeCachedItem)
	require.NotNil(t, completionItem)

	// Verify input tokens quantity excludes write cached tokens
	require.Equal(t, int64(800), inputItem.Quantity, "input quantity should be 800 (1000 - 200 write cached)")
	require.InDelta(t, 0.000024, inputItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify write cached tokens quantity
	require.Equal(t, int64(200), writeCachedItem.Quantity, "write cached quantity should be 200")
	require.InDelta(t, 0.0000075, writeCachedItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify completion tokens quantity
	require.Equal(t, int64(500), completionItem.Quantity, "completion quantity should be 500")
	require.InDelta(t, 0.00003, completionItem.Subtotal.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_WithBothCachedAndWriteCachedTokens(t *testing.T) {
	// Test with both read cache and write cache tokens
	usage := &llm.Usage{
		PromptTokens:     1000, // Includes 300 cached + 200 write cached
		CompletionTokens: 500,
		TotalTokens:      1500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens:      300, // Read from cache
			WriteCachedTokens: 200, // Write to cache
		},
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.06"),
				},
			},
			{
				ItemCode: objects.PriceItemCodePromptCachedToken,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.015"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeWriteCachedTokens,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.0375"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	// Expected cost:
	// - Input tokens (billable): (500 / 1_000_000) * 0.03 = 0.000015
	// - Cached tokens: (300 / 1_000_000) * 0.015 = 0.0000045
	// - Write cached tokens: (200 / 1_000_000) * 0.0375 = 0.0000075
	// - Completion tokens: (500 / 1_000_000) * 0.06 = 0.00003
	// Total: 0.000015 + 0.0000045 + 0.0000075 + 0.00003 = 0.000057
	expectedTotal := 0.000057
	require.InDelta(t, expectedTotal, total.InexactFloat64(), 0.0000001)

	require.Len(t, items, 4)

	var inputItem, cachedItem, writeCachedItem, completionItem *objects.CostItem

	for i := range items {
		switch items[i].ItemCode {
		case objects.PriceItemCodeUsage:
			inputItem = &items[i]
		case objects.PriceItemCodePromptCachedToken:
			cachedItem = &items[i]
		case objects.PriceItemCodeWriteCachedTokens:
			writeCachedItem = &items[i]
		case objects.PriceItemCodeCompletion:
			completionItem = &items[i]
		}
	}

	require.NotNil(t, inputItem, "input cost item should exist")
	require.NotNil(t, cachedItem, "cached cost item should exist")
	require.NotNil(t, writeCachedItem, "write cached cost item should exist")
	require.NotNil(t, completionItem, "completion cost item should exist")

	// Verify input tokens exclude both cached and write cached tokens
	require.Equal(t, int64(500), inputItem.Quantity, "input quantity should be 500 (1000 - 300 - 200)")
	require.InDelta(t, 0.000015, inputItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify cached tokens
	require.Equal(t, int64(300), cachedItem.Quantity, "cached quantity should be 300")
	require.InDelta(t, 0.0000045, cachedItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify write cached tokens
	require.Equal(t, int64(200), writeCachedItem.Quantity, "write cached quantity should be 200")
	require.InDelta(t, 0.0000075, writeCachedItem.Subtotal.InexactFloat64(), 0.0000001)

	// Verify completion tokens
	require.Equal(t, int64(500), completionItem.Quantity, "completion quantity should be 500")
	require.InDelta(t, 0.00003, completionItem.Subtotal.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_AllTokensCached(t *testing.T) {
	// Test edge case where all prompt tokens are from cache
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens: 1000, // All tokens are cached
		},
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.06"),
				},
			},
			{
				ItemCode: objects.PriceItemCodePromptCachedToken,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.015"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	// Expected cost:
	// - Input tokens (billable): 0 tokens = 0
	// - Cached tokens: (1000 / 1_000_000) * 0.015 = 0.000015
	// - Completion tokens: (500 / 1_000_000) * 0.06 = 0.00003
	// Total: 0.000045
	expectedTotal := 0.000045
	require.InDelta(t, expectedTotal, total.InexactFloat64(), 0.0000001)

	var inputItem *objects.CostItem

	for i := range items {
		if items[i].ItemCode == objects.PriceItemCodeUsage {
			inputItem = &items[i]
			break
		}
	}

	require.NotNil(t, inputItem)
	require.Equal(t, int64(0), inputItem.Quantity, "input quantity should be 0 when all tokens are cached")
	require.True(t, inputItem.Subtotal.IsZero(), "input subtotal should be 0")
}

func mustDecimalPtr(s string) *decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}

	return &d
}

func TestComputeUsageCost_FlatFee_Requests(t *testing.T) {
	usage := &llm.Usage{
		Quantity: 10,
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeRequests,
				Pricing: objects.Pricing{
					Mode:    objects.PricingModeFlatFee,
					FlatFee: mustDecimalPtr("0.01"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 1)
	require.Equal(t, int64(10), items[0].Quantity)
	require.Equal(t, objects.PriceItemCodeRequests, items[0].ItemCode)
	require.True(t, total.Equal(decimal.NewFromFloat(0.1)))
	require.True(t, items[0].Subtotal.Equal(decimal.NewFromFloat(0.1)))
}

func TestComputeUsageCost_FlatFee_Requests_ZeroQuantity(t *testing.T) {
	usage := &llm.Usage{
		Quantity: 0,
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeRequests,
				Pricing: objects.Pricing{
					Mode:    objects.PricingModeFlatFee,
					FlatFee: mustDecimalPtr("0.01"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 1)
	require.Equal(t, int64(1), items[0].Quantity, "quantity should default to 1 when zero")
	require.True(t, total.Equal(decimal.NewFromFloat(0.01)))
}

func TestComputeUsageCost_FlatFee_LegacyLLM(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:    objects.PricingModeFlatFee,
					FlatFee: mustDecimalPtr("5.00"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 1)
	require.Equal(t, int64(1000), items[0].Quantity)
	require.True(t, total.Equal(decimal.NewFromFloat(5.00)), "flat fee should be fixed regardless of quantity")
	require.True(t, items[0].Subtotal.Equal(decimal.NewFromFloat(5.00)))
}

func TestComputeUsageCost_TieredPricing(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:     3_000_000,
		CompletionTokens: 0,
	}

	upTo1 := int64(1_000_000)
	upTo2 := int64(5_000_000)
	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode: objects.PricingModeTiered,
					UsageTiered: &objects.TieredPricing{
						Tiers: []objects.PriceTier{
							{UpTo: &upTo1, PricePerUnit: decimal.NewFromFloat(0.01)},
							{UpTo: &upTo2, PricePerUnit: decimal.NewFromFloat(0.008)},
							{UpTo: nil, PricePerUnit: decimal.NewFromFloat(0.005)},
						},
					},
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 1)
	require.Equal(t, int64(3_000_000), items[0].Quantity)
	require.Len(t, items[0].TierBreakdown, 2)

	require.Equal(t, &upTo1, items[0].TierBreakdown[0].UpTo)
	require.Equal(t, int64(1_000_000), items[0].TierBreakdown[0].Units)
	require.True(t, items[0].TierBreakdown[0].Subtotal.Equal(decimal.NewFromFloat(0.01)))

	require.Equal(t, &upTo2, items[0].TierBreakdown[1].UpTo)
	require.Equal(t, int64(2_000_000), items[0].TierBreakdown[1].Units)
	require.True(t, items[0].TierBreakdown[1].Subtotal.Equal(decimal.NewFromFloat(0.016)))

	require.True(t, total.Equal(decimal.NewFromFloat(0.026)))
}

func TestComputeUsageCost_TieredPricing_AllTiers(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:     10_000_000,
		CompletionTokens: 0,
	}

	upTo1 := int64(1_000_000)
	upTo2 := int64(5_000_000)
	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode: objects.PricingModeTiered,
					UsageTiered: &objects.TieredPricing{
						Tiers: []objects.PriceTier{
							{UpTo: &upTo1, PricePerUnit: decimal.NewFromFloat(0.01)},
							{UpTo: &upTo2, PricePerUnit: decimal.NewFromFloat(0.008)},
							{UpTo: nil, PricePerUnit: decimal.NewFromFloat(0.005)},
						},
					},
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 1)
	require.Len(t, items[0].TierBreakdown, 3)

	require.Equal(t, int64(1_000_000), items[0].TierBreakdown[0].Units)
	require.Equal(t, int64(4_000_000), items[0].TierBreakdown[1].Units)
	require.Equal(t, int64(5_000_000), items[0].TierBreakdown[2].Units)

	require.True(t, total.Equal(decimal.NewFromFloat(0.01 + 0.032 + 0.025)))
}

func TestComputeUsageCost_WriteCachedTokens_With5MinAnd1HourVariants(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			WriteCached5MinTokens:  200,
			WriteCached1HourTokens: 300,
		},
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeWriteCachedTokens,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.0375"),
				},
				PromptWriteCacheVariants: []objects.PromptWriteCacheVariant{
					{
						VariantCode: objects.PromptWriteCacheVariantCode5Min,
						Pricing: objects.Pricing{
							Mode:         objects.PricingModeUsagePerUnit,
							UsagePerUnit: mustDecimalPtr("0.0375"),
						},
					},
					{
						VariantCode: objects.PromptWriteCacheVariantCode1Hour,
						Pricing: objects.Pricing{
							Mode:         objects.PricingModeUsagePerUnit,
							UsagePerUnit: mustDecimalPtr("0.05"),
						},
					},
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 3)

	var inputItem *objects.CostItem
	var write5MinItem, write1HourItem *objects.CostItem

	for i := range items {
		switch items[i].ItemCode {
		case objects.PriceItemCodeUsage:
			inputItem = &items[i]
		case objects.PriceItemCodeWriteCachedTokens:
			if items[i].PromptWriteCacheVariantCode == objects.PromptWriteCacheVariantCode5Min {
				write5MinItem = &items[i]
			} else if items[i].PromptWriteCacheVariantCode == objects.PromptWriteCacheVariantCode1Hour {
				write1HourItem = &items[i]
			}
		}
	}

	require.NotNil(t, inputItem)
	require.Equal(t, int64(1000), inputItem.Quantity, "input quantity should be 1000 (variants are not subtracted from input)")

	require.NotNil(t, write5MinItem)
	require.Equal(t, int64(200), write5MinItem.Quantity)
	require.Equal(t, objects.PromptWriteCacheVariantCode5Min, write5MinItem.PromptWriteCacheVariantCode)
	require.InDelta(t, 0.0000075, write5MinItem.Subtotal.InexactFloat64(), 0.0000001)

	require.NotNil(t, write1HourItem)
	require.Equal(t, int64(300), write1HourItem.Quantity)
	require.Equal(t, objects.PromptWriteCacheVariantCode1Hour, write1HourItem.PromptWriteCacheVariantCode)
	require.InDelta(t, 0.000015, write1HourItem.Subtotal.InexactFloat64(), 0.0000001)

	require.InDelta(t, 0.00003+0.0000075+0.000015, total.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_WriteCachedTokens_FallbackToSharedPricing(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		PromptTokensDetails: &llm.PromptTokensDetails{
			WriteCachedTokens: 200,
		},
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
			{
				ItemCode: objects.PriceItemCodeWriteCachedTokens,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.0375"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 2)

	var inputItem, writeCachedItem *objects.CostItem

	for i := range items {
		switch items[i].ItemCode {
		case objects.PriceItemCodeUsage:
			inputItem = &items[i]
		case objects.PriceItemCodeWriteCachedTokens:
			writeCachedItem = &items[i]
		}
	}

	require.NotNil(t, inputItem)
	require.Equal(t, int64(800), inputItem.Quantity, "input quantity should be 800 (1000 - 200)")

	require.NotNil(t, writeCachedItem)
	require.Equal(t, int64(200), writeCachedItem.Quantity)
	require.Empty(t, writeCachedItem.PromptWriteCacheVariantCode, "variant code should be empty for fallback")
	require.InDelta(t, 0.0000075, writeCachedItem.Subtotal.InexactFloat64(), 0.0000001)

	require.InDelta(t, 0.000024+0.0000075, total.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_NegativeQuantityClamp(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens: 100,
		PromptTokensDetails: &llm.PromptTokensDetails{
			CachedTokens:      200,
			WriteCachedTokens: 150,
		},
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 1)
	require.Equal(t, int64(0), items[0].Quantity, "quantity should be clamped to 0 when negative")
	require.True(t, total.IsZero())
}

func TestComputeUsageCost_UnknownItemCode(t *testing.T) {
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
	}

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCode("unknown_code"),
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: mustDecimalPtr("0.03"),
				},
			},
		},
	}

	items, total := ComputeUsageCost(usage, price)

	require.Len(t, items, 1)
	require.Equal(t, int64(0), items[0].Quantity, "unknown item code should result in 0 quantity")
	require.True(t, total.IsZero())
}
