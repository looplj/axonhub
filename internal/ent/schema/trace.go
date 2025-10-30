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

// Trace holds the schema definition for the Trace entity.
type Trace struct {
	ent.Schema
}

func (Trace) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Trace) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id").
			StorageKey("traces_by_project_id"),
		index.Fields("trace_id").
			StorageKey("traces_by_trace_id").
			Unique(),
		index.Fields("thread_id").
			StorageKey("traces_by_thread_id"),
	}
}

// Fields of the Trace.
func (Trace) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID that this trace belongs to"),
		field.String("trace_id").
			Unique().
			Comment("Unique trace identifier"),
		field.Int("thread_id").
			Optional().
			Immutable().
			Comment("Thread ID that this trace belongs to"),
	}
}

// Edges of the Trace.
func (Trace) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).
			Ref("traces").
			Field("project_id").
			Immutable().
			Required().
			Unique(),
		edge.From("thread", Thread.Type).
			Ref("traces").
			Field("thread_id").
			Immutable().
			Unique(),
		edge.To("requests", Request.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Trace) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 Trace 的权限策略.
func (Trace) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			// The API key can query traces if it has write requests scope.
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
