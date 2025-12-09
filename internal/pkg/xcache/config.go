package xcache

import "time"

// Mode represents the cache backend mode
//   - memory: pure in-memory
//   - redis: pure redis
//   - two-level: memory + redis chain
const (
	ModeMemory   = "memory"
	ModeRedis    = "redis"
	ModeTwoLevel = "two-level"
)

type Config struct {
	Mode   string       `conf:"mode" yaml:"mode" json:"mode"`
	Memory MemoryConfig `conf:"memory" yaml:"memory" json:"memory"`
	Redis  RedisConfig  `conf:"redis" yaml:"redis" json:"redis"`
}

type MemoryConfig struct {
	Expiration      time.Duration `conf:"expiration" yaml:"expiration" json:"expiration"`
	CleanupInterval time.Duration `conf:"cleanup_interval" yaml:"cleanup_interval" json:"cleanup_interval"`
}

type RedisConfig struct {
	Addr                  string        `conf:"addr" yaml:"addr" json:"addr"`
	URL                   string        `conf:"url" yaml:"url" json:"url"`
	Username              string        `conf:"username" yaml:"username" json:"username"`
	Password              string        `conf:"password" yaml:"password" json:"password"`
	DB                    *int          `conf:"db" yaml:"db" json:"db"`
	TLS                   bool          `conf:"tls" yaml:"tls" json:"tls"`
	TLSInsecureSkipVerify bool          `conf:"tls_insecure_skip_verify" yaml:"tls_insecure_skip_verify" json:"tls_insecure_skip_verify"`
	Expiration            time.Duration `conf:"expiration" yaml:"expiration" json:"expiration"`
}
