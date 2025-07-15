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

type Channel struct {
	ent.Schema
}

func (Channel) Mixins() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Channel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").
			StorageKey("channels_by_name").
			Unique(),
	}
}

func (Channel) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("type").Values("openai", "anthropic", "gemini", "deepseek", "doubao", "kimi").Immutable(),
		field.String("base_url"),
		field.String("name"),
		field.String("api_key").Sensitive().NotEmpty(),
		field.Strings("supported_models"),
		field.String("default_test_model"),
		field.JSON("settings", &objects.ChannelSettings{}),
	}
}

func (Channel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("requests", Request.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Channel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}
