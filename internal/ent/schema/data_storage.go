package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

// DataStorage holds the schema definition for the DataStorage entity.
type DataStorage struct {
	ent.Schema
}

func (DataStorage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (DataStorage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").
			StorageKey("data_sources_by_name").
			Unique(),
	}
}

// Fields of the DataSource.
func (DataStorage) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("data source name"),
		field.String("description").
			Default("").
			Comment("data source description"),
		field.Bool("primary").
			Default(false).
			Comment("data source is primary, only the system database is the primary, it can not be archived."),
		field.Enum("type").
			Values("database", "fs", "s3", "gcs").
			Default("database").
			Comment("data source type"),
		field.JSON("settings", &objects.DataStorageSettings{}).
			Comment("data source setting"),
		field.Enum("status").
			Values("active", "archived").
			Default("active").
			Comment("data source status"),
	}
}

// Edges of the DataSource.
func (DataStorage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("requests", Request.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("executions", RequestExecution.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (DataStorage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 DataSource 的权限策略.
func (DataStorage) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeWriteRequests),
			scopes.UserReadScopeRule(scopes.ScopeReadDataStorages),
			scopes.OwnerRule(),
		},
		Mutation: scopes.MutationPolicy{
			scopes.UserWriteScopeRule(scopes.ScopeWriteDataStorages),
			scopes.OwnerRule(),
		},
	}
}
