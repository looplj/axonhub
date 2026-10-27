package gql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

type throughputQueryCaptureConnector struct {
	queries *throughputQueryCapture
}

func (c *throughputQueryCaptureConnector) Connect(context.Context) (driver.Conn, error) {
	return throughputQueryCaptureConn{queries: c.queries}, nil
}

func (c *throughputQueryCaptureConnector) Driver() driver.Driver {
	return throughputQueryCaptureDriver{}
}

type throughputQueryCaptureDriver struct{}

func (throughputQueryCaptureDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("throughput query capture driver requires a connector")
}

type throughputQueryCapture struct {
	mu      sync.Mutex
	queries []string
}

func (q *throughputQueryCapture) add(query string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queries = append(q.queries, query)
}

func (q *throughputQueryCapture) snapshot() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.queries...)
}

type throughputQueryCaptureConn struct {
	queries *throughputQueryCapture
}

func (throughputQueryCaptureConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not supported")
}

func (throughputQueryCaptureConn) Close() error { return nil }

func (throughputQueryCaptureConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c throughputQueryCaptureConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.queries.add(query)
	return &throughputEmptyRows{}, nil
}

type throughputEmptyRows struct{}

func (*throughputEmptyRows) Columns() []string         { return nil }
func (*throughputEmptyRows) Close() error              { return nil }
func (*throughputEmptyRows) Next([]driver.Value) error { return io.EOF }

func TestFastestResolversUseDialectSpecificRankingQuery(t *testing.T) {
	for _, tc := range []struct {
		name             string
		dialect          string
		resolver         string
		queryMode        string
		otherMode        string
		placeholder      string
		otherPlaceholder string
	}{
		{name: "fastestChannels/postgres", dialect: dialect.Postgres, resolver: "fastestChannels", queryMode: "DISTINCT ON (request_id)", otherMode: "ROW_NUMBER()", placeholder: "$1", otherPlaceholder: "?"},
		{name: "fastestChannels/sqlite", dialect: dialect.SQLite, resolver: "fastestChannels", queryMode: "ROW_NUMBER() OVER (PARTITION BY request_id", otherMode: "DISTINCT ON", placeholder: "?", otherPlaceholder: "$1"},
		{name: "fastestModels/postgres", dialect: dialect.Postgres, resolver: "fastestModels", queryMode: "DISTINCT ON (request_id)", otherMode: "ROW_NUMBER()", placeholder: "$1", otherPlaceholder: "?"},
		{name: "fastestModels/sqlite", dialect: dialect.SQLite, resolver: "fastestModels", queryMode: "ROW_NUMBER() OVER (PARTITION BY request_id", otherMode: "DISTINCT ON", placeholder: "?", otherPlaceholder: "$1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := &throughputQueryCapture{}
			db := sql.OpenDB(&throughputQueryCaptureConnector{queries: capture})
			t.Cleanup(func() { _ = db.Close() })
			client := ent.NewClient(ent.Driver(entsql.OpenDB(tc.dialect, db)))
			t.Cleanup(func() { _ = client.Close() })

			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			resolver := &queryResolver{&Resolver{
				client:        client,
				systemService: biz.NewSystemService(biz.SystemServiceParams{Ent: client}),
			}}
			input := FastestChannelsInput{TimeWindow: "day", Limit: new(5)}
			if tc.resolver == "fastestChannels" {
				_, err := resolver.fastestChannels(ctx, input)
				require.NoError(t, err)
			} else {
				_, err := resolver.fastestModels(ctx, input)
				require.NoError(t, err)
			}

			var rankingQuery string
			for _, query := range capture.snapshot() {
				if strings.Contains(query, "FROM request_executions") {
					rankingQuery = query
					break
				}
			}
			require.NotEmpty(t, rankingQuery, "expected request_executions ranking query; captured: %v", capture.snapshot())
			require.Contains(t, rankingQuery, tc.queryMode)
			require.NotContains(t, rankingQuery, tc.otherMode)
			require.Contains(t, rankingQuery, tc.placeholder)
			require.NotContains(t, rankingQuery, tc.otherPlaceholder)
		})
	}
}
