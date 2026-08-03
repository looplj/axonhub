package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/scopes"
)

// Invitation holds a registration invitation for a project.
type Invitation struct {
	ent.Schema
}

func (Invitation) Mixin() []ent.Mixin {
	return []ent.Mixin{TimeMixin{}, schematype.SoftDeleteMixin{}}
}

func (Invitation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_hash").Unique(),
		index.Fields("project_id"),
	}
}

func (Invitation) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").Sensitive(),
		field.Int("project_id"),
		field.Int("role_id").Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable(),
		field.Int("max_uses").Default(1),
		field.Int("used_count").Default(0),
	}
}

func (Invitation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).
			Ref("invitations").
			Field("project_id").
			Unique().
			Required(),
	}
}

func (Invitation) Annotations() []schema.Annotation {
	return []schema.Annotation{entgql.Skip(entgql.SkipAll)}
}

func (Invitation) Policy() ent.Policy {
	return scopes.Policy{}
}
