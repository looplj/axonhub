package conf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	axonconf "github.com/looplj/axonhub/axon/conf"
)

const (
	FileName = "config.yml"
)

type Config struct {
	BaseURL         string `conf:"base_url" yaml:"base_url"`
	APIKey          string `conf:"api_key" yaml:"api_key"`
	Model           string `conf:"model" yaml:"model"`
	TraceHeader     string `conf:"trace_header" yaml:"trace_header"`
	ThreadHeader    string `conf:"thread_header" yaml:"thread_header"`
	ReasoningEffort string `conf:"reasoning_effort" yaml:"reasoning_effort"`
}

func LoadEffectiveConfig(configDir string) (Config, error) {
	loader := axonconf.NewViperLoader[Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yml",
		SearchPaths:    []string{configDir, "."},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLI",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "conf",
	})
	res, err := loader.Load(context.Background())
	if err != nil {
		return Config{}, err
	}
	return res.Value, nil
}

func ValidateConfig(cfg Config, configDir string) error {
	var missing []string
	if cfg.BaseURL == "" {
		missing = append(missing, "base_url")
	}
	if cfg.APIKey == "" {
		missing = append(missing, "api_key")
	}
	if cfg.Model == "" {
		missing = append(missing, "model")
	}
	if len(missing) == 0 {
		return nil
	}

	path := filepath.Join(configDir, FileName)
	example := "base_url: http://localhost:8090\napi_key: your-api-key\nmodel: deepseek-chat\n"
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("missing required settings: %s (create %s or set env vars AXONCLI_BASE_URL/AXONCLI_API_KEY/AXONCLI_MODEL)\n\n%s", strings.Join(missing, ", "), path, example)
	}
	return fmt.Errorf("missing required settings: %s (config: %s)", strings.Join(missing, ", "), path)
}

func FindConfigFile(configDir string) (string, error) {
	loader := axonconf.NewViperLoader[Config](axonconf.ViperLoaderOptions{
		ConfigName:      "config",
		ConfigType:      "yml",
		SearchPaths:     []string{configDir, "."},
		AllowMissing:    true,
		DefaultFileName: FileName,
	})
	return loader.DetectConfigFile()
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
