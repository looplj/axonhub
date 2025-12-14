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

// ChannelOverrideTemplate holds the schema definition for the ChannelOverrideTemplate entity.
type ChannelOverrideTemplate struct {
	ent.Schema
}

func (ChannelOverrideTemplate) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (ChannelOverrideTemplate) Indexes() []ent.Index {
	return []ent.Index{
		// Unique template name per user, channel type, and deleted_at
		// This ensures template names are unique within a user's templates for the same channel type
		index.Fields("user_id", "channel_type", "name", "deleted_at").
			StorageKey("channel_override_templates_by_user_type_name").
			Unique(),
	}
}

func (ChannelOverrideTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").
			Immutable().
			Comment("Owner of this template").
			Annotations(
				entgql.Skip(entgql.SkipMutationUpdateInput),
			),
		field.String("name").
			NotEmpty().
			Comment("Template name, unique per user and channel type"),
		field.String("description").
			Optional().
			Comment("Template description"),
		field.String("channel_type").
			Comment("Channel type this template applies to"),
		field.String("override_parameters").
			Default("{}").
			Comment("Override request body parameters as JSON string"),
		field.JSON("override_headers", []objects.HeaderEntry{}).
			Default([]objects.HeaderEntry{}).
			Comment("Override request headers"),
	}
}

func (ChannelOverrideTemplate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("channel_override_templates").
			Field("user_id").
			Unique().
			Required().
			Immutable().
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}

func (ChannelOverrideTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy defines the privacy policy for ChannelOverrideTemplate.
// Templates are private to users - users can only access their own templates.
// Owners can access all templates.
func (ChannelOverrideTemplate) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserOwnedQueryRule(),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserOwnedMutationRule(),
		},
	}
}
