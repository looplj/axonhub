package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestPluginAuthorizationBoundary(t *testing.T) {
	owner := &ent.User{ID: 9, IsOwner: true}
	parent := contexts.WithUser(NewUserContext(context.Background(), owner.ID), owner)
	ctx, err := DelegatePlugin(parent, 10, []string{"read_channels"})
	require.NoError(t, err)
	view, _ := contexts.GetUser(ctx)
	require.NotSame(t, owner, view)
	require.Equal(t, owner.ID, view.ID)
	require.False(t, view.IsOwner)
	require.True(t, HasScope(ctx, scopes.ScopeReadChannels))
	require.False(t, HasScope(ctx, scopes.ScopeWriteChannels))
	require.False(t, scopes.IsDirectOwner(ctx))
	// Scope decisions behave the same for every principal: granted → Allow, otherwise Deny.
	allowed := WithScopeDecision(ctx, scopes.ScopeReadChannels)
	decision, hasDecision := privacy.DecisionFromContext(allowed)
	require.True(t, hasDecision)
	require.NoError(t, decision)
	denied := WithScopeDecision(ctx, scopes.ScopeWriteChannels)
	decision, hasDecision = privacy.DecisionFromContext(denied)
	require.True(t, hasDecision)
	require.Error(t, decision)
	called := false
	original, _ := GetPrincipal(ctx)
	for name, call := range map[string]func(){
		"user":        func() { NewUserContext(ctx, owner.ID) },
		"api-key":     func() { NewAPIKeyContext(ctx, 4, 1) },
		"system":      func() { NewSystemContext(ctx) },
		"test":        func() { NewTestContext(ctx) },
		"test-bypass": func() { WithTestBypass(ctx) },
	} {
		t.Run(name, func(t *testing.T) {
			require.Panics(t, call)
			current, _ := GetPrincipal(ctx)
			require.True(t, principalEqual(original, current))
			require.False(t, IsBypassActive(ctx))
		})
	}
	t.Run("system-bypass fails closed", func(t *testing.T) {
		denied := WithSystemBypass(ctx, "test-reject")
		current, _ := GetPrincipal(denied)
		require.True(t, principalEqual(original, current))
		require.False(t, IsBypassActive(denied))
		decision, ok := privacy.DecisionFromContext(denied)
		require.True(t, ok)
		require.Error(t, decision)
		require.Error(t, RunWithSystemBypassVoid(ctx, "test-reject", func(context.Context) error { called = true; return nil }))
		_, err := RunWithSystemBypass(ctx, "test-reject", func(context.Context) (bool, error) { called = true; return true, nil })
		require.Error(t, err)
		require.False(t, called)
	})
	_, err = RunWithScopeDecision(ctx, scopes.ScopeWriteChannels, func(ctx context.Context) (bool, error) {
		decision, _ := privacy.DecisionFromContext(ctx)
		return true, decision
	})
	require.Error(t, err)
	fork := contexts.ForkRequest(ctx)
	require.False(t, HasScope(fork, scopes.ScopeWriteChannels))
	// Replacing the container user cannot restore owner authority: GetUser keeps the pinned view.
	contexts.WithUser(fork, &ent.User{ID: 9, IsOwner: true})
	replaced, _ := contexts.GetUser(fork)
	require.False(t, replaced.IsOwner)
	require.False(t, scopes.IsDirectOwner(fork))
	require.False(t, HasScope(fork, scopes.ScopeWriteChannels))
	require.True(t, HasScope(ctx, scopes.ScopeReadChannels))
	require.True(t, owner.IsOwner)
	_, err = DelegatePlugin(privacy.DecisionContext(parent, privacy.Allow), 10, nil)
	require.Error(t, err)
}
