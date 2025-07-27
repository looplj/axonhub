package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/looplj/axonhub/objects"
)

type Request struct {
	ent.Schema
}

func (Request) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Request) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").
			StorageKey("requests_by_user_id"),
		index.Fields("api_key_id").
			StorageKey("requests_by_api_key_id"),
	}
}

func (Request) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
		field.Int("api_key_id").Optional().Immutable(),
		field.String("model_id").Immutable(),
		field.JSON("request_body", objects.JSONRawMessage{}).Immutable(),
		field.JSON("response_body", objects.JSONRawMessage{}).Optional(),
		field.Enum("status").Values("pending", "processing", "completed", "failed"),
	}
}

func (Request) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("requests").Field("user_id").Required().Immutable().Unique(),
		edge.From("api_key", APIKey.Type).Ref("requests").Field("api_key_id").Immutable().Unique(),
		edge.To("executions", RequestExecution.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Request) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}
