package xredis

import (
	"time"
)

type Config struct {
	Addr                  string        `conf:"addr" yaml:"addr" json:"addr"`
	URL                   string        `conf:"url" yaml:"url" json:"url"`
	Username              string        `conf:"username" yaml:"username" json:"username"`
	Password              string        `conf:"password" yaml:"password" json:"password"`
	DB                    *int          `conf:"db" yaml:"db" json:"db"`
	TLS                   bool          `conf:"tls" yaml:"tls" json:"tls"`
	TLSInsecureSkipVerify bool          `conf:"tls_insecure_skip_verify" yaml:"tls_insecure_skip_verify" json:"tls_insecure_skip_verify"`
	Expiration            time.Duration `conf:"expiration" yaml:"expiration" json:"expiration"`
	PoolSize              int           `conf:"pool_size" yaml:"pool_size" json:"pool_size"`
	MinIdleConns          int           `conf:"min_idle_conns" yaml:"min_idle_conns" json:"min_idle_conns"`
	DialTimeout           time.Duration `conf:"dial_timeout" yaml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout           time.Duration `conf:"read_timeout" yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout          time.Duration `conf:"write_timeout" yaml:"write_timeout" json:"write_timeout"`
	PoolTimeout           time.Duration `conf:"pool_timeout" yaml:"pool_timeout" json:"pool_timeout"`
	MaxRetries            int           `conf:"max_retries" yaml:"max_retries" json:"max_retries"`
}
