package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

type Job struct {
	ent.Schema
}

func (Job) Mixins() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Job) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id", "type").
			StorageKey("jobs_by_owner_id_type").
			Unique(),
	}
}

func (Job) Fields() []ent.Field {
	return []ent.Field{
		field.Int("owner_id").Immutable(),
		field.String("type").Immutable(),
		field.String("context"),
	}
}

// Policy 定义 Job 的权限策略.
func (Job) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(), // owner 用户可以访问所有任务
			scopes.UserReadScopeRule(scopes.ScopeReadJobs), // 需要 jobs 读取权限
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(), // owner 用户可以修改所有任务
			scopes.UserWriteScopeRule(scopes.ScopeWriteJobs), // 需要 jobs 写入权限
		},
	}
}
