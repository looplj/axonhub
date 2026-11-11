// 职责: 验证 Model live cache 的查询次数、权限、事务和模型变更行为。
// 关联文件:
//
//	internal/server/biz/model_cache.go
//	internal/server/biz/model.go
//	internal/server/biz/channel_price.go
package biz

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/intercept"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

func TestModelCacheReadsWithoutSQL(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:model-cache-reads?mode=memory&cache=shared&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	createModelLibraryPriceEntry(t, ctx, client, "model", model.StatusEnabled, objects.ModelCardCost{Input: 2})
	cache := NewModelCacheService(ModelCacheServiceParams{Ent: client})
	cache.Stop() // Disable background SQL while asserting hot-path query counts.
	// Warm negative entries; IndexedCache queries a missing key once, then caches it.
	for _, key := range []string{"absent", "model-high", "absent-high"} {
		_, err := cache.get(ctx, client, key)
		require.True(t, ent.IsNotFound(err))
	}
	client.Model.Intercept(intercept.Func(func(context.Context, intercept.Query) error {
		return errors.New("unexpected Model SQL")
	}))
	models := &ModelService{AbstractService: &AbstractService{db: client}, modelCache: cache}
	channels := &ChannelService{AbstractService: &AbstractService{db: client}, modelCache: cache}
	for range 3 {
		item, err := models.GetModelByModelID(ctx, "model", model.StatusEnabled)
		require.NoError(t, err)
		require.Equal(t, float64(2), item.ModelCard.Cost.Input)
		_, err = models.GetModelByModelID(ctx, "absent", model.StatusEnabled)
		require.True(t, ent.IsNotFound(err))
		_, err = models.GetModelByModelID(ctx, "model", model.StatusArchived)
		require.True(t, ent.IsNotFound(err))
		price, found := channels.resolveModelLibraryPrice(WithModelLibraryPriceMemo(ctx), "model-high")
		require.True(t, found)
		require.Equal(t, "2", price.Items[0].Pricing.UsagePerUnit.String())
		_, found = channels.resolveModelLibraryPrice(WithModelLibraryPriceMemo(ctx), "absent-high")
		require.False(t, found)
	}
	_, err := models.GetModelByModelID(context.Background(), "model", model.StatusEnabled)
	require.Error(t, err, "cache hits must not bypass Model authorization")
	_, err = models.ListModels(context.Background(), nil)
	require.Error(t, err)
	// A failed refresh must leave the last good snapshot available.
	require.Error(t, cache.cache.Load(ctx))
	_, err = models.GetModelByModelID(ctx, "model", model.StatusEnabled)
	require.NoError(t, err)
}

func TestModelCacheMutationsAndTransactions(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:model-cache-mutations?mode=memory&cache=shared&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	cache := NewModelCacheService(ModelCacheServiceParams{Ent: client})
	defer cache.Stop()
	createModelLibraryPriceEntry(t, ctx, client, "old", model.StatusEnabled, objects.ModelCardCost{Input: 2})
	item, err := cache.get(ctx, client, "old")
	require.NoError(t, err)

	_, err = cache.get(ctx, client, "new")
	require.True(t, ent.IsNotFound(err))
	var reads atomic.Int32
	client.Model.Intercept(intercept.Func(func(context.Context, intercept.Query) error {
		reads.Add(1)
		return nil
	}))
	_, err = client.Model.UpdateOneID(item.ID).SetName("unauthorized").Save(context.Background())
	require.Error(t, err)
	require.Zero(t, reads.Load(), "unauthorized mutation must not read cache invalidation keys")
	for _, commit := range []bool{false, true} {
		tx, err := client.Tx(ctx)
		require.NoError(t, err)
		// Deliberately omit ent.NewTxContext: the mutation hook must discover
		// the transaction from the mutation itself.
		txCtx := ent.NewContext(ctx, tx.Client())
		_, err = tx.Client().Model.UpdateOneID(item.ID).SetModelID("new").Save(txCtx)
		require.NoError(t, err)
		_, err = cache.get(txCtx, ent.FromContext(txCtx), "new")
		require.NoError(t, err, "transaction must read its own writes")
		_, err = cache.get(ctx, client, "new")
		require.True(t, ent.IsNotFound(err), "uncommitted data must not enter cache")
		if commit {
			require.NoError(t, tx.Commit())
		} else {
			require.NoError(t, tx.Rollback())
		}
	}
	_, err = cache.get(ctx, client, "old")
	require.True(t, ent.IsNotFound(err), "rename must remove the old key")
	_, err = cache.get(ctx, client, "new")
	require.NoError(t, err)
	_, err = client.Model.Update().Where(model.ID(item.ID)).SetStatus(model.StatusArchived).Save(ctx)
	require.NoError(t, err)
	channels := &ChannelService{AbstractService: &AbstractService{db: client}, modelCache: cache}
	_, found := channels.resolveModelLibraryPrice(ctx, "new")
	require.False(t, found, "archived models must not supply fallback prices")
	require.NoError(t, client.Model.DeleteOneID(item.ID).Exec(ctx))
	_, err = cache.get(ctx, client, "new")
	require.True(t, ent.IsNotFound(err), "soft-deleted model must disappear")
}

func TestModelCacheWatcherRefresh(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:model-cache-watcher?mode=memory&cache=shared&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	cache := NewModelCacheService(ModelCacheServiceParams{Ent: client})
	defer cache.Stop()
	// Independent reader with no local mutation hook, sharing the notification
	// stream exactly as a remote instance would through Redis.
	reader := &ModelCacheService{db: client, notifier: cache.notifier}
	reader.initCache(reader.options(time.Minute))
	defer reader.Stop()
	require.NoError(t, reader.cache.Load(ctx))
	_, err := reader.get(ctx, client, "added")
	require.True(t, ent.IsNotFound(err))

	createModelLibraryPriceEntry(t, ctx, client, "added", model.StatusDisabled, objects.ModelCardCost{Input: 3})
	require.Eventually(t, func() bool {
		_, err := reader.get(ctx, client, "added")
		return err == nil
	}, time.Second, time.Millisecond)
}

func TestModelCacheIncrementalRenameAndDelete(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:model-cache-incremental?mode=memory&cache=shared&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	createModelLibraryPriceEntry(t, ctx, client, "before", model.StatusEnabled, objects.ModelCardCost{Input: 1})
	// No mutation hook or watcher: periodic refresh must compensate for lost notifications.
	reader := &ModelCacheService{db: client}
	options := reader.options(time.Minute)
	options.RefreshInterval = 20 * time.Millisecond
	reader.initCache(options)
	defer reader.Stop()
	require.NoError(t, reader.cache.Load(ctx))
	item, err := reader.get(ctx, client, "before")
	require.NoError(t, err)
	_, err = client.Model.UpdateOneID(item.ID).SetModelID("after").Save(ctx)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, old := reader.cache.GetCached("before")
		updated, found := reader.cache.GetCached("after")
		return !old && found && updated.ID == item.ID
	}, time.Second, time.Millisecond)
	_, err = client.Model.UpdateOneID(item.ID).SetStatus(model.StatusArchived).Save(ctx)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		item, ok := reader.cache.GetCached("after")
		return ok && item.Status == model.StatusArchived
	}, time.Second, time.Millisecond)
	require.NoError(t, client.Model.DeleteOneID(item.ID).Exec(ctx))
	require.Eventually(t, func() bool {
		_, ok := reader.cache.GetCached("after")
		return !ok
	}, time.Second, time.Millisecond)
}

func TestModelCacheTTLAndLoadErrors(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:model-cache-ttl?mode=memory&cache=shared&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	createModelLibraryPriceEntry(t, ctx, client, "ttl", model.StatusEnabled, objects.ModelCardCost{Input: 1})
	reader := &ModelCacheService{db: client}
	reader.initCache(reader.options(20 * time.Millisecond))
	reader.Stop()
	require.NoError(t, reader.cache.Load(ctx))
	item, err := reader.get(ctx, client, "ttl")
	require.NoError(t, err)
	_, err = client.Model.UpdateOneID(item.ID).SetName("changed").Save(ctx)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		item, err := reader.get(ctx, client, "ttl")
		return err == nil && item.Name == "changed"
	}, time.Second, time.Millisecond)
	reader.cache.Invalidate("ttl")
	fail := true
	client.Model.Intercept(intercept.Func(func(context.Context, intercept.Query) error {
		if fail {
			return errors.New("transient query failure")
		}
		return nil
	}))
	_, err = reader.get(ctx, client, "ttl")
	require.Error(t, err)
	fail = false
	item, err = reader.get(ctx, client, "ttl")
	require.NoError(t, err, "a database error must not become a negative cache entry")
	require.Equal(t, "changed", item.Name)
}
