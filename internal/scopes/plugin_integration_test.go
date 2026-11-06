package scopes_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestPluginEntProjectAndResourceIsolation(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:plugin_scope_isolation?mode=memory&_fk=0")
	t.Cleanup(func() { db.Close() })
	setup := authz.WithTestBypass(context.Background())
	for _, projectID := range []int{100, 200} {
		_, err := db.Request.Create().SetProjectID(projectID).SetModelID("model").SetRequestBody([]byte(`{}`)).SetStatus(request.StatusCompleted).SetStream(false).Save(setup)
		require.NoError(t, err)
	}
	u := &ent.User{ID: 1, Edges: ent.UserEdges{ProjectUsers: []*ent.UserProject{{ProjectID: 100, IsOwner: true}}}}
	parent := contexts.WithUser(authz.NewUserContext(context.Background(), u.ID), u)
	parent = contexts.WithProjectID(parent, 100)
	ctx, err := authz.DelegatePlugin(parent, 22, []string{"read_requests"})
	require.NoError(t, err)
	rows, err := db.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 100, rows[0].ProjectID)
	other := contexts.WithProjectID(contexts.ForkRequest(ctx), 200)
	_, err = db.Request.Query().All(other)
	require.Error(t, err)
	_, err = db.ChannelOverrideTemplate.Query().All(ctx)
	require.Error(t, err)
	// A granted scope decision is an unbound Allow, exactly as for direct users; an
	// ungranted one denies everything.
	scopeCtx := authz.WithScopeDecision(ctx, scopes.ScopeReadRequests)
	_, err = db.Request.Query().All(scopeCtx)
	require.NoError(t, err)
	deniedCtx := authz.WithScopeDecision(ctx, scopes.ScopeReadChannels)
	_, err = db.Request.Query().All(deniedCtx)
	require.Error(t, err)
	// Stale API key entities may never supply a fallback authorization branch.
	contexts.WithAPIKey(ctx, &ent.APIKey{ID: 1, ProjectID: 100, Scopes: []string{"write_requests", "read_channels"}})
	_, err = db.Channel.Query().All(ctx)
	require.Error(t, err)
	empty, err := authz.DelegatePlugin(parent, 23, nil)
	require.NoError(t, err)
	_, err = db.Request.Query().All(empty)
	require.Error(t, err)
}
