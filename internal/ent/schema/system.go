package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/privacy"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"

	scopes2 "github.com/looplj/axonhub/internal/scopes"
)

// System holds the schema definition for the System entity.
type System struct {
	ent.Schema
}

func (System) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

// Fields of the System.
func (System) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").Unique(),
		field.String("value"),
	}
}

// Edges of the System.
func (System) Edges() []ent.Edge {
	return []ent.Edge{}
}

func (System) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 System 的权限策略
func (System) Policy() ent.Policy {
	return privacy.Policy{
		Query: privacy.QueryPolicy{
			scopes2.OwnerRule(), // owner 用户可以访问所有系统设置
			scopes2.ReadScopeRule(scopes2.ScopeReadSettings), // 需要 settings 读取权限
		},
		Mutation: privacy.MutationPolicy{
			scopes2.OwnerRule(), // owner 用户可以修改所有系统设置
			scopes2.WriteScopeRule(scopes2.ScopeWriteSettings), // 需要 settings 写入权限
		},
	}
}
