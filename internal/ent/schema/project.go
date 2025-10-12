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

// Project holds the schema definition for the Project entity.
type Project struct {
	ent.Schema
}

func (Project) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Project) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("slug").
			StorageKey("projects_by_slug").
			Unique(),
		index.Fields("name").
			StorageKey("projects_by_name").
			Unique(),
	}
}

// Fields of the Project.
func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("slug").
			Immutable().
			Comment("slug, a human-readable identifier for the project"),
		field.String("name").
			Comment("project name"),
		field.String("description").
			Default("").
			Comment("project description"),
		field.Enum("status").
			Values("active", "archived").
			Default("active").
			Comment("project status"),
	}
}

// Edges of the Project.
func (Project) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("users", User.Type).
			Through("project_users", UserProject.Type).
			StorageKey(edge.Symbol("user_projects_by_user_id_project_id")).
			Annotations(
				entgql.RelayConnection(),
			),
		edge.To("roles", Role.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("api_keys", APIKey.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("requests", Request.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("usage_logs", UsageLog.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Project) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 Project 的权限策略.
func (Project) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(), // owner 用户可以访问所有项目
			scopes.UserReadScopeRule(scopes.ScopeReadProjects), // 需要 projects 读取权限
			// TODO: Add ProjectMemberQueryRule after ent code generation
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(), // owner 用户可以修改所有项目
			scopes.UserWriteScopeRule(scopes.ScopeWriteProjects), // 需要 projects 写入权限
		},
	}
}
