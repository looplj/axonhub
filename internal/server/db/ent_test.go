package db

import (
	"context"
	"database/sql"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"
)

// TestAssertNoDuplicateAPIKeyNames_SQLite verifies the pre-migration guard that
// blocks creating the api_keys_by_project_name unique index when duplicate live
// names already exist. It must report the conflict without mutating any rows.
// Runs on SQLite (the default dialect and the dialect used by tests).
func TestAssertNoDuplicateAPIKeyNames_SQLite(t *testing.T) {
	ctx := context.Background()

	db, err := sql.Open("sqlite3", "file:assert_dup_test?mode=memory&_fk=1")
	require.NoError(t, err)
	defer db.Close()

	// Single connection so the in-memory DB persists across queries.
	db.SetMaxOpenConns(1)

	// No-op when the api_keys table doesn't exist yet (fresh database).
	require.NoError(t, assertNoDuplicateAPIKeyNames(ctx, db, dialect.SQLite))

	_, err = db.ExecContext(ctx, `CREATE TABLE api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		deleted_at INTEGER NOT NULL DEFAULT 0
	)`)
	require.NoError(t, err)

	// Unique names + a soft-deleted same-name row (deleted_at != 0) must NOT count.
	_, err = db.ExecContext(ctx, `INSERT INTO api_keys (project_id, name, deleted_at) VALUES
		(1,'a',0),(1,'b',0),(2,'a',0),(1,'a',12345)`)
	require.NoError(t, err)
	require.NoError(t, assertNoDuplicateAPIKeyNames(ctx, db, dialect.SQLite))

	// Introduce two live duplicate groups.
	_, err = db.ExecContext(ctx, `INSERT INTO api_keys (project_id, name, deleted_at) VALUES
		(1,'a',0),(2,'a',0),(2,'a',0)`)
	require.NoError(t, err)

	err = assertNoDuplicateAPIKeyNames(ctx, db, dialect.SQLite)
	require.Error(t, err)
	// Message is actionable: names the index, the conflicting groups, and the ids.
	require.Contains(t, err.Error(), "api_keys_by_project_name")
	require.Contains(t, err.Error(), `project_id=1 name="a"`)
	require.Contains(t, err.Error(), `project_id=2 name="a"`)
	require.Contains(t, err.Error(), "Rename or disable")

	// Critically: it must NOT have mutated anything (no soft-delete of keys).
	var live int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE deleted_at = 0`).Scan(&live))
	require.Equal(t, 6, live) // 4 from first insert (1 of which was soft-deleted) -> 3 live, + 3 new = 6
}
