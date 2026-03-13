package conf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	SecureDir  = ".axonclaw"
)

type Config struct {
	BaseURL                string        `yml:"base_url"`
	APIKey                 string        `yml:"api_key"`
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

func LoadOrSaveConfig(baseURL, apiKey string) (Config, error) {
	cfg := DefaultConfig()

	searchPath, err := ConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve config directory: %w", err)
	}
	loader := axonconf.NewViperLoader[Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yml",
		SearchPaths:    []string{searchPath},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLAW",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "yml",
		SetDefaults: func(v *viper.Viper) {
			v.SetDefault("base_url", "")
			v.SetDefault("api_key", "")
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

	needSave := false
	if finalBaseURL != "" {
		cfg.BaseURL = finalBaseURL
		needSave = true
	}
	if finalAPIKey != "" {
		cfg.APIKey = finalAPIKey
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

	path := DefaultPath()
	example := strings.TrimSpace(`
base_url: https://your-axonhub-server.com
api_key: your-agent-api-key
`) + "\n"

	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return fmt.Errorf(
			"missing required settings: %s (set them with 'axonclaw conf set', or pass flags -base-url/-api-key)\n\n%s",
			strings.Join(missing, ", "),
			example,
		)
	}

	return fmt.Errorf("missing required settings: %s (update them with 'axonclaw conf set' or flags -base-url/-api-key)", strings.Join(missing, ", "))
}

func SaveConfig(cfg Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return fmt.Errorf("resolve config directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	path := filepath.Join(dir, FileName)
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

	data, err := yaml.Marshal(&existing)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func DefaultPath() string {
	dir, err := ConfigDir()
	if err != nil {
		return filepath.Join(DefaultDir, FileName)
	}

	return filepath.Join(dir, FileName)
}

func LoadConfig() (Config, error) {
	searchPath, err := ConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve config directory: %w", err)
	}
	loader := axonconf.NewViperLoader[Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yml",
		SearchPaths:    []string{searchPath},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLAW",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "yml",
		SetDefaults: func(v *viper.Viper) {
			v.SetDefault("base_url", "")
			v.SetDefault("api_key", "")
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

func ConfigDir() (string, error) {
	return secureWorkspaceDir()
}

func PolicyDir() (string, error) {
	return secureWorkspaceDir()
}

func PermissionDir() (string, error) {
	return mustSecureDirFallback(), nil
}

func secureWorkspaceDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, SecureDir, workspaceHash(wd)), nil
}

func mustSecureDirFallback() string {
	dir, err := secureWorkspaceDir()
	if err != nil {
		return filepath.Join(DefaultDir)
	}

	return dir
}

func workspaceHash(wd string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(wd)))
	return hex.EncodeToString(sum[:])[:32]
}
