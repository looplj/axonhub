package scopes

import (
	"context"
	dbsql "database/sql"
	"errors"
	"slices"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/entql"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	"github.com/looplj/axonhub/internal/ent/migrate/schemahook"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/user"

	_ "github.com/looplj/axonhub/internal/pkg/sqlite"
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
	predicates   []func(*entsql.Selector)
}

func (m *mockProjectMemberMutation) Op() ent.Op {
	return m.op
}

func (m *mockProjectMemberMutation) ProjectID() (int, bool) {
	return m.projectID, m.hasProjectID
}

func (m *mockProjectMemberMutation) WhereP(ps ...func(*entsql.Selector)) {
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
								ProjectID: lo.ToPtr(100),
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
								ProjectID: lo.ToPtr(100),
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
		{
			name: "user has required scope via project role",
			user: &ent.User{
				ID: 1,
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
							ProjectID: lo.ToPtr(100),
							Scopes:    []string{"read_requests"},
						},
					},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      true,
		},
		{
			name: "role on a different project does not grant scope",
			user: &ent.User{
				ID: 1,
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
							ProjectID: lo.ToPtr(200),
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
			name: "system role does not grant project scope",
			user: &ent.User{
				ID: 1,
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
							Scopes:    []string{"read_requests"},
							ProjectID: nil,
						},
					},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      false,
		},
		{
			name: "role without required scope does not grant",
			user: &ent.User{
				ID: 1,
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
							ProjectID: lo.ToPtr(100),
							Scopes:    []string{"read_channels"},
						},
					},
				},
			},
			projectID:     100,
			requiredScope: ScopeReadRequests,
			expected:      false,
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

func TestUserIsProjectOwner(t *testing.T) {
	tests := []struct {
		name      string
		user      *ent.User
		projectID int
		expected  bool
	}{
		{
			name:      "system owner",
			user:      &ent.User{IsOwner: true},
			projectID: 100,
			expected:  true,
		},
		{
			name:      "project owner",
			user:      &ent.User{Edges: ent.UserEdges{ProjectUsers: []*ent.UserProject{{ProjectID: 100, IsOwner: true}}}},
			projectID: 100,
			expected:  true,
		},
		{
			name:      "owner of another project",
			user:      &ent.User{Edges: ent.UserEdges{ProjectUsers: []*ent.UserProject{{ProjectID: 200, IsOwner: true}}}},
			projectID: 100,
			expected:  false,
		},
		{
			name:      "regular project member",
			user:      &ent.User{Edges: ent.UserEdges{ProjectUsers: []*ent.UserProject{{ProjectID: 100}}}},
			projectID: 100,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userIsProjectOwner(tt.user, tt.projectID); got != tt.expected {
				t.Fatalf("userIsProjectOwner() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProjectMemberReadUsersRule(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectAllow bool
	}{
		{
			name:        "no project ID in context",
			ctx:         contexts.WithUser(context.Background(), &ent.User{ID: 1, Scopes: []string{"read_users"}}),
			expectAllow: false,
		},
		{
			name:        "no user in context",
			ctx:         contexts.WithProjectID(context.Background(), 100),
			expectAllow: false,
		},
		{
			name: "user with system scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{"read_users"},
				}),
				100,
			),
			expectAllow: true,
		},
		{
			name: "project member with direct scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{
								ProjectID: 100,
								Scopes:    []string{"read_users"},
							},
						},
					},
				}),
				100,
			),
			expectAllow: true,
		},
		{
			name: "project member with scope via project role",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{ProjectID: 100, Scopes: []string{}},
						},
						Roles: []*ent.Role{
							{
								ID:        1,
								ProjectID: lo.ToPtr(100),
								Scopes:    []string{"read_users"},
							},
						},
					},
				}),
				100,
			),
			expectAllow: true,
		},
		{
			name: "project member without read scope",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{ProjectID: 100, Scopes: []string{"read_api_keys"}},
						},
					},
				}),
				100,
			),
			expectAllow: false,
		},
		{
			name: "user without membership in the project",
			ctx: contexts.WithProjectID(
				contexts.WithUser(context.Background(), &ent.User{
					ID:     1,
					Scopes: []string{},
					Edges: ent.UserEdges{
						ProjectUsers: []*ent.UserProject{
							{ProjectID: 200, Scopes: []string{"read_users"}},
						},
					},
				}),
				100,
			),
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := ProjectMemberReadUsersRule(ScopeReadUsers)
			err := rule.EvalQuery(tt.ctx, &ent.UserQuery{})

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

func openScopesTestClient(t *testing.T) *ent.Client {
	t.Helper()

	db, err := dbsql.Open("sqlite3", "file:scopes_mem?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))

	if err := client.Schema.Create(
		context.Background(),
		migrate.WithGlobalUniqueID(false),
		migrate.WithForeignKeys(false),
		migrate.WithDropIndex(true),
		migrate.WithDropColumn(true),
		schema.WithHooks(schemahook.V0_3_0),
	); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Close()
		_ = db.Close()
	})

	return client
}

// TestProjectMemberReadUsersRuleAppliesProjectFilter verifies ProjectMemberReadUsersRule
// through the User schema policy against a real (in-memory) database: an allowed
// project read must only return users of the queried project, while the system
// scope keeps the unfiltered view.
func TestProjectMemberReadUsersRuleAppliesProjectFilter(t *testing.T) {
	ctx := context.Background()

	// Seeding happens as the platform owner so the per-schema mutation policies
	// accept the writes.
	seedCtx := contexts.WithUser(ctx, &ent.User{ID: 999, IsOwner: true, Scopes: []string{"read_users", "read_projects", "write_users"}})

	client := openScopesTestClient(t)

	p100 := client.Project.Create().SetName("P100").SaveX(seedCtx)
	p200 := client.Project.Create().SetName("P200").SaveX(seedCtx)

	createUser := func(email string, scopes ...string) *ent.User {
		return client.User.Create().
			SetEmail(email).
			SetFirstName("F").
			SetLastName("L").
			SetPassword("x").
			SetStatus(user.StatusActivated).
			SetScopes(scopes).
			SaveX(seedCtx)
	}

	alice := createUser("alice@test.dev")
	developer := createUser("developer@test.dev")
	bob := createUser("bob@test.dev")
	sysadmin := createUser("sysadmin@test.dev", "read_users")

	client.UserProject.Create().SetUserID(alice.ID).SetProjectID(p100.ID).SetScopes([]string{"read_users"}).SaveX(seedCtx)
	client.UserProject.Create().SetUserID(developer.ID).SetProjectID(p100.ID).SetScopes([]string{}).SaveX(seedCtx)
	client.UserProject.Create().SetUserID(bob.ID).SetProjectID(p200.ID).SetScopes([]string{"read_requests"}).SaveX(seedCtx)

	devRole := client.Role.Create().
		SetName("developer").
		SetProjectID(p100.ID).
		SetScopes([]string{"read_users"}).
		SaveX(seedCtx)
	client.UserRole.Create().SetUserID(developer.ID).SetRoleID(devRole.ID).SaveX(seedCtx)

	usersOfProject100 := []int{alice.ID, developer.ID}
	allUsers := []int{alice.ID, developer.ID, bob.ID, sysadmin.ID}

	tests := []struct {
		name        string
		principalID int
		expectIDs   []int
	}{
		{
			name:        "project member with direct scope reads only project users",
			principalID: alice.ID,
			expectIDs:   usersOfProject100,
		},
		{
			name:        "project member with role grant reads only project users",
			principalID: developer.ID,
			expectIDs:   usersOfProject100,
		},
		{
			name:        "system scope reads all users",
			principalID: sysadmin.ID,
			expectIDs:   allUsers,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal, err := client.User.Query().
				Where(user.ID(tt.principalID)).
				WithProjectUsers().
				WithRoles().
				Only(seedCtx)
			if err != nil {
				t.Fatalf("load principal: %v", err)
			}

			principalCtx := contexts.WithProjectID(contexts.WithUser(ctx, principal), p100.ID)

			users, err := client.User.Query().All(principalCtx)
			if err != nil {
				t.Fatalf("query users: %v", err)
			}

			var gotIDs []int
			for _, u := range users {
				gotIDs = append(gotIDs, u.ID)
			}
			slices.Sort(gotIDs)
			slices.Sort(tt.expectIDs)

			if !slices.Equal(gotIDs, tt.expectIDs) {
				t.Errorf("user IDs = %v, want %v", gotIDs, tt.expectIDs)
			}
		})
	}
}
