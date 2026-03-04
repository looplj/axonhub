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

type AgentRuntime struct {
	ent.Schema
}

func (AgentRuntime) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (AgentRuntime) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("agent_runtimes_by_name").
			Unique(),
		index.Fields("type", "deleted_at").
			StorageKey("agent_runtimes_by_type"),
	}
}

func (AgentRuntime) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("Runtime name"),
		field.Enum("type").
			Values("vm", "docker", "local").
			Default("vm").
			Comment("Runtime type: vm, docker or local"),
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("Runtime status"),
		field.String("host").
			Default("").
			Comment("Runtime host address"),
		field.String("user").
			Default("").
			Comment("Runtime user for authentication"),
		field.Enum("auth_method").
			Values("password", "ssh_key").
			Default("password").
			Comment("Authentication method: password or ssh_key"),
		field.String("password").
			Default("").
			Sensitive().
			Comment("Runtime password for authentication"),
		field.String("ssh_private_key").
			Default("").
			SchemaType(map[string]string{
				dialect.MySQL: "text",
			}).
			Sensitive().
			Comment("SSH private key for authentication"),
	}
}

func (AgentRuntime) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("instances", AgentInstance.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (AgentRuntime) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (AgentRuntime) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadAgents),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteAgents),
		},
	}
}
