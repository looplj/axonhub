package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	lib_store "github.com/eko/gocache/lib/v4/store"
	"github.com/looplj/axonhub/internal/log"
	redis "github.com/redis/go-redis/v9"
)

// safeUnmarshal wraps json.Unmarshal with panic recovery.
// On decode failure it logs a warning and returns an error,
// allowing callers to treat corrupted cache data as a miss.
func safeUnmarshal[T any](ctx context.Context, key string, data []byte, dest *T) error {
	defer func() {
		if r := recover(); r != nil {
			log.Warn(ctx, "Redis cache data caused unmarshal panic, treating as cache miss",
				log.String("cache_key", key),
				log.Any("panic", r))
		}
	}()
	if err := json.Unmarshal(data, dest); err != nil {
		log.Warn(ctx, "Redis cache data failed to unmarshal, treating as cache miss",
			log.String("cache_key", key),
			log.Cause(err))
		return err
	}
	return nil
}

// RedisClientInterface represents a go-redis/redis client.
type RedisClientInterface interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Set(ctx context.Context, key string, values any, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	FlushAll(ctx context.Context) *redis.StatusCmd
	SAdd(ctx context.Context, key string, members ...any) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
}

const (
	// RedisType represents the storage type as a string value.
	RedisType = "redis"
	// RedisTagPattern represents the tag pattern to be used as a key in specified storage.
	RedisTagPattern = "gocache_tag_%s"
)

// RedisStore wraps the RedisStore to provide type-safe operations.
type RedisStore[T any] struct {
	client  RedisClientInterface
	options *lib_store.Options
}

// NewRedisStore creates a new generic store.
func NewRedisStore[T any](client RedisClientInterface, options ...lib_store.Option) *RedisStore[T] {
	return &RedisStore[T]{
		client:  client,
		options: lib_store.ApplyOptions(options...),
	}
}

// Get returns typed data stored from a given key.
func (gs *RedisStore[T]) Get(ctx context.Context, key any) (any, error) {
	var result T

	keyString, ok := key.(string)
	if !ok {
		return result, lib_store.NotFoundWithCause(fmt.Errorf("expected string key, got %T", key))
	}

	object, err := gs.client.Get(ctx, keyString).Result()
	if errors.Is(err, redis.Nil) {
		return result, lib_store.NotFoundWithCause(err)
	}

	if err != nil {
		return result, err
	}

	// JSON object or array - unmarshal into the target type
	if err := safeUnmarshal(ctx, keyString, []byte(object), &result); err != nil {
		var zero T
		return zero, lib_store.NotFoundWithCause(err)
	}

	return result, nil
}

// GetWithTTL returns typed data stored from a given key and its corresponding TTL.
func (gs *RedisStore[T]) GetWithTTL(ctx context.Context, key any) (any, time.Duration, error) {
	var result T

	keyString, ok := key.(string)
	if !ok {
		return result, 0, lib_store.NotFoundWithCause(fmt.Errorf("expected string key, got %T", key))
	}

	object, err := gs.client.Get(ctx, keyString).Result()
	if errors.Is(err, redis.Nil) {
		return result, 0, lib_store.NotFoundWithCause(err)
	}

	if err != nil {
		return result, 0, err
	}

	// JSON object or array - unmarshal into the target type
	if err := safeUnmarshal(ctx, keyString, []byte(object), &result); err != nil {
		var zero T
		return zero, 0, lib_store.NotFoundWithCause(err)
	}

	ttl, err := gs.client.TTL(ctx, keyString).Result()
	if err != nil {
		var zero T
		return zero, 0, err
	}

	return result, ttl, err
}

// Set defines data in Redis for given key identifier.
func (s *RedisStore[T]) Set(ctx context.Context, key any, value any, options ...lib_store.Option) error {
	opts := lib_store.ApplyOptionsWithDefault(s.options, options...)

	keyString, ok := key.(string)
	if !ok {
		return fmt.Errorf("expected string key, got %T", key)
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	encodedValue := string(raw)

	err = s.client.Set(ctx, keyString, encodedValue, opts.Expiration).Err()
	if err != nil {
		return err
	}

	if tags := opts.Tags; len(tags) > 0 {
		if ttl := opts.TagsTTL; ttl == 0 {
			s.setTags(ctx, key, tags)
		} else {
			s.setTagsWithTTL(ctx, key, tags, ttl)
		}
	}

	return nil
}

func (s *RedisStore[T]) setTagsWithTTL(ctx context.Context, key any, tags []string, ttl time.Duration) {
	keyString, ok := key.(string)
	if !ok {
		return
	}
	for _, tag := range tags {
		tagKey := fmt.Sprintf(RedisTagPattern, tag)
		s.client.SAdd(ctx, tagKey, keyString)
		s.client.Expire(ctx, tagKey, ttl)
	}
}

func (s *RedisStore[T]) setTags(ctx context.Context, key any, tags []string) {
	s.setTagsWithTTL(ctx, key, tags, 168*time.Hour)
}

// Delete removes data from Redis for given key identifier.
func (gs *RedisStore[T]) Delete(ctx context.Context, key any) error {
	keyString, ok := key.(string)
	if !ok {
		return fmt.Errorf("expected string key, got %T", key)
	}
	return gs.client.Del(ctx, keyString).Err()
}

// GetType returns the store type.
func (gs *RedisStore[T]) GetType() string {
	return RedisType
}

// Clear resets all data in this store by scanning and deleting keys.
// Uses SCAN + DEL instead of FlushAll to avoid affecting other services
// that may share the same Redis instance.
func (gs *RedisStore[T]) Clear(ctx context.Context) error {
	var cursor uint64
	for {
		keys, nextCursor, err := gs.client.Scan(ctx, cursor, "*", 100).Result()
		if err != nil {
			return fmt.Errorf("scan keys: %w", err)
		}
		if len(keys) > 0 {
			if err := gs.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("delete keys: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Invalidate invalidates cache entries by tag using the tag-set mechanism.
// Only keys associated with the given tags are deleted, leaving other
// services' data untouched.
func (gs *RedisStore[T]) Invalidate(ctx context.Context, options ...lib_store.InvalidateOption) error {
	opts := lib_store.ApplyInvalidateOptions(options...)
	for _, tag := range opts.Tags {
		tagKey := fmt.Sprintf(RedisTagPattern, tag)
		keys, err := gs.client.SMembers(ctx, tagKey).Result()
		if err != nil {
			return fmt.Errorf("get tag members: %w", err)
		}
		if len(keys) > 0 {
			if err := gs.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("delete tag keys: %w", err)
			}
		}
		if err := gs.client.Del(ctx, tagKey).Err(); err != nil {
			return fmt.Errorf("delete tag: %w", err)
		}
	}
	return nil
}
