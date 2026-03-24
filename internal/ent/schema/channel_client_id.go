package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ChannelClientID struct {
	ent.Schema
}

func (ChannelClientID) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Immutable(),
		field.String("principal_kind").Immutable(),
		field.String("principal_hash").Immutable(),
		field.String("client_id_hex").Immutable(),
		field.Time("created_at").Immutable().Default(time.Now),
	}
}

func (ChannelClientID) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "principal_hash").
			StorageKey("channel_client_ids_by_channel_principal").
			Unique(),
	}
}
