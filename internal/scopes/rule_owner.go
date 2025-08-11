package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/ent/privacy"
)

// OwnerRule allows owner users to access all functionality.
func OwnerRule() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		// Owner users have all permissions
		if user.IsOwner {
			return privacy.Allow
		}

		return privacy.Skip
	})
}
