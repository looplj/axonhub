package schemahook

import (
	"context"

	"entgo.io/ent/dialect/sql/schema"
)

// V0_4_0 is a schema hook for version 0.4.0.
// Currently no schema modifications are needed for this version.
func V0_4_0(next schema.Creator) schema.Creator {
	return schema.CreateFunc(func(ctx context.Context, tables ...*schema.Table) error {
		// No schema modifications needed for v0.4.0
		return next.Create(ctx, tables...)
	})
}
