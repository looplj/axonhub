package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ProviderQuotaStatus struct {
	ent.Schema
}

func (ProviderQuotaStatus) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{}, // Provides created_at, updated_at
	}
}

func (ProviderQuotaStatus) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id").Unique(),
		index.Fields("next_check_at"),
	}
}

func (ProviderQuotaStatus) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Immutable(),
		field.Enum("provider_type").
			Values("claudecode", "codex").
			Immutable(),
		field.String("status").
			Comment("Overall status: ok, throttled, overage, error"),
		field.JSON("quota_data", map[string]interface{}{}).
			Comment("Provider-specific quota data"),
		field.Time("next_check_at").
			Comment("Timestamp for next scheduled quota check"),
	}
}

func (ProviderQuotaStatus) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("provider_quota_status").
			Field("channel_id").
			Required().
			Immutable().
			Unique(),
	}
}
