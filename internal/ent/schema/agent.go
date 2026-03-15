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

// Agent holds the schema definition for the Agent entity.
type Agent struct {
	ent.Schema
}

func (Agent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Agent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "name", "deleted_at").
			StorageKey("agents_by_project_id_name").
			Unique(),
	}
}

// Fields of the Agent.
func (Agent) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID that this agent belongs to").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Int("created_by_user_id").
			Immutable().
			Comment("Created by user").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.String("name").
			Comment("Agent name"),
		field.String("description").
			Default("").
			Comment("Agent description"),
		field.Enum("status").
			Values("enabled", "disabled", "archived").
			Default("disabled").
			Comment("Agent status").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Int("prompt_id").
			Immutable().
			Comment("Prompt ID for the agent system prompt").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.String("model").
			Default("").
			Comment("Model override (empty means default/profile)"),
		field.Enum("reasoning_effort").
			Values("none", "low", "medium", "high").
			Default("none").
			Comment("Reasoning effort level for extended thinking"),
		field.JSON("agent_builtin_tools", []objects.AgentBuiltinTool{}).
			Default([]objects.AgentBuiltinTool{}).
			Comment("Agent built-in tools configuration (JSON)").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.JSON("agent_builtin_skills", []objects.AgentBuiltinSkill{}).
			Default([]objects.AgentBuiltinSkill{}).
			Comment("Agent built-in skills configuration (JSON)").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.JSON("skills_policy", objects.AgentSkillsPolicy{}).
			Default(objects.AgentSkillsPolicy{Add: "open"}).
			Comment("Skill add/install policy (JSON)").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}

// Edges of the Agent.
func (Agent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).
			Ref("agents").
			Field("project_id").
			Immutable().
			Required().
			Unique(),
		edge.From("created_by_user", User.Type).
			Ref("agents").
			Field("created_by_user_id").
			Immutable().
			Required().
			Unique(),
		edge.From("prompt", Prompt.Type).
			Ref("agents").
			Field("prompt_id").
			Immutable().
			Required().
			Unique(),
		edge.To("tool_bindings", AgentTool.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("skill_bindings", AgentSkill.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("instances", AgentInstance.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("threads", AgentThread.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("messages", AgentMessage.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("memories", AgentMemory.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Agent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
	}
}

func (Agent) Policy() ent.Policy {
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
