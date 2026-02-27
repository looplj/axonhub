package conf

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	axonconf "github.com/looplj/axonhub/axon/conf"
	"gopkg.in/yaml.v3"
)

const FileName = "config.yml"

type Config struct {
	BaseURL           string        `yaml:"base_url"`
	APIKey            string        `yaml:"api_key"`
	InstanceID        string        `yaml:"instance_id"`
	Name              string        `yaml:"name"`
	PollInterval      time.Duration `yaml:"poll_interval"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	Debug             bool          `yaml:"debug"`
}

func DefaultConfig() Config {
	return Config{
		PollInterval:      5 * time.Second,
		HeartbeatInterval: 1 * time.Minute,
	}
}

func LoadOrSaveConfig(baseURL, apiKey, instanceID, name string) (Config, error) {
	cfg := DefaultConfig()
	loader := axonconf.NewViperLoader[Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yml",
		SearchPaths:    []string{".axonclaw"},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLAW",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "yaml",
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
	if res.Value.InstanceID != "" {
		cfg.InstanceID = res.Value.InstanceID
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
	if res.Value.Debug {
		cfg.Debug = res.Value.Debug
	}

	finalBaseURL := strings.TrimSpace(baseURL)
	finalAPIKey := strings.TrimSpace(apiKey)
	finalInstanceID := strings.TrimSpace(instanceID)
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
	if finalInstanceID != "" {
		cfg.InstanceID = finalInstanceID
		needSave = true
	}
	if finalName != "" {
		cfg.Name = finalName
		needSave = true
	}

	if cfg.InstanceID == "" {
		cfg.InstanceID = generateInstanceID()
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

	configDir := ".axonclaw"
	path := filepath.Join(configDir, FileName)
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
	configDir := ".axonclaw"
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	path := filepath.Join(configDir, FileName)
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
	if cfg.InstanceID != "" {
		existing.InstanceID = cfg.InstanceID
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

func generateInstanceID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

func ReadYAMLFile(path string) (map[string]any, error) {
	return axonconf.ReadYAMLFile(path)
}

func WriteYAMLFile(path string, data map[string]any) error {
	return axonconf.WriteYAMLFile(path, data)
}

func SetYAMLKey(path string, key string, value string) error {
	return axonconf.SetYAMLKey(path, key, value)
}

func GetYAMLString(path string, key string) (string, bool, error) {
	return axonconf.GetYAMLString(path, key)
}
