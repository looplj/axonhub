package scopes

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/entql"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// Mock implementations for testing.

// mockProjectOwnedFilter implements ProjectOwnedFilter for testing queries.
type mockProjectOwnedFilter struct {
	whereProjectIDCalled bool
	projectIDPredicate   entql.IntP
}

func (m *mockProjectOwnedFilter) WhereProjectID(p entql.IntP) {
	m.whereProjectIDCalled = true
	m.projectIDPredicate = p
}

// mockProjectOwnedQuery wraps the filter and implements ent.Query.
type mockProjectOwnedQuery struct {
	filter *mockProjectOwnedFilter
}

func (m *mockProjectOwnedQuery) Filter() any {
	if m.filter == nil {
		m.filter = &mockProjectOwnedFilter{}
	}

	return m.filter
}

// mockProjectMemberMutation implements ProjectMemberMutation for testing mutations.
type mockProjectMemberMutation struct {
	ent.Mutation

	op           ent.Op
	projectID    int
	hasProjectID bool
	wherePCalled bool
	predicates   []func(*sql.Selector)
}

func (m *mockProjectMemberMutation) Op() ent.Op {
	return m.op
}

func (m *mockProjectMemberMutation) ProjectID() (int, bool) {
	return m.projectID, m.hasProjectID
}

func (m *mockProjectMemberMutation) WhereP(ps ...func(*sql.Selector)) {
	m.wherePCalled = true
	m.predicates = ps
}

func TestProjectMemberQueryRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope ScopeSlug
		expectAllow   bool
	}{
		{
			name:          "no user in context",
			ctx:           context.Background(),
			requiredScope: ScopeReadProjects,
			expectAllow:   false,
		},
		{
			name:          "nil user in context",
			ctx:           contexts.WithUser(context.Background(), nil),
			requiredScope: ScopeReadProjects,
			expectAllow:   false,
		},
		{
			name: "no project ID in context",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{"read_projects"},
			}),
			requiredScope: ScopeReadProjects,
			expectAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := UserProjectScopeReadRule(tt.requiredScope)
			// Use actual ent query
			query := &ent.ProjectQuery{}
			err := rule.EvalQuery(tt.ctx, query)

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

// Helper function to create int pointer.
func intPtr(i int) *int {
	return &i
}

func TestProjectMemberQueryRuleWithProjectID(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		requiredScope ScopeSlug
		expectAllow   bool
	}{
		{
			name: "user with global scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{"read_requests"},
				}),
				100,
			),
			requiredScope: ScopeReadRequests,
			expectAllow:   true,
		},
		{
			name: "user is project member with required scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"read_requests"},
							},
						},
					},
				}),
				100,
			),
			requiredScope: ScopeReadRequests,
			expectAllow:   true,
		},
		{
			name: "user is project owner",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								IsOwner:   true,
								Scopes:    []string{},
							},
						},
					},
				}),
				100,
			),
			requiredScope: ScopeReadRequests,
			expectAllow:   true,
		},
		{
			name: "user has project-scoped role with required scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{},
							},
						},
						Roles: []*ent.Role{
							{
								ID:        1,
								ProjectID: intPtr(100),
								Scopes:    []string{"read_requests"},
							},
						},
					},
				}),
				100,
			),
			requiredScope: ScopeReadRequests,
			expectAllow:   true,
		},
		{
			name: "user has project role but is not member",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						Roles: []*ent.Role{
							{
								ID:        1,
								ProjectID: intPtr(100),
								Scopes:    []string{"read_requests"},
							},
						},
					},
				}),
				100,
			),
			requiredScope: ScopeReadRequests,
			expectAllow:   false,
		},
		{
			name: "user is not project member",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 200, // Different project
								Scopes:    []string{"read_requests"},
							},
						},
					},
				}),
				100,
			),
			requiredScope: ScopeReadRequests,
			expectAllow:   false,
		},
		{
			name: "user is project member but without required scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"read_channels"}, // Wrong scope
							},
						},
					},
				}),
				100,
			),
			requiredScope: ScopeReadRequests,
			expectAllow:   false,
		},
		{
			name: "no project ID in context",
			ctx: contexts.WithUser(context.Background(), &ent.User{
				ID:     1,
				Scopes: []string{},
				Edges: ent.UserEdges{
					ProjectUsers: []*ent.UserProject{
						{
							ProjectID: 100,
							Scopes:    []string{"read_requests"},
						},
					},
				},
			}),
			requiredScope: ScopeReadRequests,
			expectAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := UserProjectScopeReadRule(tt.requiredScope)
			// Use actual ent query that has WhereProjectID
			query := &ent.APIKeyQuery{}
			err := rule.EvalQuery(tt.ctx, query)

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

func TestProjectMemberMutationRule(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		mutation      *mockProjectMemberMutation
		requiredScope ScopeSlug
		expectAllow   bool
	}{
		{
			name:          "no user in context",
			ctx:           context.Background(),
			mutation:      &mockProjectMemberMutation{op: ent.OpCreate, projectID: 100, hasProjectID: true},
			requiredScope: ScopeWriteRequests,
			expectAllow:   false,
		},
		{
			name: "user with global scope can create",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{"write_requests"},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpCreate, projectID: 100, hasProjectID: true},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
		{
			name: "owner user can create",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:      1,
					IsOwner: true,
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpCreate, projectID: 100, hasProjectID: true},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
		{
			name: "project member with scope can create",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"write_requests"},
							},
						},
					},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpCreate, projectID: 100, hasProjectID: true},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
		{
			name: "project owner can create",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								IsOwner:   true,
							},
						},
					},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpCreate, projectID: 100, hasProjectID: true},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
		{
			name: "non-member cannot create",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpCreate, projectID: 100, hasProjectID: true},
			requiredScope: ScopeWriteRequests,
			expectAllow:   false,
		},
		{
			name: "member without scope cannot create",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"read_requests"}, // Wrong scope
							},
						},
					},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpCreate, projectID: 100, hasProjectID: true},
			requiredScope: ScopeWriteRequests,
			expectAllow:   false,
		},
		{
			name: "create with project ID from context",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"write_requests"},
							},
						},
					},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpCreate, projectID: 100, hasProjectID: true},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
		{
			name: "update with project member scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"write_requests"},
							},
						},
					},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpUpdateOne},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
		{
			name: "delete with project member scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"write_requests"},
							},
						},
					},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpDeleteOne},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
		{
			name: "batch update with project member scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"write_requests"},
							},
						},
					},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpUpdate},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
		{
			name: "batch delete with project member scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"write_requests"},
							},
						},
					},
				}),
				100,
			),
			mutation:      &mockProjectMemberMutation{op: ent.OpDelete},
			requiredScope: ScopeWriteRequests,
			expectAllow:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := UserProjectScopeWriteRule(tt.requiredScope)
			err := rule.EvalMutation(tt.ctx, tt.mutation)

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

func TestUserHasProjectScope(t *testing.T) {
	tests := []struct {
		name          string
		user          *ent.User
		projectID     int
		requiredScope ScopeSlug
		expected      bool
	}{
		{
			name: "user has project scope",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					ProjectUsers: []*ent.UserProject{
						{
							ProjectID: 100,
							Scopes:    []string{"read_requests", "write_requests"},
						},
					},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      true,
		},
		{
			name: "user is project owner",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					ProjectUsers: []*ent.UserProject{
						{
							ProjectID: 100,
							IsOwner:   true,
							Scopes:    []string{},
						},
					},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      true,
		},
		{
			name: "user doesn't have project scope",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					ProjectUsers: []*ent.UserProject{
						{
							ProjectID: 100,
							Scopes:    []string{"read_channels"},
						},
					},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      false,
		},
		{
			name: "user is not member of project",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					ProjectUsers: []*ent.UserProject{
						{
							ProjectID: 200,
							Scopes:    []string{"read_requests"},
						},
					},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      false,
		},
		{
			name: "user has no project memberships",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					ProjectUsers: []*ent.UserProject{},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      false,
		},
		{
			name: "user has multiple projects, one matches",
			user: &ent.User{
				ID: 1,
				Edges: ent.UserEdges{
					ProjectUsers: []*ent.UserProject{
						{
							ProjectID: 200,
							Scopes:    []string{"read_channels"},
						},
						{
							ProjectID: 100,
							Scopes:    []string{"read_requests"},
						},
					},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := userHasProjectScope(tt.user, tt.projectID, tt.requiredScope)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
