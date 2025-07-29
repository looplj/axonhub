package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/looplj/axonhub/ent/schema/schematype"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").Unique(),
		field.String("password").Sensitive(),
		field.String("first_name").Default(""),
		field.String("last_name").Default(""),
		field.Bool("is_owner").Default(false),
		field.Strings("scopes").
			Comment("User-specific scopes: write_channels, read_channels, add_users, read_users, etc.").
			Default([]string{}).
			Optional(),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("requests", Request.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("api_keys", APIKey.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("roles", Role.Type).
			Annotations(
				entgql.RelayConnection(),
			),
	}
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

type UserOwnedMixin struct {
	mixin.Schema
}

func (UserOwnedMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
	}
}

func (UserOwnedMixin) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Field("user_id").Unique(),
	}
}
