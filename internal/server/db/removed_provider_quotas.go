package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/log"
)

// removedProviderQuotaTypes lists provider_type values that were removed from
// the provider_quota_status schema. Ent stores the field as a real ENUM on
// MySQL/TiDB, so removing a value rewrites the column in place. Any surviving
// row holding a removed value would therefore fail the ALTER under strict
// mode or be truncated to an empty string — both corrupt the upgrade. The
// rows are therefore purged before schema creation (see
// cleanupRemovedProviderQuotaTypes) so the enum DDL never sees them.
var removedProviderQuotaTypes = []string{"wafer"}

// dialectFamily maps every config dialect alias (see driverName/entDialect)
// to the driver family the raw SQL in this file is written for.
func dialectFamily(dialectName string) string {
	switch dialectName {
	case "sqlite3", "sqlite":
		return "sqlite"
	case "mysql", "tidb":
		return "mysql"
	case "postgres", "pgx", "postgresdb", "pg", "postgresql":
		return "postgres"
	default:
		return dialectName
	}
}

// placeholderList renders n bind placeholders in the syntax of the given
// family: "?" for SQLite/MySQL and "$1..$n" for PostgreSQL (pgx).
func placeholderList(family string, n int) string {
	placeholders := make([]string, n)
	for i := range placeholders {
		if family == "postgres" {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		} else {
			placeholders[i] = "?"
		}
	}
	return strings.Join(placeholders, ", ")
}

// tableExists reports whether the given table exists in the current database,
// scoped per dialect so the check resolves exactly like the unqualified
// DELETE below.
func tableExists(ctx context.Context, masterDB *sql.DB, family, table string) (bool, error) {
	var query string
	switch family {
	case "sqlite":
		query = `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`
	case "mysql":
		query = `SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`
	case "postgres":
		// Restrict to the current search path, i.e. the schema the
		// unqualified DELETE resolves against.
		query = `SELECT 1 FROM information_schema.tables
			WHERE table_schema = ANY(current_schemas(false))
			  AND table_name = $1`
	default:
		return false, fmt.Errorf("cleanupRemovedProviderQuotaTypes: unknown dialect family %q", family)
	}

	var one any
	err := masterDB.QueryRowContext(ctx, query, table).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// cleanupRemovedProviderQuotaTypes hard-deletes provider_quota_status rows whose
// provider_type is no longer part of the enum, before Ent's schema migration
// runs. A quota row is not worth salvaging once its provider is retired, so
// the delete deliberately includes soft-deleted rows.
//
// The function is fail-closed: any query error is returned so NewEntClient
// aborts startup. The enum DDL must not run after a failed cleanup — that is
// exactly the truncation hazard this exists to prevent. A missing table
// (fresh install) is not an error.
func cleanupRemovedProviderQuotaTypes(ctx context.Context, masterDB *sql.DB, dialectName string) error {
	const table = "provider_quota_status"
	family := dialectFamily(dialectName)

	exists, err := tableExists(ctx, masterDB, family, table)
	if err != nil {
		return fmt.Errorf("check %s existence: %w", table, err)
	}
	if !exists {
		return nil
	}

	// The table name is a package constant and the list holds only bind
	// placeholders; no user input reaches the query text.
	//nolint:gosec // G202, SQL assembled from a constant and bind placeholders only
	query := "DELETE FROM " + table + " WHERE provider_type IN (" +
		placeholderList(family, len(removedProviderQuotaTypes)) + ")"

	args := make([]any, len(removedProviderQuotaTypes))
	for i, providerType := range removedProviderQuotaTypes {
		args[i] = providerType
	}

	res, err := masterDB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete removed provider quota rows: %w", err)
	}

	if affected, err := res.RowsAffected(); err == nil {
		log.Info(ctx, "purged provider quota rows for removed providers",
			zap.String("table", table),
			zap.Strings("providers", removedProviderQuotaTypes),
			zap.Int64("affected", affected),
		)
	}
	return nil
}
