package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

// AgentThread holds the schema definition for the AgentThread entity.
type AgentThread struct {
	ent.Schema
}

func (AgentThread) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (AgentThread) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "thread_row_id").
			StorageKey("agent_threads_by_agent_id_thread_row_id").
			Unique(),
	}
}

func (AgentThread) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable(),
		field.Int("agent_id").
			Immutable(),
		field.Int("thread_row_id").
			Immutable(),
	}
}

func (AgentThread) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("thread_bindings").
			Field("agent_id").
			Immutable().
			Required().
			Unique(),
		edge.From("thread", Thread.Type).
			Ref("agent_threads").
			Field("thread_row_id").
			Immutable().
			Required().
			Unique(),
	}
}

func (AgentThread) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate()),
	}
}

func (AgentThread) Policy() ent.Policy {
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
