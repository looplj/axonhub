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
		index.Fields("agent_id", "thread_row_id", "sequence", "deleted_at").
			StorageKey("agent_messages_by_agent_id_thread_row_id_sequence").
			Unique(),
		index.Fields("agent_id", "status", "deleted_at", "created_at").
			StorageKey("agent_messages_by_agent_id_status_created_at"),
	}
}

func (AgentMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable(),
		field.Int("agent_id").
			Immutable(),
		field.Int("thread_row_id").
			Immutable(),
		field.Enum("direction").
			Values("to_runtime", "to_user").
			Comment("Message direction"),
		field.Enum("sender_type").
			Values("user", "runtime", "system").
			Comment("Message sender type"),
		field.Int("sender_id").
			Optional().
			Nillable().
			Comment("Sender ID, user_id or agent_instance_id"),
		field.JSON("content", objects.JSONRawMessage{}).
			Default(objects.JSONRawMessage([]byte("{}"))).
			Comment("Message content (JSON)"),
		field.Enum("status").
			Values("pending", "acked", "expired").
			Default("pending"),
		field.Int64("sequence").
			Comment("Monotonic sequence in the thread"),
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
		edge.From("thread", Thread.Type).
			Ref("agent_messages").
			Field("thread_row_id").
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

