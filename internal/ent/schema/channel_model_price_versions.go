package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type ChannelModelPriceVersion struct {
	ent.Schema
}

func (ChannelModelPriceVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (ChannelModelPriceVersion) Indexes() []ent.Index {
	return []ent.Index{}
}

func (ChannelModelPriceVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Immutable(),
		field.String("model_id").Immutable(),
		field.Int("channel_model_price_id").Immutable(),
		field.JSON("price", objects.ModelPrice{}).Comment("The model price, if changed, it will genearte a new reference id.").Immutable(),
		field.Enum("status").Values("active", "archived"),
		field.Time("effective_start_at").
			Comment("The effective start time of the model price.").
			Immutable(),
		field.Time("effective_end_at").
			Comment("The effective end time of the model price, null means it is effective until the next version.").
			Optional().
			Nillable(),
		field.String("refreance_id").
			Comment("The bill should reference this id.").
			Unique().
			Immutable(),
	}
}

func (ChannelModelPriceVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel_model_price", ChannelModelPrice.Type).
			Ref("versions").
			Field("channel_model_price_id").
			Required().
			Immutable().
			Unique(),
	}
}

func (ChannelModelPriceVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
	}
}

func (ChannelModelPriceVersion) Policy() ent.Policy {
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
