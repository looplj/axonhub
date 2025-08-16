package scopes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

func TestAlwaysDeny(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectError assert.ErrorAssertionFunc
	}{
		{
			name: "no user in context",
			ctx:  context.Background(),
			expectError: func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(tt, err, privacy.Deny)
			},
		},
		{
			name: "nil user in context",
			ctx:  contexts.WithUser(context.Background(), nil),
			expectError: func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(tt, err, privacy.Deny)
			},
		},
		{
			name: "valid user in context",
			ctx:  contexts.WithUser(context.Background(), &ent.User{ID: 1}),
			expectError: func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(tt, err, privacy.Deny)
			},
		},
	}

	rule := AlwaysDeny()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.EvalQuery(tt.ctx, nil)

			tt.expectError(t, err)
		})
	}
}

func TestHasScope(t *testing.T) {
	tests := []struct {
		name          string
		userScopes    []string
		requiredScope string
		expected      bool
	}{
		{
			name:          "scope exists",
			userScopes:    []string{"read_users", "write_users", "read_channels"},
			requiredScope: "read_users",
			expected:      true,
		},
		{
			name:          "scope does not exist",
			userScopes:    []string{"read_users", "write_users"},
			requiredScope: "read_channels",
			expected:      false,
		},
		{
			name:          "empty scopes",
			userScopes:    []string{},
			requiredScope: "read_users",
			expected:      false,
		},
		{
			name:          "nil scopes",
			userScopes:    nil,
			requiredScope: "read_users",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasScope(tt.userScopes, tt.requiredScope)
			if result != tt.expected {
				t.Errorf("hasScope() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestHasRoleScope(t *testing.T) {
	tests := []struct {
		name          string
		user          *ent.User
		requiredScope Scope
		expected      bool
	}{
		{
			name: "user has role with required scope",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"read_users", "write_users"},
						},
					},
				},
			},
			requiredScope: ScopeReadUsers,
			expected:      true,
		},
		{
			name: "user has role without required scope",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"read_channels", "write_channels"},
						},
					},
				},
			},
			requiredScope: ScopeReadUsers,
			expected:      false,
		},
		{
			name: "user has multiple roles, one with required scope",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"read_channels"},
						},
						{
							ID:     2,
							Scopes: []string{"read_users", "write_users"},
						},
					},
				},
			},
			requiredScope: ScopeReadUsers,
			expected:      true,
		},
		{
			name: "user has no roles",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					Roles: nil,
				},
			},
			requiredScope: ScopeReadUsers,
			expected:      false,
		},
		{
			name: "user has empty roles",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					Roles: []*ent.Role{},
				},
			},
			requiredScope: ScopeReadUsers,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRoleScope(tt.user, tt.requiredScope)
			if result != tt.expected {
				t.Errorf("hasRoleScope() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCheckUserPermission(t *testing.T) {
	tests := []struct {
		name          string
		user          *ent.User
		requiredScope Scope
		expected      bool
	}{
		{
			name: "owner user",
			user: &ent.User{
				ID:      1,
				IsOwner: true,
			},
			requiredScope: ScopeReadUsers,
			expected:      true,
		},
		{
			name: "user with direct scope",
			user: &ent.User{
				ID:      1,
				IsOwner: false,
				Scopes:  []string{"read_users", "write_users"},
			},
			requiredScope: ScopeReadUsers,
			expected:      true,
		},
		{
			name: "user with role scope",
			user: &ent.User{
				ID:      1,
				IsOwner: false,
				Scopes:  []string{},
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"read_users"},
						},
					},
				},
			},
			requiredScope: ScopeReadUsers,
			expected:      true,
		},
		{
			name: "user without required scope",
			user: &ent.User{
				ID:      1,
				IsOwner: false,
				Scopes:  []string{"read_channels"},
				Edges: ent.UserEdges{
					Roles: []*ent.Role{
						{
							ID:     1,
							Scopes: []string{"write_channels"},
						},
					},
				},
			},
			requiredScope: ScopeReadUsers,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkUserPermission(tt.user, tt.requiredScope)
			if result != tt.expected {
				t.Errorf("checkUserPermission() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetUserFromContext(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectError bool
		expectedID  int
	}{
		{
			name:        "no user in context",
			ctx:         context.Background(),
			expectError: true,
		},
		{
			name:        "nil user in context",
			ctx:         contexts.WithUser(context.Background(), nil),
			expectError: true,
		},
		{
			name:        "valid user in context",
			ctx:         contexts.WithUser(context.Background(), &ent.User{ID: 123}),
			expectError: false,
			expectedID:  123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := getUserFromContext(tt.ctx)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if user.ID != tt.expectedID {
				t.Errorf("expected user ID %d, got %d", tt.expectedID, user.ID)
			}
		})
	}
}

func TestGetAPIKeyFromContext(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectError bool
		expectedKey string
	}{
		{
			name:        "no API key in context",
			ctx:         context.Background(),
			expectError: true,
		},
		{
			name:        "nil API key in context",
			ctx:         contexts.WithAPIKey(context.Background(), nil),
			expectError: true,
		},
		{
			name:        "valid API key in context",
			ctx:         contexts.WithAPIKey(context.Background(), &ent.APIKey{ID: 1, Key: "test-key"}),
			expectError: false,
			expectedKey: "test-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiKey, err := getAPIKeyFromContext(tt.ctx)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if apiKey.Key != tt.expectedKey {
				t.Errorf("expected API key %s, got %s", tt.expectedKey, apiKey.Key)
			}
		})
	}
}
