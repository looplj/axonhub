package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
)

type ProviderQuotaStatus struct {
	ent.Schema
}

func (ProviderQuotaStatus) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},                  // Provides created_at, updated_at
		schematype.SoftDeleteMixin{}, // Provides deleted_at for soft delete
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
		field.Enum("status").
			Values("available", "warning", "exhausted", "unknown").
			Comment("Overall status: available, warning, exhausted, unknown"),
		field.JSON("quota_data", map[string]any{}).
			Comment("Provider-specific quota data"),
		field.Time("next_reset_at").
			Optional().
			Nillable().
			Comment("Timestamp for next quota reset (primary window)"),
		field.Bool("ready").
			Default(true).
			Comment("True if status is available or warning"),
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
