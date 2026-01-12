package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type Model struct {
	ent.Schema
}

func (Model) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Model) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("models_by_name").
			Unique(),
		index.Fields("model_id", "deleted_at").
			StorageKey("models_by_model_id").
			Unique(),
	}
}

func (Model) Fields() []ent.Field {
	return []ent.Field{
		field.String("developer").Immutable().Comment("developer of the model, eg. deeepseek"),
		field.String("model_id").Immutable().Comment("model id, eg. deeepseek-chat"),
		field.Enum("type").Immutable().Values("chat", "embedding", "rerank").Default("chat").Comment("model type"),
		field.String("name").Comment("model name, eg. DeepSeek Chat").
			Annotations(
				entgql.OrderField("NAME"),
			),
		field.String("icon").Comment("icon of the model from the lobe-icons, eg. DeepSeek"),
		field.String("group").Comment("model group, eg. deepseek"),
		field.JSON("model_card", &objects.ModelCard{}),
		field.JSON("settings", &objects.ModelSettings{}),
		field.Enum("status").Values("enabled", "disabled", "archived").Default("disabled").Annotations(
			entgql.Skip(entgql.SkipMutationCreateInput),
		),
		field.String("remark").
			Optional().Nillable().
			Comment("User-defined remark or note for the Model"),
	}
}

func (Model) Edges() []ent.Edge {
	return []ent.Edge{}
}

func (Model) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (Model) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadChannels),
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadChannels),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels),
		},
	}
}
