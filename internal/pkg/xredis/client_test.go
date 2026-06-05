package xredis

import (
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestNewUniversalOptions(t *testing.T) {
	t.Run("plain addr with tls flag", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			Addr: "127.0.0.1:6379",
			TLS:  true,
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:6379"}, opts.Addrs)
		assert.NotNil(t, opts.TLSConfig)
	})

	t.Run("invalid url scheme", func(t *testing.T) {
		_, err := newUniversalOptions(Config{
			URL: "http://127.0.0.1:6379",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported redis scheme")
	})

	t.Run("valid redis url", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL: "redis://user:pass@127.0.0.1:6379/1",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:6379"}, opts.Addrs)
		assert.Equal(t, "user", opts.Username)
		assert.Equal(t, "pass", opts.Password)
		assert.Equal(t, 1, opts.DB)
	})

	t.Run("valid rediss url", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL: "rediss://127.0.0.1:6379",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:6379"}, opts.Addrs)
		assert.NotNil(t, opts.TLSConfig)
	})

	t.Run("override url credentials", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL:      "redis://user:pass@127.0.0.1:6379/1",
			Username: "newuser",
			Password: "newpassword",
			DB:       lo.ToPtr(2),
		})
		assert.NoError(t, err)
		assert.Equal(t, "newuser", opts.Username)
		assert.Equal(t, "newpassword", opts.Password)
		assert.Equal(t, 2, opts.DB)
	})

	t.Run("config overrides url db to 0", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL: "redis://127.0.0.1:6379/1",
			DB:  lo.ToPtr(0),
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:6379"}, opts.Addrs)
		assert.Equal(t, 0, opts.DB)
	})

	t.Run("redis url without credentials", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL: "redis://127.0.0.1:6379",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:6379"}, opts.Addrs)
		assert.Empty(t, opts.Username)
		assert.Empty(t, opts.Password)
		assert.Equal(t, 0, opts.DB)
	})

	t.Run("plain addr without tls", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			Addr: "127.0.0.1:6379",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:6379"}, opts.Addrs)
		assert.Nil(t, opts.TLSConfig)
	})

	t.Run("tls_insecure_skip_verify requires tls", func(t *testing.T) {
		_, err := newUniversalOptions(Config{
			Addr:                  "127.0.0.1:6379",
			TLSInsecureSkipVerify: true,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires TLS to be enabled")
	})

	t.Run("empty addr and url", func(t *testing.T) {
		_, err := newUniversalOptions(Config{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis addr or url is required")
	})

	t.Run("whitespace only addr", func(t *testing.T) {
		_, err := newUniversalOptions(Config{
			Addr: "   ",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis addr or url is required")
	})

	t.Run("invalid scheme", func(t *testing.T) {
		_, err := newUniversalOptions(Config{URL: "http://example.com"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported redis scheme")
	})

	t.Run("redis url with invalid db", func(t *testing.T) {
		_, err := newUniversalOptions(Config{
			URL: "redis://127.0.0.1:6379/invalid",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid redis db in url")
	})

	t.Run("redis url missing host", func(t *testing.T) {
		_, err := newUniversalOptions(Config{
			URL: "redis://",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis addr or url is required")
	})

	t.Run("invalid url format", func(t *testing.T) {
		_, err := newUniversalOptions(Config{
			URL: "redis://:invalid",
		})
		assert.Error(t, err)
	})

	t.Run("explicit tls_insecure_skip_verify", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			Addr:                  "127.0.0.1:6379",
			TLS:                   true,
			TLSInsecureSkipVerify: true,
		})
		assert.NoError(t, err)
		assert.True(t, opts.TLSConfig.InsecureSkipVerify)
	})

	t.Run("redis url with explicit tls flag", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL:      "redis://127.0.0.1:6379",
			TLS:      true,
			Username: "user",
			Password: "pass",
			DB:       lo.ToPtr(5),
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:6379"}, opts.Addrs)
		assert.Equal(t, "user", opts.Username)
		assert.Equal(t, "pass", opts.Password)
		assert.Equal(t, 5, opts.DB)
		assert.NotNil(t, opts.TLSConfig)
		assert.False(t, opts.TLSConfig.InsecureSkipVerify)
	})

	t.Run("redis addr override url host", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL:  "redis://127.0.0.1:6379",
			Addr: "127.0.0.1:7379",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:7379"}, opts.Addrs)
	})

	t.Run("redis addrs override addr", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			Addr:  "127.0.0.1:6379",
			Addrs: []string{"127.0.0.1:26379", "127.0.0.1:26380", "127.0.0.1:26381"},
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:26379", "127.0.0.1:26380", "127.0.0.1:26381"}, opts.Addrs)
	})

	t.Run("redis sentinel mode with url", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL: "redis://?sentinel_username=sentinel_user&sentinel_password=sentinel_pass&" +
				"username=user&password=pass&" +
				"master_name=mymaster&addrs=127.0.0.1:26379&addrs=127.0.0.1:26380&addrs=127.0.0.1:26381",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:26379", "127.0.0.1:26380", "127.0.0.1:26381"}, opts.Addrs)
		assert.Equal(t, "user", opts.Username)
		assert.Equal(t, "pass", opts.Password)
		assert.Equal(t, "sentinel_user", opts.SentinelUsername)
		assert.Equal(t, "sentinel_pass", opts.SentinelPassword)
		assert.Equal(t, "mymaster", opts.MasterName)
	})

	t.Run("redis sentinel mode with config", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			Addrs:            []string{"127.0.0.1:26379", "127.0.0.1:26380", "127.0.0.1:26381"},
			Username:         "user",
			Password:         "pass",
			SentinelUsername: "sentinel_user",
			SentinelPassword: "sentinel_pass",
			MasterName:       "mymaster",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:26379", "127.0.0.1:26380", "127.0.0.1:26381"}, opts.Addrs)
		assert.Equal(t, "user", opts.Username)
		assert.Equal(t, "pass", opts.Password)
		assert.Equal(t, "sentinel_user", opts.SentinelUsername)
		assert.Equal(t, "sentinel_pass", opts.SentinelPassword)
		assert.Equal(t, "mymaster", opts.MasterName)
	})

	t.Run("redis sentinel mode config override url", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL: "redis://?sentinel_username=sentinel_user&sentinel_password=sentinel_pass&" +
				"username=user&password=pass&" +
				"master_name=mymaster&addrs=127.0.0.1:26379&addrs=127.0.0.1:26380&addrs=127.0.0.1:26381",
			Addrs:            []string{"127.0.0.1:36379", "127.0.0.1:36380", "127.0.0.1:36381"},
			Username:         "new_user",
			Password:         "new_pass",
			SentinelUsername: "new_sentinel_user",
			SentinelPassword: "new_sentinel_pass",
			MasterName:       "new_mymaster",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:36379", "127.0.0.1:36380", "127.0.0.1:36381"}, opts.Addrs)
		assert.Equal(t, "new_user", opts.Username)
		assert.Equal(t, "new_pass", opts.Password)
		assert.Equal(t, "new_sentinel_user", opts.SentinelUsername)
		assert.Equal(t, "new_sentinel_pass", opts.SentinelPassword)
		assert.Equal(t, "new_mymaster", opts.MasterName)
	})

	t.Run("redis cluster mode with url", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL: "redis://?is_cluster_mode=true&username=user&password=pass&" +
				"addrs=127.0.0.1:7001&addrs=127.0.0.1:7002&addrs=127.0.0.1:7003",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:7001", "127.0.0.1:7002", "127.0.0.1:7003"}, opts.Addrs)
		assert.True(t, opts.IsClusterMode)
	})

	t.Run("redis cluster mode with config", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			Addrs:         []string{"127.0.0.1:7001", "127.0.0.1:7002", "127.0.0.1:7003"},
			Username:      "user",
			Password:      "pass",
			IsClusterMode: lo.ToPtr(true),
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:7001", "127.0.0.1:7002", "127.0.0.1:7003"}, opts.Addrs)
		assert.True(t, opts.IsClusterMode)
	})

	t.Run("redis cluster mode config override url", func(t *testing.T) {
		opts, err := newUniversalOptions(Config{
			URL: "redis://?is_cluster_mode=false&username=user&password=pass&" +
				"addrs=127.0.0.1:7001&addrs=127.0.0.1:7002&addrs=127.0.0.1:7003",
			Addrs:         []string{"127.0.0.1:8001", "127.0.0.1:8002", "127.0.0.1:8003"},
			Username:      "new_user",
			Password:      "new_pass",
			IsClusterMode: lo.ToPtr(true),
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"127.0.0.1:8001", "127.0.0.1:8002", "127.0.0.1:8003"}, opts.Addrs)
		assert.True(t, opts.IsClusterMode)
	})
}

func TestParseUniversalURL(t *testing.T) {
	t.Run("parse the empty URL", func(t *testing.T) {
		opts, err := ParseUniversalURL("")
		assert.NoError(t, err)
		assert.Equal(t, redis.UniversalOptions{}, *opts)
	})
	t.Run("parse the short URL", func(t *testing.T) {
		opts, err := ParseUniversalURL("redis://")
		assert.NoError(t, err)
		assert.Equal(t, redis.UniversalOptions{}, *opts)
	})
	t.Run("parse the full URL", func(t *testing.T) {
		opts, err := ParseUniversalURL(
			"rediss://user:pass@127.0.0.1:6379/1?" + strings.Join([]string{
				"client_name=axonhub_client", "db=2", "protocol=2",
				"username=user1", "password=pass1",
				"sentinel_username=sentinel_user", "sentinel_password=sentinel_pass",
				"max_retries=3", "min_retry_backoff=8ms", "max_retry_backoff=512ms",
				"dial_timeout=5s", "read_timeout=3s", "write_timeout=3s",
				"context_timeout_enabled=false", "read_buffer_size=1024", "write_buffer_size=1024",
				"pool_fifo=false", "pool_size=100", "pool_timeout=4s",
				"min_idle_conns=5", "max_idle_conns=10", "max_active_conns=30",
				"conn_max_lifetime=5m", "conn_max_idle_time=30s",
				"max_redirects=8", "read_only=false", "route_by_latency=true", "route_randomly=false",
				"master_name=mymaster", "disable_identity=false", "identity_suffix=axonhub",
				"failing_timeout_seconds=15", "unstable_resp3=false", "is_cluster_mode=true",
				"addrs=127.0.0.1:7001", "addrs=127.0.0.1:7002", "addrs=127.0.0.1:7003",
				"tls_insecure_skip_verify=true",
			}, "&"))
		assert.NoError(t, err)
		assert.Equal(t, "axonhub_client", opts.ClientName)
		assert.Equal(t, 2, opts.DB)
		assert.Equal(t, 2, opts.Protocol)
		assert.Equal(t, "user1", opts.Username)
		assert.Equal(t, "pass1", opts.Password)
		assert.Equal(t, "sentinel_user", opts.SentinelUsername)
		assert.Equal(t, "sentinel_pass", opts.SentinelPassword)
		assert.Equal(t, 3, opts.MaxRetries)
		assert.Equal(t, 8*time.Millisecond, opts.MinRetryBackoff)
		assert.Equal(t, 512*time.Millisecond, opts.MaxRetryBackoff)
		assert.Equal(t, 5*time.Second, opts.DialTimeout)
		assert.Equal(t, 3*time.Second, opts.ReadTimeout)
		assert.Equal(t, 3*time.Second, opts.WriteTimeout)
		assert.False(t, opts.ContextTimeoutEnabled)
		assert.Equal(t, 1024, opts.ReadBufferSize)
		assert.Equal(t, 1024, opts.WriteBufferSize)
		assert.False(t, opts.PoolFIFO)
		assert.Equal(t, 100, opts.PoolSize)
		assert.Equal(t, 4*time.Second, opts.PoolTimeout)
		assert.Equal(t, 5, opts.MinIdleConns)
		assert.Equal(t, 10, opts.MaxIdleConns)
		assert.Equal(t, 30, opts.MaxActiveConns)
		assert.Equal(t, 5*time.Minute, opts.ConnMaxLifetime)
		assert.Equal(t, 30*time.Second, opts.ConnMaxIdleTime)
		assert.Equal(t, 8, opts.MaxRedirects)
		assert.False(t, opts.ReadOnly)
		assert.True(t, opts.RouteByLatency)
		assert.False(t, opts.RouteRandomly)
		assert.Equal(t, "mymaster", opts.MasterName)
		assert.False(t, opts.DisableIdentity)
		assert.Equal(t, "axonhub", opts.IdentitySuffix)
		assert.Equal(t, 15, opts.FailingTimeoutSeconds)
		assert.False(t, opts.UnstableResp3)
		assert.True(t, opts.IsClusterMode)
		assert.Equal(t, []string{"127.0.0.1:7001", "127.0.0.1:7002", "127.0.0.1:7003"}, opts.Addrs)
		assert.NotNil(t, opts.TLSConfig)
		assert.True(t, opts.TLSConfig.InsecureSkipVerify)
	})
}
