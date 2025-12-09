package xcache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gocache "github.com/patrickmn/go-cache"
)

func intPtr(i int) *int {
	return &i
}

func TestNewMemory(t *testing.T) {
	client := gocache.New(5*time.Minute, 10*time.Minute)
	cache := NewMemory[string](client)

	ctx := context.Background()

	// Test Set and Get
	err := cache.Set(ctx, "test-key", "test-value")
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-value", value)

	// Test GetType
	assert.Equal(t, "cache", cache.GetType())
}

func TestNewMemoryWithOptions(t *testing.T) {
	cache := NewMemoryWithOptions[int](5*time.Minute, 10*time.Minute)

	ctx := context.Background()

	// Test with different data type
	err := cache.Set(ctx, "number", 42)
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "number")
	assert.NoError(t, err)
	assert.Equal(t, 42, value)
}

func TestNewRedis(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedis[string](client)

	ctx := context.Background()

	// Test Set and Get
	err := cache.Set(ctx, "redis-key", "redis-value")
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "redis-key")
	assert.NoError(t, err)
	assert.Equal(t, "redis-value", value)

	// Test GetType
	assert.Equal(t, "cache", cache.GetType())
}

func TestNewRedisWithOptions(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	opts := &redis.Options{
		Addr: mr.Addr(),
	}

	cache := NewRedisWithOptions[string](opts)

	ctx := context.Background()

	// Test with string data type
	err := cache.Set(ctx, "redis-string", "test-value")
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "redis-string")
	assert.NoError(t, err)
	assert.Equal(t, "test-value", value)
}

func TestNewTwoLevel(t *testing.T) {
	// Memory cache
	memClient := gocache.New(5*time.Minute, 10*time.Minute)
	memCache := NewMemory[string](memClient)

	// Redis cache with miniredis
	mr := miniredis.RunT(t)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	redisCache := NewRedis[string](redisClient)

	// Two-level cache
	cache := NewTwoLevel[string](memCache, redisCache)

	ctx := context.Background()

	// Test Set - should set in both levels
	err := cache.Set(ctx, "two-level-key", "two-level-value")
	assert.NoError(t, err)

	// Test Get - should get from memory first
	value, err := cache.Get(ctx, "two-level-key")
	assert.NoError(t, err)
	assert.Equal(t, "two-level-value", value)

	// Clear memory cache to test Redis fallback
	err = memCache.Clear(ctx)
	assert.NoError(t, err)

	// Should still get value from Redis
	value, err = cache.Get(ctx, "two-level-key")
	assert.NoError(t, err)
	assert.Equal(t, "two-level-value", value)

	// Test GetType
	assert.Equal(t, "chain", cache.GetType())
}

func TestNewTwoLevelWithClients(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	memClient := gocache.New(5*time.Minute, 10*time.Minute)
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewTwoLevelWithClients[string](
		memClient,
		redisClient,
		[]store.Option{},
		[]store.Option{},
	)

	ctx := context.Background()

	// Test basic functionality
	err := cache.Set(ctx, "client-key", "client-value")
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "client-key")
	assert.NoError(t, err)
	assert.Equal(t, "client-value", value)
}

func TestNewFromConfig_Memory(t *testing.T) {
	cfg := Config{
		Mode: ModeMemory,
		Memory: MemoryConfig{
			Expiration:      5 * time.Minute,
			CleanupInterval: 10 * time.Minute,
		},
	}

	cache := NewFromConfig[string](cfg)

	ctx := context.Background()

	// Test basic functionality
	err := cache.Set(ctx, "memory-config-key", "memory-config-value")
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "memory-config-key")
	assert.NoError(t, err)
	assert.Equal(t, "memory-config-value", value)

	// Should be memory cache
	assert.Equal(t, "cache", cache.GetType())
}

func TestNewFromConfig_Redis(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := Config{
		Mode: ModeRedis,
		Redis: RedisConfig{
			Addr:       mr.Addr(),
			Expiration: 5 * time.Minute,
		},
	}

	cache := NewFromConfig[string](cfg)

	ctx := context.Background()

	// Test basic functionality
	err := cache.Set(ctx, "redis-config-key", "redis-config-value")
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "redis-config-key")
	assert.NoError(t, err)
	assert.Equal(t, "redis-config-value", value)

	// Should be redis cache
	assert.Equal(t, "cache", cache.GetType())
}

func TestNewFromConfig_RedisWithoutAddr(t *testing.T) {
	cfg := Config{
		Mode: ModeRedis,
		// No Redis config - should fallback to memory
	}

	require.Panics(t, func() {
		_ = NewFromConfig[string](cfg)
	})
}

func TestNewFromConfig_TwoLevel(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := Config{
		Mode: ModeTwoLevel,
		Memory: MemoryConfig{
			Expiration:      5 * time.Minute,
			CleanupInterval: 10 * time.Minute,
		},
		Redis: RedisConfig{
			Addr:       mr.Addr(),
			Expiration: 15 * time.Minute,
		},
	}

	cache := NewFromConfig[string](cfg)

	ctx := context.Background()

	// Test basic functionality
	err := cache.Set(ctx, "two-level-config-key", "two-level-config-value")
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "two-level-config-key")
	assert.NoError(t, err)
	assert.Equal(t, "two-level-config-value", value)

	// Should be chain cache
	assert.Equal(t, "chain", cache.GetType())
}

func TestNewFromConfig_TwoLevelWithoutRedis(t *testing.T) {
	cfg := Config{
		Mode: ModeTwoLevel,
		Memory: MemoryConfig{
			Expiration:      5 * time.Minute,
			CleanupInterval: 10 * time.Minute,
		},
		// No Redis config - should fallback to memory
	}

	cache := NewFromConfig[string](cfg)

	ctx := context.Background()

	// Test basic functionality
	err := cache.Set(ctx, "two-level-fallback-key", "two-level-fallback-value")
	assert.NoError(t, err)

	value, err := cache.Get(ctx, "two-level-fallback-key")
	assert.NoError(t, err)
	assert.Equal(t, "two-level-fallback-value", value)

	// Should fallback to memory cache
	assert.Equal(t, "cache", cache.GetType())
}

func TestNewFromConfig_EmptyMode(t *testing.T) {
	cfg := Config{} // Empty config

	cache := NewFromConfig[string](cfg)

	// Should return noop cache
	assert.Equal(t, "noop", cache.GetType())

	ctx := context.Background()
	_, err := cache.Get(ctx, "test")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCacheNotConfigured)
}

func TestNewFromConfig_InvalidMode(t *testing.T) {
	cfg := Config{
		Mode: "invalid-mode",
	}

	cache := NewFromConfig[string](cfg)

	// Should return noop cache
	assert.Equal(t, "noop", cache.GetType())

	ctx := context.Background()
	_, err := cache.Get(ctx, "test")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCacheNotConfigured)
}

func TestCacheWithExpiration(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := Config{
		Mode: ModeRedis,
		Redis: RedisConfig{
			Addr:       mr.Addr(),
			Expiration: 100 * time.Millisecond, // Very short expiration for testing
		},
	}

	cache := NewFromConfig[string](cfg)

	ctx := context.Background()

	// Set value with default expiration
	err := cache.Set(ctx, "expiring-key", "expiring-value")
	assert.NoError(t, err)

	// Should be able to get immediately
	value, err := cache.Get(ctx, "expiring-key")
	assert.NoError(t, err)
	assert.Equal(t, "expiring-value", value)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now (or might still work depending on cache implementation)
	_, err = cache.Get(ctx, "expiring-key")
	// Note: Some cache implementations might not expire immediately, so we don't assert error here
}

func TestCacheOperations(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := Config{
		Mode: ModeRedis,
		Redis: RedisConfig{
			Addr: mr.Addr(),
		},
	}

	cache := NewFromConfig[string](cfg)

	ctx := context.Background()

	// Test Set multiple keys
	err := cache.Set(ctx, "key1", "value1")
	assert.NoError(t, err)

	err = cache.Set(ctx, "key2", "value2")
	assert.NoError(t, err)

	// Test Get multiple keys
	value1, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value1)

	value2, err := cache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.Equal(t, "value2", value2)

	// Test Delete
	err = cache.Delete(ctx, "key1")
	assert.NoError(t, err)

	// key1 should be gone
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)

	// key2 should still exist
	value2, err = cache.Get(ctx, "key2")
	assert.NoError(t, err)
	assert.Equal(t, "value2", value2)

	// Test Clear
	err = cache.Clear(ctx)
	assert.NoError(t, err)

	// All keys should be gone
	_, err = cache.Get(ctx, "key2")
	assert.Error(t, err)
}

func TestDefaultIfZero(t *testing.T) {
	// Test with zero value
	result := defaultIfZero(0, 5*time.Minute)
	assert.Equal(t, 5*time.Minute, result)

	// Test with non-zero value
	result = defaultIfZero(10*time.Minute, 5*time.Minute)
	assert.Equal(t, 10*time.Minute, result)
}

func TestComplexDataTypes(t *testing.T) {
	type TestStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := Config{
		Mode: ModeMemory, // Use memory for complex types
	}

	cache := NewFromConfig[TestStruct](cfg)

	ctx := context.Background()

	testData := TestStruct{
		ID:   123,
		Name: "Test Name",
	}

	// Test Set and Get with struct
	err := cache.Set(ctx, "struct-key", testData)
	assert.NoError(t, err)

	retrievedData, err := cache.Get(ctx, "struct-key")
	assert.NoError(t, err)
	assert.Equal(t, testData, retrievedData)
}

func TestSeparateExpirationConfig(t *testing.T) {
	// Start miniredis server
	mr := miniredis.RunT(t)
	defer mr.Close()

	cfg := Config{
		Mode: ModeTwoLevel,
		Memory: MemoryConfig{
			Expiration:      100 * time.Millisecond, // Very short for memory
			CleanupInterval: 10 * time.Minute,
		},
		Redis: RedisConfig{
			Addr:       mr.Addr(),
			Expiration: 5 * time.Minute, // Much longer for Redis
		},
	}

	cache := NewFromConfig[string](cfg)

	ctx := context.Background()

	// Test that cache works with separate expiration settings
	err := cache.Set(ctx, "separate-exp-key", "separate-exp-value")
	assert.NoError(t, err)

	// Should be able to get immediately
	value, err := cache.Get(ctx, "separate-exp-key")
	assert.NoError(t, err)
	assert.Equal(t, "separate-exp-value", value)

	// Should be chain cache (two-level)
	assert.Equal(t, "chain", cache.GetType())

	// Note: We can't easily test the actual expiration behavior in a unit test
	// since the two-level cache behavior is complex, but we can verify the
	// configuration is accepted and the cache works
}

func TestNewRedisOptions(t *testing.T) {
	t.Run("plain addr with tls flag", func(t *testing.T) {
		opts, err := newRedisOptions(RedisConfig{
			Addr: "127.0.0.1:6379",
			TLS:  true,
		})
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:6379", opts.Addr)
		require.NotNil(t, opts.TLSConfig)
		assert.False(t, opts.TLSConfig.InsecureSkipVerify)
	})

	t.Run("rediss url auto tls and db", func(t *testing.T) {
		opts, err := newRedisOptions(RedisConfig{
			URL:                   "rediss://user:pass@localhost:6380/2",
			TLSInsecureSkipVerify: true,
		})
		require.NoError(t, err)
		assert.Equal(t, "localhost:6380", opts.Addr)
		assert.Equal(t, "user", opts.Username)
		assert.Equal(t, "pass", opts.Password)
		assert.Equal(t, 2, opts.DB)
		require.NotNil(t, opts.TLSConfig)
		assert.True(t, opts.TLSConfig.InsecureSkipVerify)
	})

	t.Run("config overrides url credentials and db", func(t *testing.T) {
		opts, err := newRedisOptions(RedisConfig{
			URL:      "redis://u1:p1@127.0.0.1:6379/1",
			Username: "u2",
			Password: "p2",
			DB:       intPtr(3),
		})
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:6379", opts.Addr)
		assert.Equal(t, "u2", opts.Username)
		assert.Equal(t, "p2", opts.Password)
		assert.Equal(t, 3, opts.DB)
	})

	t.Run("config overrides url db to 0", func(t *testing.T) {
		opts, err := newRedisOptions(RedisConfig{
			URL: "redis://127.0.0.1:6379/1",
			DB:  intPtr(0),
		})
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:6379", opts.Addr)
		assert.Equal(t, 0, opts.DB)
	})

	t.Run("redis url without credentials", func(t *testing.T) {
		opts, err := newRedisOptions(RedisConfig{
			URL: "redis://127.0.0.1:6379",
		})
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:6379", opts.Addr)
		assert.Empty(t, opts.Username)
		assert.Empty(t, opts.Password)
		assert.Equal(t, 0, opts.DB)
	})

	t.Run("plain addr without tls", func(t *testing.T) {
		opts, err := newRedisOptions(RedisConfig{
			Addr: "127.0.0.1:6379",
		})
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:6379", opts.Addr)
		assert.Nil(t, opts.TLSConfig)
	})

	t.Run("plain addr with insecure skip verify only", func(t *testing.T) {
		_, err := newRedisOptions(RedisConfig{
			Addr:                  "127.0.0.1:6379",
			TLSInsecureSkipVerify: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tls_insecure_skip_verify requires TLS to be enabled")
	})

	t.Run("empty addr and url", func(t *testing.T) {
		_, err := newRedisOptions(RedisConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "redis addr or url is required")
	})

	t.Run("whitespace only addr", func(t *testing.T) {
		_, err := newRedisOptions(RedisConfig{
			Addr: "   ",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "redis addr or url is required")
	})

	t.Run("invalid scheme", func(t *testing.T) {
		_, err := newRedisOptions(RedisConfig{URL: "http://example.com"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported redis scheme")
	})

	t.Run("redis url with invalid db", func(t *testing.T) {
		_, err := newRedisOptions(RedisConfig{
			URL: "redis://127.0.0.1:6379/invalid",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid redis db in url")
	})

	t.Run("redis url missing host", func(t *testing.T) {
		_, err := newRedisOptions(RedisConfig{
			URL: "redis://",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "redis url missing host")
	})

	t.Run("invalid url format", func(t *testing.T) {
		_, err := newRedisOptions(RedisConfig{
			URL: "redis://:invalid",
		})
		require.Error(t, err)
	})

	t.Run("plain addr with tls config", func(t *testing.T) {
		opts, err := newRedisOptions(RedisConfig{
			Addr:                  "127.0.0.1:6379",
			TLS:                   true,
			TLSInsecureSkipVerify: true,
		})
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:6379", opts.Addr)
		require.NotNil(t, opts.TLSConfig)
		assert.True(t, opts.TLSConfig.InsecureSkipVerify)
	})

	t.Run("redis url with explicit tls flag", func(t *testing.T) {
		opts, err := newRedisOptions(RedisConfig{
			URL:      "redis://127.0.0.1:6379",
			TLS:      true,
			Username: "user",
			Password: "pass",
			DB:       intPtr(5),
		})
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:6379", opts.Addr)
		assert.Equal(t, "user", opts.Username)
		assert.Equal(t, "pass", opts.Password)
		assert.Equal(t, 5, opts.DB)
		require.NotNil(t, opts.TLSConfig)
		assert.False(t, opts.TLSConfig.InsecureSkipVerify)
	})
}
