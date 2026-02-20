package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/scopes"
)

// AgentMemory holds the schema definition for the AgentMemory entity.
type AgentMemory struct {
	ent.Schema
}

func (AgentMemory) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (AgentMemory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "agent_id", "deleted_at", "created_at").
			StorageKey("agent_memories_by_project_id_agent_id_created_at"),
		index.Fields("project_id", "path").
			StorageKey("agent_memories_by_project_id_path"),
	}
}

func (AgentMemory) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID that this memory belongs to"),
		field.Int("agent_id").
			Optional().
			Comment("Agent ID that this memory belongs to (null means project scope)"),
		field.String("path").
			Comment("Memory path"),
		field.String("content").
			SchemaType(map[string]string{
				dialect.MySQL: "mediumtext",
			}).
			Comment("Memory content"),
		field.String("source").
			Default("").
			Comment("Optional source metadata"),
	}
}

func (AgentMemory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("memories").
			Field("agent_id").
			Unique(),
	}
}

func (AgentMemory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (AgentMemory) Policy() ent.Policy {
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
