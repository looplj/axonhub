// 职责: 使用与 API key 相同的 IndexedCache 按 model ID 缓存完整 Model 实体。
// 关联文件:
//
//	internal/server/biz/api_key.go
//	internal/server/biz/model.go
//	internal/server/biz/channel_price.go
//	internal/pkg/xcache/live/indexed.go
//
// 注意事项:
//
//	缓存实体只读；事务读取绕过缓存，提交后才失效并广播新旧键。
//	增量读取包含软删除记录；列表不能用可能过期或尚未加载完整的 GetAll 代替数据库查询。

package biz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/watcher"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
)

type ModelCacheServiceParams struct {
	fx.In

	Ent         *ent.Client
	CacheConfig xcache.Config
}

// ModelCacheService shares immutable Model entities between routing and pricing.
type ModelCacheService struct {
	db       *ent.Client
	cacheMu  sync.Mutex
	cache    *live.IndexedCache[string, *ent.Model]
	notifier watcher.Notifier[live.CacheEvent[string]]
	stopped  atomic.Bool
}

func NewModelCacheService(params ModelCacheServiceParams) *ModelCacheService {
	mode := params.CacheConfig.Mode
	if mode == "" {
		mode = xcache.ModeMemory
	}
	if mode == xcache.ModeTwoLevel {
		mode = watcher.ModeRedis
	}
	notifier, err := watcher.NewWatcherFromConfig[live.CacheEvent[string]](watcher.Config{
		Mode: mode, Redis: params.CacheConfig.Redis,
	}, watcher.WatcherFromConfigOptions{RedisChannel: "axonhub:cache:models:indexed:v1", Buffer: 32})
	if err != nil {
		panic(fmt.Errorf("model watcher init failed: %w", err))
	}
	ttl := params.CacheConfig.Memory.Expiration
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	svc := &ModelCacheService{db: params.Ent, notifier: notifier}
	svc.initCache(svc.options(ttl))
	if err := svc.cache.Load(context.Background()); err != nil {
		svc.Stop()
		panic(fmt.Errorf("model cache initial load failed: %w", err))
	}
	// Cover bulk writes and backup restoration as well as ModelService mutations.
	params.Ent.Model.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			if svc.stopped.Load() {
				return next.Mutate(ctx, mutation)
			}
			m, ok := mutation.(*ent.ModelMutation)
			if !ok {
				return next.Mutate(ctx, mutation)
			}
			// Client hooks precede schema privacy hooks. Authorize before host reads
			// used to collect the old cache keys; keep normal mutation policies too.
			if err := model.Policy.EvalMutation(ctx, m); err != nil {
				return nil, err
			}
			keys, err := svc.mutationKeys(ctx, m)
			if err != nil {
				return nil, err
			}
			value, err := next.Mutate(ctx, mutation)
			if err != nil {
				return value, err
			}
			if key, ok := m.ModelID(); ok {
				keys = append(keys, key)
			}
			if tx, txErr := m.Tx(); txErr == nil {
				ctx = ent.NewTxContext(ctx, tx)
			}
			runAfterCommit(ctx, func(ctx context.Context) { svc.invalidate(ctx, keys) })
			return value, nil
		})
	})
	return svc
}

func (s *ModelCacheService) initCache(options live.IndexedOptions[string, *ent.Model]) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache = live.NewIndexedCache(options)
}

func (s *ModelCacheService) options(ttl time.Duration) live.IndexedOptions[string, *ent.Model] {
	return live.IndexedOptions[string, *ent.Model]{
		Name: "axonhub:models:indexed:v1", TTL: ttl, RefreshInterval: 30 * time.Second,
		DebounceDelay: 500 * time.Millisecond,
		KeyFunc:       func(m *ent.Model) string { return m.ModelID },
		DeletedFunc:   func(m *ent.Model) bool { return m.DeletedAt != 0 },
		Watcher:       s.notifier, LoadOneFunc: s.loadOne, LoadSinceFunc: s.loadSince,
	}
}

func (s *ModelCacheService) Stop() {
	s.stopped.Store(true)
	s.cache.Stop()
}

func (s *ModelCacheService) loadOne(ctx context.Context, key string) (*ent.Model, error) {
	return authz.RunAsHost(ctx, s.db, "model-cache-load", func(ctx context.Context) (*ent.Model, error) {
		item, err := s.db.Model.Query().Where(model.ModelID(key)).First(ctx)
		if ent.IsNotFound(err) {
			return nil, live.ErrKeyNotFound
		}
		return item, err
	})
}

func (s *ModelCacheService) loadSince(ctx context.Context, since time.Time) ([]*ent.Model, time.Time, error) {
	items, err := authz.RunAsHost(ctx, s.db, "model-cache-refresh", func(ctx context.Context) ([]*ent.Model, error) {
		query := s.db.Model.Query().Order(ent.Asc(model.FieldID))
		if !since.IsZero() {
			query = query.Where(model.UpdatedAtGTE(since))
		}
		return query.All(schematype.SkipSoftDelete(ctx))
	})
	if err != nil {
		return nil, since, err
	}
	latest := since
	// ModelID is mutable, unlike an entity's primary key. A notification may be
	// lost, so incremental refresh must also remove old names of changed entities.
	renamed := make(map[int]string, len(items))
	for _, item := range items {
		renamed[item.ID] = item.ModelID
		if item.UpdatedAt.After(latest) {
			latest = item.UpdatedAt
		}
	}
	s.cacheMu.Lock()
	cache := s.cache
	s.cacheMu.Unlock()
	if cache != nil {
		for key, item := range cache.GetAll() {
			if newKey, ok := renamed[item.ID]; ok && newKey != key {
				cache.Invalidate(key)
			}
		}
	}
	return items, latest, nil
}

func (s *ModelCacheService) mutationKeys(ctx context.Context, m *ent.ModelMutation) ([]string, error) {
	if m.Op().Is(ent.OpCreate) {
		return nil, nil
	}
	client := m.Client()
	return authz.RunAsHost(ctx, client, "model-cache-mutation-keys", func(ctx context.Context) ([]string, error) {
		ids, err := m.IDs(ctx)
		if err != nil {
			return nil, err
		}
		return client.Model.Query().Where(model.IDIn(ids...)).Select(model.FieldModelID).Strings(ctx)
	})
}

func (s *ModelCacheService) invalidate(ctx context.Context, keys []string) {
	if s.stopped.Load() || len(keys) == 0 {
		return
	}
	unique := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		unique[key] = struct{}{}
	}
	keys = keys[:0]
	for key := range unique {
		s.cache.Invalidate(key)
		keys = append(keys, key)
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.notifier.Notify(ctx, live.NewInvalidateKeysEvent(keys...)); err != nil {
		log.Warn(ctx, "model cache watcher notify failed", log.Cause(err))
	}
}

// get is an internal host lookup; public entry points must authorize callers.
func (s *ModelCacheService) get(ctx context.Context, client *ent.Client, id string) (*ent.Model, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ent.TxFromContext(ctx) != nil || client != s.db {
		return client.Model.Query().Where(model.ModelID(id)).First(ctx)
	}
	item, err := s.cache.Get(ctx, id)
	if errors.Is(err, live.ErrKeyNotFound) {
		return nil, &ent.NotFoundError{}
	}
	return item, err
}
