package biz

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/usagelog"
)

type UsageMetadata struct {
	TotalInputTokens       int
	TotalOutputTokens      int
	TotalTokens            int
	TotalCachedTokens      int
	TotalCachedWriteTokens int
	TotalCost              decimal.Decimal
}

type usageMetadataAgg struct {
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
			entsql.As(entsql.Sum(usagelog.FieldPromptTokens), "total_input_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldCompletionTokens), "total_output_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldTotalTokens), "total_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldPromptCachedTokens), "total_cached_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldPromptWriteCachedTokens), "total_cached_write_tokens"),
			entsql.As(entsql.Sum(usagelog.FieldTotalCost), "total_cost"),
		)
	}).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate usage metadata: %w", err)
	}

	if len(rows) == 0 {
		return &UsageMetadata{TotalCost: decimal.Zero}, nil
	}

	cost := decimal.Zero
	if rows[0].TotalCost.Valid {
		cost = decimal.NewFromFloat(rows[0].TotalCost.Float64)
	}

	return &UsageMetadata{
		TotalInputTokens:       int(rows[0].TotalInputTokens.Int64),
		TotalOutputTokens:      int(rows[0].TotalOutputTokens.Int64),
		TotalTokens:            int(rows[0].TotalTokens.Int64),
		TotalCachedTokens:      int(rows[0].TotalCachedTokens.Int64),
		TotalCachedWriteTokens: int(rows[0].TotalCachedWriteTokens.Int64),
		TotalCost:              cost,
	}, nil
}
