// 职责: 验证缓存样本口径、项目隔离及 PostgreSQL/SQLite 与 Ent 路径的一致性。
// 关联文件:
//
//	internal/server/biz/plugin_cache_usage.go
//	internal/server/biz/plugin_cache_usage_batch.go
//	internal/server/plugin/hostapi_cache_usage.go
package biz

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/authz/identity"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/llm"
)

func TestCacheUsageSQLite(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	testCacheUsagePaths(t, client)
}

func TestCacheUsagePostgres(t *testing.T) {
	dsn := os.Getenv("AXONHUB_CACHE_METRICS_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set AXONHUB_CACHE_METRICS_TEST_PG_DSN to a disposable local axonhub_metrics_test database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme != "postgres" || parsed.Hostname() != "127.0.0.1" || parsed.Path != "/axonhub_metrics_test" || parsed.RawQuery != "sslmode=disable" {
		t.Fatal("use a postgres://127.0.0.1/axonhub_metrics_test URL with sslmode=disable for the disposable test database")
	}
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(ent.Driver(entsql.OpenDB(dialect.Postgres, db))))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	testCacheUsagePaths(t, client)
}

func testCacheUsagePaths(t *testing.T, client *ent.Client) {
	t.Helper()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	require.False(t, canBatchCacheUsage(ctx, client.Driver().Dialect()))
	project := client.Project.Create().SetName("cache-" + uuid.NewString()).SaveX(ctx).ID
	otherProject := client.Project.Create().SetName("cache-" + uuid.NewString()).SaveX(ctx).ID
	newKey := func(project int) int {
		return client.APIKey.Create().SetProjectID(project).SetName("cache").SetKey(uuid.NewString()).SaveX(ctx).ID
	}
	a, b, empty, foreign := newKey(project), newKey(project), newKey(project), newKey(otherProject)
	deleted := newKey(project)
	client.APIKey.DeleteOneID(deleted).ExecX(ctx)
	end := time.Date(2026, 10, 23, 12, 30, 0, 0, time.UTC)
	addRequest := func(project, key int, at time.Time, status request.Status, source request.Source) int {
		return client.Request.Create().SetProjectID(project).SetAPIKeyID(key).SetModelID("model").
			SetCreatedAt(at).SetRequestBody([]byte(`{}`)).SetStatus(status).SetSource(source).SaveX(ctx).ID
	}
	addUsage := func(project, key, req int, prompt, cached int64, reported bool) int {
		row := client.UsageLog.Create().SetProjectID(project).SetAPIKeyID(key).SetRequestID(req).
			SetModelID("model").SetPromptTokens(prompt).SetPromptCachedTokens(cached)
		if reported {
			row.SetCacheReadStatus(string(llm.CacheReadReported))
		}
		return row.SaveX(ctx).ID
	}
	old := addRequest(project, a, end.Add(-time.Hour), request.StatusCompleted, request.SourceAPI)
	addUsage(project, a, old, 300, 150, true)
	legacy := addRequest(project, a, end, request.StatusCompleted, request.SourceAPI)
	addUsage(project, a, legacy, 100, 0, false)
	zero := addRequest(project, a, end, request.StatusCompleted, request.SourceAPI)
	addUsage(project, a, zero, 100, 90, true)
	addUsage(project, a, zero, 100, 0, true) // latest same-scope usage per request
	addUsage(otherProject, a, zero, 700, 700, true)
	addUsage(project, b, zero, 900, 900, true)
	missing := addRequest(project, b, end, request.StatusCompleted, request.SourceAPI)
	addUsage(otherProject, b, missing, 999, 999, true)
	invalid := addRequest(project, b, end.Add(-time.Minute), request.StatusCompleted, request.SourceAPI)
	addUsage(project, b, invalid, 10, 11, true)
	addRequest(otherProject, a, end, request.StatusCompleted, request.SourceAPI)
	addRequest(project, a, end, request.StatusPending, request.SourceAPI)
	addRequest(project, a, end, request.StatusCompleted, request.SourcePlayground)
	addRequest(project, a, end.Add(time.Second), request.StatusCompleted, request.SourceAPI)
	addRequest(project, a, end.Add(-time.Hour-time.Second), request.StatusCompleted, request.SourceAPI)

	compare := func(q CacheUsageQuery) CacheUsagePage {
		t.Helper()
		ordinary, err := QueryCacheUsage(ctx, client, q)
		require.NoError(t, err)
		batch, err := authz.RunAsHost(ctx, client, "test-cache-usage-batch", func(hostCtx context.Context) (CacheUsagePage, error) {
			require.True(t, canBatchCacheUsage(hostCtx, client.Driver().Dialect()))
			return QueryCacheUsage(hostCtx, client, q)
		})
		require.NoError(t, err)
		require.Equal(t, ordinary, batch)
		return batch
	}
	q := CacheUsageQuery{ProjectID: project, KeyIDs: []int{foreign, b, a}, End: end, WindowSeconds: 3600, LastRequests: 2, IncludeSamples: true}
	page := compare(q)
	require.Len(t, page.Items, 2)
	require.Equal(t, a, page.Items[0].KeyID)
	require.True(t, page.Items[0].Truncated)
	require.Equal(t, 2, page.Items[0].Count)
	require.Equal(t, 1, page.Items[0].ValidCount)
	require.Equal(t, 1, page.Items[0].UnknownCount)
	require.Equal(t, int64(100), page.Items[0].PromptTokens)
	require.Equal(t, float64(0), *page.Items[0].CacheRate)
	require.Equal(t, zero, page.Items[0].Samples[0].RequestID)
	require.Equal(t, "reported", page.Items[0].Samples[0].Status)
	require.Equal(t, legacy, page.Items[0].Samples[1].RequestID)
	require.Equal(t, "unknown", page.Items[0].Samples[1].Status)
	require.Equal(t, b, page.Items[1].KeyID)
	require.False(t, page.Items[1].Truncated)
	require.Equal(t, 1, page.Items[1].InvalidCount)
	require.Equal(t, 1, page.Items[1].UnknownCount)
	require.Nil(t, page.Items[1].CacheRate)
	require.Equal(t, "unknown", page.Items[1].Samples[0].Status)
	require.Equal(t, "invalid", page.Items[1].Samples[1].Status)

	q.AllKeys, q.KeyIDs, q.Limit, q.LastRequests, q.IncludeSamples = true, nil, 1, 100, false
	page = compare(q)
	require.Equal(t, a, page.NextAfterKeyID)
	require.Equal(t, int64(400), page.Items[0].PromptTokens)
	require.Equal(t, int64(150), page.Items[0].CachedTokens)
	require.Equal(t, 0.375, *page.Items[0].CacheRate)
	require.Empty(t, page.Items[0].Samples)
	q.End, q.AfterKeyID, q.Limit = page.End, page.NextAfterKeyID, 100
	page = compare(q)
	require.Len(t, page.Items, 2)
	require.Equal(t, b, page.Items[0].KeyID)
	require.Equal(t, empty, page.Items[1].KeyID)
	require.Zero(t, page.Items[1].Count)
	require.Zero(t, page.NextAfterKeyID)

	pluginCtx := identity.ForPluginInstallation(t.Context(), 17, []string{"read_cache_usage"})
	_, err := QueryCacheUsage(pluginCtx, client, q)
	require.ErrorIs(t, err, privacy.Deny)
	_, err = queryCacheUsageBatch(pluginCtx, client.Driver(), q, []int{a})
	require.ErrorIs(t, err, privacy.Deny)
	_, err = client.UsageLog.Query().Where(usagelog.RequestIDEQ(zero)).All(pluginCtx)
	require.ErrorIs(t, err, privacy.Deny)
	_, err = client.Request.Query().Where(request.IDEQ(zero)).All(pluginCtx)
	require.ErrorIs(t, err, privacy.Deny)

	// Exercise the parameter and branch limits of a full batch on both dialects.
	for range 100 {
		newKey(project)
	}
	q.AfterKeyID = 0
	page, err = authz.RunAsHost(ctx, client, "test-cache-usage-full-batch", func(hostCtx context.Context) (CacheUsagePage, error) {
		return QueryCacheUsage(hostCtx, client, q)
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 100)
	require.Equal(t, page.Items[99].KeyID, page.NextAfterKeyID)
}

func TestCacheUsageQueryBounds(t *testing.T) {
	for _, q := range []CacheUsageQuery{
		{AllKeys: true},
		{ProjectID: 1},
		{ProjectID: 1, AllKeys: true, KeyIDs: []int{1}},
		{ProjectID: 1, KeyIDs: []int{0}},
		{ProjectID: 1, AllKeys: true, AfterKeyID: -1},
		{ProjectID: 1, AllKeys: true, LastRequests: 101},
		{ProjectID: 1, AllKeys: true, WindowSeconds: -1},
		{ProjectID: 1, AllKeys: true, Limit: 101},
	} {
		_, err := QueryCacheUsage(t.Context(), nil, q)
		require.ErrorIs(t, err, ErrInvalidCacheUsageQuery, "%+v", q)
	}
}
