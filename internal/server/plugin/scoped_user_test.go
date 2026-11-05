package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
)

func TestBuildPluginScopedUser(t *testing.T) {
	t.Run("owner keeps only ceiling scopes", func(t *testing.T) {
		projectID := 4
		ceiling := []string{"read_channels", "write_channels"}
		owner := &ent.User{
			ID:      1,
			Email:   "owner@example.com",
			IsOwner: true,
			Scopes:  []string{"read_settings"},
			Edges: ent.UserEdges{
				Roles: []*ent.Role{{
					ID:     1,
					Name:   "admin",
					Scopes: []string{"write_plugins"},
				}},
				ProjectUsers: []*ent.UserProject{{
					ID:        9,
					UserID:    1,
					ProjectID: projectID,
					IsOwner:   true,
					Scopes:    []string{"read_prompts"},
				}},
			},
		}

		copied := BuildPluginScopedUser(owner, ceiling)

		require.False(t, copied.IsOwner)
		require.Nil(t, copied.Edges.Roles)
		require.ElementsMatch(t, ceiling, copied.Scopes)
		require.Equal(t, "owner@example.com", copied.Email)
		require.Len(t, copied.Edges.ProjectUsers, 1)
		require.False(t, copied.Edges.ProjectUsers[0].IsOwner)
		require.Equal(t, projectID, copied.Edges.ProjectUsers[0].ProjectID)
		require.ElementsMatch(t, ceiling, copied.Edges.ProjectUsers[0].Scopes)

		require.True(t, owner.IsOwner)
		require.Equal(t, []string{"read_settings"}, owner.Scopes)
		require.Len(t, owner.Edges.Roles, 1)
		require.True(t, owner.Edges.ProjectUsers[0].IsOwner)
		require.Equal(t, []string{"read_prompts"}, owner.Edges.ProjectUsers[0].Scopes)
		copied.Scopes[0] = "mutated"
		require.NotContains(t, ceiling, "mutated")
	})

	t.Run("direct scope outside ceiling disappears and system role scope remains", func(t *testing.T) {
		projectID := 7
		otherProjectID := 8
		ceiling := []string{"read_channels", "read_requests"}
		systemRole := &ent.Role{
			ID:     2,
			Name:   "reader",
			Scopes: []string{"read_channels"},
		}
		projectRole := &ent.Role{
			ID:        3,
			ProjectID: &projectID,
			Scopes:    []string{"read_requests"},
		}
		otherRole := &ent.Role{
			ID:        4,
			ProjectID: &otherProjectID,
			Scopes:    []string{"read_channels"},
		}
		user := &ent.User{
			ID:      3,
			Email:   "member@example.com",
			IsOwner: false,
			Scopes:  []string{"write_users"},
			Edges: ent.UserEdges{
				Roles: []*ent.Role{systemRole, projectRole, otherRole},
				ProjectUsers: []*ent.UserProject{{
					ID:        11,
					UserID:    3,
					ProjectID: projectID,
					IsOwner:   false,
					Scopes:    []string{"write_users"},
				}},
			},
		}

		copied := BuildPluginScopedUser(user, ceiling)

		require.False(t, copied.IsOwner)
		require.Nil(t, copied.Edges.Roles)
		require.NotContains(t, copied.Scopes, "write_users")
		require.Equal(t, []string{"read_channels"}, copied.Scopes)
		require.Len(t, copied.Edges.ProjectUsers, 1)
		require.False(t, copied.Edges.ProjectUsers[0].IsOwner)
		require.Equal(t, []string{"read_requests"}, copied.Edges.ProjectUsers[0].Scopes)
		require.NotContains(t, copied.Edges.ProjectUsers[0].Scopes, "read_channels")
		require.NotContains(t, copied.Edges.ProjectUsers[0].Scopes, "write_users")

		require.Equal(t, []string{"write_users"}, user.Scopes)
		require.Len(t, user.Edges.Roles, 3)
		require.False(t, user.Edges.ProjectUsers[0].IsOwner)
		require.Equal(t, []string{"write_users"}, user.Edges.ProjectUsers[0].Scopes)
	})

	t.Run("project owner membership keeps only ceiling scopes", func(t *testing.T) {
		projectID := 5
		ceiling := []string{"read_channels"}
		user := &ent.User{
			ID:     4,
			Email:  "project-owner@example.com",
			Scopes: []string{"read_settings"},
			Edges: ent.UserEdges{
				ProjectUsers: []*ent.UserProject{{
					ID:        12,
					UserID:    4,
					ProjectID: projectID,
					IsOwner:   true,
					Scopes:    []string{"write_users"},
				}},
			},
		}

		copied := BuildPluginScopedUser(user, ceiling)

		require.False(t, copied.IsOwner)
		require.Empty(t, copied.Scopes)
		require.Len(t, copied.Edges.ProjectUsers, len(user.Edges.ProjectUsers))
		require.False(t, copied.Edges.ProjectUsers[0].IsOwner)
		require.Equal(t, []string{"read_channels"}, copied.Edges.ProjectUsers[0].Scopes)
		require.True(t, user.Edges.ProjectUsers[0].IsOwner)
		require.Equal(t, []string{"write_users"}, user.Edges.ProjectUsers[0].Scopes)
	})
}
