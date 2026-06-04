package xredis

import (
	"testing"

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
}
