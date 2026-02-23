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

// AgentMessage holds the schema definition for the AgentMessage entity.
type AgentMessage struct {
	ent.Schema
}

func (AgentMessage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (AgentMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "sequence", "deleted_at").
			StorageKey("agent_messages_by_agent_id_sequence").
			Unique(),
		index.Fields("agent_id", "agent_instance_id", "status", "deleted_at", "created_at").
			StorageKey("agent_messages_by_agent_id_status_created_at"),
		index.Fields("agent_id", "correlation_id", "deleted_at").
			StorageKey("agent_messages_by_agent_id_correlation_id"),
	}
}

func (AgentMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable(),
		field.Int("agent_id").
			Immutable(),
		field.Int("agent_instance_id").
			Immutable().
			Comment("Agent instance ID if the message is from an agent"),
		field.Enum("direction").
			Values("to_agent", "to_user").
			Comment("Message direction"),
		field.Enum("sender_type").
			Values("user", "agent", "system").
			Comment("Message sender type"),
		field.Int("sender_id").
			Optional().
			Nillable().
			Comment("Sender ID, user_id or agent_instance_id"),
		field.Enum("kind").
			Values("chat", "approval_request", "approval_result", "system_event").
			Default("chat").
			Comment("Message kind for operator/runtime routing"),
		field.String("correlation_id").
			Default("").
			Comment("Correlation ID for request/response matching (e.g. approval request id)"),
		field.JSON("content", objects.JSONRawMessage{}).
			Default(objects.JSONRawMessage([]byte("{}"))).
			Comment("Message content (JSON)"),
		field.Enum("status").
			Values("pending", "acked", "expired").
			Default("pending"),
		field.Int64("sequence").
			Comment("Monotonic sequence for the agent"),
		field.Time("expires_at").
			Optional().
			Nillable(),
	}
}

func (AgentMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("agent", Agent.Type).
			Ref("messages").
			Field("agent_id").
			Immutable().
			Required().
			Unique(),
		edge.From("agent_instance", AgentInstance.Type).
			Ref("messages").
			Field("agent_instance_id").
			Immutable().
			Required().
			Unique(),
	}
}

func (AgentMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (AgentMessage) Policy() ent.Policy {
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
