package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

// MessageChannelAgentInstance holds the schema definition for the MessageChannelAgentInstance entity.
// This is a join table that represents a many-to-many relationship between MessageChannel and AgentInstance
type MessageChannelAgentInstance struct {
	ent.Schema
}

func (MessageChannelAgentInstance) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (MessageChannelAgentInstance) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("message_channel_id", "agent_instance_id").
			StorageKey("message_channel_agent_instances_by_channel_agent").
			Unique(),
		index.Fields("agent_instance_id").
			StorageKey("message_channel_agent_instances_by_agent_instance"),
	}
}

func (MessageChannelAgentInstance) Fields() []ent.Field {
	return []ent.Field{
		field.Int("message_channel_id").
			Immutable(),
		field.Int("agent_instance_id").
			Immutable(),
		field.Bool("enabled").
			Default(true).
			Comment("Whether this binding is enabled"),
		field.JSON("config", objects.MessageChannelAgentInstanceBinding{}).
			Default(objects.MessageChannelAgentInstanceBinding{}).
			Comment("Channel-specific config for this agent instance binding"),
	}
}

func (MessageChannelAgentInstance) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("message_channel", MessageChannel.Type).
			Ref("agent_instance_bindings").
			Field("message_channel_id").
			Immutable().
			Required().
			Unique(),
		edge.From("agent_instance", AgentInstance.Type).
			Ref("message_channel_bindings").
			Field("agent_instance_id").
			Immutable().
			Required().
			Unique(),
	}
}

func (MessageChannelAgentInstance) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (MessageChannelAgentInstance) Policy() ent.Policy {
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
