package objects

import (
	"github.com/shopspring/decimal"
)

type PricingMode string

const (
	// PricingModeFlatFee means the request is charged a fixed fee.
	PricingModeFlatFee PricingMode = "flat_fee"

	// PricingModeUsagePerUnit means the request is charged a fee bases on the token usage.
	// e.g. $0.01 per token, if the usage is 1,500 then the fee is $0.01 x 1,500 = $15.00.
	PricingModeUsagePerUnit PricingMode = "usage_per_unit"

	// PricingModeTiered means the request is charged a fee based on the token usage tiers.
	// e.g. a tier price is [1,000, 2,000, 3,000] for [$0.01, $0.02, $0.03], if the usage is 1,500 then the fee is
	// $0.01 x 1000 + $0.02 x (1500-1000) = $20.00.
	PricingModeTiered PricingMode = "usage_tiered"
)

type Pricing struct {
	Mode PricingMode `json:"mode"`

	// FlatFee is the fixed fee for the pricing.
	FlatFee *decimal.Decimal `json:"flatFee,omitempty"`

	// UsagePerUnit is the price per token for the pricing.
	UsagePerUnit *decimal.Decimal `json:"usagePerUnit,omitempty"`

	// UsageTiered is the tiered pricing for the pricing.
	UsageTiered *TieredPricing `json:"usageTiered,omitempty"`
}

func (p *Pricing) Equals(other *Pricing) bool {
	if p == nil || other == nil {
		return p == other
	}

	if p.Mode != other.Mode {
		return false
	}

	switch p.Mode {
	case PricingModeFlatFee:
		if (p.FlatFee == nil) != (other.FlatFee == nil) {
			return false
		}

		if p.FlatFee != nil && !p.FlatFee.Equal(*other.FlatFee) {
			return false
		}
	case PricingModeUsagePerUnit:
		if (p.UsagePerUnit == nil) != (other.UsagePerUnit == nil) {
			return false
		}

		if p.UsagePerUnit != nil && !p.UsagePerUnit.Equal(*other.UsagePerUnit) {
			return false
		}
	case PricingModeTiered:
		return p.UsageTiered.Equals(other.UsageTiered)
	}

	return true
}

type TieredPricing struct {
	Tiers []PriceTier `json:"tiers"`
}

func (p *TieredPricing) Equals(other *TieredPricing) bool {
	if p == nil || other == nil {
		return p == other
	}

	if len(p.Tiers) != len(other.Tiers) {
		return false
	}

	for i := range p.Tiers {
		if !p.Tiers[i].Equals(&other.Tiers[i]) {
			return false
		}
	}

	return true
}

// PriceTier is the price tier for the tiered pricing.
type PriceTier struct {
	// UpTo is the upper bound of the token usage for the price tier.
	// If the upper bound is nil, it means no upper bound, it must be the last price tier.
	UpTo *int64 `json:"upTo,omitempty"`

	// PricePerUnit is the price per token for the price tier.
	PricePerUnit decimal.Decimal `json:"pricePerUnit"`
}

func (p *PriceTier) Equals(other *PriceTier) bool {
	if p == nil || other == nil {
		return p == other
	}

	if (p.UpTo == nil) != (other.UpTo == nil) {
		return false
	}

	if p.UpTo != nil && *p.UpTo != *other.UpTo {
		return false
	}

	return p.PricePerUnit.Equal(other.PricePerUnit)
}

type PriceItemCode string

const (
	// PriceItemCodeUsage is the price item code for the token usage.
	PriceItemCodeUsage PriceItemCode = "prompt_tokens"

	// PriceItemCodeCompletion is the price item code for the token completion.
	PriceItemCodeCompletion PriceItemCode = "completion_tokens"

	// PriceItemCodePromptCachedToken is the price item code for the cached token usage.
	PriceItemCodePromptCachedToken PriceItemCode = "prompt_cached_tokens"

	// PriceItemCodeWriteCachedTokens is the price item code for the cached token write.
	//nolint:gosec // not token.
	PriceItemCodeWriteCachedTokens PriceItemCode = "prompt_write_cached_tokens"
)

type PromptWriteCacheVariantCode string

const (
	// PromptWriteCacheVariantCode5Min is the variant code for cached token write in 5 minutes.
	PromptWriteCacheVariantCode5Min PromptWriteCacheVariantCode = "five_min"

	// PromptWriteCacheVariantCode1Hour is the variant code for cached token write in 1 hour.
	PromptWriteCacheVariantCode1Hour PromptWriteCacheVariantCode = "one_hour"
)

// PromptWriteCacheVariant is the variant for cached token write.
type PromptWriteCacheVariant struct {
	// VariantCode is the code of the variant.
	VariantCode PromptWriteCacheVariantCode `json:"variantCode"`

	// Pricing is the pricing for the variant.
	Pricing Pricing `json:"pricing"`
}

func (p *PromptWriteCacheVariant) Equals(other *PromptWriteCacheVariant) bool {
	if p == nil || other == nil {
		return p == other
	}

	if p.VariantCode != other.VariantCode {
		return false
	}

	return p.Pricing.Equals(&other.Pricing)
}

// FindPromptWriteCacheVariantPricing finds the variant pricing for the item prompt write cached tokens.
// If the variant pricing is not found, it will return the item pricing.
func (i *ModelPriceItem) FindPromptWriteCacheVariantPricing(variantCode PromptWriteCacheVariantCode) Pricing {
	for _, v := range i.PromptWriteCacheVariants {
		if v.VariantCode == variantCode {
			return v.Pricing
		}
	}

	return i.Pricing
}

type ModelPriceItem struct {
	// ItemCode is the code of the item.
	ItemCode PriceItemCode `json:"itemCode"`

	// Pricing is the pricing for the item.
	Pricing Pricing `json:"pricing"`

	// PromptWriteCacheVariants is the list of variants for the item prompt write cached tokens.
	// If the variants present, it will find the variant price first, if not hit, it will use the item pricing.
	PromptWriteCacheVariants []PromptWriteCacheVariant `json:"promptWriteCacheVariants,omitempty"`
}

func (i *ModelPriceItem) Equals(other *ModelPriceItem) bool {
	if i == nil || other == nil {
		return i == other
	}

	if i.ItemCode != other.ItemCode {
		return false
	}

	if !i.Pricing.Equals(&other.Pricing) {
		return false
	}

	if len(i.PromptWriteCacheVariants) != len(other.PromptWriteCacheVariants) {
		return false
	}

	for idx := range i.PromptWriteCacheVariants {
		if !i.PromptWriteCacheVariants[idx].Equals(&other.PromptWriteCacheVariants[idx]) {
			return false
		}
	}

	return true
}

// ModelPrice is the price for the thing.
type ModelPrice struct {
	// Items is the list of price items for the price.
	Items []ModelPriceItem `json:"items"`
}

func (p *ModelPrice) Equals(other ModelPrice) bool {
	if len(p.Items) != len(other.Items) {
		return false
	}

	for i := range p.Items {
		if !p.Items[i].Equals(&other.Items[i]) {
			return false
		}
	}

	return true
}
