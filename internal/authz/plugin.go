package authz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/authz/identity"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/scopes"
)

// DelegatePlugin creates a separate request container whose user is a restricted view:
// scopes intersected with grants, no owner flags and no roles. The view keeps the real
// user ID for audit and ownership. Installation and grants must be resolved by the
// host, never supplied by the client.
func DelegatePlugin(ctx context.Context, installationID int, grants []string) (context.Context, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("plugin delegation requires authenticated user")
	}
	if decision, exists := privacy.DecisionFromContext(ctx); exists && decision == nil {
		return nil, fmt.Errorf("cannot delegate a privacy bypass context")
	}
	if _, exists := contexts.GetAPIKey(ctx); exists {
		return nil, fmt.Errorf("plugin delegation cannot inherit API key identity")
	}
	if IsBypassActive(ctx) {
		return nil, fmt.Errorf("cannot delegate a system bypass context")
	}
	view := scopes.DelegatedUserView(user, grants)
	return identity.Delegate(contexts.WithUser(contexts.ForkRequest(ctx), view), user.ID, installationID, grants, view)
}

func replacePrincipal(ctx context.Context, p Principal, reason string) (context.Context, error) {
	next, err := identity.Replace(ctx, p)
	if err != nil {
		auditTransitionDenied(ctx, p, reason)
	}
	return next, err
}

// RequireDirectUser protects self-service actions that cannot be delegated.
func RequireDirectUser(ctx context.Context) error {
	p, ok := GetPrincipal(ctx)
	user, exists := contexts.GetUser(ctx)
	if !ok || p.Type != PrincipalTypeUser || p.UserID == nil || !exists || user == nil || *p.UserID != user.ID {
		return privacy.Denyf("direct authenticated user required")
	}
	return nil
}

func auditTransitionDenied(ctx context.Context, p Principal, reason string) {
	original, _ := GetPrincipal(ctx)
	log.Warn(ctx, "authz: principal transition denied", log.String("principal", original.String()), log.String("target", p.String()), log.Int("plugin_installation_id", original.PluginInstallationID), log.String("reason", reason))
}

// mustReplacePrincipal enforces a host programming invariant, not a business permission decision.
func mustReplacePrincipal(ctx context.Context, p Principal, reason string) context.Context {
	next, err := replacePrincipal(ctx, p, reason)
	if err != nil {
		panic(err)
	}
	return next
}
