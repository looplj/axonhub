package identity

import (
	"context"
	"fmt"
)

// PrincipalType defines authorization principal types.
type PrincipalType int

const (
	// PrincipalTypeUnknown unknown principal type.
	PrincipalTypeUnknown PrincipalType = iota
	// PrincipalTypeSystem system principal (background tasks, internal operations).
	PrincipalTypeSystem PrincipalType = iota
	// PrincipalTypeUser user principal.
	PrincipalTypeUser
	// PrincipalTypeAPIKey API Key principal.
	PrincipalTypeAPIKey
	// PrincipalTypeTest test principal (only for test environment).
	PrincipalTypeTest
	PrincipalTypePlugin
)

// String returns string representation of PrincipalType.
func (p PrincipalType) String() string {
	switch p {
	case PrincipalTypeUnknown:
		return "unknown"
	case PrincipalTypeSystem:
		return "system"
	case PrincipalTypeUser:
		return "user"
	case PrincipalTypeAPIKey:
		return "apikey"
	case PrincipalTypePlugin:
		return "plugin"
	case PrincipalTypeTest:
		return "test"
	default:
		return "unknown"
	}
}

// Principal represents authorization principal.
// Each request can only have one Principal, guaranteed by WithPrincipal's set-once semantics.
type Principal struct {
	Type                 PrincipalType
	UserID               *int
	APIKeyID             *int
	ProjectID            *int
	PluginInstallationID int
}

// IsSystem checks if it is a system principal.
func (p Principal) IsSystem() bool {
	return p.Type == PrincipalTypeSystem
}

// IsUser checks if it is a user principal.
func (p Principal) IsUser() bool {
	return p.Type == PrincipalTypeUser
}

// IsAPIKey checks if it is an API Key principal.
func (p Principal) IsAPIKey() bool {
	return p.Type == PrincipalTypeAPIKey
}

// IsTest checks if it is a test principal.
func (p Principal) IsTest() bool {
	return p.Type == PrincipalTypeTest
}

// String returns string representation of Principal (for audit logs).
func (p Principal) String() string {
	switch p.Type {
	case PrincipalTypePlugin:
		if p.UserID != nil {
			return fmt.Sprintf("plugin:%d:user:%d", p.PluginInstallationID, *p.UserID)
		}
		return fmt.Sprintf("plugin:%d:user:unknown", p.PluginInstallationID)
	case PrincipalTypeUnknown:
		return "unknown"
	case PrincipalTypeSystem:
		return "system"
	case PrincipalTypeUser:
		if p.UserID != nil {
			return fmt.Sprintf("user:%d", *p.UserID)
		}

		return "user:unknown"
	case PrincipalTypeAPIKey:
		if p.APIKeyID != nil {
			return fmt.Sprintf("apikey:%d", *p.APIKeyID)
		}

		return "apikey:unknown"
	case PrincipalTypeTest:
		return "test"
	default:
		return "unknown"
	}
}

// principalKey is an unexported key type to prevent external forgery.
type principalKey struct{}

// WithPrincipal sets Principal, returns error if already exists.
// Ensures each context can only set Principal once, preventing principal mixing.
func WithPrincipal(ctx context.Context, p Principal) (context.Context, error) {
	if existing, ok := GetPrincipal(ctx); ok {
		if existing.Type != p.Type || !Equal(existing, p) {
			return ctx, fmt.Errorf("authz: principal conflict: existing=%s, new=%s", existing.String(), p.String())
		}

		return ctx, nil // Same principal, idempotent
	}

	if p.Type == PrincipalTypePlugin {
		return ctx, fmt.Errorf("plugin principal requires delegation")
	}
	return context.WithValue(ctx, principalKey{}, clone(p)), nil
}

// Equal reports whether two Principals are equal.
func Equal(a, b Principal) bool {
	if a.Type != b.Type || a.PluginInstallationID != b.PluginInstallationID {
		return false
	}

	if !intPtrEqual(a.UserID, b.UserID) {
		return false
	}

	if !intPtrEqual(a.APIKeyID, b.APIKeyID) {
		return false
	}

	if !intPtrEqual(a.ProjectID, b.ProjectID) {
		return false
	}

	return true
}

// intPtrEqual compares if two *int are equal.
func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return *a == *b
}

// GetPrincipal reads Principal.
func GetPrincipal(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return clone(p), ok
}

// MustGetPrincipal reads Principal, panics if not exists (used in chains where principal is confirmed).
func MustGetPrincipal(ctx context.Context) Principal {
	p, ok := GetPrincipal(ctx)
	if !ok {
		panic("authz: no principal in context")
	}

	return p
}

func clone(p Principal) Principal {
	if p.UserID != nil {
		v := *p.UserID
		p.UserID = &v
	}
	if p.APIKeyID != nil {
		v := *p.APIKeyID
		p.APIKeyID = &v
	}
	if p.ProjectID != nil {
		v := *p.ProjectID
		p.ProjectID = &v
	}
	return p
}

// Replace preserves existing non-plugin internal transitions, but cannot erase a delegation.
func Replace(ctx context.Context, p Principal) (context.Context, error) {
	if IsPlugin(ctx) {
		return nil, fmt.Errorf("plugin principal cannot be replaced")
	}
	if p.Type == PrincipalTypePlugin {
		return nil, fmt.Errorf("plugin principal requires delegation")
	}
	return context.WithValue(ctx, principalKey{}, clone(p)), nil
}

func IsPlugin(ctx context.Context) bool {
	p, ok := GetPrincipal(ctx)
	return ok && p.Type == PrincipalTypePlugin
}

type (
	delegationKey struct{}
	delegation    struct {
		userID, installationID int
		grants                 map[string]bool
		// userView is the restricted user entity, opaque here to keep this package free of Ent.
		userView any
	}
)

// Delegate is the shared identity primitive. Authentication and privacy checks belong to authz.
// userView is pinned for the delegation, so replacing the request's user entity later
// cannot restore the real user's authority.
func Delegate(ctx context.Context, userID, installationID int, grants []string, userView any) (context.Context, error) {
	p, ok := GetPrincipal(ctx)
	if !ok || p.Type != PrincipalTypeUser || p.UserID == nil || *p.UserID != userID || userID <= 0 || installationID <= 0 || userView == nil {
		return nil, fmt.Errorf("invalid plugin delegation")
	}
	snapshot := delegation{userID: userID, installationID: installationID, grants: make(map[string]bool, len(grants)), userView: userView}
	for _, grant := range grants {
		snapshot.grants[grant] = true
	}
	p.Type = PrincipalTypePlugin
	p.PluginInstallationID = installationID
	ctx = context.WithValue(ctx, delegationKey{}, snapshot)
	return context.WithValue(ctx, principalKey{}, clone(p)), nil
}

func ValidPlugin(ctx context.Context, userID int) bool {
	p, ok := GetPrincipal(ctx)
	d, exists := ctx.Value(delegationKey{}).(delegation)
	return ok && exists && p.Type == PrincipalTypePlugin && p.UserID != nil && *p.UserID == userID && d.userID == userID && p.PluginInstallationID > 0 && p.PluginInstallationID == d.installationID && p.APIKeyID == nil
}

// DelegatedUserView returns the user entity pinned by Delegate.
func DelegatedUserView(ctx context.Context) (any, bool) {
	d, ok := ctx.Value(delegationKey{}).(delegation)
	if !ok {
		return nil, false
	}
	return d.userView, true
}

func PluginGranted(ctx context.Context, userID int, scope string) bool {
	if !ValidPlugin(ctx, userID) {
		return false
	}
	d, ok := ctx.Value(delegationKey{}).(delegation)
	return ok && d.grants[scope]
}

func (p Principal) IsPlugin() bool { return p.Type == PrincipalTypePlugin }
