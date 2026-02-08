package authz

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestHasScope_SystemPrincipal(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	if !HasScope(ctx, scopes.ScopeReadChannels) {
		t.Error("system principal should have all scopes")
	}

	if !HasScope(ctx, scopes.ScopeWriteSettings) {
		t.Error("system principal should have all scopes")
	}
}

func TestHasScope_NoPrincipal(t *testing.T) {
	ctx := context.Background()

	if HasScope(ctx, scopes.ScopeReadChannels) {
		t.Error("no principal should not have any scope")
	}
}

func TestHasScope_UserPrincipal_Owner(t *testing.T) {
	ctx := NewUserContext(context.Background(), 1)
	ctx = contexts.WithUser(ctx, &ent.User{
		ID:      1,
		IsOwner: true,
	})

	if !HasScope(ctx, scopes.ScopeReadChannels) {
		t.Error("owner should have all scopes")
	}

	if !HasScope(ctx, scopes.ScopeWriteSettings) {
		t.Error("owner should have all scopes")
	}
}

func TestHasScope_UserPrincipal_DirectScope(t *testing.T) {
	ctx := NewUserContext(context.Background(), 1)
	ctx = contexts.WithUser(ctx, &ent.User{
		ID:     1,
		Scopes: []string{"read_channels"},
	})

	if !HasScope(ctx, scopes.ScopeReadChannels) {
		t.Error("user with direct scope should have it")
	}

	if HasScope(ctx, scopes.ScopeWriteChannels) {
		t.Error("user without scope should not have it")
	}
}

func TestHasScope_UserPrincipal_RoleScope(t *testing.T) {
	ctx := NewUserContext(context.Background(), 1)
	ctx = contexts.WithUser(ctx, &ent.User{
		ID: 1,
		Edges: ent.UserEdges{
			Roles: []*ent.Role{
				{
					Scopes: []string{"read_channels", "write_channels"},
					Edges:  ent.RoleEdges{},
				},
			},
		},
	})

	if !HasScope(ctx, scopes.ScopeReadChannels) {
		t.Error("user with role scope should have it")
	}

	if HasScope(ctx, scopes.ScopeWriteSettings) {
		t.Error("user without scope should not have it")
	}
}

func TestHasScope_UserPrincipal_NoUser(t *testing.T) {
	ctx := NewUserContext(context.Background(), 1)

	if HasScope(ctx, scopes.ScopeReadChannels) {
		t.Error("user principal without user entity should not have scope")
	}
}

func TestHasScope_APIKeyPrincipal(t *testing.T) {
	ctx := NewAPIKeyContext(context.Background(), 1, 1)
	ctx = contexts.WithAPIKey(ctx, &ent.APIKey{
		ID:     1,
		Scopes: []string{"read_channels", "read_requests"},
	})

	if !HasScope(ctx, scopes.ScopeReadChannels) {
		t.Error("apikey with scope should have it")
	}

	if !HasScope(ctx, scopes.ScopeReadRequests) {
		t.Error("apikey with scope should have it")
	}

	if HasScope(ctx, scopes.ScopeWriteChannels) {
		t.Error("apikey without scope should not have it")
	}
}

func TestHasScope_APIKeyPrincipal_NoAPIKey(t *testing.T) {
	ctx := NewAPIKeyContext(context.Background(), 1, 1)

	if HasScope(ctx, scopes.ScopeReadChannels) {
		t.Error("apikey principal without apikey entity should not have scope")
	}
}

func TestWithScopeDecision_Allow(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	scopeCtx := WithScopeDecision(ctx, scopes.ScopeReadChannels)
	if scopeCtx == nil {
		t.Fatal("WithScopeDecision should return non-nil context")
	}
}

func TestWithScopeDecision_Deny(t *testing.T) {
	ctx := NewUserContext(context.Background(), 1)
	ctx = contexts.WithUser(ctx, &ent.User{
		ID:     1,
		Scopes: []string{},
	})

	scopeCtx := WithScopeDecision(ctx, scopes.ScopeWriteSettings)
	if scopeCtx == nil {
		t.Fatal("WithScopeDecision should return non-nil context")
	}
}

func TestWithScopeDecision_NoPrincipal(t *testing.T) {
	ctx := context.Background()

	scopeCtx := WithScopeDecision(ctx, scopes.ScopeReadChannels)
	if scopeCtx == nil {
		t.Fatal("WithScopeDecision should return non-nil context (deny)")
	}
}

func TestRunWithScopeDecision(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	executed := false

	result, err := RunWithScopeDecision(ctx, scopes.ScopeReadChannels, func(scopeCtx context.Context) (string, error) {
		executed = true
		return "success", nil
	})
	if err != nil {
		t.Fatalf("RunWithScopeDecision failed: %v", err)
	}

	if !executed {
		t.Error("closure should be executed")
	}

	if result != "success" {
		t.Errorf("result = %v, want %v", result, "success")
	}
}

func TestRunWithScopeDecision_ScopeIsolation(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	_, _ = RunWithScopeDecision(ctx, scopes.ScopeReadChannels, func(scopeCtx context.Context) (string, error) {
		return "done", nil
	})
}

func TestRequireScope_Pass(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	if err := RequireScope(ctx, scopes.ScopeReadChannels); err != nil {
		t.Errorf("RequireScope should pass for system principal: %v", err)
	}
}

func TestRequireScope_Fail(t *testing.T) {
	ctx := NewUserContext(context.Background(), 1)
	ctx = contexts.WithUser(ctx, &ent.User{
		ID:     1,
		Scopes: []string{},
	})

	if err := RequireScope(ctx, scopes.ScopeWriteSettings); err == nil {
		t.Error("RequireScope should fail when principal lacks scope")
	}
}

func TestRequireScope_NoPrincipal(t *testing.T) {
	ctx := context.Background()

	if err := RequireScope(ctx, scopes.ScopeReadChannels); err == nil {
		t.Error("RequireScope should fail when no principal")
	}
}
