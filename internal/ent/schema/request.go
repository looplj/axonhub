package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type Request struct {
	ent.Schema
}

func (Request) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Request) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("api_key_id").
			StorageKey("requests_by_api_key_id"),
		index.Fields("project_id").
			StorageKey("requests_by_project_id"),
		index.Fields("channel_id").
			StorageKey("requests_by_channel_id"),
		index.Fields("trace_id").
			StorageKey("requests_by_trace_id"),
		// Performance indexes for dashboard queries
		index.Fields("created_at").
			StorageKey("requests_by_created_at"),
		index.Fields("status").
			StorageKey("requests_by_status"),
	}
}

func (Request) Fields() []ent.Field {
	return []ent.Field{
		field.Int("api_key_id").
			Optional().
			Immutable().
			Comment("API Key ID of the request, null for the request from the Admin."),
		field.Int("project_id").
			Immutable().
			Default(1).
			Comment("Project ID, default to 1 for backward compatibility"),
		field.Int("trace_id").
			Optional().
			Immutable().
			Comment("Trace ID that this request belongs to"),
		field.Int("data_storage_id").
			Optional().
			Immutable().
			Comment("Data Storage ID that this request belongs to"),
		field.Enum("source").Values("api", "playground", "test").Default("api").Immutable(),
		field.String("model_id").Immutable(),
		// The format of the request, e.g: openai/chat_completions, claude/messages, openai/response.
		field.String("format").Immutable().Default("openai/chat_completions"),
		// Request headers
		field.JSON("request_headers", objects.JSONRawMessage{}).
			Optional().
			Comment("Request headers"),
		// The original request from the user.
		// e.g: the user request via OpenAI request format, but the actual request to the provider with Claude format, the request_body is the OpenAI request format.
		field.JSON("request_body", objects.JSONRawMessage{}).
			Immutable().
			Annotations(
				entgql.Directives(forceResolver()),
			),
		// The final response to the user.
		// e.g: the provider response with Claude format, but the user expects the response with OpenAI format, the response_body is the OpenAI response format.
		field.JSON("response_body", objects.JSONRawMessage{}).Optional().Annotations(
			entgql.Directives(forceResolver()),
		),
		// The response chunks to the user.
		field.JSON("response_chunks", []objects.JSONRawMessage{}).Optional().Annotations(
			entgql.Directives(forceResolver()),
		),
		field.Int("channel_id").Optional(),
		// External ID for tracking requests in external systems
		field.String("external_id").Optional(),
		// The status of the request.
		field.Enum("status").Values("pending", "processing", "completed", "failed", "canceled"),
		// Whether the request is a streaming request
		field.Bool("stream").Default(false).Immutable(),
		field.String("client_ip").Default("").Immutable(),
		// Total latency in milliseconds from request start to completion
		field.Int64("metrics_latency_ms").Optional().Nillable(),
		// First token latency in milliseconds (only for streaming requests)
		field.Int64("metrics_first_token_latency_ms").Optional().Nillable(),
	}
}

func (Request) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("api_key", APIKey.Type).Ref("requests").Field("api_key_id").Immutable().Unique(),
		edge.From("project", Project.Type).
			Ref("requests").
			Field("project_id").
			Immutable().
			Required().
			Unique(),
		edge.From("trace", Trace.Type).
			Ref("requests").
			Immutable().
			Field("trace_id").
			Unique(),
		edge.From("data_storage", DataStorage.Type).
			Ref("requests").
			Field("data_storage_id").
			Immutable().
			Unique(),
		edge.To("executions", RequestExecution.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.From("channel", Channel.Type).
			Ref("requests").
			Field("channel_id").
			Annotations(
				entgql.Directives(forceResolver()),
			).
			Unique(),
		edge.To("usage_logs", UsageLog.Type).
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
