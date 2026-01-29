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
}
