package gql

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

var errCaptureAnalyticsPerformanceQuery = errors.New("capture analytics performance query")

// postgresQueryCaptureDriver captures the SQL emitted for a PostgreSQL analytics query.
type postgresQueryCaptureDriver struct {
	query string
}

// Exec stops unexpected write operations because this driver is only used for SELECT generation.
func (*postgresQueryCaptureDriver) Exec(context.Context, string, any, any) error {
	return errCaptureAnalyticsPerformanceQuery
}

// Query captures the generated SQL and stops execution before a database connection is required.
func (d *postgresQueryCaptureDriver) Query(_ context.Context, query string, _ any, _ any) error {
	d.query = query
	return errCaptureAnalyticsPerformanceQuery
}

// Tx is unsupported because the analytics performance query does not open transactions.
func (*postgresQueryCaptureDriver) Tx(context.Context) (dialect.Tx, error) {
	return nil, errCaptureAnalyticsPerformanceQuery
}

// Close has no resources to release for the capture-only driver.
func (*postgresQueryCaptureDriver) Close() error {
	return nil
}

// Dialect makes Ent render the production query with PostgreSQL identifier rules.
func (*postgresQueryCaptureDriver) Dialect() string {
	return dialect.Postgres
}

// TestAnalyticsPerformanceStatsPostgresQualifiesUsageLogFilters verifies that
// filters remain qualified after the performance query joins executions.
func TestAnalyticsPerformanceStatsPostgresQualifiesUsageLogFilters(t *testing.T) {
	driver := &postgresQueryCaptureDriver{}
	resolver := &queryResolver{&Resolver{client: ent.NewClient(ent.Driver(driver))}}

	ctx := authz.WithTestBypass(t.Context())
	filter := &AnalyticsFilter{
		ProjectIDs: []*objects.GUID{{ID: 1}},
		ChannelIDs: []*objects.GUID{{ID: 2}},
		ModelIDs:   []string{"test-model"},
	}
	_, err := resolver.queryDimensionPerformanceStats(ctx, filter, []int{3}, false, nil, "apiKey")
	require.ErrorIs(t, err, errCaptureAnalyticsPerformanceQuery)
	require.Contains(t, driver.query, `"usage_logs"."project_id" IN`)
	require.Contains(t, driver.query, `"usage_logs"."channel_id" IN`)
	require.Contains(t, driver.query, `"usage_logs"."model_id" IN`)
	require.Contains(t, driver.query, `"usage_logs"."api_key_id" IN`)
	require.NotContains(t, driver.query, ` AND "project_id" IN`)
	require.NotContains(t, driver.query, ` AND "channel_id" IN`)
	require.NotContains(t, driver.query, ` AND "model_id" IN`)
	require.NotContains(t, driver.query, ` AND "api_key_id" IN`)
}
