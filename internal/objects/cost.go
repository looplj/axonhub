package objects

import "github.com/shopspring/decimal"

type TierCost struct {
	UpTo     *int64          `json:"upTo,omitempty"`
	Units    int64           `json:"units"`
	Subtotal decimal.Decimal `json:"subtotal"`
}

type CostItem struct {
	ItemCode      PriceItemCode   `json:"itemCode"`
	Quantity      int64           `json:"quantity"`
	TierBreakdown []TierCost      `json:"tierBreakdown,omitempty"`
	Subtotal      decimal.Decimal `json:"subtotal"`
}
