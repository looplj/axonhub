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
	"github.com/looplj/axonhub/internal/scopes"
)

// PromptVersion holds the schema definition for the PromptVersion entity.
type PromptVersion struct {
	ent.Schema
}

func (PromptVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (PromptVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("prompt_id", "version", "deleted_at").
			StorageKey("prompt_versions_by_prompt_id_version").
			Unique(),
		index.Fields("prompt_id", "status", "deleted_at", "created_at").
			StorageKey("prompt_versions_by_prompt_id_status_created_at"),
	}
}

func (PromptVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int("project_id").
			Immutable().
			Comment("Project ID that this prompt version belongs to").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Int("prompt_id").
			Immutable().
			Comment("Prompt ID that this version belongs to").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Int("version").
			Comment("Prompt version number (monotonic)"),
		field.String("content").
			SchemaType(map[string]string{
				dialect.MySQL: "mediumtext",
			}).
			Comment("Prompt content"),
		field.Enum("status").
			Values("draft", "active", "archived").
			Default("draft").
			Comment("Prompt version status"),
		field.String("change_log").
			Default("").
			Comment("Change log for this version"),
		field.Int("created_by_user_id").
			Optional().
			Nillable().
			Comment("Creator user ID (optional)"),
	}
}

func (PromptVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("prompt", Prompt.Type).
			Ref("versions").
			Field("prompt_id").
			Immutable().
			Required().
			Unique(),
		edge.From("project", Project.Type).
			Ref("prompt_versions").
			Field("project_id").
			Immutable().
			Required().
			Unique(),
		edge.From("created_by_user", User.Type).
			Ref("prompt_versions").
			Field("created_by_user_id").
			Unique(),
		edge.To("active_for_prompts", Prompt.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("draft_for_prompts", Prompt.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
	}
}

func (PromptVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (PromptVersion) Policy() ent.Policy {
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

