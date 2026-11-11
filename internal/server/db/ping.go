package db

import (
	"context"
	"fmt"
)

// Ping opens dsn with the driver of dialectName, checks that the database
// answers, and closes it. It runs no migration, so a caller can test a DSN
// before the server starts on it.
func Ping(ctx context.Context, dialectName, dsn string) error {
	_, sqlDB, err := openDB(dialectName, ensureSQLiteDSN(dialectName, dsn, false), 1, 1, 0, 0)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping %s: %w", dialectName, err)
	}

	return nil
}
