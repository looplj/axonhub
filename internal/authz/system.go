package authz

import (
	"context"
	"fmt"
)

// NewSystemContext creates context with System principal (for background tasks).
func NewSystemContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, principalKey{}, Principal{Type: PrincipalTypeSystem})
}

func WithSystemBypass(ctx context.Context, reason string) context.Context {
	bypassCtx, _ := WithBypassPrivacy(NewSystemContext(ctx), reason)
	return bypassCtx
}

func RunWithSystemBypass[T any](ctx context.Context, reason string, fn func(ctx context.Context) (T, error)) (T, error) {
	bypassCtx := WithSystemBypass(ctx, reason)
	return RunWithBypass(bypassCtx, reason, fn)
}

// RequireSystemPrincipal checks if current principal is System, otherwise returns error.
// Used to protect sensitive background operations.
func RequireSystemPrincipal(ctx context.Context) error {
	p, ok := GetPrincipal(ctx)
	if !ok {
		return fmt.Errorf("authz: no principal in context")
	}

	if !p.IsSystem() {
		return fmt.Errorf("authz: operation requires system principal, got %s", p.String())
	}

	return nil
}
