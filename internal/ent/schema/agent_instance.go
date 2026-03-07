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
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

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
		index.Fields("agent_id", "name", "deleted_at").
			StorageKey("agent_instances_by_agent_id_name").
			Unique(),
		index.Fields("api_key_id").
			StorageKey("agent_instances_by_api_key_id").
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
		field.Int("agent_host_id").
			Nillable().
			Optional().
			Comment("Agent Host ID (null means unknown/CLI started)"),
		field.String("name").
			Default("").
			Comment("Human readable name"),
		field.String("description").
			Default("").
			Comment("Instance description"),
		field.String("platform").
			Default("").
			Comment("Platform information like os/arch"),

		field.Int("api_key_id").
			Immutable().
			Unique().
			Comment("Service account API key ID bound to this agent instance").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Time("last_heartbeat_at").
			SchemaType(map[string]string{
				dialect.MySQL: "datetime(6)",
			}).
			Comment("Last heartbeat timestamp"),

		field.JSON("deployment", objects.AgentInstanceDeployment{}).
			Optional().
			Comment("Deployment info - host specific deployment details"),
		field.Enum("status").
			Values("pending", "running", "stopped", "error").
			Default("running").
			Comment("Instance status"),
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
		edge.From("host", AgentHost.Type).
			Ref("instances").
			Field("agent_host_id").
			Unique(),
		edge.From("api_key", APIKey.Type).
			Ref("agent_instance").
			Field("api_key_id").
			Immutable().
			Required().
			Unique(),
		edge.To("messages", AgentMessage.Type).Annotations(
			entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			entgql.RelayConnection(),
		),
		edge.To("message_channel_bindings", MessageChannelAgentInstance.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
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
