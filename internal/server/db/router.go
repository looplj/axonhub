package db

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
)

// routerDriver wraps two dialect.Driver instances (master and replica).
// See: https://entgo.io/docs/faq/#how-to-configure-two-or-more-db-to-separate-read-and-write
type routerDriver struct {
	master  dialect.Driver
	replica dialect.Driver
}

func newRouterDriver(master, replica dialect.Driver) *routerDriver {
	return &routerDriver{master: master, replica: replica}
}

func (d *routerDriver) Dialect() string { return d.master.Dialect() }

func (d *routerDriver) Exec(ctx context.Context, query string, args, v any) error {
	return d.master.Exec(ctx, query, args, v)
}

func (d *routerDriver) Query(ctx context.Context, query string, args, v any) error {
	// Non-Ent queries (raw SQL) or mutations with RETURNING: fall back to master.
	if ent.QueryFromContext(ctx) == nil {
		return d.master.Query(ctx, query, args, v)
	}
	return d.replica.Query(ctx, query, args, v)
}

func (d *routerDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	return d.master.Tx(ctx)
}

func (d *routerDriver) Close() error {
	err1 := d.master.Close()
	var err2 error
	if d.replica != nil {
		err2 = d.replica.Close()
	}
	if err1 != nil {
		return err1
	}
	return err2
}

var _ dialect.Driver = (*routerDriver)(nil)
