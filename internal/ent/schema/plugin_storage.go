package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/privacy"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PluginStorage is one KV row bound to a plugin installation id (plugins.id).
// Rows are not soft-deleted and have no foreign key. The main GraphQL schema
// does not expose this type; plugin callers use the manager API.
type PluginStorage struct {
	ent.Schema
}

func (PluginStorage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (PluginStorage) Indexes() []ent.Index {
	return []ent.Index{
		// Equality lookup and uniqueness for one installation's key.
		// utf8mb4 worst case is 4 + 128*4 + 128*4 = 1028 bytes, within the
		// MySQL 3072-byte and PostgreSQL index limits.
		index.Fields("installation_id", "namespace", "key").
			StorageKey("plugin_storages_by_installation_namespace_key").
			Unique(),
	}
}

func (PluginStorage) Fields() []ent.Field {
	return []ent.Field{
		field.Int("installation_id").
			Immutable().
			Positive().
			Comment("plugins.id of the owning installation. Callers pass this id; the row does not resolve a manifest id."),
		field.String("namespace").
			Default("default").
			MaxLen(128).
			Comment("KV namespace. An empty namespace is stored as default."),
		field.String("key").
			NotEmpty().
			MaxLen(128),
		field.String("value").
			SchemaType(map[string]string{
				dialect.MySQL: "mediumtext",
			}).
			Comment("UTF-8 value, at most 256 KiB. Empty string is valid. MySQL uses mediumtext."),
		field.Time("expires_at").
			Optional().
			Nillable().
			Comment("Null means the row does not expire."),
	}
}

func (PluginStorage) Edges() []ent.Edge {
	return nil
}

func (PluginStorage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.Skip(entgql.SkipAll),
	}
}

// Policy denies every caller that does not already carry a privacy allow decision.
// System bypass sets that decision before these rules run.
func (PluginStorage) Policy() ent.Policy {
	return privacy.Policy{
		Query: privacy.QueryPolicy{
			privacy.AlwaysDenyRule(),
		},
		Mutation: privacy.MutationPolicy{
			privacy.AlwaysDenyRule(),
		},
	}
}
