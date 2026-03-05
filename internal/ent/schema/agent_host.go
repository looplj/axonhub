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

type AgentHost struct {
	ent.Schema
}

func (AgentHost) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (AgentHost) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("agent_hosts_by_name").
			Unique(),
		index.Fields("type", "deleted_at").
			StorageKey("agent_hosts_by_type"),
	}
}

func (AgentHost) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("Host name"),
		field.Enum("type").
			Values("vm", "docker", "local").
			Default("vm").
			Comment("Host type: vm, docker or local"),
		field.Enum("status").
			Values("active", "inactive").
			Default("active").
			Comment("Host status"),
		field.String("addr").
			Default("").
			Comment("Host address"),
		field.String("user").
			Default("").
			Comment("Host user for authentication"),
		field.Enum("auth_method").
			Values("password", "ssh_key").
			Default("password").
			Comment("Authentication method: password or ssh_key"),
		field.String("password").
			Default("").
			Sensitive().
			Comment("Host password for authentication"),
		field.String("ssh_private_key").
			Default("").
			SchemaType(map[string]string{
				dialect.MySQL: "text",
			}).
			Sensitive().
			Comment("SSH private key for authentication"),
	}
}

func (AgentHost) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("instances", AgentInstance.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (AgentHost) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (AgentHost) Policy() ent.Policy {
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
