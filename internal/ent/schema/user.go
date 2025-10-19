package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/scopes"
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
		field.Enum("status").Values("activated", "deactivated").Default("activated"),
		field.String("prefer_language").Default("en").Comment("用户偏好语言"),
		field.String("password").Sensitive(),
		field.String("first_name").Default(""),
		field.String("last_name").Default(""),
		field.String("avatar").Optional().Comment("用户头像URL").SchemaType(
			map[string]string{
				// The avatar is stored as base64 image, it is too long to store in varchar, so we use mediumtext to store it.
				dialect.MySQL: "mediumtext",
			},
		),
		field.Bool("is_owner").Default(false),
		field.Strings("scopes").
			Comment("User scopes in system level: write_channels, read_channels, add_users, read_users, etc.").
			Default([]string{}).
			Optional(),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("projects", Project.Type).
			Through("project_users", UserProject.Type).
			Ref("users").
			Annotations(
				entgql.RelayConnection(),
			),
		edge.To("api_keys", APIKey.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("roles", Role.Type).
			Through("user_roles", UserRole.Type).
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

func (User) Policy() ent.Policy {
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
