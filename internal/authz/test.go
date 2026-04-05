package authz

import (
	"context"
)

// NewTestContext creates context with Test principal (only for test environment).
func NewTestContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, principalKey{}, Principal{Type: PrincipalTypeTest})
}

// WithTestBypass creates context with Test principal and bypass privacy.
// Used to replace privacy.DecisionContext(ctx, privacy.Allow) in tests.
func WithTestBypass(ctx context.Context) context.Context {
	bypassCtx, _ := WithBypassPrivacy(NewTestContext(ctx), "test")
	return bypassCtx
}
