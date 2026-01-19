package schema

import (
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/looplj/axonhub/internal/pkg/xtime"
)

// TimeMixin implements the ent.Mixin for sharing
// time fields with package schemas.
type TimeMixin struct {
	// We embed the `mixin.Schema` to avoid
	// implementing the rest of the methods.
	mixin.Schema
}

func (TimeMixin) Fields() []ent.Field {
	nowUTC := func() time.Time {
		return xtime.Now()
	}

	return []ent.Field{
		field.Time("created_at").
			Immutable().
			Default(nowUTC).
			Annotations(
				entgql.OrderField("CREATED_AT"),
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Time("updated_at").
			Default(nowUTC).
			UpdateDefault(nowUTC).
			Annotations(
				entgql.OrderField("UPDATED_AT"),
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}
