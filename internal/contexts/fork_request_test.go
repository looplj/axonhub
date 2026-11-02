package contexts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
)

func TestForkRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	key := &ent.APIKey{ID: 12}
	ctx = WithAPIKey(ctx, key)
	ctx = WithProjectID(ctx, 7)
	ctx = WithTraceID(ctx, "trace")
	ctx = WithRequestID(ctx, "parent")
	ctx = WithChannelAPIKey(ctx, "parent-upstream")
	AddError(ctx, errors.New("parent error"))
	observed := 0
	ctx = WithRequestRecordObserver(ctx, func(int) { observed++ })
	child := ForkRequest(ctx)
	child = WithRequestID(child, "child")
	child = WithChannelAPIKey(child, "child-upstream")
	child = WithTraceID(child, "child-trace")
	NotifyRequestRecord(child, 42)
	require.Zero(t, observed)
	require.Empty(t, GetErrors(child))
	got, ok := GetAPIKey(child)
	require.True(t, ok)
	require.Same(t, key, got)
	project, _ := GetProjectID(child)
	require.Equal(t, 7, project)
	id, _ := GetRequestID(ctx)
	require.Equal(t, "parent", id)
	trace, _ := GetTraceID(ctx)
	require.Equal(t, "trace", trace)
	channel, _ := GetChannelAPIKey(ctx)
	require.Equal(t, "parent-upstream", channel)
	deadline, _ := ctx.Deadline()
	childDeadline, _ := child.Deadline()
	require.Equal(t, deadline, childDeadline)
	cancel()
	require.ErrorIs(t, child.Err(), context.Canceled)
	NotifyRequestRecord(ctx, 42)
	require.Equal(t, 1, observed)
}
