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

type UsageLog struct {
	ent.Schema
}

func (UsageLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (UsageLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").
			StorageKey("usage_logs_by_user_id"),
		index.Fields("request_id").
			StorageKey("usage_logs_by_request_id"),
		index.Fields("channel_id").
			StorageKey("usage_logs_by_channel_id"),
		// Performance indexes for analytics queries
		index.Fields("created_at").
			StorageKey("usage_logs_by_created_at"),
		index.Fields("model_id").
			StorageKey("usage_logs_by_model_id"),
		// Composite index for cost analysis
		index.Fields("user_id", "created_at").
			StorageKey("usage_logs_by_user_created_at"),
		index.Fields("channel_id", "created_at").
			StorageKey("usage_logs_by_channel_created_at"),
	}
}

func (UsageLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable().Comment("User ID who made the request"),
		field.Int("request_id").Immutable().Comment("Related request ID"),
		field.Int("channel_id").Optional().Comment("Channel ID used for the request"),
		field.String("model_id").Immutable().Comment("Model identifier used for the request"),

		// Core usage metrics from llm.Usage
		field.Int("prompt_tokens").Default(0).Comment("Number of tokens in the prompt"),
		field.Int("completion_tokens").Default(0).Comment("Number of tokens in the completion"),
		field.Int("total_tokens").Default(0).Comment("Total number of tokens used"),

		// Prompt tokens details from llm.PromptTokensDetails
		field.Int("prompt_audio_tokens").Default(0).Optional().Comment("Number of audio tokens in the prompt"),
		field.Int("prompt_cached_tokens").Default(0).Optional().Comment("Number of cached tokens in the prompt"),

		// Completion tokens details from llm.CompletionTokensDetails
		field.Int("completion_audio_tokens").Default(0).Optional().Comment("Number of audio tokens in the completion"),
		field.Int("completion_reasoning_tokens").Default(0).Optional().Comment("Number of reasoning tokens in the completion"),
		field.Int("completion_accepted_prediction_tokens").Default(0).Optional().Comment("Number of accepted prediction tokens"),
		field.Int("completion_rejected_prediction_tokens").Default(0).Optional().Comment("Number of rejected prediction tokens"),

		// Additional metadata
		field.Enum("source").Values("api", "playground", "test").Default("api").Immutable().Comment("Source of the request"),
		field.String("format").Immutable().Default("openai/chat_completions").Comment("Request format used"),
	}
}

func (UsageLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("usage_logs").
			Field("user_id").
			Required().
			Immutable().
			Unique(),
		edge.From("request", Request.Type).
			Ref("usage_logs").
			Field("request_id").
			Required().
			Immutable().
			Unique(),
		edge.From("channel", Channel.Type).
			Ref("usage_logs").
			Field("channel_id").
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
			scopes.OwnerRule(), // owner users can access all usage logs
			scopes.UserReadScopeRule(scopes.ScopeReadRequests), // requires requests read permission
			scopes.UserOwnedQueryRule(),                        // users can only view their own usage logs
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(), // owner users can modify all usage logs
			scopes.UserWriteScopeRule(scopes.ScopeWriteRequests), // requires requests write permission
			scopes.UserOwnedMutationRule(),
		},
	}
}
