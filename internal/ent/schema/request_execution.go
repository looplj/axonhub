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
		// The original request to the provider.
		// e.g: the user request via OpenAI request format, but the actual request to the provider with Claude format, the request_body is the Claude request format.
		field.JSON("request_body", objects.JSONRawMessage{}).Immutable(),
		// The final response from the provider.
		// e.g: the provider response with Claude format, and the user expects the response with OpenAI format, the response_body is the Claude response format.
		field.JSON("response_body", objects.JSONRawMessage{}).Optional(),
		// The streaming response chunks from the provider.
		// e.g: the provider response with Claude format, and the user expects the response with OpenAI format, the response_chunks is the Claude response format.
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
