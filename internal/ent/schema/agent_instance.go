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

// AgentInstance holds the schema definition for the AgentInstance entity.
type AgentInstance struct {
	ent.Schema
}

func (AgentInstance) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (AgentInstance) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "last_heartbeat_at").
			StorageKey("agent_instances_by_agent_id_last_heartbeat_at"),
		index.Fields("agent_id", "instance_id", "deleted_at").
			StorageKey("agent_instances_by_agent_id_instance_id").
			Unique(),
	}
}

func (AgentInstance) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID that this agent instance belongs to"),
		field.Int("agent_id").
			Immutable(),
		field.String("instance_id").
			Immutable().
			Comment("Runtime generated instance identifier"),
		field.String("name").
			Default("").
			Comment("Human readable name"),
		field.String("platform").
			Default("").
			Comment("Platform information like os/arch"),
		field.String("version").
			Default("").
			Comment("Runtime version"),
		field.Time("last_heartbeat_at").
			SchemaType(map[string]string{
				dialect.MySQL: "datetime(6)",
			}).
			Comment("Last heartbeat timestamp"),
	}
}

func (AgentInstance) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("instances").
			Field("agent_id").
			Immutable().
			Required().
			Unique(),
	}
}

func (AgentInstance) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (AgentInstance) Policy() ent.Policy {
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

