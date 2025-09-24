package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	lib_store "github.com/eko/gocache/lib/v4/store"
	redis "github.com/redis/go-redis/v9"
)

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

	//nolint:forcetypeassert // Expected type is string.
	object, err := gs.client.Get(ctx, key.(string)).Result()
	if errors.Is(err, redis.Nil) {
		return result, lib_store.NotFoundWithCause(err)
	}

	if err != nil {
		return result, err
	}

	// JSON object or array - unmarshal into the target type
	if err := json.Unmarshal([]byte(object), &result); err != nil {
		var zero T
		// If JSON decoding fails, return error
		return zero, err
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
	if err := json.Unmarshal([]byte(object), &result); err != nil {
		var zero T
		// If JSON decoding fails, return error
		return zero, 0, err
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

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	encodedValue := string(raw)

	//nolint:forcetypeassert // Expected type is string.
	err = s.client.Set(ctx, key.(string), encodedValue, opts.Expiration).Err()
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
	for _, tag := range tags {
		tagKey := fmt.Sprintf(RedisTagPattern, tag)
		//nolint:forcetypeassert // Expected type is string.
		s.client.SAdd(ctx, tagKey, key.(string))
		s.client.Expire(ctx, tagKey, ttl)
	}
}

func (s *RedisStore[T]) setTags(ctx context.Context, key any, tags []string) {
	s.setTagsWithTTL(ctx, key, tags, 720*time.Hour)
}

// Delete removes data from Redis for given key identifier.
func (gs *RedisStore[T]) Delete(ctx context.Context, key any) error {
	//nolint:forcetypeassert // Expected type is string.
	return gs.client.Del(ctx, key.(string)).Err()
}

// GetType returns the store type.
func (gs *RedisStore[T]) GetType() string {
	return RedisType
}

// Clear resets all data in the store.
func (gs *RedisStore[T]) Clear(ctx context.Context) error {
	return gs.client.FlushAll(ctx).Err()
}

// Invalidate invalidates some cache data in Redis for given options.
func (gs *RedisStore[T]) Invalidate(ctx context.Context, options ...lib_store.InvalidateOption) error {
	return gs.client.FlushAll(ctx).Err()
}
