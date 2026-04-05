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

// AgentTool holds the schema definition for the AgentTool entity.
type AgentTool struct {
	ent.Schema
}

func (AgentTool) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (AgentTool) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "tool_id").
			StorageKey("agent_tools_by_agent_id_tool_id").
			Unique(),
		index.Fields("agent_id", "enabled", "order").
			StorageKey("agent_tools_by_agent_id_enabled_order"),
	}
}

func (AgentTool) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID for scoping"),
		field.Int("agent_id").
			Immutable(),
		field.Int("tool_id").
			Immutable(),
		field.Bool("enabled").
			Default(true),
		field.Int("order").
			Default(0).
			Comment("Tool order in agent"),
		field.JSON("config", objects.JSONRawMessage{}).
			Default(objects.JSONRawMessage([]byte("{}"))).
			Comment("Agent-specific tool config override (JSON)"),
	}
}

func (AgentTool) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("tool_bindings").
			Field("agent_id").
			Immutable().
			Required().
			Unique(),
		edge.From("tool", Tool.Type).
			Ref("agent_bindings").
			Field("tool_id").
			Immutable().
			Required().
			Unique(),
		edge.From("project", Project.Type).
			Ref("agent_tool_bindings").
			Field("project_id").
			Immutable().
			Required().
			Unique(),
	}
}

func (AgentTool) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (AgentTool) Policy() ent.Policy {
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

