package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/authz/identity"
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
	if identity.IsPluginInstallation(ctx) {
		return pluginInstallationScopeDecision(ctx, r.requiredScope)
	}
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	if EffectiveSystemScope(ctx, user, r.requiredScope) {
		return privacy.Allow
	}

	return privacy.Skipf("user does not have required read scope: %s", r.requiredScope)
}

// UserWriteScopeRule checks write permissions.
func UserWriteScopeRule(writeScope ScopeSlug) privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		if identity.IsPluginInstallation(ctx) {
			return pluginInstallationScopeDecision(ctx, writeScope)
		}
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		if EffectiveSystemScope(ctx, user, writeScope) {
			return privacy.Allow
		}

		return privacy.Skipf("user does not have required write scope: %s", writeScope)
	})
}

// UserScopeQueryMutationRule checks both read and write permissions.
func UserScopeQueryMutationRule(requiredScope ScopeSlug) privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		if identity.IsPluginInstallation(ctx) {
			return pluginInstallationScopeDecision(ctx, requiredScope)
		}
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		if EffectiveSystemScope(ctx, user, requiredScope) {
			return privacy.Allow
		}

		return privacy.Skipf("user does not have required scope: %s", requiredScope)
	})
}

func pluginInstallationScopeDecision(ctx context.Context, requiredScope ScopeSlug) error {
	if identity.PluginInstallationGranted(ctx, string(requiredScope)) {
		return privacy.Allow
	}
	return privacy.Skipf("plugin installation does not have required scope: %s", requiredScope)
}

// HasSiteScope accepts an existing site grant from either a user or a plugin installation.
func HasSiteScope(ctx context.Context, requiredScope ScopeSlug) bool {
	return identity.PluginInstallationGranted(ctx, string(requiredScope)) || UserHasScope(ctx, requiredScope)
}

func UserHasScope(ctx context.Context, requiredScope ScopeSlug) bool {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return false
	}

	if EffectiveSystemScope(ctx, user, requiredScope) {
		return true
	}

	return false
}
