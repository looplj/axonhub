package scopes_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestUserProjectScopeReadRequestsRuleIsolatesPlaygroundRequests(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:request_user_isolation?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	const projectID = 100
	setupCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	creatorRequest, err := client.Request.Create().
		SetProjectID(projectID).
		SetUserID(1).
		SetSource(request.SourcePlayground).
		SetModelID("playground-owner-model").
		SetRequestBody([]byte(`{}`)).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		Save(setupCtx)
	require.NoError(t, err)

	otherRequest, err := client.Request.Create().
		SetProjectID(projectID).
		SetUserID(2).
		SetSource(request.SourcePlayground).
		SetModelID("playground-other-model").
		SetRequestBody([]byte(`{}`)).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		Save(setupCtx)
	require.NoError(t, err)

	legacyRequest, err := client.Request.Create().
		SetProjectID(projectID).
		SetSource(request.SourcePlayground).
		SetModelID("legacy-playground-model").
		SetRequestBody([]byte(`{}`)).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		Save(setupCtx)
	require.NoError(t, err)

	apiRequest, err := client.Request.Create().
		SetProjectID(projectID).
		SetSource(request.SourceAPI).
		SetModelID("api-model").
		SetRequestBody([]byte(`{}`)).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		Save(setupCtx)
	require.NoError(t, err)

	for _, req := range []*ent.Request{creatorRequest, otherRequest, legacyRequest, apiRequest} {
		_, err = client.UsageLog.Create().
			SetRequestID(req.ID).
			SetProjectID(projectID).
			SetModelID(req.ModelID).
			SetSource(usagelog.Source(req.Source)).
			Save(setupCtx)
		require.NoError(t, err)
	}

	tests := []struct {
		name       string
		user       *ent.User
		requestIDs []int
		usageCount int
	}{
		{
			name: "project member sees own playground requests and shared API requests",
			user: projectRequestUser(1, projectID, false, nil),
			requestIDs: []int{
				creatorRequest.ID,
				apiRequest.ID,
			},
			usageCount: 2,
		},
		{
			name: "other project member cannot query another users playground request by ID",
			user: projectRequestUser(2, projectID, false, nil),
			requestIDs: []int{
				otherRequest.ID,
				apiRequest.ID,
			},
			usageCount: 2,
		},
		{
			name: "project owner can audit all requests including legacy playground requests",
			user: projectRequestUser(3, projectID, true, nil),
			requestIDs: []int{
				creatorRequest.ID,
				otherRequest.ID,
				legacyRequest.ID,
				apiRequest.ID,
			},
			usageCount: 4,
		},
		{
			name: "system scoped user can audit all requests in the selected project",
			user: projectRequestUser(4, projectID, false, []string{string(scopes.ScopeReadRequests)}),
			requestIDs: []int{
				creatorRequest.ID,
				otherRequest.ID,
				legacyRequest.ID,
				apiRequest.ID,
			},
			usageCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ent.NewContext(
				contexts.WithProjectID(contexts.WithUser(context.Background(), tt.user), projectID),
				client,
			)

			requests, err := client.Request.Query().All(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.requestIDs, sortedRequestIDs(requests))

			usageCount, err := client.UsageLog.Query().Count(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.usageCount, usageCount)
		})
	}

	ctx := ent.NewContext(
		contexts.WithProjectID(contexts.WithUser(context.Background(), projectRequestUser(2, projectID, false, nil)), projectID),
		client,
	)
	_, err = client.Request.Get(ctx, creatorRequest.ID)
	require.True(t, ent.IsNotFound(err))
}

func projectRequestUser(id, projectID int, isOwner bool, systemScopes []string) *ent.User {
	return &ent.User{
		ID:     id,
		Scopes: systemScopes,
		Edges: ent.UserEdges{
			ProjectUsers: []*ent.UserProject{{
				ProjectID: projectID,
				IsOwner:   isOwner,
				Scopes:    []string{string(scopes.ScopeReadRequests)},
			}},
		},
	}
}

func sortedRequestIDs(requests []*ent.Request) []int {
	ids := make([]int, len(requests))
	for i, req := range requests {
		ids[i] = req.ID
	}
	sort.Ints(ids)
	return ids
}
