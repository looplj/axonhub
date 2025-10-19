package schema

import (
	"time"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
)

// UserRole holds the schema definition for the UserRole entity.
type UserRole struct {
	ent.Schema
}

func (UserRole) Mixin() []ent.Mixin {
	return []ent.Mixin{
		schematype.SoftDeleteMixin{},
	}
}

func (UserRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "role_id", "deleted_at").
			StorageKey("user_roles_by_user_id_role_id").
			Unique(),
		index.Fields("role_id").
			StorageKey("user_roles_by_role_id"),
	}
}

// Fields of the UserRole.
func (UserRole) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").
			Immutable(),
		field.Int("role_id").
			Immutable(),
		// Mark as nullable for compatibility with old data.
		field.Time("created_at").
			Optional().
			Nillable().
			Default(time.Now).
			Annotations(
				entgql.OrderField("CREATED_AT"),
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Time("updated_at").
			Optional().
			Nillable().
			Default(time.Now).
			UpdateDefault(time.Now).
			Annotations(
				entgql.OrderField("UPDATED_AT"),
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}

func (UserRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Field("user_id").
			Immutable().
			Unique().
			Required(),
		edge.To("role", Role.Type).
			Field("role_id").
			Immutable().
			Unique().
			Required(),
	}
}
