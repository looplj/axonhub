package xcache

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/eko/gocache/lib/v4/store"

	cachelib "github.com/eko/gocache/lib/v4/cache"
	gocache_store "github.com/eko/gocache/store/go_cache/v4"
	gocache "github.com/patrickmn/go-cache"
	redis "github.com/redis/go-redis/v9"

	"github.com/looplj/axonhub/internal/log"
	redis_store "github.com/looplj/axonhub/internal/pkg/xcache/redis"
)

// Cache is an alias to the gocache CacheInterface for convenience.
// It allows you to depend on xcache while still exposing the common methods:
//   - Get(ctx, key) (T, error)
//   - Set(ctx, key, value, options ...Option) error
//   - Delete(ctx, key) error
//   - Invalidate(ctx, options ...store.InvalidateOption) error
//   - Clear(ctx) error
//   - GetType() string
// For setter caches (memory/redis/chain), GetWithTTL and GetCodec are also available on the returned value.
// See: github.com/eko/gocache/lib/v4/cache
//
// Usage example:
//   mem := xcache.NewMemory[string](gocache.New(5*time.Minute, 10*time.Minute))
//   rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
//   rds := xcache.NewRedis[string](rdb, store.WithExpiration(15*time.Second))
//   l2 := xcache.NewTwoLevel[string](mem, rds)
//
//   _ = mem.Set(ctx, "foo", "bar")
//   val, _ := l2.Get(ctx, "foo")

type Cache[T any] = cachelib.CacheInterface[T]

type SetterCache[T any] = cachelib.SetterCacheInterface[T]

// NewMemory creates a pure in-memory cache using patrickmn/go-cache as the backend.
// Pass an existing *gocache.Cache so you control default expiration & cleanup interval.
// Optionally pass store options (e.g., store.WithExpiration) when setting values.
func NewMemory[T any](client *gocache.Cache, options ...Option) SetterCache[T] {
	store := gocache_store.NewGoCache(client, options...)
	return cachelib.New[T](store)
}

// NewMemoryWithOptions is a convenience constructor that builds the patrickmn/go-cache client
// for you using the provided default expiration and cleanup interval.
func NewMemoryWithOptions[T any](defaultExpiration, cleanupInterval time.Duration, options ...Option) SetterCache[T] {
	client := gocache.New(defaultExpiration, cleanupInterval)
	return NewMemory[T](client, options...)
}

// NewFromConfig builds a typed cache from the given Config.
// Modes:
//   - memory: in-memory only
//   - redis: redis only
//   - two-level: memory + redis chain
//
// Memory and Redis expiration can be configured separately.
// If mode is not set or invalid, returns a noop cache that does nothing.
func NewFromConfig[T any](cfg Config) Cache[T] {
	// If mode is not set or empty, return noop cache
	if cfg.Mode == "" {
		return NewNoop[T]()
	}

	// Build memory setter cache with separate expiration settings
	memExpiration := defaultIfZero(cfg.Memory.Expiration, 5*time.Minute)
	memCleanupInterval := defaultIfZero(cfg.Memory.CleanupInterval, 10*time.Minute)

	memClient := gocache.New(memExpiration, memCleanupInterval)
	memStore := gocache_store.NewGoCache(memClient, store.WithExpiration(memExpiration))
	mem := cachelib.New[T](memStore)

	// Build redis setter cache if configured
	var rds SetterCache[T]

	if (cfg.Redis.Addr != "" || cfg.Redis.URL != "") && cfg.Mode != ModeMemory {
		opts, err := newRedisOptions(cfg.Redis)
		if err != nil {
			panic(fmt.Errorf("invalid redis config: %w", err))
		}

		client := redis.NewClient(opts)

		if err := client.Ping(context.Background()).Err(); err != nil {
			panic(fmt.Errorf("failed to ping redis: %w", err))
		}

		redisExpiration := defaultIfZero(cfg.Redis.Expiration, 30*time.Minute) // Default longer for Redis
		rdsStore := redis_store.NewRedisStore[T](client, store.WithExpiration(redisExpiration))
		rds = cachelib.New[T](rdsStore)
	}

	switch cfg.Mode {
	case ModeTwoLevel:
		if rds != nil {
			log.Info(context.Background(), "Using two-level cache")
			return cachelib.NewChain[T](mem, rds)
		}

		return mem
	case ModeRedis:
		if rds == nil {
			panic(errors.New("redis cache config is invalid"))
		}

		log.Info(context.Background(), "Using redis cache")

		return rds
	case ModeMemory:
		log.Info(context.Background(), "Using memory cache")
		return mem
	default:
		log.Info(context.Background(), "Disable cache")
		// Return noop cache for invalid modes
		return NewNoop[T]()
	}
}

func defaultIfZero(d, def time.Duration) time.Duration {
	if d == 0 {
		return def
	}

	return d
}

// NewRedis creates a pure Redis cache using github.com/redis/go-redis/v9 as the client.
// You can pass store options like store.WithExpiration, store.WithTags, etc.
func NewRedis[T any](client *redis.Client, options ...Option) SetterCache[T] {
	store := redis_store.NewRedisStore[T](client, options...)
	return cachelib.New[T](store)
}

// NewRedisWithOptions builds a redis.Client for you and returns the cache.
func NewRedisWithOptions[T any](opts *redis.Options, options ...Option) SetterCache[T] {
	client := redis.NewClient(opts)
	return NewRedis[T](client, options...)
}

// NewTwoLevel constructs a 2-level cache: memory first, then Redis.
// It takes already constructed setter caches so you can mix any memory/redis implementations.
// Typical usage: NewTwoLevel(NewMemory(...), NewRedis(...)).
func NewTwoLevel[T any](memory SetterCache[T], redis SetterCache[T]) Cache[T] {
	return cachelib.NewChain[T](memory, redis)
}

// NewTwoLevelWithClients is a convenience to create a 2-level cache from raw clients directly.
func NewTwoLevelWithClients[T any](memClient *gocache.Cache, redisClient *redis.Client, memOptions []Option, redisOptions []Option) Cache[T] {
	mem := NewMemory[T](memClient, memOptions...)
	rds := NewRedis[T](redisClient, redisOptions...)

	return NewTwoLevel[T](mem, rds)
}

// newRedisOptions constructs redis.Options from RedisConfig.
// Supports two modes:
// 1. URL mode: Use cfg.URL for full redis:// or rediss:// URLs with credentials
// 2. Simple mode: Use cfg.Addr for plain host:port format
// Priority: URL > Addr
func newRedisOptions(cfg RedisConfig) (*redis.Options, error) {
	opts := &redis.Options{}

	// Priority 1: URL mode (redis:// or rediss://)
	if cfg.URL != "" {
		u, err := url.Parse(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}

		switch u.Scheme {
		case "redis", "rediss":
		default:
			return nil, fmt.Errorf("unsupported redis scheme: %s (expected redis:// or rediss://)", u.Scheme)
		}

		if u.Host == "" {
			return nil, errors.New("redis url missing host")
		}

		opts.Addr = u.Host

		if u.User != nil {
			opts.Username = u.User.Username()
			if pwd, ok := u.User.Password(); ok {
				opts.Password = pwd
			}
		}

		if u.Path != "" && u.Path != "/" {
			dbStr := strings.TrimPrefix(u.Path, "/")
			if dbStr != "" {
				db, err := strconv.Atoi(dbStr)
				if err != nil {
					return nil, fmt.Errorf("invalid redis db in url: %w", err)
				}
				opts.DB = db
			}
		}

		if u.Scheme == "rediss" {
			opts.TLSConfig = &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: cfg.TLSInsecureSkipVerify, // #nosec G402 -- User explicitly controls this via config
			}
		}
	} else if cfg.Addr != "" {
		// Priority 2: Simple addr mode (host:port)
		opts.Addr = strings.TrimSpace(cfg.Addr)
		if opts.Addr == "" {
			return nil, errors.New("redis addr or url is required")
		}
	} else {
		return nil, errors.New("redis addr or url is required")
	}

	// Config fields override URL credentials/DB when explicitly set
	if cfg.Username != "" {
		opts.Username = cfg.Username
	}
	if cfg.Password != "" {
		opts.Password = cfg.Password
	}
	if cfg.DB != nil {
		opts.DB = *cfg.DB
	}

	// Explicit TLS flag
	if cfg.TLS {
		if opts.TLSConfig == nil {
			opts.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12, // #nosec G402 -- User can explicitly enable InsecureSkipVerify via config
			}
		}
		opts.TLSConfig.InsecureSkipVerify = cfg.TLSInsecureSkipVerify // #nosec G402 -- User explicitly controls this via config
	}

	// Ensure TLSInsecureSkipVerify is not silently set without TLS
	if opts.TLSConfig == nil && cfg.TLSInsecureSkipVerify {
		return nil, errors.New("tls_insecure_skip_verify requires TLS to be enabled (tls=true or rediss://)")
	}

	return opts, nil
}
