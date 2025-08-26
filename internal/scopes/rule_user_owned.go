package scopes

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/entql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// UserOwnedFilter interface for filtering queries by user ID.
type UserOwnedFilter interface {
	WhereUserID(entql.IntP)
}

// UserOwnedQueryRule checks if user owns the resource (for user-owned resources like API Keys).
func UserOwnedQueryRule() privacy.QueryRule {
	return privacy.FilterFunc(userOwnedQueryFilter)
}

func userOwnedQueryFilter(ctx context.Context, q privacy.Filter) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	switch q := q.(type) {
	case UserOwnedFilter:
		q.WhereUserID(entql.IntEQ(user.ID))
		return privacy.Allowf("User %d can query their own API Keys", user.ID)
	default:
		return privacy.Skipf("User %d can only query their own API Keys", user.ID)
	}
}

// UserOwnedMutationRule ensures users can only modify their own resources.
func UserOwnedMutationRule() privacy.MutationRule {
	return userOwnedMutationRule{}
}

type UserOwnedMutation interface {
	ent.Mutation
	UserID() (r int, exists bool)
	WhereP(ps ...func(*sql.Selector))
}

type userOwnedMutationRule struct{}

func (userOwnedMutationRule) EvalMutation(ctx context.Context, m ent.Mutation) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// For mutations, check if operating on own resources
	switch mutation := m.(type) {
	case UserOwnedMutation:
		switch mutation.Op() {
		case ent.OpCreate:
			userId, ok := mutation.UserID()
			if !ok {
				return privacy.Skipf("User %d can only modify their own data", user.ID)
			}

			if userId != user.ID {
				return privacy.Skipf("User %d can only modify their own data", user.ID)
			}

			return privacy.Allowf("User %d can modify their own data", user.ID)
		case ent.OpUpdateOne:
			_, ok := mutation.UserID()
			if ok {
				return privacy.Denyf("User id can not be modified")
			}

			mutation.WhereP(func(s *sql.Selector) {
				s.Where(sql.EQ("user_id", user.ID))
			})

			return privacy.Allowf("User %d can modify their own data", user.ID)
		case ent.OpDeleteOne:
			mutation.WhereP(func(s *sql.Selector) {
				s.Where(sql.EQ("user_id", user.ID))
			})

			return privacy.Allowf("User %d can delete their own data", user.ID)
		case ent.OpDelete:
			mutation.WhereP(func(s *sql.Selector) {
				s.Where(sql.EQ("user_id", user.ID))
			})

			return privacy.Allowf("User %d can delete their own data", user.ID)
		case ent.OpUpdate:
			mutation.WhereP(func(s *sql.Selector) {
				s.Where(sql.EQ("user_id", user.ID))
			})

			return privacy.Allowf("User %d can update their own data", user.ID)
		default:
			return privacy.Denyf("Unsupported operation %s", mutation.Op())
		}
	case *ent.UserMutation:
		userId, ok := mutation.ID()
		if !ok {
			return privacy.Skipf("User %d can only modify their own data", user.ID)
		}

		if userId != user.ID {
			return privacy.Skipf("User %d can only modify their own data", user.ID)
		}

		return privacy.Allowf("User %d can modify their own data", user.ID)
	default:
		return privacy.Denyf("Unsupported mutation type %T", m)
	}
}
