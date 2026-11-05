package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

// Plugin is one installed plugin row. Builtin rows may leave every column
// except plugin_id and enabled empty. GraphQL exposure is a later task.
type Plugin struct {
	ent.Schema
}

func (Plugin) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Plugin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plugin_id", "deleted_at").
			StorageKey("plugins_by_plugin_id").
			Unique(),
	}
}

func (Plugin) Fields() []ent.Field {
	return []ent.Field{
		field.String("plugin_id").
			Immutable().
			MaxLen(objects.MaxPluginIDLength).
			NotEmpty().
			Comment("Manifest id. Unique among rows with deleted_at = 0."),
		field.String("name").
			Optional().
			Default("").
			MaxLen(objects.MaxPluginNameLength).
			Comment("Empty for builtin plugins. Uploaded packages require a name in the manifest."),
		field.String("version").
			Optional().
			Default("").
			MaxLen(objects.MaxPluginVersionLength).
			Comment("Empty for builtin plugins. Uploaded packages store a semver."),
		field.String("description").
			Optional().
			Default("").
			MaxLen(objects.MaxPluginDescriptionLength).
			Comment("Empty for builtin plugins."),
		field.JSON("manifest", objects.PluginManifest{}).
			Optional().
			Default(objects.PluginManifest{}).
			Comment("Parsed manifest. Empty for builtin plugins."),
		field.Bytes("package").
			Optional().
			SchemaType(map[string]string{
				dialect.MySQL: "longblob",
			}).
			Comment("Original .axplugin ZIP bytes. Empty for builtin plugins. SQLite uses blob and PostgreSQL uses bytea."),
		field.String("package_sha256").
			Optional().
			MaxLen(64).
			Default("").
			Comment("Lowercase hex sha256 of the original ZIP. Empty for builtin plugins."),
		field.Bool("enabled").
			Default(false),
		field.Int("priority").
			Default(100).
			Comment("Lower values run first."),
		field.JSON("granted_permissions", []string{}).
			Optional().
			Default([]string{}).
			Comment("Permissions the owner granted. Must stay a subset of manifest.permissions."),
		field.JSON("config", map[string]any{}).
			Optional().
			Default(map[string]any{}).
			Comment("Plugin configuration object validated against manifest.configSchema."),
	}
}

func (Plugin) Edges() []ent.Edge {
	return nil
}

func (Plugin) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.Skip(entgql.SkipAll),
	}
}

// Policy defines who can read and change plugin rows.
// Installing, enabling, and granting permissions stay owner-only in a later resolver.
func (Plugin) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(), // owner 用户可以访问所有插件
			scopes.UserReadScopeRule(scopes.ScopeReadPlugins), // 需要 plugins 读取权限
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(), // owner 用户可以修改所有插件
			scopes.UserWriteScopeRule(scopes.ScopeWritePlugins), // 需要 plugins 写入权限
		},
	}
}
