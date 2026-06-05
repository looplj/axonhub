package xredis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewClient(cfg Config) (redis.UniversalClient, error) {
	opts, err := newUniversalOptions(cfg)
	if err != nil {
		return nil, err
	}
	client := redis.NewUniversalClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

func newUniversalOptions(cfg Config) (*redis.UniversalOptions, error) {
	opts, err := ParseUniversalURL(cfg.URL)
	if err != nil {
		return nil, err
	}

	// Config fields override when explicitly set
	if strings.TrimSpace(cfg.Addr) != "" {
		opts.Addrs = []string{strings.TrimSpace(cfg.Addr)}
	}
	if len(cfg.Addrs) != 0 {
		opts.Addrs = cfg.Addrs
	}
	if len(opts.Addrs) == 0 {
		return nil, errors.New("redis addr or url is required")
	}

	if cfg.Username != "" {
		opts.Username = cfg.Username
	}

	if cfg.Password != "" {
		opts.Password = cfg.Password
	}

	if cfg.MasterName != "" {
		opts.MasterName = cfg.MasterName
	}

	if cfg.SentinelUsername != "" {
		opts.SentinelUsername = cfg.SentinelUsername
	}

	if cfg.SentinelPassword != "" {
		opts.SentinelPassword = cfg.SentinelPassword
	}

	if cfg.RouteByLatency != nil {
		opts.RouteByLatency = *cfg.RouteByLatency
	}

	if cfg.RouteRandomly != nil {
		opts.RouteRandomly = *cfg.RouteRandomly
	}

	if cfg.IsClusterMode != nil {
		opts.IsClusterMode = *cfg.IsClusterMode
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

func ParseUniversalURL(redisURL string) (*redis.UniversalOptions, error) {
	opts := &redis.UniversalOptions{}

	if redisURL == "" {
		return opts, nil
	}

	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	switch u.Scheme {
	case "redis", "rediss":
	default:
		return nil, fmt.Errorf("unsupported redis scheme: %s (expected redis:// or rediss://)", u.Scheme)
	}

	if u.Host != "" {
		h, p := getHostPortWithDefaults(u)
		opts.Addrs = []string{net.JoinHostPort(h, p)}
	}

	opts.Username, opts.Password = getUserPassword(u)

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
			MinVersion: tls.VersionTLS12,
		}
	}

	q := queryOptions{q: u.Query()}

	opts.ClientName = q.string("client_name")
	if q.has("db") {
		opts.DB = q.int("db")
	}
	opts.Protocol = q.int("protocol")
	if q.has("username") {
		opts.Username = q.string("username")
	}
	if q.has("password") {
		opts.Password = q.string("password")
	}
	opts.SentinelUsername = q.string("sentinel_username")
	opts.SentinelPassword = q.string("sentinel_password")
	opts.MaxRetries = q.int("max_retries")
	opts.MinRetryBackoff = q.duration("min_retry_backoff")
	opts.MaxRetryBackoff = q.duration("max_retry_backoff")
	opts.DialTimeout = q.duration("dial_timeout")
	opts.ReadTimeout = q.duration("read_timeout")
	opts.WriteTimeout = q.duration("write_timeout")
	opts.ContextTimeoutEnabled = q.bool("context_timeout_enabled")
	opts.ReadBufferSize = q.int("read_buffer_size")
	opts.WriteBufferSize = q.int("write_buffer_size")
	opts.PoolFIFO = q.bool("pool_fifo")
	opts.PoolSize = q.int("pool_size")
	opts.PoolTimeout = q.duration("pool_timeout")
	opts.MinIdleConns = q.int("min_idle_conns")
	opts.MaxIdleConns = q.int("max_idle_conns")
	opts.MaxActiveConns = q.int("max_active_conns")
	opts.ConnMaxLifetime = q.duration("conn_max_lifetime")
	opts.ConnMaxIdleTime = q.duration("conn_max_idle_time")
	opts.MaxRedirects = q.int("max_redirects")
	opts.ReadOnly = q.bool("read_only")
	opts.RouteByLatency = q.bool("route_by_latency")
	opts.RouteRandomly = q.bool("route_randomly")
	opts.MasterName = q.string("master_name")
	opts.DisableIdentity = q.bool("disable_identity")
	opts.IdentitySuffix = q.string("identity_suffix")
	opts.FailingTimeoutSeconds = q.int("failing_timeout_seconds")
	opts.UnstableResp3 = q.bool("unstable_resp3")
	opts.IsClusterMode = q.bool("is_cluster_mode")

	if q.has("addrs") {
		addrs := q.strings("addrs")
		opts.Addrs = []string{}
		for _, addr := range addrs {
			h, p, err := net.SplitHostPort(addr)
			if err != nil || h == "" || p == "" {
				return nil, fmt.Errorf("redis: unable to parse addr param: %s", addr)
			}

			opts.Addrs = append(opts.Addrs, net.JoinHostPort(h, p))
		}
	}

	if opts.TLSConfig != nil && q.has("tls_insecure_skip_verify") {
		opts.TLSConfig.InsecureSkipVerify = q.bool("tls_insecure_skip_verify")
	}

	if q.err != nil {
		return nil, q.err
	}

	return opts, nil
}

type queryOptions struct {
	q   url.Values
	err error
}

func (o *queryOptions) has(name string) bool {
	return len(o.q[name]) > 0
}

func (o *queryOptions) string(name string) string {
	vs := o.q[name]
	if len(vs) == 0 {
		return ""
	}
	return vs[len(vs)-1]
}

func (o *queryOptions) strings(name string) []string {
	vs := o.q[name]
	return vs
}

func (o *queryOptions) int(name string) int {
	s := o.string(name)
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err == nil {
		return i
	}
	if o.err == nil {
		o.err = fmt.Errorf("redis: invalid %s number: %w", name, err)
	}
	return 0
}

func (o *queryOptions) duration(name string) time.Duration {
	s := o.string(name)
	if s == "" {
		return 0
	}
	// try plain number first
	if i, err := strconv.Atoi(s); err == nil {
		if i <= 0 {
			// disable timeouts
			return -1
		}
		return time.Duration(i) * time.Second
	}
	dur, err := time.ParseDuration(s)
	if err == nil {
		return dur
	}
	if o.err == nil {
		o.err = fmt.Errorf("redis: invalid %s duration: %w", name, err)
	}
	return 0
}

func (o *queryOptions) bool(name string) bool {
	switch s := o.string(name); s {
	case "true", "1":
		return true
	case "false", "0", "":
		return false
	default:
		if o.err == nil {
			o.err = fmt.Errorf("redis: invalid %s boolean: expected true/false/1/0 or an empty string, got %q", name, s)
		}
		return false
	}
}

func getUserPassword(u *url.URL) (string, string) {
	var user, password string
	if u.User != nil {
		user = u.User.Username()
		if p, ok := u.User.Password(); ok {
			password = p
		}
	}
	return user, password
}

func getHostPortWithDefaults(u *url.URL) (string, string) {
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "6379"
	}
	return host, port
}
