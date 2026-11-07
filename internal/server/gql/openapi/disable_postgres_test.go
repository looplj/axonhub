package openapi

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/stretchr/testify/require"
)

func TestDisableAPIKeyPostgresTransaction(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("AXONHUB_TEST_POSTGRES_DSN is required")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	schema := fmt.Sprintf("revocation_%d", time.Now().UnixNano())
	_, err = db.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
	})
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	scoped, err := sql.Open("pgx", parsed.String())
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(ent.Driver(entsql.OpenDB("postgres", scoped))))
	mr, fx, ctx, _ := setupOpenAPIClient(t, []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)}, client)
	ctx = authz.NewAPIKeyContext(ctx, fx.serviceAccount.ID, fx.project.ID)
	verifyDisableAPIKeyHTTP(t, mr, fx, ctx, client)
}
