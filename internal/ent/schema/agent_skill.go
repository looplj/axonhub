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

// AgentSkill holds the schema definition for the AgentSkill entity.
type AgentSkill struct {
	ent.Schema
}

func (AgentSkill) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (AgentSkill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "skill_id").
			StorageKey("agent_skills_by_agent_id_skill_id").
			Unique(),
		index.Fields("agent_id", "enabled", "order").
			StorageKey("agent_skills_by_agent_id_enabled_order"),
	}
}

func (AgentSkill) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID for scoping"),
		field.Int("agent_id").
			Immutable(),
		field.Int("skill_id").
			Immutable(),
		field.Bool("enabled").
			Default(true),
		field.Int("order").
			Default(0).
			Comment("Skill order in agent"),
		field.String("args").
			Default("").
			Comment("Default args for the skill"),
		field.JSON("metadata", objects.JSONRawMessage{}).
			Optional().
			Comment("Reserved for future extension (JSON)").
			Annotations(
				entgql.Skip(entgql.SkipAll),
			),
	}
}

func (AgentSkill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("skill_bindings").
			Field("agent_id").
			Immutable().
			Required().
			Unique(),
		edge.From("skill", Skill.Type).
			Ref("agent_bindings").
			Field("skill_id").
			Immutable().
			Required().
			Unique(),
		edge.From("project", Project.Type).
			Ref("agent_skill_bindings").
			Field("project_id").
			Immutable().
			Required().
			Unique(),
	}
}

func (AgentSkill) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (AgentSkill) Policy() ent.Policy {
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

