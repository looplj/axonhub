package schema

import (
	"errors"
	"unicode/utf8"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/objects"
)

type RequestExecution struct {
	ent.Schema
}

func (RequestExecution) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (RequestExecution) Indexes() []ent.Index {
	return []ent.Index{
		// Index for window function: find latest execution per request
		index.Fields("request_id", "status", "created_at").
			StorageKey("request_executions_by_request_id_status_created_at"),
		// Index for ordering executions by created_at per request
		index.Fields("request_id", "created_at").
			StorageKey("request_executions_by_request_id_created_at"),
		index.Fields("channel_id", "created_at").
			StorageKey("request_executions_by_channel_id_created_at"),
	}
}

func (RequestExecution) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").Immutable().Default(1),
		field.Int("request_id").Immutable(),
		field.Int("channel_id").Immutable().Optional(), // Optional for deleted channel, this field is not null.
		field.Int("data_storage_id").
			Optional().
			Immutable().
			Comment("Data Storage ID that this request belongs to"),
		// External ID for tracking requests in external systems
		field.String("external_id").
			Optional().
			MaxLen(512),
		field.String("model_id").
			Immutable().
			Comment("Channel model ID selected after model mapping, used for routing and pricing. May differ from the final wire model and the upstream-reported model."),
		// UpstreamModelID is the raw model reported by the provider response, captured
		// before AxonHub rewrites it back to the client-requested model.
		// Empty means no supported model metadata was recorded. Intra-stream model
		// changes are not tracked; only the first reported name is kept.
		field.String("upstream_model_id").
			Optional().
			Comment("Raw model reported by the upstream provider response, before client-model rewrite"),
		//  The format of the request, e.g: openai/chat_completions, claude/messages, openai/response.
		field.String("format").Immutable().Default("openai/chat_completions"),
		field.String("reasoning_effort").
			Optional().
			Nillable().
			Immutable().
			Comment("Final reasoning effort sent to the upstream provider"),
		field.String("channel_api_key_suffix").
			Optional().
			Nillable().
			Immutable().
			Validate(func(s string) error {
				if utf8.RuneCountInString(s) > 4 {
					return errors.New("channel_api_key_suffix must be at most 4 characters")
				}
				return nil
			}).
			Comment("Last 4 characters of the channel API key used for this execution").
			Annotations(
				entgql.Directives(forceResolver()),
				entgql.Skip(entgql.SkipWhereInput),
			),
		// The original request to the provider.
		// e.g: the user request via OpenAI request format, but the actual request to the provider with Claude format, the request_body is the Claude request format.
		field.JSON("request_body", objects.JSONRawMessage{}).Immutable().Annotations(
			entgql.Directives(forceResolver()),
		),
		// The final response from the provider.
		// e.g: the provider response with Claude format, and the user expects the response with OpenAI format, the response_body is the Claude response format.
		field.JSON("response_headers", objects.JSONRawMessage{}).
			Optional().
			Comment("Response headers received from the upstream provider, with sensitive values masked"),
		field.JSON("response_body", objects.JSONRawMessage{}).Optional().Annotations(
			entgql.Directives(forceResolver()),
		),
		// The streaming response chunks from the provider.
		// e.g: the provider response with Claude format, and the user expects the response with OpenAI format, the response_chunks is the Claude response format.
		field.JSON("response_chunks", []objects.JSONRawMessage{}).Optional().Annotations(
			entgql.Directives(forceResolver()),
		),
		field.String("error_message").Optional(),
		field.Int("response_status_code").Optional().Nillable().
			Comment("HTTP status code from the upstream provider"),
		// The status of the request execution.
		field.Enum("status").Values("pending", "processing", "completed", "failed", "canceled"),
		// Whether the request is a streaming request
		field.Bool("stream").Default(false).Immutable(),
		// Total latency in milliseconds from request start to completion
		field.Int64("metrics_latency_ms").Optional().Nillable(),
		// First token latency in milliseconds (only for streaming requests)
		field.Int64("metrics_first_token_latency_ms").Optional().Nillable(),
		// Reasoning/thinking duration in milliseconds
		field.Int64("metrics_reasoning_duration_ms").Optional().Nillable().Comment("Reasoning/thinking duration in milliseconds"),
		// Request headers
		field.JSON("request_headers", objects.JSONRawMessage{}).
			Optional().
			Comment("Request headers"),
		// The actual upstream request URL sent to the provider.
		field.String("request_url").
			Optional().
			Comment("Actual upstream request URL sent to the provider"),
		// Whether the inbound request body was substituted during pass-through.
		field.Bool("pass_through_applied").
			Default(false).
			Comment("Whether pass-through was active for this execution attempt"),
	}
}

func (RequestExecution) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("request", Request.Type).
			Field("request_id").
			Ref("executions").
			Required().
			Immutable().
			Unique(),
		edge.From("channel", Channel.Type).
			Field("channel_id").
			Ref("executions").
			Annotations(
				entgql.Directives(forceResolver()),
			).
			Immutable().
			Unique(),
		edge.From("data_storage", DataStorage.Type).
			Ref("executions").
			Field("data_storage_id").
			Immutable().
			Unique(),
	}
}

func (RequestExecution) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.RelayConnection(),
	}
}
