package conf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	axonconf "github.com/looplj/axonhub/axon/conf"

	"github.com/looplj/axonhub/cmd/axonclaw/claw"
)

const (
	FileName              = "config.yaml"
	builtinSkillsFileName = "builtin_skills.yaml"
	DefaultDir            = ".axonclaw"
	runtimeDirName        = "axonclaw"
)

type BuiltinSkill = claw.BuiltinSkill

func LoadOrSaveConfig() (claw.Config, error) {
	cfg := claw.DefaultConfig()

	searchPath, err := ConfigDir()
	if err != nil {
		return claw.Config{}, fmt.Errorf("resolve config directory: %w", err)
	}

	loader := axonconf.NewViperLoader[claw.Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yaml",
		SearchPaths:    []string{searchPath},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLAW",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "yaml",
		SetDefaults: func(v *viper.Viper) {
			v.SetDefault("base_url", "")
			v.SetDefault("api_key", "")
			v.SetDefault("poll_interval", "5s")
			v.SetDefault("heartbeat_interval", "1m")
			v.SetDefault("auto_sync_config", false)
			v.SetDefault("auto_sync_config_interval", "5m")
			v.SetDefault("context_token_limit", 120000)
			v.SetDefault("context_summary_max_chars", 16000)
			v.SetDefault("debug", false)
		},
	})
	res, err := loader.Load(context.Background())
	if err != nil {
		return claw.Config{}, err
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

	if res.Value.ContextTokenLimit > 0 {
		cfg.ContextTokenLimit = res.Value.ContextTokenLimit
	}
	if res.Value.ContextSummaryMaxChars > 0 {
		cfg.ContextSummaryMaxChars = res.Value.ContextSummaryMaxChars
	}
	if res.Value.Debug {
		cfg.Debug = res.Value.Debug
	}

	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.APIKey) == "" {
		return claw.Config{}, newMissingConfigError(cfg.BaseURL, cfg.APIKey)
	}

	// Save config if it's valid (from environment variables or merged sources)
	if err := SaveConfig(cfg); err != nil {
		return claw.Config{}, fmt.Errorf("save config: %w", err)
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
			"missing required settings: %s (set them with 'axonclaw conf set', or use environment variables AXONCLAW_BASE_URL/AXONCLAW_API_KEY)\n\n%s",
			strings.Join(missing, ", "),
			example,
		)
	}

	return fmt.Errorf("missing required settings: %s (update them with 'axonclaw conf set' or use environment variables AXONCLAW_BASE_URL/AXONCLAW_API_KEY)", strings.Join(missing, ", "))
}

func SaveConfig(cfg claw.Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return fmt.Errorf("resolve config directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	existing, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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

	path := filepath.Join(dir, FileName)

	return os.WriteFile(path, data, 0o600)
}

func SaveBuiltinSkills(skills []claw.BuiltinSkill) error {
	dir, err := RuntimeDir()
	if err != nil {
		return fmt.Errorf("resolve runtime directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create runtime directory: %w", err)
	}

	data, err := yaml.Marshal(skills)
	if err != nil {
		return fmt.Errorf("marshal builtin skills: %w", err)
	}

	path := filepath.Join(dir, builtinSkillsFileName)

	return os.WriteFile(path, data, 0o600)
}

func LoadBuiltinSkills() ([]claw.BuiltinSkill, error) {
	dir, err := RuntimeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve runtime directory: %w", err)
	}

	path := filepath.Join(dir, builtinSkillsFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read builtin skills file: %w", err)
	}

	var skills []claw.BuiltinSkill
	if err := yaml.Unmarshal(data, &skills); err != nil {
		return nil, fmt.Errorf("unmarshal builtin skills: %w", err)
	}

	return skills, nil
}

func DefaultPath() string {
	dir, err := ConfigDir()
	if err != nil {
		if base, baseErr := runtimeBaseDir(); baseErr == nil {
			return filepath.Join(base, FileName)
		}

		return filepath.Join(DefaultDir, FileName)
	}

	return filepath.Join(dir, FileName)
}

func LoadConfig() (claw.Config, error) {
	searchPath, err := ConfigDir()
	if err != nil {
		return claw.Config{}, fmt.Errorf("resolve config directory: %w", err)
	}

	loader := axonconf.NewViperLoader[claw.Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yaml",
		SearchPaths:    []string{searchPath},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLAW",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "yaml",
		SetDefaults: func(v *viper.Viper) {
			v.SetDefault("base_url", "")
			v.SetDefault("api_key", "")
			v.SetDefault("poll_interval", "5s")
			v.SetDefault("heartbeat_interval", "1m")
			v.SetDefault("auto_sync_config", false)
			v.SetDefault("auto_sync_config_interval", "5m")
			v.SetDefault("enable_context_manager", false)
			v.SetDefault("context_token_limit", 120000)
			v.SetDefault("context_summary_max_chars", 16000)
			v.SetDefault("debug", false)
		},
	})
	res, err := loader.Load(context.Background())
	if err != nil {
		return claw.Config{}, err
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
	return RuntimeDir()
}

func PolicyDir() (string, error) {
	return RuntimeDir()
}

func PolicyDirForWorkspace(workspace string) (string, error) {
	return RuntimeDirForWorkspace(workspace)
}

func PermissionDirForWorkspace(workspace string) (string, error) {
	return RuntimeDirForWorkspace(workspace)
}

func RuntimeDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	return RuntimeDirForWorkspace(wd)
}

func RuntimeDirForWorkspace(workspace string) (string, error) {
	base, err := runtimeBaseDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, workspaceHash(workspace)), nil
}

func runtimeBaseDir() (string, error) {
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("get user config directory: %w", err)
		}

		return filepath.Join(dir, runtimeDirName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, ".config", runtimeDirName), nil
}

func workspaceHash(wd string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(wd)))
	return hex.EncodeToString(sum[:])[:32]
}
