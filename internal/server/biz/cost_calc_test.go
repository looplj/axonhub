package biz

import (
	"testing"
	"time"

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

	items, total := ComputeUsageCost(usage, price, time.Now())

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

	items, total := ComputeUsageCost(usage, price, time.Now())

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

	items, total := ComputeUsageCost(usage, price, time.Now())

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

	items, total := ComputeUsageCost(usage, price, time.Now())

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

	items, total := ComputeUsageCost(usage, price, time.Now())

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

	items, total := ComputeUsageCost(usage, price, time.Now())

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

func TestComputeUsageCost_WithSchedule_Override(t *testing.T) {
	// Test that when a schedule override matches, the override prices are used
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}

	// Default price: $0.03 per 1M input, $0.06 per 1M output
	// Override price: $0.01 per 1M input, $0.02 per 1M output (night discount)
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
		Schedule: &objects.PriceSchedule{
			Timezone: "UTC",
			Overrides: []objects.PriceOverride{
				{
					Name:     "Night Discount",
					Priority: 1,
					When: objects.OverrideWhen{
						DailyTime: &objects.DailyTimeRange{
							Start: "00:00",
							End:   "08:00",
						},
					},
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.01"),
							},
						},
						{
							ItemCode: objects.PriceItemCodeCompletion,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.02"),
							},
						},
					},
				},
			},
		},
	}

	// Test at 03:00 UTC - should match night discount
	now := time.Date(2026, 7, 21, 3, 0, 0, 0, time.UTC)
	items, total := ComputeUsageCost(usage, price, now)

	// Expected: (1000/1M)*0.01 + (500/1M)*0.02 = 0.00001 + 0.00001 = 0.00002
	require.InDelta(t, 0.00002, total.InexactFloat64(), 0.0000001)
	require.Len(t, items, 2)
}

func TestComputeUsageCost_WithSchedule_NoMatch(t *testing.T) {
	// Test that when no schedule override matches, default prices are used
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
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
		Schedule: &objects.PriceSchedule{
			Timezone: "UTC",
			Overrides: []objects.PriceOverride{
				{
					Name:     "Night Discount",
					Priority: 1,
					When: objects.OverrideWhen{
						DailyTime: &objects.DailyTimeRange{
							Start: "00:00",
							End:   "08:00",
						},
					},
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.01"),
							},
						},
						{
							ItemCode: objects.PriceItemCodeCompletion,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.02"),
							},
						},
					},
				},
			},
		},
	}

	// Test at 14:00 UTC - should NOT match night discount, use default
	now := time.Date(2026, 7, 21, 14, 0, 0, 0, time.UTC)
	items, total := ComputeUsageCost(usage, price, now)

	// Expected: (1000/1M)*0.03 + (500/1M)*0.06 = 0.00003 + 0.00003 = 0.00006
	require.InDelta(t, 0.00006, total.InexactFloat64(), 0.0000001)
	require.Len(t, items, 2)
}

func TestComputeUsageCost_WithSchedule_CrossMidnight(t *testing.T) {
	// Test cross-midnight time range (e.g., 22:00-06:00)
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
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
		Schedule: &objects.PriceSchedule{
			Timezone: "UTC",
			Overrides: []objects.PriceOverride{
				{
					Name:     "Night Discount",
					Priority: 1,
					When: objects.OverrideWhen{
						DailyTime: &objects.DailyTimeRange{
							Start: "22:00",
							End:   "06:00",
						},
					},
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.01"),
							},
						},
						{
							ItemCode: objects.PriceItemCodeCompletion,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.02"),
							},
						},
					},
				},
			},
		},
	}

	// Test at 23:00 UTC - should match cross-midnight range
	now := time.Date(2026, 7, 21, 23, 0, 0, 0, time.UTC)
	items, total := ComputeUsageCost(usage, price, now)
	require.InDelta(t, 0.00002, total.InexactFloat64(), 0.0000001)
	require.Len(t, items, 2)

	// Test at 03:00 UTC - should also match cross-midnight range
	now = time.Date(2026, 7, 22, 3, 0, 0, 0, time.UTC)
	_, total = ComputeUsageCost(usage, price, now)
	require.InDelta(t, 0.00002, total.InexactFloat64(), 0.0000001)

	// Test at 12:00 UTC - should NOT match
	now = time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	_, total = ComputeUsageCost(usage, price, now)
	require.InDelta(t, 0.00006, total.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_WithSchedule_Weekdays(t *testing.T) {
	// Test weekday-based override (weekend discount)
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
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
		Schedule: &objects.PriceSchedule{
			Timezone: "UTC",
			Overrides: []objects.PriceOverride{
				{
					Name:     "Weekend Discount",
					Priority: 1,
					When: objects.OverrideWhen{
						Weekdays: []int{6, 7}, // Saturday and Sunday
					},
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.01"),
							},
						},
						{
							ItemCode: objects.PriceItemCodeCompletion,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.02"),
							},
						},
					},
				},
			},
		},
	}

	// 2026-07-25 is Saturday (weekday 6)
	saturday := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	_, total := ComputeUsageCost(usage, price, saturday)
	require.InDelta(t, 0.00002, total.InexactFloat64(), 0.0000001)

	// 2026-07-26 is Sunday (weekday 7)
	sunday := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	_, total = ComputeUsageCost(usage, price, sunday)
	require.InDelta(t, 0.00002, total.InexactFloat64(), 0.0000001)

	// 2026-07-21 is Tuesday (weekday 2) - should use default
	tuesday := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	_, total = ComputeUsageCost(usage, price, tuesday)
	require.InDelta(t, 0.00006, total.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_WithSchedule_DateRange(t *testing.T) {
	// Test date range override (promotion)
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
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
		Schedule: &objects.PriceSchedule{
			Timezone: "UTC",
			Overrides: []objects.PriceOverride{
				{
					Name:     "Summer Promotion",
					Priority: 1,
					When: objects.OverrideWhen{
						DateRange: &objects.DateRange{
							Start: "2026-07-01",
							End:   "2026-07-31",
						},
					},
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.01"),
							},
						},
						{
							ItemCode: objects.PriceItemCodeCompletion,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.02"),
							},
						},
					},
				},
			},
		},
	}

	// July 15 - within date range
	july15 := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	_, total := ComputeUsageCost(usage, price, july15)
	require.InDelta(t, 0.00002, total.InexactFloat64(), 0.0000001)

	// August 1 - outside date range
	aug1 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	_, total = ComputeUsageCost(usage, price, aug1)
	require.InDelta(t, 0.00006, total.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_WithSchedule_Priority(t *testing.T) {
	// Test that lower priority number wins when multiple overrides match
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
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
		Schedule: &objects.PriceSchedule{
			Timezone: "UTC",
			Overrides: []objects.PriceOverride{
				{
					Name:     "Low Priority",
					Priority: 10,
					When: objects.OverrideWhen{
						DailyTime: &objects.DailyTimeRange{
							Start: "00:00",
							End:   "23:59",
						},
					},
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.02"),
							},
						},
						{
							ItemCode: objects.PriceItemCodeCompletion,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.04"),
							},
						},
					},
				},
				{
					Name:     "High Priority",
					Priority: 1,
					When: objects.OverrideWhen{
						DailyTime: &objects.DailyTimeRange{
							Start: "10:00",
							End:   "14:00",
						},
					},
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.01"),
							},
						},
						{
							ItemCode: objects.PriceItemCodeCompletion,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.02"),
							},
						},
					},
				},
			},
		},
	}

	// At 12:00 - both match, but high priority (1) wins
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	_, total := ComputeUsageCost(usage, price, now)
	require.InDelta(t, 0.00002, total.InexactFloat64(), 0.0000001)

	// At 16:00 - only low priority matches
	now = time.Date(2026, 7, 21, 16, 0, 0, 0, time.UTC)
	_, total = ComputeUsageCost(usage, price, now)
	require.InDelta(t, 0.00004, total.InexactFloat64(), 0.0000001)
}

func TestComputeUsageCost_WithSchedule_Timezone(t *testing.T) {
	// Test timezone handling
	usage := &llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
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
		Schedule: &objects.PriceSchedule{
			Timezone: "Asia/Shanghai", // UTC+8
			Overrides: []objects.PriceOverride{
				{
					Name:     "Night Discount",
					Priority: 1,
					When: objects.OverrideWhen{
						DailyTime: &objects.DailyTimeRange{
							Start: "00:00",
							End:   "08:00",
						},
					},
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.01"),
							},
						},
						{
							ItemCode: objects.PriceItemCodeCompletion,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: mustDecimalPtr("0.02"),
							},
						},
					},
				},
			},
		},
	}

	// 22:00 UTC = 06:00 Shanghai - within night discount
	now := time.Date(2026, 7, 21, 22, 0, 0, 0, time.UTC)
	_, total := ComputeUsageCost(usage, price, now)
	require.InDelta(t, 0.00002, total.InexactFloat64(), 0.0000001)

	// 10:00 UTC = 18:00 Shanghai - outside night discount
	now = time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	_, total = ComputeUsageCost(usage, price, now)
	require.InDelta(t, 0.00006, total.InexactFloat64(), 0.0000001)
}

func mustDecimalPtr(s string) *decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}

	return &d
}
