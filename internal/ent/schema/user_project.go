package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/scopes"
)

// UserProject holds the schema definition for the UserProject entity.
type UserProject struct {
	ent.Schema
}

func (UserProject) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (UserProject) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "project_id", "deleted_at").
			StorageKey("user_projects_by_user_id_project_id").
			Unique(),
		index.Fields("project_id").
			StorageKey("user_projects_by_project_id"),
	}
}

// Fields of the UserProject.
func (UserProject) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").
			Immutable(),
		field.Int("project_id").
			Immutable(),
		field.Bool("is_owner").
			Default(false).
			Comment(
				"Indicates whether the user is the owner of the project. This field is mutable to allow transferring ownership between users. Only users with sufficient permissions (e.g., current owner) can modify this field.",
			),
		field.Strings("scopes").
			Comment("User-specific scopes: write_channels, read_channels, add_users, read_users, etc.").
			Default([]string{}).
			Optional(),
	}
}

func (UserProject) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Field("user_id").
			Immutable().
			Unique().
			Required(),
		edge.To("project", Project.Type).
			Field("project_id").
			Immutable().
			Unique().
			Required(),
	}
}

func (UserProject) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserProjectScopeReadRule(scopes.ScopeReadUsers),
			scopes.UserOwnedQueryRule(),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserProjectScopeWriteRule(scopes.ScopeWriteUsers),
		},
	}
}
