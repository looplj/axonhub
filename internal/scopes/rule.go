package scopes

import (
	"context"
	"slices"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// Common error messages.
const (
	ErrNoUser = "no user in context"
)

// AlwaysDeny returns a rule that denies access if no user is in context.
func AlwaysDeny() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		return privacy.Deny
	})
}

// hasScope checks if a scope exists in the given scopes slice.
func hasScope(scopes []string, requiredScope string) bool {
	return slices.Contains(scopes, requiredScope)
}

// hasRoleScope checks if a user has a required scope through their roles.
func hasRoleScope(user *ent.User, requiredScope Scope) bool {
	for _, role := range user.Edges.Roles {
		if hasScope(role.Scopes, string(requiredScope)) {
			return true
		}
	}

	return false
}

// userHasScope checks if a user has the required scope either directly or through roles.
func userHasScope(user *ent.User, requiredScope Scope) bool {
	// Owner has all permissions
	if user.IsOwner {
		return true
	}

	// Check user's direct scopes
	if hasScope(user.Scopes, string(requiredScope)) {
		return true
	}

	// Check user's role scopes
	return hasRoleScope(user, requiredScope)
}

// getUserFromContext safely retrieves user from context.
func getUserFromContext(ctx context.Context) (*ent.User, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, privacy.Denyf(ErrNoUser)
	}
	return user, nil
}

// getAPIKeyFromContext safely retrieves API key from context.
func getAPIKeyFromContext(ctx context.Context) (*ent.APIKey, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil {
		return nil, privacy.Skip
	}

	return apiKey, nil
}
