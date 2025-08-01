package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/privacy"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"

	scopes2 "github.com/looplj/axonhub/internal/scopes"
)

// Role holds the schema definition for the Role entity.
type Role struct {
	ent.Schema
}

func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Role) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").
			StorageKey("roles_by_code").
			Unique(),
	}
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").Unique().Immutable(),
		field.String("name"),
		field.Strings("scopes").
			Comment("Available scopes for this role: write_channels, read_channels, add_users, read_users, etc.").
			Default([]string{}).
			Optional(),
	}
}

// Edges of the Role.
func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("users", User.Type).
			Ref("roles").
			Annotations(
				entgql.RelayConnection(),
			),
	}
}

func (Role) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 Role 的权限策略
func (Role) Policy() ent.Policy {
	return privacy.Policy{
		Query: privacy.QueryPolicy{
			scopes2.OwnerRule(),
			scopes2.UserOwnedQueryRule(),                  // 用户可以查询自己的角色
			scopes2.ReadScopeRule(scopes2.ScopeReadRoles), // 需要 roles 读取权限
		},
		Mutation: privacy.MutationPolicy{
			scopes2.OwnerRule(),                             // owner 用户可以修改所有角色
			scopes2.WriteScopeRule(scopes2.ScopeWriteRoles), // 需要 roles 写入权限
		},
	}
}
