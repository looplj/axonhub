package scopes

import (
	"context"
	"errors"
	"testing"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// Mock query for testing.
type mockQuery struct{}

func TestAPIKeyQueryRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope Scope
		expectAllow   bool
	}{
		{
			name:          "no API key in context",
			ctx:           context.Background(),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name:          "nil API key in context",
			ctx:           contexts.WithAPIKey(context.Background(), nil),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "API key with required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{"read_users", "write_users"},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   true,
		},
		{
			name: "API key without required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{"read_channels", "write_channels"},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
		{
			name: "API key with empty scopes",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{},
			}),
			requiredScope: ScopeReadUsers,
			expectAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := APIKeyQueryRule(tt.requiredScope)
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

func TestAPIKeyScopeMutationRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope Scope
		expectAllow   bool
	}{
		{
			name:          "no API key in context",
			ctx:           context.Background(),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
		{
			name:          "nil API key in context",
			ctx:           contexts.WithAPIKey(context.Background(), nil),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
		{
			name: "API key with required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{"read_users", "write_users"},
			}),
			requiredScope: ScopeWriteUsers,
			expectAllow:   true,
		},
		{
			name: "API key without required scope",
			ctx: contexts.WithAPIKey(context.Background(), &ent.APIKey{
				ID:     1,
				Scopes: []string{"read_users"},
			}),
			requiredScope: ScopeWriteUsers,
			expectAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := APIKeyScopeMutationRule(tt.requiredScope)
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

// Mock mutation for testing.
type mockMutation struct {
	ent.Mutation
}

func (m *mockMutation) Op() ent.Op {
	return ent.OpCreate
}

func (m *mockMutation) Type() string {
	return "Mock"
}
