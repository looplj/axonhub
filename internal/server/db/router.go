package db

import (
	"context"
	"strings"
	"unicode"

	"entgo.io/ent/dialect"
)

// routerDriver wraps master and replica drivers, routing queries automatically
// based on SQL statement analysis. Transactions always go to master.
type routerDriver struct {
	master  dialect.Driver
	replica dialect.Driver // nil means no replica configured
}

// newRouterDriver creates a router. If replica is nil, falls back to master for all queries.
func newRouterDriver(master, replica dialect.Driver) *routerDriver {
	return &routerDriver{master: master, replica: replica}
}

func (d *routerDriver) Dialect() string { return d.master.Dialect() }

func (d *routerDriver) Exec(ctx context.Context, query string, args, v any) error {
	return d.master.Exec(ctx, query, args, v)
}

func (d *routerDriver) Query(ctx context.Context, query string, args, v any) error {
	// Writes (Exec) always go to master.
	// Reads (Query): use replica if configured and query is read-only.
	if d.replica != nil && isReadOnlyQuery(query) {
		return d.replica.Query(ctx, query, args, v)
	}
	return d.master.Query(ctx, query, args, v)
}

func (d *routerDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	tx, err := d.master.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return &masterTx{tx: tx}, nil
}

func (d *routerDriver) Close() error {
	// Only close the drivers we created; let the underlying *sql.DB leak
	// only if both drivers reference the same *sql.DB (shouldn't happen in practice).
	if d.replica != nil {
		d.replica.Close()
	}
	return nil // master driver is closed via the sql.DB pool lifecycle
}

var _ dialect.Driver = (*routerDriver)(nil)

// masterTx ensures all operations within a transaction run on master,
// avoiding replication lag issues. Returned by routerDriver.Tx().
type masterTx struct {
	tx dialect.Tx
}

func (t *masterTx) Exec(ctx context.Context, query string, args, v any) error {
	return t.tx.Exec(ctx, query, args, v)
}

func (t *masterTx) Query(ctx context.Context, query string, args, v any) error {
	return t.tx.Query(ctx, query, args, v)
}

func (t *masterTx) Commit() error    { return t.tx.Commit() }
func (t *masterTx) Rollback() error  { return t.tx.Rollback() }

var _ dialect.Tx = (*masterTx)(nil)

var _ dialect.Driver = (*routerDriver)(nil)

// isReadOnlyQuery returns true if the SQL statement is a read-only query.
// It examines the first meaningful token after stripping comments and whitespace.
func isReadOnlyQuery(sqlStr string) bool {
	token := nextSQLToken(strings.TrimSpace(sqlStr))
	upper := strings.ToUpper(token)

	// SELECT, WITH (CTE), TABLE (metadata), EXPLAIN, SHOW, DESCRIBE, DESC
	readOnlyPrefixes := []string{
		"SELECT", "WITH", "TABLE", "EXPLAIN", "SHOW",
		"DESCRIBE", "DESC",
	}
	for _, prefix := range readOnlyPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

// nextSQLToken extracts the first token from a SQL string, skipping
// block comments (/* */), line comments (--), and whitespace.
func nextSQLToken(sqlStr string) string {
	var result strings.Builder
	runes := []rune(sqlStr)
	i := 0

	for i < len(runes) {
		r := runes[i]

		// Skip block comment start
		if r == '/' && i+1 < len(runes) && runes[i+1] == '*' {
			i += 2
			for i+1 < len(runes) {
				if runes[i] == '*' && runes[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		// Skip line comment
		if r == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			continue
		}

		// Whitespace: return accumulated token
		if unicode.IsSpace(r) {
			if result.Len() > 0 {
				return result.String()
			}
			i++
			continue
		}

		// Delimiter: stop
		if r == ';' || r == '(' {
			if result.Len() > 0 {
				return result.String()
			}
			i++
			continue
		}

		result.WriteRune(r)
		i++
	}

	return result.String()
}

