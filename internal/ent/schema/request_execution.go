package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/looplj/axonhub/internal/objects"
)

type RequestExecution struct {
	ent.Schema
}

func (RequestExecution) Mixin() []ent.Mixin {
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
		field.Int("user_id").Immutable(),
		field.Int("request_id").Immutable(),
		field.Int("channel_id").Immutable(),
		field.String("model_id").Immutable(),
		field.JSON("request_body", objects.JSONRawMessage{}).Immutable(),
		field.JSON("response_body", objects.JSONRawMessage{}).Optional(),
		field.JSON("response_chunks", []objects.JSONRawMessage{}).Optional(),
		field.String("error_message").Optional(),
		field.Enum("status").Values("pending", "processing", "completed", "failed"),
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
		edge.From("channel", Channel.Type).
			Field("channel_id").
			Ref("executions").
			Required().
			Immutable().
			Unique(),
	}
}

func (RequestExecution) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
	}
}
