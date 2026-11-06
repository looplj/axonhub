package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/looplj/axonhub/internal/authz/identity"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// ErrPluginSystemBypass identifies an attempted System elevation by a plugin.
var ErrPluginSystemBypass = errors.New("plugin principal cannot use system bypass")

// NewSystemContext panics on an illegal Plugin identity replacement, before any side effect.
func NewSystemContext(ctx context.Context) context.Context {
	return mustReplacePrincipal(ctx, Principal{Type: PrincipalTypeSystem}, "new-system-context")
}

// WithSystemBypass grants System authority for internal operations. A delegated (plugin)
// caller never gains it: the denial is audited and the returned context keeps the Plugin
// principal and denies every Ent operation. Fixed host operations use RunAsHost.
func WithSystemBypass(ctx context.Context, reason string) context.Context {
	if err := denyPluginSystemBypass(ctx, reason); err != nil {
		return privacy.DecisionContext(ctx, fmt.Errorf("%w: %w", privacy.Deny, err))
	}
	next := mustReplacePrincipal(ctx, Principal{Type: PrincipalTypeSystem}, reason)
	bypass, err := WithBypassPrivacy(next, reason)
	if err != nil {
		panic(err)
	}
	return bypass
}

// RunWithSystemBypass returns an error without calling fn for a delegated (plugin) caller.
func RunWithSystemBypass[T any](ctx context.Context, reason string, fn func(context.Context) (T, error)) (T, error) {
	if err := denyPluginSystemBypass(ctx, reason); err != nil {
		var zero T
		return zero, err
	}
	return fn(WithSystemBypass(ctx, reason))
}

// RunWithSystemBypassVoid returns an error without calling fn for a delegated (plugin) caller.
func RunWithSystemBypassVoid(ctx context.Context, reason string, fn func(context.Context) error) error {
	if err := denyPluginSystemBypass(ctx, reason); err != nil {
		return err
	}
	return fn(WithSystemBypass(ctx, reason))
}

func denyPluginSystemBypass(ctx context.Context, reason string) error {
	if !identity.IsPlugin(ctx) {
		return nil
	}
	auditTransitionDenied(ctx, Principal{Type: PrincipalTypeSystem}, reason)
	return fmt.Errorf("authz: %w: %s", ErrPluginSystemBypass, reason)
}

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
