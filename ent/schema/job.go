package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Job struct {
	ent.Schema
}

func (Job) Mixins() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Job) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id", "type").
			StorageKey("jobs_by_owner_id_type").
			Unique(),
	}
}

func (Job) Fields() []ent.Field {
	return []ent.Field{
		field.Int("owner_id").Immutable(),
		field.String("type").Immutable(),
		field.String("context"),
	}
}
