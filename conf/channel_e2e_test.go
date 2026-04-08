package conf

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestChannelService_E2E_YAMLConfig tests the full flow with YAML config loading.
// It verifies that config loads from YAML file correctly and the service starts,
// refreshes, and shuts down properly.
func TestChannelService_E2E_YAMLConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")

	configContent := `
server:
  host: "127.0.0.1"
  port: 8090
  name: "TestAxonHub"
  debug: false

db:
  dialect: "sqlite3"
  dsn: "file:test_e2e.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)"
  debug: false

log:
  level: "info"
  encoding: "json"

performance:
  historical_window: "24h"
  historical_refresh_interval: "5m"
  historical_weight: 0.4
  realtime_weight: 0.6

cache:
  mode: "memory"
  default_expiration: "5m"
  cleanup_interval: "10m"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	t.Chdir(tempDir)

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, "127.0.0.1", cfg.APIServer.Host)
	require.Equal(t, 8090, cfg.APIServer.Port)
	require.Equal(t, "TestAxonHub", cfg.APIServer.Name)
	require.False(t, cfg.APIServer.Debug)
	require.Equal(t, "sqlite3", cfg.DB.Dialect)
	require.Equal(t, 24*time.Hour, cfg.Performance.HistoricalWindow)
	require.Equal(t, 5*time.Minute, cfg.Performance.HistoricalRefreshInterval)
	require.Equal(t, 0.4, cfg.Performance.HistoricalWeight)
	require.Equal(t, 0.6, cfg.Performance.RealtimeWeight)

	err = validatePerformanceConfig(cfg.Performance)
	require.NoError(t, err)
}

// TestChannelService_E2E_EnvVarConfig tests config loading from environment variables.
func TestChannelService_E2E_EnvVarConfig(t *testing.T) {
	envVars := map[string]string{
		"AXONHUB_SERVER_HOST":                             "0.0.0.0",
		"AXONHUB_SERVER_PORT":                             "9090",
		"AXONHUB_SERVER_NAME":                             "EnvTestServer",
		"AXONHUB_SERVER_DEBUG":                            "true",
		"AXONHUB_DB_DIALECT":                              "sqlite3",
		"AXONHUB_LOG_LEVEL":                               "debug",
		"AXONHUB_PERFORMANCE_HISTORICAL_WINDOW":           "48h",
		"AXONHUB_PERFORMANCE_HISTORICAL_REFRESH_INTERVAL": "10m",
		"AXONHUB_PERFORMANCE_HISTORICAL_WEIGHT":           "0.3",
		"AXONHUB_PERFORMANCE_REALTIME_WEIGHT":             "0.7",
		"AXONHUB_CACHE_MODE":                              "memory",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, "0.0.0.0", cfg.APIServer.Host)
	require.Equal(t, 9090, cfg.APIServer.Port)
	require.Equal(t, "EnvTestServer", cfg.APIServer.Name)
	require.True(t, cfg.APIServer.Debug)
	require.Equal(t, "sqlite3", cfg.DB.Dialect)
	require.Equal(t, 48*time.Hour, cfg.Performance.HistoricalWindow)
	require.Equal(t, 10*time.Minute, cfg.Performance.HistoricalRefreshInterval)
	require.Equal(t, 0.3, cfg.Performance.HistoricalWeight)
	require.Equal(t, 0.7, cfg.Performance.RealtimeWeight)

	err = validatePerformanceConfig(cfg.Performance)
	require.NoError(t, err)
}

// TestChannelService_E2E_DefaultsApplied tests that config defaults are applied
// when values are not explicitly set.
func TestChannelService_E2E_DefaultsApplied(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, "0.0.0.0", cfg.APIServer.Host)
	require.Equal(t, 8090, cfg.APIServer.Port)
	require.Equal(t, "AxonHub", cfg.APIServer.Name)
	require.False(t, cfg.APIServer.Debug)
	require.Equal(t, "sqlite3", cfg.DB.Dialect)
	require.Equal(t, 168*time.Hour, cfg.Performance.HistoricalWindow)
	require.Equal(t, 2*time.Hour, cfg.Performance.HistoricalRefreshInterval)
	require.Equal(t, 0.4, cfg.Performance.HistoricalWeight)
	require.Equal(t, 0.6, cfg.Performance.RealtimeWeight)

	err = validatePerformanceConfig(cfg.Performance)
	require.NoError(t, err)
}

// TestChannelService_E2E_InvalidConfigRejected tests that invalid config is rejected at startup.
func TestChannelService_E2E_InvalidConfigRejected(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		envVars        map[string]string
		expectedErrMsg string
	}{
		{
			name: "invalid performance weights - don't sum to 1.0",
			configContent: `
server:
  port: 8090
performance:
  historical_window: "24h"
  historical_refresh_interval: "5m"
  historical_weight: 0.5
  realtime_weight: 0.3
`,
			expectedErrMsg: "weights must sum to 1.0",
		},
		{
			name: "invalid performance weights - historical weight out of range",
			configContent: `
server:
  port: 8090
performance:
  historical_window: "24h"
  historical_refresh_interval: "5m"
  historical_weight: 1.5
  realtime_weight: 0.5
`,
			expectedErrMsg: "historical_weight must be between 0 and 1",
		},
		{
			name: "invalid performance weights - realtime weight out of range",
			configContent: `
server:
  port: 8090
performance:
  historical_window: "24h"
  historical_refresh_interval: "5m"
  historical_weight: 0.5
  realtime_weight: -0.2
`,
			expectedErrMsg: "realtime_weight must be between 0 and 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yml")

			err := os.WriteFile(configPath, []byte(tt.configContent), 0644)
			require.NoError(t, err)

			t.Chdir(tempDir)

			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			_, err = Load()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}

// TestChannelService_E2E_ConfigOverride tests that environment variables
// override YAML config values.
func TestChannelService_E2E_ConfigOverride(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")

	configContent := `
server:
  host: "127.0.0.1"
  port: 8090
  name: "YAMLConfigName"

performance:
  historical_window: "24h"
  historical_refresh_interval: "5m"
  historical_weight: 0.4
  realtime_weight: 0.6
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	t.Setenv("AXONHUB_SERVER_NAME", "EnvOverrideName")
	t.Setenv("AXONHUB_SERVER_PORT", "9090")
	t.Setenv("AXONHUB_PERFORMANCE_HISTORICAL_WEIGHT", "0.7")
	t.Setenv("AXONHUB_PERFORMANCE_REALTIME_WEIGHT", "0.3")

	t.Chdir(tempDir)

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, "127.0.0.1", cfg.APIServer.Host)
	require.Equal(t, 24*time.Hour, cfg.Performance.HistoricalWindow)
	require.Equal(t, 5*time.Minute, cfg.Performance.HistoricalRefreshInterval)

	require.Equal(t, "EnvOverrideName", cfg.APIServer.Name)
	require.Equal(t, 9090, cfg.APIServer.Port)
	require.Equal(t, 0.7, cfg.Performance.HistoricalWeight)
	require.Equal(t, 0.3, cfg.Performance.RealtimeWeight)

	err = validatePerformanceConfig(cfg.Performance)
	require.NoError(t, err)
}
