package gql

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/predicate"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/usagelog"
)

func clientRequestUsagePredicate() predicate.UsageLog {
	return usagelog.Or(
		usagelog.RequestExecutionIDIsNil(),
		usagelog.HasRequestExecutionWith(requestexecution.PurposeEQ(requestexecution.PurposePrimary)),
	)
}

func clientRequestCountExpr(s *sql.Selector) string {
	return fmt.Sprintf(
		"COUNT(DISTINCT CASE WHEN %s IS NULL OR %s IN (SELECT %s FROM %s WHERE %s = '%s') THEN %s END)",
		s.C(usagelog.FieldRequestExecutionID),
		s.C(usagelog.FieldRequestExecutionID),
		requestexecution.FieldID,
		requestexecution.Table,
		requestexecution.FieldPurpose,
		requestexecution.PurposePrimary,
		s.C(usagelog.FieldRequestID),
	)
}

func distinctRequestCountExpr(s *sql.Selector) string {
	return fmt.Sprintf("COUNT(DISTINCT %s)", s.C(usagelog.FieldRequestID))
}

func distinctRequestCountAggregate() ent.AggregateFunc {
	return distinctRequestCountExpr
}

func countClientRequests(ctx context.Context, query *ent.UsageLogQuery) (int, error) {
	type countResult struct {
		Count int `json:"count"`
	}

	var results []countResult
	err := query.
		Where(clientRequestUsagePredicate()).
		Modify(func(s *sql.Selector) {
			s.Select(sql.As(distinctRequestCountExpr(s), "count"))
		}).
		Scan(ctx, &results)
	if err != nil || len(results) == 0 {
		return 0, err
	}

	return results[0].Count, nil
}
