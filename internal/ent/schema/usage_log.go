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

type UsageLog struct {
	ent.Schema
}

func (UsageLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (UsageLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("request_id").
			StorageKey("usage_logs_by_request_id"),
		index.Fields("project_id").
			StorageKey("usage_logs_by_project_id"),
		index.Fields("channel_id").
			StorageKey("usage_logs_by_channel_id"),
		// Performance indexes for analytics queries
		index.Fields("created_at").
			StorageKey("usage_logs_by_created_at"),
		index.Fields("model_id").
			StorageKey("usage_logs_by_model_id"),
		index.Fields("project_id", "created_at").
			StorageKey("usage_logs_by_project_created_at"),
		index.Fields("channel_id", "created_at").
			StorageKey("usage_logs_by_channel_created_at"),
	}
}

func (UsageLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int("request_id").Immutable().Comment("Related request ID"),
		field.Int("project_id").Immutable().Default(1).Comment("Project ID, default to 1 for backward compatibility"),
		field.Int("channel_id").Optional().Comment("Channel ID used for the request"),
		field.String("model_id").Immutable().Comment("Model identifier used for the request"),

		// Core usage metrics from llm.Usage
		field.Int64("prompt_tokens").Default(0).Comment("Number of tokens in the prompt"),
		field.Int64("completion_tokens").Default(0).Comment("Number of tokens in the completion"),
		field.Int64("total_tokens").Default(0).Comment("Total number of tokens used"),

		// Prompt tokens details from llm.PromptTokensDetails
		field.Int64("prompt_audio_tokens").Default(0).Optional().Comment("Number of audio tokens in the prompt"),
		field.Int64("prompt_cached_tokens").Default(0).Optional().Comment("Number of cached tokens in the prompt"),
		field.Int64("prompt_write_cached_tokens").Default(0).Optional(),

		// Completion tokens details from llm.CompletionTokensDetails
		field.Int64("completion_audio_tokens").Default(0).Optional().Comment("Number of audio tokens in the completion"),
		field.Int64("completion_reasoning_tokens").Default(0).Optional().Comment("Number of reasoning tokens in the completion"),
		field.Int64("completion_accepted_prediction_tokens").Default(0).Optional().Comment("Number of accepted prediction tokens"),
		field.Int64("completion_rejected_prediction_tokens").Default(0).Optional().Comment("Number of rejected prediction tokens"),

		// Additional metadata
		field.Enum("source").Values("api", "playground", "test").Default("api").Immutable().Comment("Source of the request"),
		field.String("format").Immutable().Default("openai/chat_completions").Comment("Request format used"),
	}
}

func (UsageLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("request", Request.Type).
			Ref("usage_logs").
			Field("request_id").
			Required().
			Immutable().
			Unique(),
		edge.From("project", Project.Type).
			Ref("usage_logs").
			Field("project_id").
			Immutable().
			Required().
			Unique(),
		edge.From("channel", Channel.Type).
			Ref("usage_logs").
			Field("channel_id").
			Annotations(
				entgql.Directives(forceResolver()),
			).
			Unique(),
	}
}

func (UsageLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy defines the permission policies for UsageLog.
func (UsageLog) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.UserProjectScopeReadRule(scopes.ScopeReadRequests),
			scopes.OwnerRule(), // owner users can access all usage logs
			scopes.UserReadScopeRule(scopes.ScopeReadRequests), // requires requests read permission
		},
		Mutation: scopes.MutationPolicy{
			scopes.APIKeyScopeMutationRule(scopes.ScopeWriteRequests),
			scopes.UserProjectScopeWriteRule(scopes.ScopeWriteRequests),
			scopes.OwnerRule(), // owner users can modify all usage logs
			scopes.UserWriteScopeRule(scopes.ScopeWriteRequests), // requires requests write permission
		},
	}
}
