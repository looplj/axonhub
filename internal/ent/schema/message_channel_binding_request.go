package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

type MessageChannelBindingRequest struct {
	ent.Schema
}

func (MessageChannelBindingRequest) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (MessageChannelBindingRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("message_channel_id").
			StorageKey("message_channel_binding_requests_by_channel"),
		index.Fields("agent_instance_id").
			StorageKey("message_channel_binding_requests_by_agent_instance"),
		index.Fields("pair_code").
			StorageKey("message_channel_binding_requests_by_pair_code").
			Unique(),
	}
}

func (MessageChannelBindingRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Int("message_channel_id").
			Immutable(),
		field.Int("agent_instance_id").
			Immutable(),
		field.Enum("type").
			Values("pair").
			Default("pair").
			Comment("Type of the binding request"),
		field.String("pair_code").
			Comment("Pair code for request/response matching (e.g. approval request id)").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Enum("status").
			Values("pending", "approved").
			Default("pending").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			).
			Comment("Status of the binding request"),
		field.Time("expires_at").
			Comment("Expiration time for the binding request").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}

func (MessageChannelBindingRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (MessageChannelBindingRequest) Policy() ent.Policy {
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
