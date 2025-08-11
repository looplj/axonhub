package scopes

import (
	"context"
	"errors"
	"testing"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

func TestOwnerRule(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectAllow bool
	}{
		{
			name:        "no user in context",
			ctx:         context.Background(),
			expectAllow: false,
		},
		{
			name:        "nil user in context",
			ctx:         contexts.WithUser(context.Background(), nil),
			expectAllow: false,
		},
		{
			name: "owner user in context",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:      1,
				IsOwner: true,
			}),
			expectAllow: true,
		},
		{
			name: "non-owner user in context",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:      1,
				IsOwner: false,
			}),
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := OwnerRule()

			// Test query evaluation
			err := rule.EvalQuery(tt.ctx, &mockQuery{})
			if tt.expectAllow {
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("EvalQuery: expected privacy.Allow, got %v", err)
				}
			} else {
				if errors.Is(err, privacy.Allow) {
					t.Error("EvalQuery: expected error or deny, got privacy.Allow")
				}
			}

			// Test mutation evaluation
			err = rule.EvalMutation(tt.ctx, &mockMutation{})
			if tt.expectAllow {
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("EvalMutation: expected privacy.Allow, got %v", err)
				}
			} else {
				if errors.Is(err, privacy.Allow) {
					t.Error("EvalMutation: expected error or deny, got privacy.Allow")
				}
			}
		})
	}
}
