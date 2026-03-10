package conf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	axonconf "github.com/looplj/axonhub/axon/conf"
)

const (
	FileName   = "config.yml"
	DefaultDir = ".axonclaw"
)

type Config struct {
	BaseURL                string        `yml:"base_url"`
	APIKey                 string        `yml:"api_key"`
	Name                   string        `yml:"name"`
	PollInterval           time.Duration `yml:"poll_interval"`
	HeartbeatInterval      time.Duration `yml:"heartbeat_interval"`
	AutoSyncConfig         bool          `yml:"auto_sync_config"`
	AutoSyncConfigInterval time.Duration `yml:"auto_sync_config_interval"`
	ContextRecentMessages  int           `yml:"context_recent_messages"`
	ContextSoftTokenLimit  int           `yml:"context_soft_token_limit"`
	ContextSummaryMaxChars int           `yml:"context_summary_max_chars"`
	Debug                  bool          `yml:"debug"`
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

func LoadOrSaveConfig(baseURL, apiKey, name string) (Config, error) {
	cfg := DefaultConfig()
	loader := axonconf.NewViperLoader[Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yml",
		SearchPaths:    []string{DefaultDir},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLAW",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "yml",
		SetDefaults: func(v *viper.Viper) {
			v.SetDefault("base_url", "")
			v.SetDefault("api_key", "")
			v.SetDefault("name", "")
			v.SetDefault("poll_interval", "5s")
			v.SetDefault("heartbeat_interval", "1m")
			v.SetDefault("auto_sync_config", false)
			v.SetDefault("auto_sync_config_interval", "5m")
			v.SetDefault("context_recent_messages", 80)
			v.SetDefault("context_soft_token_limit", 120000)
			v.SetDefault("context_summary_max_chars", 16000)
			v.SetDefault("debug", false)
		},
	})
	res, err := loader.Load(context.Background())
	if err != nil {
		return Config{}, err
	}
	if res.Value.BaseURL != "" {
		cfg.BaseURL = res.Value.BaseURL
	}
	if res.Value.APIKey != "" {
		cfg.APIKey = res.Value.APIKey
	}
	if res.Value.Name != "" {
		cfg.Name = res.Value.Name
	}
	if res.Value.PollInterval > 0 {
		cfg.PollInterval = res.Value.PollInterval
	}
	if res.Value.HeartbeatInterval > 0 {
		cfg.HeartbeatInterval = res.Value.HeartbeatInterval
	}
	if res.Value.AutoSyncConfig {
		cfg.AutoSyncConfig = res.Value.AutoSyncConfig
	}
	if res.Value.AutoSyncConfigInterval > 0 {
		cfg.AutoSyncConfigInterval = res.Value.AutoSyncConfigInterval
	}

	if res.Value.ContextRecentMessages > 0 {
		cfg.ContextRecentMessages = res.Value.ContextRecentMessages
	}
	if res.Value.ContextSoftTokenLimit > 0 {
		cfg.ContextSoftTokenLimit = res.Value.ContextSoftTokenLimit
	}
	if res.Value.ContextSummaryMaxChars > 0 {
		cfg.ContextSummaryMaxChars = res.Value.ContextSummaryMaxChars
	}
	if res.Value.Debug {
		cfg.Debug = res.Value.Debug
	}

	finalBaseURL := strings.TrimSpace(baseURL)
	finalAPIKey := strings.TrimSpace(apiKey)
	finalName := strings.TrimSpace(name)

	needSave := false
	if finalBaseURL != "" {
		cfg.BaseURL = finalBaseURL
		needSave = true
	}
	if finalAPIKey != "" {
		cfg.APIKey = finalAPIKey
		needSave = true
	}
	if finalName != "" {
		cfg.Name = finalName
		needSave = true
	}

	if needSave {
		if err := SaveConfig(cfg); err != nil {
			return Config{}, fmt.Errorf("save config: %w", err)
		}
	}

	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" {
		return Config{}, newMissingConfigError(cfg.BaseURL, cfg.APIKey)
	}

	return cfg, nil
}

func newMissingConfigError(baseURL, apiKey string) error {
	var missing []string
	if strings.TrimSpace(baseURL) == "" {
		missing = append(missing, "base_url")
	}
	if strings.TrimSpace(apiKey) == "" {
		missing = append(missing, "api_key")
	}

	path := filepath.Join(DefaultDir, FileName)
	example := strings.TrimSpace(`
base_url: https://your-axonhub-server.com
api_key: your-agent-api-key
`) + "\n"

	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return fmt.Errorf(
			"missing required settings: %s (create %s in .axonclaw directory, or pass flags -base-url/-api-key)\n\n%s",
			strings.Join(missing, ", "),
			FileName,
			example,
		)
	}
	return fmt.Errorf("missing required settings: %s (config: %s)", strings.Join(missing, ", "), path)
}

func SaveConfig(cfg Config) error {
	if err := os.MkdirAll(DefaultDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	path := filepath.Join(DefaultDir, FileName)
	var existing Config
	if data, err := os.ReadFile(path); err == nil {
		yaml.Unmarshal(data, &existing)
	}

	if cfg.BaseURL != "" {
		existing.BaseURL = cfg.BaseURL
	}
	if cfg.APIKey != "" {
		existing.APIKey = cfg.APIKey
	}
	if cfg.Name != "" {
		existing.Name = cfg.Name
	}

	data, err := yaml.Marshal(&existing)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func DefaultPath() string {
	return filepath.Join(DefaultDir, FileName)
}

func LoadConfig() (Config, error) {
	loader := axonconf.NewViperLoader[Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yml",
		SearchPaths:    []string{DefaultDir},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLAW",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "yml",
		SetDefaults: func(v *viper.Viper) {
			v.SetDefault("base_url", "")
			v.SetDefault("api_key", "")
			v.SetDefault("name", "")
			v.SetDefault("poll_interval", "5s")
			v.SetDefault("heartbeat_interval", "1m")
			v.SetDefault("auto_sync_config", false)
			v.SetDefault("auto_sync_config_interval", "5m")
			v.SetDefault("enable_context_manager", false)
			v.SetDefault("context_recent_messages", 80)
			v.SetDefault("context_soft_token_limit", 120000)
			v.SetDefault("context_summary_max_chars", 16000)
			v.SetDefault("debug", false)
		},
	})
	res, err := loader.Load(context.Background())
	if err != nil {
		return Config{}, err
	}
	return res.Value, nil
}

func GetYAMLString(key string) (string, bool, error) {
	return axonconf.GetYAMLString(DefaultPath(), key)
}

func SetYAMLKey(key string, value string) error {
	return axonconf.SetYAMLKey(DefaultPath(), key, value)
}
