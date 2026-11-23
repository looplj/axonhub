// 职责: 保存插件表的存储代次和可重试的 DDL 操作。
// 关联文件:
//
//	internal/ent/schema/plugin.go
//	internal/server/plugin/table_storage.go
//	internal/objects/plugin_tables.go
//
// 注意事项:
//
//	删除中的存储有独立 ID，异步清理只能操作这一代的物理表。
package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/privacy"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/objects"
)

type PluginTableStorage struct{ ent.Schema }

func (PluginTableStorage) Fields() []ent.Field {
	return []ent.Field{
		field.String("plugin_id").Immutable().MaxLen(objects.MaxPluginIDLength).NotEmpty(),
		field.String("active_plugin_id").Optional().Nillable().MaxLen(objects.MaxPluginIDLength).Unique().StorageKey("active_plugin_id"),
		field.JSON("tables", map[string]objects.PluginTableSpec{}).Default(map[string]objects.PluginTableSpec{}),
		field.JSON("names", map[string]string{}).Default(map[string]string{}),
		field.Int("revision").Default(0),
	}
}

func (PluginTableStorage) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.Skip(entgql.SkipAll)}
}

func (PluginTableStorage) Policy() ent.Policy {
	return pluginTablePolicy()
}

type PluginTableOperation struct{ ent.Schema }

func (PluginTableOperation) Fields() []ent.Field {
	return []ent.Field{
		field.Int("storage_id").Immutable().Positive(),
		field.String("kind").Immutable().MaxLen(16).NotEmpty(),
		field.JSON("tables", map[string]objects.PluginTableSpec{}).Default(map[string]objects.PluginTableSpec{}),
		field.JSON("names", map[string]string{}).Default(map[string]string{}),
		field.String("package_sha").MaxLen(64).Default(""),
		field.String("last_error").Default("").MaxLen(512),
		field.Int64("retry_at").Default(0),
	}
}

func (PluginTableOperation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("storage_id", "kind").StorageKey("plugin_table_operations_by_storage_kind").Unique(),
		index.Fields("kind", "retry_at", "id").StorageKey("plugin_table_operations_by_kind_retry"),
	}
}

func (PluginTableOperation) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.Skip(entgql.SkipAll)}
}

func (PluginTableOperation) Policy() ent.Policy {
	return pluginTablePolicy()
}

func pluginTablePolicy() ent.Policy {
	return privacy.Policy{
		Query:    privacy.QueryPolicy{privacy.AlwaysDenyRule()},
		Mutation: privacy.MutationPolicy{privacy.AlwaysDenyRule()},
	}
}
