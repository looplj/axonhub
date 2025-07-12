package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type APIKey struct {
	ent.Schema
}

func (APIKey) Mixins() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (APIKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").
			StorageKey("api_keys_by_user_id"),
		index.Fields("key").
			StorageKey("api_keys_by_key").
			Unique(),
	}
}

func (APIKey) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
		field.Int("key").Immutable(),
		field.String("name"),
	}
}

func (APIKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Unique().
			Immutable().
			Required().
			Ref("api_keys").Field("user_id"),
		edge.To("requests", Request.Type).
			Annotations(
				entgql.RelayConnection(),
			),
	}
}

func (APIKey) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
	}
}
