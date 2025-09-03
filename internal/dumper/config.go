package dumper

import (
	"time"
)

// Config holds the configuration for the dumper.
type Config struct {
	// Enabled enables or disables the dumper.
	Enabled bool `conf:"enabled" yaml:"enabled" json:"enabled"`

	// DumpPath is the directory path where data will be dumped.
	DumpPath string `conf:"dump_path" yaml:"dump_path" json:"dump_path"`

	// MaxSize is the maximum size of dump files in MB.
	MaxSize int `conf:"max_size" yaml:"max_size" json:"max_size"`

	// MaxAge is the maximum age of dump files to keep.
	MaxAge time.Duration `conf:"max_age" yaml:"max_age" json:"max_age"`

	// MaxBackups is the maximum number of old dump files to retain.
	MaxBackups int `conf:"max_backups" yaml:"max_backups" json:"max_backups"`
}

// DefaultConfig returns the default configuration for the dumper.
func DefaultConfig() Config {
	return Config{
		Enabled:    false,
		DumpPath:   "./dumps",
		MaxSize:    100, // 100 MB
		MaxAge:     24 * time.Hour,
		MaxBackups: 10,
	}
}
