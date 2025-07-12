package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
		field.String("email"),
		field.String("name"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("requests", Request.Type).
			Annotations(
				entgql.RelayConnection(),
			),
		edge.To("api_keys", APIKey.Type).
			Annotations(
				entgql.RelayConnection(),
			),
	}
}

type UserOwnedMixin struct {
	mixin.Schema
}

func (UserOwnedMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
	}
}

func (UserOwnedMixin) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Field("user_id").Unique(),
	}
}
