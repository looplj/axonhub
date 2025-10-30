package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

// Thread holds the schema definition for the Thread entity.
type Thread struct {
	ent.Schema
}

func (Thread) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Thread) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id").
			StorageKey("threads_by_project_id"),
		index.Fields("thread_id").
			StorageKey("threads_by_thread_id").
			Unique(),
	}
}

// Fields of the Thread.
func (Thread) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID that this thread belongs to"),
		field.String("thread_id").
			Unique().
			Comment("Unique thread identifier for this thread"),
	}
}

// Edges of the Thread.
func (Thread) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).
			Ref("threads").
			Field("project_id").
			Immutable().
			Required().
			Unique(),
		edge.To("traces", Trace.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Thread) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 Thread 的权限策略.
func (Thread) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeWriteRequests),
			scopes.UserProjectScopeReadRule(scopes.ScopeReadRequests),
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadRequests),
		},
		Mutation: scopes.MutationPolicy{
			scopes.APIKeyScopeMutationRule(scopes.ScopeWriteRequests),
			scopes.UserProjectScopeWriteRule(scopes.ScopeWriteRequests),
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteRequests),
		},
	}
}
