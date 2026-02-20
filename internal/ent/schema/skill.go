package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

// Skill holds the schema definition for the Skill entity.
type Skill struct {
	ent.Schema
}

func (Skill) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Skill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "name", "deleted_at").
			StorageKey("skills_by_project_id_name").
			Unique(),
	}
}

func (Skill) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Optional().
			Nillable().
			Immutable().
			Comment("Project ID, null means global skill"),
		field.String("name").
			Comment("Skill name"),
		field.String("description").
			Default("").
			Comment("Skill description"),
		field.Enum("kind").
			Values("prompt", "script", "hybrid").
			Default("prompt").
			Comment("Skill kind"),
		field.String("content").
			Optional().
			Nillable().
			Comment("Skill content (prompt/instructions)").
			SchemaType(map[string]string{
				dialect.MySQL: "mediumtext",
			}),
		field.String("entrypoint").
			Default("").
			Comment("Skill entrypoint (e.g. scripts/run.sh)"),
		field.JSON("bundle", objects.JSONRawMessage{}).
			Optional().
			Comment("Optional bundle metadata (JSON)"),
		field.Enum("status").
			Values("enabled", "disabled", "archived").
			Default("enabled").
			Comment("Skill status"),
		field.Int("created_by_user_id").
			Optional().
			Nillable().
			Comment("Creator user ID (optional)"),
	}
}

func (Skill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).
			Ref("skills").
			Field("project_id").
			Immutable().
			Unique(),
		edge.From("created_by_user", User.Type).
			Ref("skills").
			Field("created_by_user_id").
			Unique(),
		edge.To("agent_bindings", AgentSkill.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (Skill) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (Skill) Policy() ent.Policy {
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
