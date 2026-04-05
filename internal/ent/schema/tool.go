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

// Tool holds the schema definition for the Tool entity.
type Tool struct {
	ent.Schema
}

func (Tool) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Tool) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "name", "deleted_at").
			StorageKey("tools_by_project_id_name").
			Unique(),
	}
}

func (Tool) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Optional().
			Nillable().
			Immutable().
			Comment("Project ID, null means global tool"),
		field.String("name").
			Comment("Tool name"),
		field.String("description").
			Default("").
			Comment("Tool description"),
		field.Enum("type").
			Values("custom").
			Default("custom").
			Comment("Tool type"),
		field.JSON("schema", objects.JSONRawMessage{}).
			Default(objects.JSONRawMessage([]byte("{}"))).
			Comment("Tool parameters schema (JSON schema)"),
		field.JSON("default_policy", objects.JSONRawMessage{}).
			Default(objects.JSONRawMessage([]byte("{}"))).
			Comment("Tool default policy (JSON)"),
		field.Enum("status").
			Values("enabled", "disabled", "archived").
			Default("enabled").
			Comment("Tool status"),
		field.Int("created_by_user_id").
			Optional().
			Nillable().
			Comment("Creator user ID (optional)"),
	}
}

func (Tool) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).
			Ref("tools").
			Field("project_id").
			Immutable().
			Unique(),
		edge.From("created_by_user", User.Type).
			Ref("tools").
			Field("created_by_user_id").
			Unique(),
		edge.To("agent_bindings", AgentTool.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Tool) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (Tool) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.UserProjectScopeReadRule(scopes.ScopeReadAgents),
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadAgents),
		},
		Mutation: scopes.MutationPolicy{
			scopes.UserProjectScopeWriteRule(scopes.ScopeWriteAgents),
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteAgents),
		},
	}
}
