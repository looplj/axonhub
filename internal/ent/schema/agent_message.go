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

// AgentMessage holds the schema definition for the AgentMessage entity.
type AgentMessage struct {
	ent.Schema
}

func (AgentMessage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (AgentMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent_id", "sequence").
			StorageKey("agent_messages_by_agent_id_sequence").
			Unique(),
		index.Fields("agent_id", "agent_instance_id", "status", "created_at").
			StorageKey("agent_messages_by_agent_id_status_created_at"),
		index.Fields("agent_id", "correlation_id").
			StorageKey("agent_messages_by_agent_id_correlation_id"),
		index.Fields("sender_id", "sender_type").
			StorageKey("agent_messages_by_sender_id_sender_type"),
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
			Values("user", "agent", "system", "message_channel").
			Comment("Message sender type"),
		field.Int("sender_id").
			Optional().
			Nillable().
			Comment("Sender ID, user_id or agent_instance_id or message_channel_id"),
		field.Enum("type").
			Values("chat", "approval_request", "approval_result", "system_event").
			Default("chat").
			Comment("Message type for operator/runtime routing"),
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
		field.String("external_message_id").
			Optional().
			Nillable().
			Comment("External platform message ID (e.g., feishu message_id) for inbound message tracking"),
		field.Int("reply_to_message_id").
			Optional().
			Nillable().
			Comment("The agent message id that this message replies to"),
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
		edge.From("message_channel", MessageChannel.Type).
			Ref("messages").
			Field("sender_id").
			Unique(),
	}
}

func (AgentMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
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
