package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
)

type ChannelPerformance struct {
	ent.Schema
}

func (ChannelPerformance) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (ChannelPerformance) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "deleted_at").
			StorageKey("channel_performances_by_channel_id").
			Unique(),
	}
}

func (ChannelPerformance) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Unique().Immutable(),
		field.Enum("health_status").
			Values("good", "warning", "critical", "panic").
			Default("good").
			Comment("Health status of the channel"),

		// Total request stats
		field.Int("total_count").Default(0),
		field.Int("total_success_count").Default(0),
		field.Int("total_token_count").Default(0),

		// Total request performance
		field.Int("total_avg_latency_ms").Default(0),
		field.Int("total_avg_token_per_second").Default(0),
		// For stream
		field.Int("total_avg_stream_first_token_latench_ms").Default(0),
		field.Float("total_avg_stream_token_per_second").Default(0),

		// Last period
		field.Time("last_period_start"),
		field.Time("last_period_end"),
		field.Int("last_period_seconds").Default(0),

		// Last period request stats
		field.Int("last_period_count").Default(0),
		field.Int("last_period_success_count").Default(0),
		field.Int("last_period_token_count").Default(0),

		// Last period request performance
		field.Int("last_period_avg_latency_ms").Default(0),
		field.Int("last_period_avg_token_per_second").Default(0),

		// For stream
		field.Int("last_period_avg_stream_first_token_latench_ms").Default(0),
		field.Float("last_period_avg_stream_token_per_second").Default(0),

		// Last request
		field.Time("last_success_at").Nillable(),
		field.Time("last_failure_at").Nillable(),
		field.Time("last_attempt_at").Nillable(),
	}
}

func (ChannelPerformance) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("channel_performance").
			Field("channel_id").
			Required().
			Immutable().
			Unique(),
	}
}
