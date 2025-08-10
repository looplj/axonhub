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

type APIKey struct {
	ent.Schema
}

func (APIKey) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (APIKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").
			StorageKey("api_keys_by_user_id"),
		index.Fields("key").
			StorageKey("api_keys_by_key").
			Unique(),
	}
}

func (APIKey) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
		field.String("key").Immutable(),
		field.String("name"),
		field.Enum("status").Values("enabled", "disabled").Default("enabled"),
	}
}

func (APIKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Unique().
			Immutable().
			Required().
			Ref("api_keys").Field("user_id"),
		edge.To("requests", Request.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (APIKey) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 APIKey 的权限策略.
func (APIKey) Policy() ent.Policy {
	return privacy.Policy{
		Query: privacy.QueryPolicy{
			scopes2.OwnerRule(),                             // owner 用户可以访问所有 API Keys
			scopes2.UserOwnedQueryRule(),                    // 用户只能查看自己的 API Keys
			scopes2.ReadScopeRule(scopes2.ScopeReadAPIKeys), // 需要 API Keys 读取权限
		},
		Mutation: privacy.MutationPolicy{
			scopes2.OwnerRule(),                               // owner 用户可以修改所有 API Keys
			scopes2.UserOwnedMutationRule(),                   // 用户只能修改自己的 API Keys
			scopes2.WriteScopeRule(scopes2.ScopeWriteAPIKeys), // 需要 API Keys 写入权限
		},
	}
}
