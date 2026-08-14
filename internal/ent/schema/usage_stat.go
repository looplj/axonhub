package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/scopes"
)

// UsageStat holds day-granularity usage aggregates independent of the
// usage_logs detail table. Dashboard and analytics queries read from this
// table so that GC log cleanup (deleting old usage_logs rows) never erases
// historical statistics.
type UsageStat struct {
	ent.Schema
}

func (UsageStat) Fields() []ent.Field {
	return []ent.Field{
		field.String("date").Immutable().Comment("Local calendar date (YYYY-MM-DD) the usage belongs to"),
		field.Int("api_key_id").Default(0).Immutable().Comment("API key ID; 0 means no API key"),
		field.Int("project_id").Default(1).Immutable().Comment("Project ID"),
		field.Int("channel_id").Default(0).Immutable().Comment("Channel ID; 0 means no channel"),
		field.String("model_id").Immutable().Comment("Model identifier used for the request"),

		field.Int64("request_count").Default(0).Comment("Number of requests aggregated"),
		field.Int64("prompt_tokens").Default(0).Comment("Sum of prompt tokens"),
		field.Int64("completion_tokens").Default(0).Comment("Sum of completion tokens"),
		field.Int64("total_tokens").Default(0).Comment("Sum of total tokens"),
		field.Int64("prompt_cached_tokens").Default(0).Comment("Sum of cached prompt tokens"),
		field.Int64("prompt_write_cached_tokens").Default(0).Comment("Sum of write cache tokens"),
		field.Int64("completion_reasoning_tokens").Default(0).Comment("Sum of reasoning tokens"),
		field.Float("total_cost").Default(0).Comment("Sum of total cost"),
	}
}

func (UsageStat) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("date", "api_key_id", "model_id", "channel_id", "project_id").
			Unique().
			StorageKey("usage_stats_unique"),
		index.Fields("date").StorageKey("usage_stats_by_date"),
		index.Fields("api_key_id", "date").StorageKey("usage_stats_by_api_key_date"),
		index.Fields("model_id", "date").StorageKey("usage_stats_by_model_date"),
		index.Fields("channel_id", "date").StorageKey("usage_stats_by_channel_date"),
		index.Fields("project_id", "date").StorageKey("usage_stats_by_project_date"),
	}
}

func (UsageStat) Annotations() []schema.Annotation {
	return nil
}

// Policy defines the permission policies for UsageStat. It mirrors the
// UsageLog query policy so internal dashboard aggregation through the
// privacy-enforced client keeps the same access semantics as before.
func (UsageStat) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.UserProjectScopeReadRequestsRule(scopes.ScopeReadRequests),
			scopes.OwnerRule(),                      // owner users can access all usage stats
			scopes.UserReadScopeRule(scopes.ScopeReadRequests), // requires requests read permission
		},
		Mutation: scopes.MutationPolicy{
			scopes.APIKeyScopeMutationRule(scopes.ScopeWriteRequests),
			scopes.UserProjectScopeWriteRule(scopes.ScopeWriteRequests),
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteRequests),
		},
	}
}
