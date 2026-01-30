package xcache

import (
	"time"

	"github.com/looplj/axonhub/internal/pkg/xredis"
)

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
	Mode   string        `conf:"mode" yaml:"mode" json:"mode"`
	Memory MemoryConfig  `conf:"memory" yaml:"memory" json:"memory"`
	Redis  xredis.Config `conf:"redis" yaml:"redis" json:"redis"`
}

type MemoryConfig struct {
	Expiration      time.Duration `conf:"expiration" yaml:"expiration" json:"expiration"`
	CleanupInterval time.Duration `conf:"cleanup_interval" yaml:"cleanup_interval" json:"cleanup_interval"`
}
