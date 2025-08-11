package scopes

import (
	"context"
	"errors"
	"testing"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

func TestReadScopeRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope Scope
		expectAllow   bool
	}{
		{
			name:          "no user in context",
			ctx:           context.Background(),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name:          "nil user in context",
			ctx:           contexts.WithUser(context.Background(), nil),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "user with required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{"read_users", "write_users"},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   true,
		},
		{
			name: "user without required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{"read_channels", "write_channels"},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "user with empty scopes",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "user with role that has required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{},
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"read_users", "write_users"},
						},
					},
				},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   true,
		},
		{
			name: "user with role that doesn't have required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{},
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"read_channels"},
						},
					},
				},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "user with multiple roles, one has required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{},
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"read_channels"},
						},
						{
							ID:     2,
							Scopes: []string{"read_users"},
						},
					},
				},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := UserReadScopeRule(tt.requiredScope)
			err := rule.EvalQuery(tt.ctx, &mockQuery{})

			if tt.expectAllow {
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("expected privacy.Allow, got %v", err)
				}
			} else {
				if errors.Is(err, privacy.Allow) {
					t.Error("expected error or deny, got privacy.Allow")
				}
			}
		})
	}
}

func TestWriteScopeRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope Scope
		expectAllow   bool
	}{
		{
			name:          "no user in context",
			ctx:           context.Background(),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
		{
			name:          "nil user in context",
			ctx:           contexts.WithUser(context.Background(), nil),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
		{
			name: "user with required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{"read_users", "write_users"},
			}),
			requiredScope: ScopeWriteUsers,
			expectAllow:   true,
		},
		{
			name: "user without required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{"read_users"},
			}),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
		{
			name: "user with role that has required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{},
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"write_users"},
						},
					},
				},
			}),
			requiredScope: ScopeWriteUsers,
			expectAllow:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := UserWriteScopeRule(tt.requiredScope)
			err := rule.EvalMutation(tt.ctx, &mockMutation{})

			if tt.expectAllow {
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("expected privacy.Allow, got %v", err)
				}
			} else {
				if errors.Is(err, privacy.Allow) {
					t.Error("expected error or deny, got privacy.Allow")
				}
			}
		})
	}
}

func TestScopeQueryRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope Scope
		expectAllow   bool
	}{
		{
			name:          "no user in context",
			ctx:           context.Background(),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "user with required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{"read_users"},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   true,
		},
		{
			name: "user without required scope",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{"read_channels"},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := userScopeQueryRule{requiredScope: tt.requiredScope}
			err := rule.EvalQuery(tt.ctx, &mockQuery{})

			if tt.expectAllow {
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("expected privacy.Allow, got %v", err)
				}
			} else {
				if errors.Is(err, privacy.Allow) {
					t.Error("expected error or deny, got privacy.Allow")
				}
			}
		})
	}
}
