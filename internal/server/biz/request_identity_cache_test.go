// 职责: 验证会话身份缓存的 SQL 收益、隔离及事务失效行为。
// 关联文件:
//
//	internal/server/biz/request_identity_cache.go
//	internal/server/biz/trace.go
//	internal/server/biz/thread.go

package biz

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/thread"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
)

func newRequestIOClient(t *testing.T) (*ent.Client, *requestIODriver) {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_fk=0"
	schemaClient := enttest.NewEntClient(t, "sqlite3", dsn)
	t.Cleanup(func() { schemaClient.Close() })
	raw, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	raw.DB().SetMaxOpenConns(1)
	driver := &requestIODriver{Driver: raw}
	client := ent.NewClient(ent.Driver(driver))
	t.Cleanup(func() { client.Close() })
	return client, driver
}

func TestRequestIdentityCache(t *testing.T) {
	client, driver := newRequestIOClient(t)
	ctx := contexts.WithProjectID(authz.WithTestBypass(ent.NewContext(context.Background(), client)), 1)
	traces := NewTraceService(TraceServiceParams{Ent: client})
	threads := NewThreadService(client, traces)
	th, err := threads.ResolveThreadForRequest(ctx, 1, "thread")
	require.NoError(t, err)
	tr, err := traces.ResolveTraceForRequest(ctx, 1, "trace", &th.ID)
	require.NoError(t, err)
	driver.queries, driver.writes = 0, 0
	again, err := traces.ResolveTraceForRequest(ctx, 1, "trace", nil)
	require.NoError(t, err)
	same, err := threads.ResolveThreadForRequest(ctx, 1, "thread")
	require.NoError(t, err)
	require.Equal(t, tr.ID, again.ID)
	require.Equal(t, th.ID, again.ThreadID)
	require.Equal(t, th.ID, same.ID)
	require.Zero(t, driver.queries)
	require.Zero(t, driver.writes)
	// Callers receive their own identity value; mutable entity state is never cached.
	again.ThreadID = 999
	again, err = traces.ResolveTraceForRequest(ctx, 1, "trace", nil)
	require.NoError(t, err)
	require.Equal(t, th.ID, again.ThreadID)
	require.NoError(t, client.Thread.UpdateOneID(th.ID).SetStatus(thread.StatusArchived).Exec(ctx))
	entity, err := threads.GetOrCreateThread(ctx, 1, "thread")
	require.NoError(t, err)
	require.Equal(t, thread.StatusArchived, entity.Status)
	_, err = traces.ResolveTraceForRequest(ctx, 2, "trace", nil)
	require.Error(t, err)
	otherCtx := contexts.WithProjectID(authz.WithTestBypass(ent.NewContext(context.Background(), client)), 2)
	_, err = traces.ResolveTraceForRequest(otherCtx, 2, "trace", nil)
	require.Error(t, err, "must not return another project's globally unique trace")
	// Rename invalidates the old lookup, and deletion cannot leave a reused ID.
	require.NoError(t, client.Trace.UpdateOneID(tr.ID).SetTraceID("renamed").Exec(ctx))
	renamed, err := traces.ResolveTraceForRequest(ctx, 1, "renamed", nil)
	require.NoError(t, err)
	require.Equal(t, tr.ID, renamed.ID)
	require.NoError(t, client.Trace.DeleteOneID(tr.ID).Exec(ctx))
	recreated, err := traces.ResolveTraceForRequest(ctx, 1, "renamed", nil)
	require.NoError(t, err)
	require.NotEqual(t, tr.ID, recreated.ID)
	require.NoError(t, client.Thread.DeleteOneID(th.ID).Exec(ctx))
	recreatedThread, err := threads.ResolveThreadForRequest(ctx, 1, "thread")
	require.NoError(t, err)
	require.NotEqual(t, th.ID, recreatedThread.ID)
	// Expired entries must query again, even if the entry remains in the LRU.
	key := requestIdentityKey{"trace", 1, "renamed"}
	cached, _ := traces.identities.entries.Get(key)
	cached.expires = time.Now().Add(-time.Second)
	traces.identities.entries.Add(key, cached)
	driver.queries = 0
	_, err = traces.ResolveTraceForRequest(ctx, 1, "renamed", nil)
	require.NoError(t, err)
	require.Equal(t, 1, driver.queries)
}

func TestRequestIdentityCacheTransactions(t *testing.T) {
	client, _ := newRequestIOClient(t)
	ctx := contexts.WithProjectID(authz.WithTestBypass(ent.NewContext(context.Background(), client)), 1)
	svc := NewTraceService(TraceServiceParams{Ent: client})
	tr, err := svc.ResolveTraceForRequest(ctx, 1, "trace", nil)
	require.NoError(t, err)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := ent.NewContext(ctx, tx.Client()) // Deliberately omit ent.NewTxContext.
	require.NoError(t, tx.Trace.DeleteOneID(tr.ID).Exec(txCtx))
	replacement, err := svc.ResolveTraceForRequest(txCtx, 1, "trace", nil)
	require.NoError(t, err)
	require.NotEqual(t, tr.ID, replacement.ID)
	require.NoError(t, tx.Rollback())
	restored, err := svc.ResolveTraceForRequest(ctx, 1, "trace", nil)
	require.NoError(t, err)
	require.Equal(t, tr.ID, restored.ID)
	tx, err = client.Tx(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.Trace.DeleteOneID(tr.ID).Exec(ent.NewContext(ctx, tx.Client())))
	// Simulate an old committed row being cached by a concurrent reader.
	svc.identities.entries.Add(requestIdentityKey{"trace", 1, "trace"}, requestIdentity{id: tr.ID, expires: time.Now().Add(time.Minute)})
	require.NoError(t, tx.Commit())
	restored, err = svc.ResolveTraceForRequest(ctx, 1, "trace", nil)
	require.NoError(t, err)
	require.NotEqual(t, tr.ID, restored.ID)
}

func TestRequestIdentityCacheDoesNotReinsertInvalidatedLoad(t *testing.T) {
	client, _ := newRequestIOClient(t)
	cache := newRequestIdentityCache(client, xcache.Config{})
	key := requestIdentityKey{"trace", 1, "trace"}
	_, err := cache.resolve(context.Background(), client, key, func() (requestIdentity, error) {
		cache.clear()
		return requestIdentity{id: 1}, nil
	})
	require.NoError(t, err)
	_, ok := cache.entries.Get(key)
	require.False(t, ok)
}

func TestRequestIdentityCacheCrossInstanceInvalidation(t *testing.T) {
	redis := miniredis.RunT(t)
	client, driver := newRequestIOClient(t)
	cfg := xcache.Config{Mode: xcache.ModeRedis, Redis: xredis.Config{Addr: redis.Addr()}}
	first := newRequestIdentityCache(client, cfg)
	defer first.stop()
	otherClient := ent.NewClient(ent.Driver(driver.Driver))
	second := newRequestIdentityCache(otherClient, cfg)
	defer second.stop()
	ctx := authz.WithTestBypass(context.Background())
	row := client.Trace.Create().SetProjectID(1).SetTraceID("trace").SaveX(ctx)
	key := requestIdentityKey{"trace", 1, "trace"}
	second.entries.Add(key, requestIdentity{id: row.ID, expires: time.Now().Add(time.Minute)})
	require.NoError(t, client.Trace.DeleteOneID(row.ID).Exec(ctx))
	require.Eventually(t, func() bool { _, ok := second.entries.Get(key); return !ok }, time.Second, 5*time.Millisecond)
}
