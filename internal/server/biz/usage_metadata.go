package biz

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
)

type UsageMetadata struct {
	TotalInputTokens       int
	TotalOutputTokens      int
	TotalTokens            int
	TotalCachedTokens      int
	TotalCachedWriteTokens int
	TotalCostByCurrency    []*objects.CurrencyAmount
}

type usageMetadataAgg struct {
	CurrencyCode           string          `json:"currency_code"`
	TotalInputTokens       sql.NullInt64   `json:"total_input_tokens"`
	TotalOutputTokens      sql.NullInt64   `json:"total_output_tokens"`
	TotalTokens            sql.NullInt64   `json:"total_tokens"`
	TotalCachedTokens      sql.NullInt64   `json:"total_cached_tokens"`
	TotalCachedWriteTokens sql.NullInt64   `json:"total_cached_write_tokens"`
	TotalCost              sql.NullFloat64 `json:"total_cost"`
}

func aggregateUsageMetadata(ctx context.Context, q *ent.UsageLogQuery) (*UsageMetadata, error) {
	var rows []usageMetadataAgg

	err := q.Modify(func(s *entsql.Selector) {
		s.Select(
			entsql.As(s.C(usagelog.FieldCostCurrencyCode), "currency_code"),
			entsql.As(entsql.Sum(usagelog.FieldPromptTokens), "total_input_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldCompletionTokens), "total_output_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldTotalTokens), "total_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldPromptCachedTokens), "total_cached_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldPromptWriteCachedTokens), "total_cached_write_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldTotalCost), "total_cost"),
		).GroupBy(s.C(usagelog.FieldCostCurrencyCode))
	}).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate usage metadata: %w", err)
	}

	metadata := &UsageMetadata{}

	costs := make(map[string]float64, len(rows))
	for _, row := range rows {
		metadata.TotalInputTokens += int(row.TotalInputTokens.Int64)
		metadata.TotalOutputTokens += int(row.TotalOutputTokens.Int64)
		metadata.TotalTokens += int(row.TotalTokens.Int64)
		metadata.TotalCachedTokens += int(row.TotalCachedTokens.Int64)
		metadata.TotalCachedWriteTokens += int(row.TotalCachedWriteTokens.Int64)

		if row.TotalCost.Valid {
			costs[row.CurrencyCode] += row.TotalCost.Float64
		}
	}

	for currencyCode, amount := range costs {
		metadata.TotalCostByCurrency = append(metadata.TotalCostByCurrency, &objects.CurrencyAmount{
			CurrencyCode: currencyCode,
			Amount:       amount,
		})
	}
	sort.Slice(metadata.TotalCostByCurrency, func(i, j int) bool {
		return metadata.TotalCostByCurrency[i].CurrencyCode < metadata.TotalCostByCurrency[j].CurrencyCode
	})

	return metadata, nil
}

// CostInCurrency returns the aggregated cost in the given currency.
func (m *UsageMetadata) CostInCurrency(currencyCode string) float64 {
	for _, item := range m.TotalCostByCurrency {
		if item.CurrencyCode == currencyCode {
			return item.Amount
		}
	}

	return 0
}
