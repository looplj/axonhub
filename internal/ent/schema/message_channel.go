package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type MessageChannel struct {
	ent.Schema
}

func (MessageChannel) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (MessageChannel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "name", "deleted_at").
			StorageKey("message_channels_by_project_name").
			Unique(),
	}
}

func (MessageChannel) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID that this message channel belongs to").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.String("name").
			Comment("Human readable name for this message channel"),
		field.String("description").
			Default("").
			Comment("Description of this message channel"),
		field.Enum("type").
			Values("feishu").
			Default("feishu").
			Annotations(
				entgql.OrderField("TYPE"),
			).
			Comment("Message channel type"),
		field.Enum("status").
			Values("enabled", "disabled").
			Default("enabled").
			Annotations(
				entgql.OrderField("STATUS"),
			),
		field.JSON("settings", objects.MessageChannelSettings{}).
			Default(objects.MessageChannelSettings{}).
			Optional().
			Comment("Channel-specific settings"),
	}
}

func (MessageChannel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).
			Ref("message_channels").
			Field("project_id").
			Immutable().
			Required().
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			).
			Unique(),
		edge.To("agent_instance_bindings", MessageChannelAgentInstance.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("messages", AgentMessage.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (MessageChannel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (MessageChannel) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.UserProjectScopeReadRule(scopes.ScopeReadAgents),
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadAgents),
		},
		Mutation: scopes.MutationPolicy{
			scopes.UserProjectScopeWriteRule(scopes.ScopeWriteAgents),
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteAgents),
		},
	}
}
