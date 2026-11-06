package authz

import "context"

func NewTestContext(ctx context.Context) context.Context {
	return mustReplacePrincipal(ctx, Principal{Type: PrincipalTypeTest}, "new-test-context")
}
func WithTestBypass(ctx context.Context) context.Context {
	next := NewTestContext(ctx)
	bypass, err := WithBypassPrivacy(next, "test")
	if err != nil {
		panic(err)
	}
	return bypass
}
