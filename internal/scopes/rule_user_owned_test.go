package scopes

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// Mock implementations for testing.
type mockFilterMutation struct {
	ent.Mutation

	wherePCalled bool
	wherePFuncs  []func(*sql.Selector)
	userID       int
	userIDExists bool
}

func (m *mockFilterMutation) WhereP(ps ...func(*sql.Selector)) {
	m.wherePCalled = true
	m.wherePFuncs = append(m.wherePFuncs, ps...)
}

func (m *mockFilterMutation) UserID() (int, bool) {
	return m.userID, m.userIDExists
}

func (m *mockFilterMutation) Op() ent.Op {
	return ent.OpCreate
}

func (m *mockFilterMutation) Type() string {
	return "MockFilterMutation"
}

// Mock mutation for update operations.
type mockUpdateMutation struct {
	ent.Mutation

	op           ent.Op
	wherePCalled bool
	wherePFuncs  []func(*sql.Selector)
	userID       int
	userIDExists bool
}

func (m *mockUpdateMutation) WhereP(ps ...func(*sql.Selector)) {
	m.wherePCalled = true
	m.wherePFuncs = append(m.wherePFuncs, ps...)
}

func (m *mockUpdateMutation) UserID() (int, bool) {
	return m.userID, m.userIDExists
}

func (m *mockUpdateMutation) Op() ent.Op {
	return m.op
}

func (m *mockUpdateMutation) Type() string {
	return "MockUpdateMutation"
}

// Mock mutation for other tests.
type mockBasicMutation struct {
	ent.Mutation
}

func (m *mockBasicMutation) Op() ent.Op {
	return ent.OpCreate
}

func (m *mockBasicMutation) Type() string {
	return "MockBasic"
}

func TestUserOwnedQueryRule(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		setupQuery func() ent.Query
		assertErr  assert.ErrorAssertionFunc
	}{
		{
			name: "no user in context",
			ctx:  context.Background(),
			setupQuery: func() ent.Query {
				return &ent.APIKeyQuery{}
			},
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				return assert.Error(t, err) && !errors.Is(err, privacy.Skip) && !errors.Is(err, privacy.Allow)
			},
		},
		{
			name: "valid user with APIKeyQuery (has WhereUserID)",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID: 123,
			}),
			setupQuery: func() ent.Query {
				return &ent.APIKeyQuery{}
			},
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				return assert.Error(t, err) && errors.Is(err, privacy.Allow)
			},
		},
		{
			name: "valid user with UserQuery (no WhereUserID)",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID: 123,
			}),
			setupQuery: func() ent.Query {
				return &ent.UserQuery{}
			},
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				return assert.Error(t, err) && !errors.Is(err, privacy.Skip) && !errors.Is(err, privacy.Allow)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := UserOwnedQueryRule()
			query := tt.setupQuery()
			err := rule.EvalQuery(tt.ctx, query)

			tt.assertErr(t, err)
		})
	}
}

func TestUserOwnedMutationRule(t *testing.T) {
	tests := []struct {
		name         string
		ctx          context.Context
		mutation     ent.Mutation
		expectError  bool
		expectSkip   bool
		expectWhereP bool
	}{
		{
			name:        "no user in context",
			ctx:         context.Background(),
			mutation:    &mockFilterMutation{},
			expectError: true,
			expectSkip:  false,
		},
		{
			name: "valid user with filter mutation - create with matching user ID",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID: 123,
			}),
			mutation: &mockFilterMutation{
				userID:       123,
				userIDExists: true,
			},
			expectError: false,
			expectSkip:  false,
		},
		{
			name: "valid user with filter mutation - update operation",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID: 123,
			}),
			mutation: &mockUpdateMutation{
				op: ent.OpUpdateOne,
			},
			expectError:  false,
			expectSkip:   false,
			expectWhereP: true,
		},
		{
			name: "valid user with filter mutation - create with non-matching user ID",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID: 123,
			}),
			mutation: &mockFilterMutation{
				userID:       456,
				userIDExists: true,
			},
			expectError: false,
			expectSkip:  true,
		},
		{
			name: "invalid mutation type",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID: 123,
			}),
			mutation:    &mockBasicMutation{},
			expectError: true,
			expectSkip:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := UserOwnedMutationRule()
			err := rule.EvalMutation(tt.ctx, tt.mutation)

			if tt.expectError {
				if err == nil || errors.Is(err, privacy.Skip) || errors.Is(err, privacy.Allow) {
					t.Errorf("expected error, got %v", err)
				}
			} else if tt.expectSkip {
				if !errors.Is(err, privacy.Skip) {
					t.Errorf("expected privacy.Skip, got %v", err)
				}
			} else {
				// Expect Allow
				if !errors.Is(err, privacy.Allow) {
					t.Errorf("expected privacy.Allow, got %v", err)
				}
			}

			// Check WhereP was called when expected
			if tt.expectWhereP {
				if mockMutation, ok := tt.mutation.(*mockUpdateMutation); ok {
					if !mockMutation.wherePCalled {
						t.Error("expected WhereP to be called")
					}
				}
			}
		})
	}
}
