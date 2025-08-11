package scopes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// mockQueryRule is a mock implementation of privacy.QueryRule for testing.
type mockQueryRule struct {
	decision error
}

func (m mockQueryRule) EvalQuery(ctx context.Context, q ent.Query) error {
	return m.decision
}

// mockMutationRule is a mock implementation of privacy.MutationRule for testing.
type mockMutationRule struct {
	decision error
}

func (m mockMutationRule) EvalMutation(ctx context.Context, mutation ent.Mutation) error {
	return m.decision
}

func TestQueryPolicy_EvalQuery(t *testing.T) {
	tests := []struct {
		name     string
		policies QueryPolicy
		expected error
	}{
		{
			name:     "empty policy should deny",
			policies: QueryPolicy{},
			expected: privacy.Deny,
		},
		{
			name: "policy with allow rule should allow",
			policies: QueryPolicy{
				mockQueryRule{decision: privacy.Allow},
			},
			expected: privacy.Allow,
		},
		{
			name: "policy with deny rule should deny",
			policies: QueryPolicy{
				mockQueryRule{decision: privacy.Deny},
			},
			expected: privacy.Deny,
		},
		{
			name: "policy with skip rule should deny by default",
			policies: QueryPolicy{
				mockQueryRule{decision: privacy.Skip},
			},
			expected: privacy.Deny,
		},
		{
			name: "policy with nil rule should deny by default",
			policies: QueryPolicy{
				mockQueryRule{decision: nil},
			},
			expected: privacy.Deny,
		},
		{
			name: "multiple rules - first allow should return allow",
			policies: QueryPolicy{
				mockQueryRule{decision: privacy.Allow},
				mockQueryRule{decision: privacy.Deny},
			},
			expected: privacy.Allow,
		},
		{
			name: "multiple rules - first deny should return deny",
			policies: QueryPolicy{
				mockQueryRule{decision: privacy.Deny},
				mockQueryRule{decision: privacy.Allow},
			},
			expected: privacy.Deny,
		},
		{
			name: "multiple rules - skip then allow should return allow",
			policies: QueryPolicy{
				mockQueryRule{decision: privacy.Skip},
				mockQueryRule{decision: privacy.Allow},
			},
			expected: privacy.Allow,
		},
		{
			name: "multiple rules - all skip should deny by default",
			policies: QueryPolicy{
				mockQueryRule{decision: privacy.Skip},
				mockQueryRule{decision: privacy.Skip},
			},
			expected: privacy.Deny,
		},
		{
			name: "custom error should be returned",
			policies: QueryPolicy{
				mockQueryRule{decision: errors.New("custom error")},
			},
			expected: errors.New("custom error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := tt.policies.EvalQuery(ctx, nil)

			if tt.expected == nil {
				assert.NoError(t, err)
			} else if errors.Is(tt.expected, privacy.Allow) {
				assert.ErrorIs(t, err, privacy.Allow)
			} else if errors.Is(tt.expected, privacy.Deny) {
				assert.ErrorIs(t, err, privacy.Deny)
			} else if errors.Is(tt.expected, privacy.Skip) {
				assert.ErrorIs(t, err, privacy.Skip)
			} else {
				assert.EqualError(t, err, tt.expected.Error())
			}
		})
	}
}

func TestMutationPolicy_EvalMutation(t *testing.T) {
	tests := []struct {
		name     string
		policies MutationPolicy
		expected error
	}{
		{
			name:     "empty policy should deny",
			policies: MutationPolicy{},
			expected: privacy.Deny,
		},
		{
			name: "policy with allow rule should allow",
			policies: MutationPolicy{
				mockMutationRule{decision: privacy.Allow},
			},
			expected: privacy.Allow,
		},
		{
			name: "policy with deny rule should deny",
			policies: MutationPolicy{
				mockMutationRule{decision: privacy.Deny},
			},
			expected: privacy.Deny,
		},
		{
			name: "policy with skip rule should deny by default",
			policies: MutationPolicy{
				mockMutationRule{decision: privacy.Skip},
			},
			expected: privacy.Deny,
		},
		{
			name: "policy with nil rule should deny by default",
			policies: MutationPolicy{
				mockMutationRule{decision: nil},
			},
			expected: privacy.Deny,
		},
		{
			name: "multiple rules - first allow should return allow",
			policies: MutationPolicy{
				mockMutationRule{decision: privacy.Allow},
				mockMutationRule{decision: privacy.Deny},
			},
			expected: privacy.Allow,
		},
		{
			name: "multiple rules - first deny should return deny",
			policies: MutationPolicy{
				mockMutationRule{decision: privacy.Deny},
				mockMutationRule{decision: privacy.Allow},
			},
			expected: privacy.Deny,
		},
		{
			name: "multiple rules - skip then allow should return allow",
			policies: MutationPolicy{
				mockMutationRule{decision: privacy.Skip},
				mockMutationRule{decision: privacy.Allow},
			},
			expected: privacy.Allow,
		},
		{
			name: "multiple rules - all skip should deny by default",
			policies: MutationPolicy{
				mockMutationRule{decision: privacy.Skip},
				mockMutationRule{decision: privacy.Skip},
			},
			expected: privacy.Deny,
		},
		{
			name: "custom error should be returned",
			policies: MutationPolicy{
				mockMutationRule{decision: errors.New("custom error")},
			},
			expected: errors.New("custom error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := tt.policies.EvalMutation(ctx, nil)

			if tt.expected == nil {
				assert.NoError(t, err)
			} else if errors.Is(tt.expected, privacy.Allow) {
				assert.ErrorIs(t, err, privacy.Allow)
			} else if errors.Is(tt.expected, privacy.Deny) {
				assert.ErrorIs(t, err, privacy.Deny)
			} else if errors.Is(tt.expected, privacy.Skip) {
				assert.ErrorIs(t, err, privacy.Skip)
			} else {
				assert.EqualError(t, err, tt.expected.Error())
			}
		})
	}
}

func TestPolicy_Structure(t *testing.T) {
	// Test that Policy struct can be created and used properly
	policy := Policy{
		Query: QueryPolicy{
			mockQueryRule{decision: privacy.Allow},
		},
		Mutation: MutationPolicy{
			mockMutationRule{decision: privacy.Allow},
		},
	}

	assert.NotNil(t, policy.Query)
	assert.NotNil(t, policy.Mutation)
	assert.Len(t, policy.Query, 1)
	assert.Len(t, policy.Mutation, 1)

	// Test that policies can be evaluated
	ctx := context.Background()

	queryErr := policy.Query.EvalQuery(ctx, nil)
	assert.ErrorIs(t, queryErr, privacy.Allow)

	mutationErr := policy.Mutation.EvalMutation(ctx, nil)
	assert.ErrorIs(t, mutationErr, privacy.Allow)
}
