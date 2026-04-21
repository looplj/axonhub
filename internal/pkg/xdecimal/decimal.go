package xdecimal

import (
	"strconv"

	"github.com/shopspring/decimal"
)

// Float64ToDecimal converts a float64 to decimal.Decimal without float64
// representation artifacts. decimal.NewFromFloat(f) preserves the exact
// binary representation of f (e.g. 0.1 → 0.1000000000000000055…), which
// causes incorrect comparisons when the cost limit was deserialized from
// exact JSON decimal. Formatting via strconv.FormatFloat with 'f' notation
// and re-parsing as decimal produces an exact match for the intended value.
func Float64ToDecimal(f float64) decimal.Decimal {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	d, _ := decimal.NewFromString(s)
	return d
}