package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/scopes"
)

// OIDCIdentity holds the schema definition for the OIDCIdentity entity.
type OIDCIdentity struct {
	ent.Schema
}

func (OIDCIdentity) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

// Fields of the OIDCIdentity.
func (OIDCIdentity) Fields() []ent.Field {
	return []ent.Field{
		field.String("issuer").
			Comment("OIDC provider issuer URL"),
		field.String("subject").
			Comment("OIDC subject identifier"),
		field.String("email").
			Optional().
			Comment("Email from OIDC provider"),
		field.String("idp_name").
			Optional().
			Comment("Identity provider name"),
		field.Time("last_login_at").
			Optional().
			Nillable().
			Comment("Last login timestamp"),
		field.Int("user_id").
			Comment("Reference to the User entity"),
	}
}

// Edges of the OIDCIdentity.
func (OIDCIdentity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Field("user_id").
			Required().
			Unique().
			Annotations(
				entsql.OnDelete(entsql.Cascade),
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}

// Indexes of the OIDCIdentity.
func (OIDCIdentity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("issuer", "subject", "deleted_at").
			StorageKey("oidc_identities_by_issuer_subject_deleted_at").
			Unique(),
		index.Fields("user_id").
			StorageKey("oidc_identities_by_user_id"),
	}
}

func (OIDCIdentity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (OIDCIdentity) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadUsers),
			scopes.UserOwnedQueryRule(),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteUsers),
			scopes.UserOwnedMutationRule(),
		},
	}
}
