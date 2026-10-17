package gql

import (
	"database/sql"
	"sort"

	"github.com/looplj/axonhub/internal/objects"
)

// currencyAmountList converts a currency→amount map into the deterministic list
// shape returned by cost aggregations. A nil list is never returned because the
// GraphQL fields are non-null lists.
func currencyAmountList(costs map[string]float64) []*objects.CurrencyAmount {
	result := make([]*objects.CurrencyAmount, 0, len(costs))

	codes := make([]string, 0, len(costs))
	for code := range costs {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	for _, code := range codes {
		currencyCode := code
		if currencyCode == "" {
			currencyCode = objects.DefaultCurrencyCode
		}

		result = append(result, &objects.CurrencyAmount{
			CurrencyCode: currencyCode,
			Amount:       costs[code],
		})
	}

	return result
}

// addCurrencyCost accumulates a nullable SUM(total_cost) under its currency.
func addCurrencyCost(costs map[string]float64, currencyCode string, cost sql.NullFloat64) {
	if !cost.Valid {
		return
	}

	if currencyCode == "" {
		currencyCode = objects.DefaultCurrencyCode
	}

	costs[currencyCode] += cost.Float64
}

func sortCurrencyAmounts(amounts []*objects.CurrencyAmount) {
	sort.Slice(amounts, func(i, j int) bool {
		return amounts[i].CurrencyCode < amounts[j].CurrencyCode
	})
}
