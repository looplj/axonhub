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

// Prompt holds the schema definition for the Prompt entity.
type Prompt struct {
	ent.Schema
}

func (Prompt) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Prompt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id").
			StorageKey("prompts_by_project_id"),
		index.Fields("project_id", "name", "deleted_at").
			StorageKey("prompts_by_project_id_name").
			Unique(),
	}
}

// Fields of the Prompt.
func (Prompt) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID that this prompt belongs to").Annotations(
			entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
		field.String("name").
			Comment("prompt name"),
		field.String("description").
			Default("").
			Comment("prompt description"),
		field.String("role").
			Comment("prompt role"),
		field.String("content").
			Comment("prompt content"),
		field.Enum("status").
			Values("enabled", "disabled").
			Default("disabled"),
		field.JSON("settings", objects.PromptSettings{}).
			Comment("prompt settings in JSON format"),
	}
}

// Edges of the Prompt.
func (Prompt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("projects", Project.Type).
			Ref("prompts").
			Annotations(
				entgql.RelayConnection(),
			),
	}
}

func (Prompt) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (Prompt) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserProjectScopeReadRule(scopes.ScopeReadPrompts),
			scopes.UserReadScopeRule(scopes.ScopeReadPrompts),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserProjectScopeWriteRule(scopes.ScopeWritePrompts),
			scopes.UserWriteScopeRule(scopes.ScopeWritePrompts),
		},
	}
}
