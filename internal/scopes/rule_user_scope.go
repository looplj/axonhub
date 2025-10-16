package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// UserReadScopeRule checks read permissions.
func UserReadScopeRule(readScope ScopeSlug) privacy.QueryRule {
	return userScopeQueryRule{requiredScope: readScope}
}

// userScopeQueryRule custom QueryRule implementation.
type userScopeQueryRule struct {
	requiredScope ScopeSlug
}

func (r userScopeQueryRule) EvalQuery(ctx context.Context, q ent.Query) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	if userHasScope(user, r.requiredScope) {
		return privacy.Allow
	}

	return privacy.Skipf("user does not have required read scope: %s", r.requiredScope)
}

// UserWriteScopeRule checks write permissions.
func UserWriteScopeRule(writeScope ScopeSlug) privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		if userHasScope(user, writeScope) {
			return privacy.Allow
		}

		return privacy.Skipf("user does not have required write scope: %s", writeScope)
	})
}

// UserScopeQueryMutationRule checks both read and write permissions.
func UserScopeQueryMutationRule(requiredScope ScopeSlug) privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		if userHasScope(user, requiredScope) {
			return privacy.Allow
		}

		return privacy.Skipf("user does not have required scope: %s", requiredScope)
	})
}

func WithUserScopeDecision(ctx context.Context, requiredScope ScopeSlug) context.Context {
	user, ok := contexts.GetUser(ctx)
	if !ok {
		return privacy.DecisionContext(ctx, privacy.Deny)
	}

	if userHasScope(user, requiredScope) {
		return privacy.DecisionContext(ctx, privacy.Allow)
	}

	return privacy.DecisionContext(ctx, privacy.Deny)
}
