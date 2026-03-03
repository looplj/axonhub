package conf

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

type testConfig struct {
	Name        string        `yaml:"name" mapstructure:"name"`
	Port        int           `yaml:"port" mapstructure:"port"`
	Timeout     time.Duration `yaml:"timeout" mapstructure:"timeout"`
	Interval    time.Duration `yaml:"interval" mapstructure:"interval"`
	Enabled     bool          `yaml:"enabled" mapstructure:"enabled"`
	EmptyDur    time.Duration `yaml:"empty_dur" mapstructure:"empty_dur"`
}

func TestViperLoader_EnvVarParsing(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	configContent := `
name: test-app
port: 8080
timeout: 30s
interval: 5m
enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loader := NewViperLoader[testConfig](ViperLoaderOptions{
		ConfigName:   "config",
		ConfigType:   "yml",
		SearchPaths:  []string{tmpDir},
		AllowMissing: false,
	})

	result, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if result.Value.Name != "test-app" {
		t.Errorf("expected name 'test-app', got '%s'", result.Value.Name)
	}
	if result.Value.Port != 8080 {
		t.Errorf("expected port 8080, got %d", result.Value.Port)
	}
	if result.Value.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", result.Value.Timeout)
	}
	if result.Value.Interval != 5*time.Minute {
		t.Errorf("expected interval 5m, got %v", result.Value.Interval)
	}
	if !result.Value.Enabled {
		t.Errorf("expected enabled true, got %v", result.Value.Enabled)
	}
}

func TestViperLoader_EnvVarOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	configContent := `
name: test-app
port: 8080
timeout: 30s
interval: 5m
enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	os.Setenv("TESTAPP_NAME", "env-override-name")
	os.Setenv("TESTAPP_PORT", "9090")
	os.Setenv("TESTAPP_TIMEOUT", "1h")
	os.Setenv("TESTAPP_INTERVAL", "10m")
	os.Setenv("TESTAPP_ENABLED", "false")
	defer func() {
		os.Unsetenv("TESTAPP_NAME")
		os.Unsetenv("TESTAPP_PORT")
		os.Unsetenv("TESTAPP_TIMEOUT")
		os.Unsetenv("TESTAPP_INTERVAL")
		os.Unsetenv("TESTAPP_ENABLED")
	}()

	loader := NewViperLoader[testConfig](ViperLoaderOptions{
		ConfigName:      "config",
		ConfigType:      "yml",
		SearchPaths:     []string{tmpDir},
		AllowMissing:    false,
		EnvPrefix:       "TESTAPP",
		EnvKeyReplacer:  nil,
		UnmarshalTag:    "yaml",
	})

	result, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if result.Value.Name != "env-override-name" {
		t.Errorf("expected name 'env-override-name' from env, got '%s'", result.Value.Name)
	}
	if result.Value.Port != 9090 {
		t.Errorf("expected port 9090 from env, got %d", result.Value.Port)
	}
	if result.Value.Timeout != 1*time.Hour {
		t.Errorf("expected timeout 1h from env, got %v", result.Value.Timeout)
	}
	if result.Value.Interval != 10*time.Minute {
		t.Errorf("expected interval 10m from env, got %v", result.Value.Interval)
	}
	if result.Value.Enabled {
		t.Errorf("expected enabled false from env, got %v", result.Value.Enabled)
	}
}

func TestViperLoader_EnvVarDurationParsing(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		expected time.Duration
	}{
		{"seconds", "30s", 30 * time.Second},
		{"minutes", "5m", 5 * time.Minute},
		{"hours", "2h", 2 * time.Hour},
		{"combined", "1h30m", 1*time.Hour + 30*time.Minute},
		{"milliseconds", "500ms", 500 * time.Millisecond},
		{"empty string", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yml")

			configContent := `name: test`
			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			envKey := "TESTDUR_TIMEOUT"
			if tt.envVal != "" {
				os.Setenv(envKey, tt.envVal)
			} else {
				os.Unsetenv(envKey)
			}
			defer os.Unsetenv(envKey)

			loader := NewViperLoader[testConfig](ViperLoaderOptions{
				ConfigName:   "config",
				ConfigType:   "yml",
				SearchPaths:  []string{tmpDir},
				AllowMissing: false,
				EnvPrefix:    "TESTDUR",
				SetDefaults: func(v *viper.Viper) {
					v.SetDefault("timeout", 0)
				},
			})

			result, err := loader.Load(context.Background())
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}

			if result.Value.Timeout != tt.expected {
				t.Errorf("expected timeout %v, got %v", tt.expected, result.Value.Timeout)
			}
		})
	}
}

func TestViperLoader_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	os.Setenv("NOCONFIG_NAME", "env-only-name")
	os.Setenv("NOCONFIG_TIMEOUT", "45s")
	defer func() {
		os.Unsetenv("NOCONFIG_NAME")
		os.Unsetenv("NOCONFIG_TIMEOUT")
	}()

	loader := NewViperLoader[testConfig](ViperLoaderOptions{
		ConfigName:   "config",
		ConfigType:   "yml",
		SearchPaths:  []string{tmpDir},
		AllowMissing: true,
		EnvPrefix:    "NOCONFIG",
		SetDefaults: func(v *viper.Viper) {
			v.SetDefault("name", "")
			v.SetDefault("timeout", 0)
		},
	})

	result, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if result.Value.Name != "env-only-name" {
		t.Errorf("expected name 'env-only-name' from env, got '%s'", result.Value.Name)
	}
	if result.Value.Timeout != 45*time.Second {
		t.Errorf("expected timeout 45s from env, got %v", result.Value.Timeout)
	}
}

func TestViperLoader_AllowMissingFalse(t *testing.T) {
	tmpDir := t.TempDir()

	loader := NewViperLoader[testConfig](ViperLoaderOptions{
		ConfigName:   "nonexistent",
		ConfigType:   "yml",
		SearchPaths:  []string{tmpDir},
		AllowMissing: false,
	})

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Error("expected error when config file not found and AllowMissing is false")
	}
}
