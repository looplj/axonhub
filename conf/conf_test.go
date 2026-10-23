package conf

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

func TestLoaderReloadUsesFreshViper(t *testing.T) {
	configFile := writeTestConfig(t, `
log:
  level: info
`)
	loader := &Loader{configFile: configFile}

	initial, err := loader.Reload()
	if err != nil {
		t.Fatalf("initial Reload() error = %v", err)
	}
	if initial.Log.Level != zapcore.InfoLevel {
		t.Fatalf("initial log level = %s, want %s", initial.Log.Level, zapcore.InfoLevel)
	}

	if err := os.WriteFile(configFile, []byte(`
log:
  level: debug
`), 0o600); err != nil {
		t.Fatalf("rewrite test config: %v", err)
	}

	reloaded, err := loader.Reload()
	if err != nil {
		t.Fatalf("reloaded Reload() error = %v", err)
	}
	if reloaded.Log.Level != zapcore.DebugLevel {
		t.Fatalf("reloaded log level = %s, want %s", reloaded.Log.Level, zapcore.DebugLevel)
	}
}

func TestSSEKeepAliveConfig(t *testing.T) {
	configFile := writeTestConfig(t, `
server:
  sse_keep_alive:
    enabled: true
    interval: 30s
`)

	cfg, _, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if !cfg.APIServer.SSEKeepAlive.Enabled {
		t.Fatal("SSE keep-alive should be enabled")
	}
	if cfg.APIServer.SSEKeepAlive.Interval != 30*time.Second {
		t.Fatalf("SSE keep-alive interval = %s, want 30s", cfg.APIServer.SSEKeepAlive.Interval)
	}
}

func TestSSEKeepAliveDefaultsToDisabled(t *testing.T) {
	configFile := writeTestConfig(t, "")

	cfg, _, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.APIServer.SSEKeepAlive.Enabled {
		t.Fatal("SSE keep-alive should default to disabled")
	}
	if cfg.APIServer.SSEKeepAlive.Interval != 15*time.Second {
		t.Fatalf("SSE keep-alive interval = %s, want 15s", cfg.APIServer.SSEKeepAlive.Interval)
	}
}

func TestMetricsExporterConfigFromEnvironment(t *testing.T) {
	t.Setenv("AXONHUB_METRICS_ENABLED", "true")
	t.Setenv("AXONHUB_METRICS_EXPORTER_TYPE", "otlphttp")
	t.Setenv("AXONHUB_METRICS_EXPORTER_ENDPOINT", "alloy.alloy.svc.cluster.local:4318")
	t.Setenv("AXONHUB_METRICS_EXPORTER_INSECURE", "true")

	configFile := writeTestConfig(t, "")
	cfg, _, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if !cfg.Metrics.Enabled {
		t.Fatal("metrics should be enabled")
	}
	if cfg.Metrics.Exporter.Type != "otlphttp" {
		t.Fatalf("metrics exporter type = %q, want %q", cfg.Metrics.Exporter.Type, "otlphttp")
	}
	if cfg.Metrics.Exporter.Endpoint != "alloy.alloy.svc.cluster.local:4318" {
		t.Fatalf(
			"metrics exporter endpoint = %q, want %q",
			cfg.Metrics.Exporter.Endpoint,
			"alloy.alloy.svc.cluster.local:4318",
		)
	}
	if !cfg.Metrics.Exporter.Insecure {
		t.Fatal("metrics exporter should use an insecure connection")
	}
}

func TestTraceExtractionDefaultsToEnabled(t *testing.T) {
	configFile := writeTestConfig(t, "")

	cfg, _, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	trace := cfg.APIServer.Trace
	if !trace.ClaudeCodeTraceEnabled {
		t.Error("Claude Code trace extraction should default to enabled")
	}
	if !trace.CodexTraceEnabled {
		t.Error("Codex trace extraction should default to enabled")
	}
	if !trace.OpenCodeTraceEnabled {
		t.Error("OpenCode trace extraction should default to enabled")
	}
}

func writeTestConfig(t *testing.T, contents string) string {
	t.Helper()

	configFile := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configFile, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return configFile
}

func TestSafeguardsConfigDefaults(t *testing.T) {
	cfg, _, err := loadConfig(writeTestConfig(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.APIServer.Safeguards
	if got.EscalateOnDenial || got.ClassifierModel != "" || len(got.FallbackModels) != 0 || got.AttemptTimeout != 0 || got.Timeout != 30*time.Second || got.MaxTokens != 4096 || got.TranscriptCharBudget != 24000 {
		t.Fatalf("unexpected safeguards defaults: %+v", got)
	}
}

func TestSafeguardsConfigEnvironmentOverridesYAML(t *testing.T) {
	t.Setenv("AXONHUB_SERVER_SAFEGUARDS_ESCALATE_ON_DENIAL", "true")
	t.Setenv("AXONHUB_SERVER_SAFEGUARDS_CLASSIFIER_MODEL", " gemini-3.8-flash ")
	t.Setenv("AXONHUB_SERVER_SAFEGUARDS_FALLBACK_MODELS", `[" codex-auto-review ","backup"]`)
	t.Setenv("AXONHUB_SERVER_SAFEGUARDS_ATTEMPT_TIMEOUT", "4s")
	t.Setenv("AXONHUB_SERVER_SAFEGUARDS_TIMEOUT", "12s")
	t.Setenv("AXONHUB_SERVER_SAFEGUARDS_MAX_TOKENS", "2048")
	t.Setenv("AXONHUB_SERVER_SAFEGUARDS_TRANSCRIPT_CHAR_BUDGET", "12000")
	for _, source := range []string{"", `server:
  safeguards:
    classifier_model: other-model
    fallback_models: [other-backup]
    attempt_timeout: 5s
    timeout: 20s
    max_tokens: 1024
    transcript_char_budget: 6000
`} {
		cfg, _, err := loadConfig(writeTestConfig(t, source))
		if err != nil {
			t.Fatal(err)
		}
		got := cfg.APIServer.Safeguards
		if !got.EscalateOnDenial || got.ClassifierModel != "gemini-3.8-flash" || !slices.Equal(got.FallbackModels, []string{"codex-auto-review", "backup"}) || got.AttemptTimeout != 4*time.Second || got.Timeout != 12*time.Second || got.MaxTokens != 2048 || got.TranscriptCharBudget != 12000 {
			t.Fatalf("environment overrides were not applied: %+v", got)
		}
	}
}

func TestSafeguardsConfigRejectsInvalidLimits(t *testing.T) {
	for _, config := range []string{
		"timeout: 0s", "timeout: -1s", "timeout: 121s", "timeout: invalid",
		"max_tokens: 0", "max_tokens: 16385",
		"attempt_timeout: -1s", "attempt_timeout: 121s", "attempt_timeout: invalid",
		`fallback_models: [" "]`, `fallback_models: ["backup", " backup "]`,
		"classifier_model: primary\n    fallback_models: [primary]",
		"transcript_char_budget: 0", "transcript_char_budget: 1000001",
	} {
		t.Run(config, func(t *testing.T) {
			_, _, err := loadConfig(writeTestConfig(t, "server:\n  safeguards:\n    "+config+"\n"))
			if err == nil {
				t.Fatalf("expected error for %s", config)
			}
		})
	}
}

func TestSafeguardsFallbackYAML(t *testing.T) {
	cfg, _, err := loadConfig(writeTestConfig(t, `server:
  safeguards:
    classifier_model: " codex-auto-review "
    fallback_models:
      - " gemini-3.8-flash "
      - other-reviewer
    attempt_timeout: 10s
    timeout: 30s
`))
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.APIServer.Safeguards
	if got.ClassifierModel != "codex-auto-review" || !slices.Equal(got.FallbackModels, []string{"gemini-3.8-flash", "other-reviewer"}) || got.AttemptTimeout != 10*time.Second || got.Timeout != 30*time.Second {
		t.Fatalf("unexpected fallback configuration: %+v", got)
	}
}

func TestSafeguardsEscalationYAML(t *testing.T) {
	cfg, _, err := loadConfig(writeTestConfig(t, "server:\n  safeguards:\n    escalate_on_denial: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.APIServer.Safeguards.EscalateOnDenial {
		t.Fatal("YAML escalation toggle ignored")
	}
}
