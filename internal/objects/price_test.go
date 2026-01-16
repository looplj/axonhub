package objects

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestModelPrice_Equals(t *testing.T) {
	d1 := decimal.NewFromFloat(0.01)
	d2 := decimal.NewFromFloat(0.02)
	upTo1000 := int64(1000)

	tests := []struct {
		name     string
		p1       ModelPrice
		p2       ModelPrice
		expected bool
	}{
		{
			name: "Equal simple",
			p1: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode:         PricingModeUsagePerUnit,
							UsagePerUnit: &d1,
						},
					},
				},
			},
			p2: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode:         PricingModeUsagePerUnit,
							UsagePerUnit: &d1,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Not equal mode",
			p1: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode: PricingModeFlatFee,
						},
					},
				},
			},
			p2: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode: PricingModeUsagePerUnit,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Not equal usage per unit",
			p1: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode:         PricingModeUsagePerUnit,
							UsagePerUnit: &d1,
						},
					},
				},
			},
			p2: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode:         PricingModeUsagePerUnit,
							UsagePerUnit: &d2,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Equal tiered",
			p1: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode: PricingModeTiered,
							UsageTiered: &TieredPricing{
								Tiers: []PriceTier{
									{UpTo: &upTo1000, PricePerUnit: d1},
									{UpTo: nil, PricePerUnit: d2},
								},
							},
						},
					},
				},
			},
			p2: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode: PricingModeTiered,
							UsageTiered: &TieredPricing{
								Tiers: []PriceTier{
									{UpTo: &upTo1000, PricePerUnit: d1},
									{UpTo: nil, PricePerUnit: d2},
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Not equal tiered price",
			p1: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode: PricingModeTiered,
							UsageTiered: &TieredPricing{
								Tiers: []PriceTier{
									{UpTo: &upTo1000, PricePerUnit: d1},
								},
							},
						},
					},
				},
			},
			p2: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeUsage,
						Pricing: Pricing{
							Mode: PricingModeTiered,
							UsageTiered: &TieredPricing{
								Tiers: []PriceTier{
									{UpTo: &upTo1000, PricePerUnit: d2},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Equal with variants",
			p1: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeWriteCachedTokens,
						Pricing: Pricing{
							Mode: PricingModeFlatFee,
						},
						PromptWriteCacheVariants: []PromptWriteCacheVariant{
							{
								VariantCode: PromptWriteCacheVariantCode5Min,
								Pricing: Pricing{
									Mode:         PricingModeUsagePerUnit,
									UsagePerUnit: &d1,
								},
							},
						},
					},
				},
			},
			p2: ModelPrice{
				Items: []ModelPriceItem{
					{
						ItemCode: PriceItemCodeWriteCachedTokens,
						Pricing: Pricing{
							Mode: PricingModeFlatFee,
						},
						PromptWriteCacheVariants: []PromptWriteCacheVariant{
							{
								VariantCode: PromptWriteCacheVariantCode5Min,
								Pricing: Pricing{
									Mode:         PricingModeUsagePerUnit,
									UsagePerUnit: &d1,
								},
							},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.p1.Equals(tt.p2))
			assert.Equal(t, tt.expected, tt.p2.Equals(tt.p1))
		})
	}
}
