package objects

import "github.com/shopspring/decimal"

type TierCost struct {
	UpTo         *int64          `json:"upTo,omitempty"`
	Units        int64           `json:"units"`
	PricePerUnit decimal.Decimal `json:"pricePerUnit"`
	Subtotal     decimal.Decimal `json:"subtotal"`
}

type CostItem struct {
	ItemCode      PriceItemCode    `json:"itemCode"`
	Mode          PricingMode      `json:"mode"`
	Quantity      int64            `json:"quantity"`
	UnitPrice     *decimal.Decimal `json:"unitPrice,omitempty"`
	FlatFee       *decimal.Decimal `json:"flatFee,omitempty"`
	TierBreakdown []TierCost       `json:"tierBreakdown,omitempty"`
	Subtotal      decimal.Decimal  `json:"subtotal"`
}
