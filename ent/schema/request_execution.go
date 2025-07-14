package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RequestExecution struct {
	ent.Schema
}

func (RequestExecution) Mixins() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (RequestExecution) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("request_id").
			StorageKey("request_executions_by_request_id").
			Unique(),
	}
}

func (RequestExecution) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").Immutable(),
		field.Int64("request_id").Immutable(),
	}
}

func (RequestExecution) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("request", Request.Type).
			Field("request_id").
			Ref("executions").
			Required().
			Immutable().
			Unique(),
	}
}

func (RequestExecution) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
		entgql.QueryField("request_id"),
	}
}
