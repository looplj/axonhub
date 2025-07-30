package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/privacy"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/looplj/axonhub/ent/schema/schematype"
	"github.com/looplj/axonhub/scopes"
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

// Policy 定义 User 的权限策略
func (User) Policy() ent.Policy {
	return privacy.Policy{
		Query: privacy.QueryPolicy{
			scopes.OwnerRule(),                      // owner 用户可以访问所有用户
			scopes.ReadScopeRule(scopes.ScopeReadUsers), // 需要 users 读取权限
		},
		Mutation: privacy.MutationPolicy{
			scopes.OwnerRule(),                       // owner 用户可以修改所有用户
			scopes.WriteScopeRule(scopes.ScopeWriteUsers), // 需要 users 写入权限
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
