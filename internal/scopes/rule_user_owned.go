package scopes

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/entql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// UserFilter interface for filtering queries by user ID.
type UserFilter interface {
	WhereUserID(entql.IntP)
}

// FilterMutation interface for filtering mutations.
type FilterMutation interface {
	WhereP(ps ...func(*sql.Selector))
}

// UserOwnedQueryRule checks if user owns the resource (for user-owned resources like API Keys).
func UserOwnedQueryRule() privacy.QueryRule {
	return privacy.FilterFunc(userOwnedQueryRule)
}

func userOwnedQueryRule(ctx context.Context, q privacy.Filter) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	f, ok := q.(UserFilter)
	if !ok {
		return privacy.Denyf("query type %T does not implement UserFilter", q)
	}

	f.WhereUserID(entql.IntEQ(user.ID))

	return privacy.Skip
}

// UserOwnedMutationRule ensures users can only modify their own resources.
func UserOwnedMutationRule() privacy.MutationRule {
	return userOwnedMutationRule{}
}

type userOwnedMutationRule struct{}

func (userOwnedMutationRule) EvalMutation(ctx context.Context, m ent.Mutation) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// For mutations, check if operating on own resources
	switch mutation := m.(type) {
	case FilterMutation:
		mutation.WhereP(func(s *sql.Selector) {
			s.Where(sql.EQ("user_id", user.ID))
		})

		return privacy.Skip
	default:
		return privacy.Denyf("user can only access their own resources")
	}
}
