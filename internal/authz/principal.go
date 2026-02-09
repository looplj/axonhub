package authz

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
	case PrincipalTypeTest:
		return "test"
	default:
		return "unknown"
	}
}

// Principal represents authorization principal.
// Each request can only have one Principal, guaranteed by WithPrincipal's set-once semantics.
type Principal struct {
	Type      PrincipalType
	UserID    *int
	APIKeyID  *int
	ProjectID *int
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
		if existing.Type != p.Type || !principalEqual(existing, p) {
			return ctx, fmt.Errorf("authz: principal conflict: existing=%s, new=%s", existing.String(), p.String())
		}

		return ctx, nil // Same principal, idempotent
	}

	return context.WithValue(ctx, principalKey{}, p), nil
}

// principalEqual compares if two Principals are equal.
func principalEqual(a, b Principal) bool {
	if a.Type != b.Type {
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
	return p, ok
}

// MustGetPrincipal reads Principal, panics if not exists (used in chains where principal is confirmed).
func MustGetPrincipal(ctx context.Context) Principal {
	p, ok := GetPrincipal(ctx)
	if !ok {
		panic("authz: no principal in context")
	}

	return p
}

// NewUserContext creates context with User principal.
func NewUserContext(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, principalKey{}, Principal{
		Type:   PrincipalTypeUser,
		UserID: &userID,
	})
}

// NewAPIKeyContext creates context with APIKey principal.
func NewAPIKeyContext(ctx context.Context, apiKeyID, projectID int) context.Context {
	return context.WithValue(ctx, principalKey{}, Principal{
		Type:      PrincipalTypeAPIKey,
		APIKeyID:  &apiKeyID,
		ProjectID: &projectID,
	})
}
