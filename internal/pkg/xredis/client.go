package xredis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

func NewClient(cfg Config) (*redis.Client, error) {
	opts, err := newRedisOptions(cfg)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

func newRedisOptions(cfg Config) (*redis.Options, error) {
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
