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

type ChannelModelPrice struct {
	ent.Schema
}

func (ChannelModelPrice) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (ChannelModelPrice) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "model_id", "deleted_at").
			StorageKey("channel_model_prices_by_channel_id_model_id").
			Unique(),
	}
}

func (ChannelModelPrice) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Immutable(),
		field.String("model_id").Immutable(),
		field.JSON("price", objects.ModelPrice{}).Comment("The model price, if changed, it will genearte a new reference id."),
		field.String("reference_id").Comment("The bill should reference this id.").Unique(),
	}
}

func (ChannelModelPrice) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("channel_model_prices").
			Field("channel_id").
			Required().
			Immutable().
			Unique(),
		edge.To("versions", ChannelModelPriceVersion.Type),
	}
}

func (ChannelModelPrice) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
	}
}

func (ChannelModelPrice) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadChannels),
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadChannels),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels),
		},
	}
}
