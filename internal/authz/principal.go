package authz

import (
	"context"

	"github.com/looplj/axonhub/internal/authz/identity"
)

type (
	Principal     = identity.Principal
	PrincipalType = identity.PrincipalType
)

const (
	PrincipalTypeUnknown = identity.PrincipalTypeUnknown
	PrincipalTypeSystem  = identity.PrincipalTypeSystem
	PrincipalTypeUser    = identity.PrincipalTypeUser
	PrincipalTypeAPIKey  = identity.PrincipalTypeAPIKey
	PrincipalTypeTest    = identity.PrincipalTypeTest
	PrincipalTypePlugin  = identity.PrincipalTypePlugin
)

func GetPrincipal(ctx context.Context) (Principal, bool) { return identity.GetPrincipal(ctx) }
func MustGetPrincipal(ctx context.Context) Principal     { return identity.MustGetPrincipal(ctx) }
func WithPrincipal(ctx context.Context, p Principal) (context.Context, error) {
	next, err := identity.WithPrincipal(ctx, p)
	if err != nil {
		auditTransitionDenied(ctx, p, "with-principal")
	}
	return next, err
}
func principalEqual(a, b Principal) bool { return identity.Equal(a, b) }

// NewUserContext establishes a direct user identity. Replacing a Plugin is a programming error.
func NewUserContext(ctx context.Context, userID int) context.Context {
	return mustReplacePrincipal(ctx, Principal{Type: PrincipalTypeUser, UserID: &userID}, "new-user-context")
}

func NewAPIKeyContext(ctx context.Context, apiKeyID, projectID int) context.Context {
	return mustReplacePrincipal(ctx, Principal{Type: PrincipalTypeAPIKey, APIKeyID: &apiKeyID, ProjectID: &projectID}, "new-api-key-context")
}
