package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Request struct {
	ent.Schema
}

func (Request) Mixins() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Request) Indexes() []ent.Index {
	return []ent.Index{
		// unique index.
		index.Fields("user_id").
			StorageKey("requests_by_user_id").
			Unique(),
	}
}

func (Request) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").NotEmpty().Immutable(),
		field.String("request_body").NotEmpty().Immutable(),
		field.String("response_body"),
		field.Enum("status").Values("pending", "processing", "completed", "failed"),
		field.Int64("deleted_at").Default(0),
	}
}

func (Request) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("requests"),
		edge.From("api_key", APIKey.Type).Ref("requests"),
		edge.To("executions", RequestExecution.Type).
			Annotations(
				entgql.RelayConnection(),
			),
	}
}

func (Request) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}
