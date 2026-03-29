package claw

import (
	"time"

	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
)

type BuiltinSkill = bootstrap.BuiltinSkill

type Config struct {
	BaseURL string `yaml:"base_url"`
	//nolint:gosec // Checked.
	APIKey                 string        `yaml:"api_key"`
	PollInterval           time.Duration `yaml:"poll_interval"`
	HeartbeatInterval      time.Duration `yaml:"heartbeat_interval"`
	AutoSyncConfig         bool          `yaml:"auto_sync_config"`
	AutoSyncConfigInterval time.Duration `yaml:"auto_sync_config_interval"`
	ContextRecentMessages  int           `yaml:"context_recent_messages"`
	ContextSoftTokenLimit  int           `yaml:"context_soft_token_limit"`
	ContextSummaryMaxChars int           `yaml:"context_summary_max_chars"`
	Debug                  bool          `yaml:"debug"`
}

func DefaultConfig() Config {
	return Config{
		PollInterval:           5 * time.Second,
		HeartbeatInterval:      1 * time.Minute,
		AutoSyncConfigInterval: 5 * time.Minute,
		ContextRecentMessages:  80,
		ContextSoftTokenLimit:  120000,
		ContextSummaryMaxChars: 16000,
	}
}
