package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/looplj/axonhub/internal/pkg/sqlite"
)

// openQuotaDB opens a temp-file SQLite database with a provider_quota_status
// table shaped like the production one (text provider_type, soft-delete column).
func openQuotaDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`CREATE TABLE provider_quota_status (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id INTEGER NOT NULL,
		provider_type TEXT NOT NULL,
		status TEXT NOT NULL,
		quota_data TEXT NOT NULL,
		next_reset_at INTEGER NOT NULL DEFAULT 0,
		ready INTEGER NOT NULL DEFAULT 1,
		next_check_at INTEGER NOT NULL,
		account_key TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0,
		deleted_at INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	return db
}

func insertQuotaRow(t *testing.T, db *sql.DB, providerType string, deletedAt int) {
	t.Helper()

	_, err := db.Exec(`INSERT INTO provider_quota_status
		(channel_id, provider_type, status, quota_data, next_reset_at, ready, next_check_at, account_key, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, providerType, "exhausted", "{}", 0, 0, 0, "", 0, 0, deletedAt)
	if err != nil {
		t.Fatalf("insert row %q deleted=%d: %v", providerType, deletedAt, err)
	}
}

func countQuotaRows(t *testing.T, db *sql.DB, providerType string, softDeleted bool) int {
	t.Helper()

	deletedClause := "deleted_at = 0"
	if softDeleted {
		deletedClause = "deleted_at > 0"
	}

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM provider_quota_status
		 WHERE provider_type = ? AND `+deletedClause, providerType).Scan(&count)
	if err != nil {
		t.Fatalf("count %q soft=%v: %v", providerType, softDeleted, err)
	}

	return count
}

func TestCleanupRemovedProviderQuotaTypes(t *testing.T) {
	db := openQuotaDB(t)
	ctx := context.Background()

	insertQuotaRow(t, db, "wafer", 0)
	insertQuotaRow(t, db, "wafer", 1) // soft-deleted
	insertQuotaRow(t, db, "synthetic", 0)

	if err := cleanupRemovedProviderQuotaTypes(ctx, db, "sqlite"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if got := countQuotaRows(t, db, "wafer", false); got != 0 {
		t.Fatalf("live wafer rows remain: %d", got)
	}
	if got := countQuotaRows(t, db, "wafer", true); got != 0 {
		t.Fatalf("soft-deleted wafer rows remain: %d", got)
	}
	if got := countQuotaRows(t, db, "synthetic", false); got != 1 {
		t.Fatalf("synthetic rows = %d, want 1", got)
	}

	// Idempotent: a second run does nothing and returns no error.
	if err := cleanupRemovedProviderQuotaTypes(ctx, db, "sqlite"); err != nil {
		t.Fatalf("second cleanup: %v", err)
	}
}

func TestCleanupRemovedProviderQuotaTypes_AbsentTable(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "empty.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := cleanupRemovedProviderQuotaTypes(context.Background(), db, "sqlite"); err != nil {
		t.Fatalf("cleanup on empty db: %v", err)
	}
}

func TestCleanupRemovedProviderQuotaTypes_QueryError(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "closed.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if err := cleanupRemovedProviderQuotaTypes(context.Background(), db, "sqlite"); err == nil {
		t.Fatal("expected error from closed db, got nil")
	}
}

func TestDialectFamily(t *testing.T) {
	cases := map[string]string{
		"sqlite3":         "sqlite",
		"sqlite":          "sqlite",
		"mysql":           "mysql",
		"tidb":            "mysql",
		"postgres":        "postgres",
		"pgx":             "postgres",
		"postgresdb":      "postgres",
		"pg":              "postgres",
		"postgresql":      "postgres",
		"unknown-dialect": "unknown-dialect",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := dialectFamily(in); got != want {
				t.Fatalf("dialectFamily(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestPlaceholderList(t *testing.T) {
	if got := placeholderList("mysql", 3); got != "?, ?, ?" {
		t.Fatalf("mysql placeholders = %q, want \"?, ?, ?\"", got)
	}
	if got := placeholderList("postgres", 3); got != "$1, $2, $3" {
		t.Fatalf("postgres placeholders = %q, want \"$1, $2, $3\"", got)
	}
}
