package gql

import (
	"context"
	"testing"
	"time"

	"entgo.io/contrib/entgql"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
)

func TestTracesResolverOrdersByLastRequestAt(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:traces_order?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	testProject, err := client.Project.Create().
		SetName("traces-order-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// The trace created first is the one with the most recent request, so
	// creation order and last-request order disagree.
	newer, err := client.Trace.Create().
		SetProjectID(testProject.ID).
		SetTraceID("trace-ordered-newer").
		SetCreatedAt(base).
		SetLastRequestAt(base.Add(2 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)
	older, err := client.Trace.Create().
		SetProjectID(testProject.ID).
		SetTraceID("trace-ordered-older").
		SetCreatedAt(base.Add(time.Hour)).
		SetLastRequestAt(base.Add(time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	first := 10
	resolver := &queryResolver{&Resolver{client: client}}

	conn, err := resolver.Traces(ctx, nil, &first, nil, nil, &ent.TraceOrder{
		Direction: entgql.OrderDirectionDesc,
		Field:     ent.TraceOrderFieldLastRequestAt,
	}, &ent.TraceWhereInput{})
	require.NoError(t, err)
	require.Len(t, conn.Edges, 2)
	require.Equal(t, newer.ID, conn.Edges[0].Node.ID)
	require.Equal(t, older.ID, conn.Edges[1].Node.ID)
}
