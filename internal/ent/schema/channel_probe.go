package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ChannelProbe struct {
	ent.Schema
}

func (ChannelProbe) Mixin() []ent.Mixin {
	return []ent.Mixin{}
}

func (ChannelProbe) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "timestamp").
			StorageKey("channel_probes_by_channel_id_timestamp"),
	}
}

func (ChannelProbe) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Immutable(),
		field.Int("total_request_count").Immutable(),
		field.Int("success_request_count").Immutable(),
		field.Int64("timestamp").Immutable(),
	}
}

func (ChannelProbe) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("channel_probes").
			Field("channel_id").
			Required().
			Immutable().Unique(),
	}
}
