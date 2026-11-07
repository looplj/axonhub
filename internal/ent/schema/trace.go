package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/pkg/xtime"
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
		index.Fields("project_id", "status").
			StorageKey("traces_by_project_id_status"),
		index.Fields("project_id", "last_request_at").
			StorageKey("traces_by_project_id_last_request_at"),
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
		field.Enum("status").
			Values("active", "archived", "retained").
			Default("active").
			Comment("Record status: active (default), archived (hidden), retained (protected from GC)"),
		field.Time("last_request_at").
			Default(xtime.UTCNow).
			Comment("Time of the last request recorded for this trace").
			Annotations(
				entgql.OrderField("LAST_REQUEST_AT"),
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				// Existing rows get a sentinel near the epoch rather than the
				// migration time, so traces that predate this column sort as the
				// oldest instead of looking recently active until they receive a
				// new request. It is a day past the epoch because MySQL/TiDB map
				// this field to TIMESTAMP, which cannot represent 1970-01-01
				// 00:00:00 once the session time zone is applied.
				entsql.DefaultExpr("'1970-01-02 00:00:00'"),
			),
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
			// The API key can query traces in its own project if it has the
			// write requests scope.
			scopes.APIKeyProjectScopeReadRule(scopes.ScopeWriteRequests),
			scopes.UserProjectScopeReadRule(scopes.ScopeReadRequests),
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadRequests),
		},
		Mutation: scopes.MutationPolicy{
			scopes.APIKeyProjectScopeWriteRule(scopes.ScopeWriteRequests),
			scopes.UserProjectScopeWriteRule(scopes.ScopeWriteRequests),
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteRequests),
		},
	}
}
