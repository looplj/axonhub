package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagestat"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func newUsageStatTestEnv(t *testing.T) (*ent.Client, *UsageLogService) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	svc := NewUsageLogService(client, systemService, nil)
	return client, svc
}

func createTestUsageLog(t *testing.T, ctx context.Context, client *ent.Client, createdAt time.Time, modelID string, prompt, completion int64) *ent.UsageLog {
	req, err := client.Request.Create().
		SetProjectID(1).
		SetModelID(modelID).
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	ul, err := client.UsageLog.Create().
		SetRequestID(req.ID).
		SetProjectID(1).
		SetChannelID(1).
		SetModelID(modelID).
		SetPromptTokens(prompt).
		SetCompletionTokens(completion).
		SetTotalTokens(prompt + completion).
		SetAPIKeyID(42).
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)
	return ul
}

// upsertUsageStat accumulates into a single day row for the same dimension.
func TestUsageStat_UpsertAccumulates(t *testing.T) {
	client, svc := newUsageStatTestEnv(t)
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	now := time.Now().UTC()
	ul1 := createTestUsageLog(t, ctx, client, now, "m1", 100, 50)
	ul2 := createTestUsageLog(t, ctx, client, now, "m1", 30, 20)

	require.NoError(t, svc.upsertUsageStat(ctx, ul1))
	require.NoError(t, svc.upsertUsageStat(ctx, ul2))

	rows, err := client.UsageStat.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	s := rows[0]
	require.Equal(t, int64(2), s.RequestCount)
	require.Equal(t, int64(130), s.PromptTokens)
	require.Equal(t, int64(70), s.CompletionTokens)
	require.Equal(t, int64(200), s.TotalTokens)
	require.Equal(t, 42, s.APIKeyID)
	require.Equal(t, "m1", s.ModelID)
}

// Backfill aggregates usage_logs into day rows and is idempotent.
func TestUsageStat_Backfill(t *testing.T) {
	client, svc := newUsageStatTestEnv(t)
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	now := time.Now().UTC()
	createTestUsageLog(t, ctx, client, now.AddDate(0, 0, -2), "m1", 100, 50)
	createTestUsageLog(t, ctx, client, now.AddDate(0, 0, -2), "m1", 30, 20)
	createTestUsageLog(t, ctx, client, now.AddDate(0, 0, -1), "m2", 5, 5)

	require.NoError(t, svc.BackfillUsageStats(ctx))

	rows, err := client.UsageStat.Query().Order(ent.Asc(usagestat.FieldDate), ent.Asc(usagestat.FieldModelID)).All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	var m1, m2 *ent.UsageStat
	for _, r := range rows {
		switch r.ModelID {
		case "m1":
			m1 = r
		case "m2":
			m2 = r
		}
	}
	require.NotNil(t, m1)
	require.NotNil(t, m2)
	require.Equal(t, int64(2), m1.RequestCount)
	require.Equal(t, int64(130), m1.PromptTokens)
	require.Equal(t, int64(70), m1.CompletionTokens)
	require.Equal(t, int64(1), m2.RequestCount)
	require.Equal(t, int64(5), m2.PromptTokens)

	// Second run must be a no-op (table non-empty → skip).
	require.NoError(t, svc.BackfillUsageStats(ctx))
	rowsAfter, err := client.UsageStat.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, rowsAfter)
}

// The core guarantee: after backfill, deleting usage_logs (GC cleanup) does
// not erase the aggregates the dashboard reads.
func TestUsageStat_SurvivesLogCleanup(t *testing.T) {
	client, svc := newUsageStatTestEnv(t)
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	now := time.Now().UTC()
	createTestUsageLog(t, ctx, client, now.AddDate(0, 0, -10), "m1", 100, 50)
	createTestUsageLog(t, ctx, client, now.AddDate(0, 0, -5), "m2", 20, 10)

	require.NoError(t, svc.BackfillUsageStats(ctx))

	// Simulate GC cleanup of old usage logs.
	deleted, err := client.UsageLog.Delete().Exec(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, deleted)

	// Aggregates must still be queryable.
	rows, err := client.UsageStat.Query().Order(ent.Asc(usagestat.FieldModelID)).All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	total := int64(0)
	for _, r := range rows {
		total += r.RequestCount
	}
	require.Equal(t, int64(2), total)
}

// Backfill paginates correctly when aggregates span multiple batches.
func TestUsageStat_BackfillMultiBatch(t *testing.T) {
	client, svc := newUsageStatTestEnv(t)
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	old := backfillBatchSize
	backfillBatchSize = 5
	defer func() { backfillBatchSize = old }()

	now := time.Now().UTC()
	// 12 logs, each a distinct model so each becomes its own aggregate row
	// (12 rows > batch 5, exercising multi-batch bulk upserts).
	for i := 0; i < 12; i++ {
		createTestUsageLog(t, ctx, client, now, fmt.Sprintf("m-%02d", i), 10, 5)
	}

	require.NoError(t, svc.BackfillUsageStats(ctx))

	rows, err := client.UsageStat.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 12)

	var total int64
	for _, r := range rows {
		total += r.RequestCount
	}
	require.Equal(t, int64(12), total)
}
