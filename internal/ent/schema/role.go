package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/scopes"
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
		index.Fields("project_id", "name").
			StorageKey("roles_by_project_id_name").
			Unique(),
		index.Fields("level").
			StorageKey("roles_by_level"),
	}
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.Enum("level").
			Immutable().
			Values("system", "project").
			Default("system").
			Comment("Role level: system or project"),
		field.Int("project_id").
			// It shoule be immutable, but we nedd it to be mutable for now to migrate old data.
			// Immutable().
			Optional().
			Nillable().
			Comment("Project ID for project-level roles, 0 for system roles, it is used to make the role unique in system level."),
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
			Through("user_roles", UserRole.Type).
			Ref("roles").
			Annotations(
				entgql.RelayConnection(),
			),
		edge.From("project", Project.Type).
			Ref("roles").
			Field("project_id").
			Unique(),
	}
}

func (Role) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 Role 的权限策略.
func (Role) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.UserProjectScopeReadRule(scopes.ScopeReadRoles),
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadRoles), // 需要 roles 读取权限
		},
		Mutation: scopes.MutationPolicy{
			scopes.UserProjectScopeWriteRule(scopes.ScopeWriteRoles),
			scopes.OwnerRule(), // owner 用户可以修改所有角色
			scopes.UserWriteScopeRule(scopes.ScopeWriteRoles), // 需要 roles 写入权限
		},
	}
}
