package authz

import (
	"context"
	"fmt"
	"slices"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/scopes"
)

func WithScopeDecision(ctx context.Context, requiredScope scopes.ScopeSlug) context.Context {
	has := HasScope(ctx, requiredScope)

	p, _ := GetPrincipal(ctx)

	log.Debug(ctx, "authz: scope decision",
		log.String("principal", p.String()),
		log.String("scope", string(requiredScope)),
		log.String("decision", lo.Ternary(has, "allow", "deny")),
	)

	if has {
		return privacy.DecisionContext(ctx, privacy.Allow)
	}

	return privacy.DecisionContext(ctx, privacy.Deny)
}

func RunWithScopeDecision[T any](ctx context.Context, requiredScope scopes.ScopeSlug, fn func(ctx context.Context) (T, error)) (T, error) {
	scopeCtx := WithScopeDecision(ctx, requiredScope)
	return fn(scopeCtx)
}

func HasScope(ctx context.Context, requiredScope scopes.ScopeSlug) bool {
	p, ok := GetPrincipal(ctx)
	if !ok {
		return false
	}

	switch p.Type {
	case PrincipalTypeSystem, PrincipalTypeTest:
		return true
	case PrincipalTypeUser:
		return userHasScope(ctx, requiredScope)
	case PrincipalTypeAPIKey:
		return apiKeyHasScope(ctx, requiredScope)
	case PrincipalTypeUnknown:
		return false
	default:
		return false
	}
}

func RequireScope(ctx context.Context, requiredScope scopes.ScopeSlug) error {
	if !HasScope(ctx, requiredScope) {
		p, _ := GetPrincipal(ctx)
		return fmt.Errorf("authz: principal %s does not have required scope %s", p.String(), requiredScope)
	}

	return nil
}

func userHasScope(ctx context.Context, requiredScope scopes.ScopeSlug) bool {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return false
	}

	if user.IsOwner {
		return true
	}

	// Check user's direct scopes
	if slices.Contains(user.Scopes, string(requiredScope)) {
		return true
	}

	// Check system-level role scopes
	for _, role := range user.Edges.Roles {
		if !role.IsSystemRole() {
			continue
		}

		if slices.Contains(role.Scopes, string(requiredScope)) {
			return true
		}
	}

	// Check project-level scopes when a project ID is in context.
	// This mirrors scopes.userHasProjectScope and ensures that users with
	// project-level roles or project membership scopes are recognized.
	projectID, hasProjectID := contexts.GetProjectID(ctx)
	if !hasProjectID {
		return false
	}

	// Check project membership scopes
	hasProjectMembership := false
	for _, up := range user.Edges.ProjectUsers {
		if up.ProjectID != projectID {
			continue
		}

		hasProjectMembership = true
		if up.IsOwner || slices.Contains(up.Scopes, string(requiredScope)) {
			return true
		}

		break
	}

	if !hasProjectMembership {
		return false
	}

	// Check project-level role scopes
	for _, role := range user.Edges.Roles {
		if role.IsSystemRole() {
			continue
		}

		if role.ProjectID != nil && *role.ProjectID == projectID && slices.Contains(role.Scopes, string(requiredScope)) {
			return true
		}
	}

	return false
}

func apiKeyHasScope(ctx context.Context, requiredScope scopes.ScopeSlug) bool {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil {
		return false
	}

	return slices.Contains(apiKey.Scopes, string(requiredScope))
}
