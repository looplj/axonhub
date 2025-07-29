package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/looplj/axonhub/ent/schema/schematype"
)

// System holds the schema definition for the System entity.
type System struct {
	ent.Schema
}

func (System) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (System) Indexes() []ent.Index {
	return []ent.Index{}
}

// Fields of the System.
func (System) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").Unique().Immutable(),
		field.String("value"),
	}
}

// Edges of the System.
func (System) Edges() []ent.Edge {
	return []ent.Edge{}
}

func (System) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}
