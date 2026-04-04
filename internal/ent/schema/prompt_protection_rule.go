package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type PromptProtectionRule struct {
	ent.Schema
}

func (PromptProtectionRule) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (PromptProtectionRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("prompt_protection_rules_by_name").
			Unique(),
	}
}

func (PromptProtectionRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("Rule name").
			Annotations(entgql.OrderField("NAME")),
		field.String("description").
			Default("").
			Comment("Rule description"),
		field.String("pattern").
			Comment("Regex pattern to match prompt content"),
		field.Enum("status").
			Values("enabled", "disabled", "archived").
			Default("disabled").
			Annotations(entgql.Skip(entgql.SkipMutationCreateInput)),
		field.JSON("settings", &objects.PromptProtectionSettings{}).
			Comment("Prompt protection rule settings"),
	}
}

func (PromptProtectionRule) Edges() []ent.Edge {
	return nil
}

func (PromptProtectionRule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (PromptProtectionRule) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadChannels),
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadChannels),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels),
		},
	}
}
