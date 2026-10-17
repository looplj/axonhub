package gql

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectTopPerCurrency(t *testing.T) {
	order := make([]string, 0, 13)

	costs := make(map[string]map[string]float64)

	for i := 1; i <= 12; i++ {
		key := fmt.Sprintf("jpy-%02d", i)
		order = append(order, key)
		costs[key] = map[string]float64{"JPY": float64(100 - i)}
	}

	order = append(order, "usd-01")
	costs["usd-01"] = map[string]float64{"USD": 50}

	got := selectTopPerCurrency(order, costs, 10)

	require.Len(t, got, 11)
	require.Contains(t, got, "usd-01")
	require.Contains(t, got, "jpy-10")
	require.NotContains(t, got, "jpy-11")
	require.NotContains(t, got, "jpy-12")
}

func TestSelectTopPerCurrency_Empty(t *testing.T) {
	require.Empty(t, selectTopPerCurrency[string](nil, nil, 10))
}
