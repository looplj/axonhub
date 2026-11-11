// 职责: 缓存请求入口解析出的 trace/thread 身份，避免复用会话时反复查询。
// 关联文件:
//
//	internal/server/biz/trace.go
//	internal/server/biz/thread.go
//	internal/server/middleware/trace.go
//	internal/server/middleware/thread.go
//
// 遵循的 Rules:
//
//	.agents/rules/biz-services.md
//	.agents/rules/cache-compat.md
//
// 注意事项:
//
//	只缓存身份字段，不提供状态或时间戳快照。放进请求 context 的 trace/thread 也只有这些字段。
//	事务和其他数据库客户端绕过缓存。
//	删除、重命名及父实体删除在提交后失效；Redis 通知为尽力交付，TTL 限制漏通知的影响。

package biz

import (
	"context"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/watcher"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

const requestIdentityTTL = time.Minute

type requestIdentityKey struct {
	kind       string
	projectID  int
	externalID string
}

type requestIdentity struct {
	id, threadID int
	expires      time.Time
}

type requestIdentityCache struct {
	mu         sync.Mutex
	generation uint64
	entries    *lru.Cache[requestIdentityKey, requestIdentity]
	client     *ent.Client
	notifier   watcher.Notifier[string]
	stop       func()
}

func newRequestIdentityCache(client *ent.Client, cfg xcache.Config) *requestIdentityCache {
	entries, _ := lru.New[requestIdentityKey, requestIdentity](4096)
	c := &requestIdentityCache{client: client, entries: entries, stop: func() {}}
	// Memory deployments need no background worker. Redis deployments publish
	// invalidations across processes; the cache payload itself stays local.
	if cfg.Mode == xcache.ModeRedis || cfg.Mode == xcache.ModeTwoLevel {
		n, err := watcher.NewWatcherFromConfig[string](watcher.Config{Mode: watcher.ModeRedis, Redis: cfg.Redis}, watcher.WatcherFromConfigOptions{RedisChannel: "axonhub:cache:request_identity:v1", Buffer: 32})
		if err != nil {
			panic(fmt.Errorf("request identity watcher: %w", err))
		}
		c.notifier = n
		events, stop := n.Watch()
		c.stop = sync.OnceFunc(stop)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error(context.Background(), "request identity watcher panicked", log.Any("panic", r))
				}
			}()
			for range events {
				c.clear()
			}
		}()
	}
	client.Trace.Use(c.hook("trace_id"))
	client.Thread.Use(c.hook("thread_id"))
	client.Project.Use(c.hook(""))
	return c
}

func (c *requestIdentityCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	c.entries.Purge()
}

func (c *requestIdentityCache) invalidate(ctx context.Context) {
	c.clear()
	if c.notifier != nil {
		if err := c.notifier.Notify(ctx, "invalidate"); err != nil {
			log.Warn(ctx, "Failed to invalidate request identities", log.Cause(err))
		}
	}
}

func (c *requestIdentityCache) hook(identityField string) ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			_, renamed := m.Field(identityField)
			deletes := m.Op().Is(ent.OpDelete | ent.OpDeleteOne)
			renames := renamed && m.Op().Is(ent.OpUpdate|ent.OpUpdateOne)
			if !deletes && !renames {
				return next.Mutate(ctx, m)
			}
			c.clear()
			value, err := next.Mutate(ctx, m)
			if err != nil {
				return value, err
			}
			// Use the mutation's transaction even when its caller supplied only tx.Client().
			if tm, ok := m.(interface{ Tx() (*ent.Tx, error) }); ok {
				if tx, txErr := tm.Tx(); txErr == nil {
					ctx = ent.NewTxContext(ctx, tx)
				}
			}
			runAfterCommit(ctx, c.invalidate)
			return value, nil
		})
	}
}

func (c *requestIdentityCache) resolve(ctx context.Context, client *ent.Client, key requestIdentityKey, load func() (requestIdentity, error)) (requestIdentity, error) {
	if err := ctx.Err(); err != nil {
		return requestIdentity{}, err
	}
	if client != c.client || ent.TxFromContext(ctx) != nil {
		return load()
	}
	c.mu.Lock()
	generation := c.generation
	cached, ok := c.entries.Get(key)
	c.mu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return cached, nil
	}
	value, err := load()
	if err != nil {
		return requestIdentity{}, err
	}
	value.expires = time.Now().Add(requestIdentityTTL)
	c.mu.Lock()
	if generation == c.generation {
		c.entries.Add(key, value)
	}
	c.mu.Unlock()
	return value, nil
}

// ResolveTraceForRequest returns ID, ProjectID, TraceID, and ThreadID only.
// Status, timestamps, and edges are zero, including the value stored in the
// request context. Entity readers keep using GetOrCreateTrace.
func (s *TraceService) ResolveTraceForRequest(ctx context.Context, projectID int, externalID string, threadID *int) (*ent.Trace, error) {
	if project, ok := contexts.GetProjectID(ctx); !ok || project != projectID {
		return nil, fmt.Errorf("trace project does not match request")
	}
	client := s.entFromContext(ctx)
	value, err := s.identities.resolve(ctx, client, requestIdentityKey{"trace", projectID, externalID}, func() (requestIdentity, error) {
		return authz.RunAsHost(ctx, client, "request-trace-identity", func(hostCtx context.Context) (requestIdentity, error) {
			row, err := s.GetOrCreateTrace(hostCtx, projectID, externalID, threadID)
			if err != nil {
				return requestIdentity{}, err
			}
			return requestIdentity{id: row.ID, threadID: row.ThreadID}, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return &ent.Trace{ID: value.id, ProjectID: projectID, TraceID: externalID, ThreadID: value.threadID}, nil
}

// ResolveThreadForRequest returns ID, ProjectID, and ThreadID only.
// Status, timestamps, and edges are zero, including the value stored in the
// request context. Status readers keep using GetOrCreateThread.
func (s *ThreadService) ResolveThreadForRequest(ctx context.Context, projectID int, externalID string) (*ent.Thread, error) {
	if project, ok := contexts.GetProjectID(ctx); !ok || project != projectID {
		return nil, fmt.Errorf("thread project does not match request")
	}
	client := s.entFromContext(ctx)
	value, err := s.traceService.identities.resolve(ctx, client, requestIdentityKey{"thread", projectID, externalID}, func() (requestIdentity, error) {
		return authz.RunAsHost(ctx, client, "request-thread-identity", func(hostCtx context.Context) (requestIdentity, error) {
			row, err := s.GetOrCreateThread(hostCtx, projectID, externalID)
			if err != nil {
				return requestIdentity{}, err
			}
			return requestIdentity{id: row.ID}, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return &ent.Thread{ID: value.id, ProjectID: projectID, ThreadID: externalID}, nil
}
