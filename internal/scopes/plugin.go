package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/authz/identity"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
)

// EffectiveSystemScope applies delegated authority before evaluating the real user.
func EffectiveSystemScope(ctx context.Context, user *ent.User, scope ScopeSlug) bool {
	return user != nil && pluginScopeAllowed(ctx, user, scope) && HasSystemScope(user, scope)
}

func EffectiveProjectScope(ctx context.Context, user *ent.User, projectID int, scope ScopeSlug) bool {
	return user != nil && pluginScopeAllowed(ctx, user, scope) && !IsSystemOnlyScope(string(scope)) && userHasProjectScope(user, projectID, scope)
}

func pluginScopeAllowed(ctx context.Context, user *ent.User, scope ScopeSlug) bool {
	if !identity.IsPlugin(ctx) {
		return true
	}
	if _, exists := contexts.GetAPIKey(ctx); exists {
		return false
	}
	return identity.PluginGranted(ctx, user.ID, string(scope))
}

// IsDirectOwner excludes delegated callers even when their real user is Owner.
func IsDirectOwner(ctx context.Context) bool {
	if identity.IsPlugin(ctx) {
		return false
	}
	user, ok := contexts.GetUser(ctx)
	return ok && user != nil && user.IsOwner
}
