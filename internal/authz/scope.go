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

// WithScopeDecision checks requiredScope and returns a context carrying an Ent
// privacy Allow or Deny decision. On success, Allow bypasses ALL Ent privacy
// policies for subsequent queries and mutations using this context, including
// policy-based row filters such as the plugin JWT signing-key exclusion. It is
// not limited to entities or operations associated with requiredScope.
//
// Use the returned context only for fixed host operations whose complete access
// has been authorized. Never pass it to plugin-supplied callbacks or use it for
// queries that rely on privacy policies to filter their results. Use RequireScope
// with the original context when those policies must remain active.
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

// RunWithScopeDecision calls fn with the context produced by WithScopeDecision.
// On success, all Ent privacy policies, including policy-based row filters, are
// bypassed inside fn. Keep fn limited to a fixed, authorized host operation and
// do not let the decision context escape the callback.
//
// fn is called even when the scope check fails; in that case its context carries
// Deny, which rejects Ent operations but does not prevent other side effects.
// Use RequireScope before invoking fn when failure must prevent its execution.
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
	case PrincipalTypePlugin:
		user, ok := contexts.GetUser(ctx)
		if !ok || user == nil {
			return false
		}
		if scopes.EffectiveSystemScope(ctx, user, requiredScope) {
			return true
		}
		projectID, ok := contexts.GetProjectID(ctx)
		return ok && scopes.EffectiveProjectScope(ctx, user, projectID, requiredScope)
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
