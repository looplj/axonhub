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
		field.Int("success_rate").Default(0),
		field.Int("avg_latency_ms").Default(0),
		field.Int("avg_token_per_second").Default(0),

		// For stream
		field.Int("avg_stream_first_token_latency_ms").Default(0),
		field.Float("avg_stream_token_per_second").Default(0),

		// Last request
		field.Time("last_success_at").Optional().Nillable(),
		field.Time("last_failure_at").Optional().Nillable(),
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
