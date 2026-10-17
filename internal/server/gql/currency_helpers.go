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

// selectTopPerCurrency returns the union of the top `limit` entities for each
// currency, preserving the original order. Ranking entities by a sum across
// currencies is meaningless without FX conversion and would hide currencies
// whose entities are not in the mixed-currency top list.
func selectTopPerCurrency[K comparable](order []K, costs map[K]map[string]float64, limit int) []K {
	currencies := make(map[string]struct{})
	for _, byCurrency := range costs {
		for currencyCode := range byCurrency {
			currencies[currencyCode] = struct{}{}
		}
	}

	selected := make(map[K]struct{})

	for currencyCode := range currencies {
		ranked := append([]K(nil), order...)
		sort.SliceStable(ranked, func(i, j int) bool {
			return costs[ranked[i]][currencyCode] > costs[ranked[j]][currencyCode]
		})

		if len(ranked) > limit {
			ranked = ranked[:limit]
		}

		for _, key := range ranked {
			selected[key] = struct{}{}
		}
	}

	result := make([]K, 0, len(selected))
	for _, key := range order {
		if _, ok := selected[key]; ok {
			result = append(result, key)
		}
	}

	return result
}
