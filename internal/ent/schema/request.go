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
	"github.com/looplj/axonhub/internal/objects"
	scopes2 "github.com/looplj/axonhub/internal/scopes"
)

type Request struct {
	ent.Schema
}

func (Request) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Request) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").
			StorageKey("requests_by_user_id"),
		index.Fields("api_key_id").
			StorageKey("requests_by_api_key_id"),
	}
}

func (Request) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
		field.Int("api_key_id").
			Optional().
			Immutable().
			Comment("API Key ID of the request, null for the request from the Admin."),
		field.String("model_id").Immutable(),
		field.JSON("request_body", objects.JSONRawMessage{}).Immutable(),
		field.JSON("response_body", objects.JSONRawMessage{}).Optional(),
		field.Enum("status").Values("pending", "processing", "completed", "failed"),
	}
}

func (Request) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("requests").
			Field("user_id").
			Required().
			Immutable().
			Unique(),
		edge.From("api_key", APIKey.Type).Ref("requests").Field("api_key_id").Immutable().Unique(),
		edge.To("executions", RequestExecution.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Request) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 Request 的权限策略.
func (Request) Policy() ent.Policy {
	return privacy.Policy{
		Query: privacy.QueryPolicy{
			scopes2.OwnerRule(),                              // owner 用户可以访问所有请求
			scopes2.UserOwnedQueryRule(),                     // 用户只能查看自己的请求
			scopes2.ReadScopeRule(scopes2.ScopeReadRequests), // 需要 requests 读取权限
		},
		Mutation: privacy.MutationPolicy{
			scopes2.OwnerRule(),                                // owner 用户可以修改所有请求
			scopes2.UserOwnedMutationRule(),                    // 用户只能修改自己的请求
			scopes2.WriteScopeRule(scopes2.ScopeWriteRequests), // 需要 requests 写入权限
		},
	}
}
