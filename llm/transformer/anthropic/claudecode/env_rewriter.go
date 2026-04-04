package claudecode

import (
	"encoding/json"
	"math/rand"
	"time"
)

// EnvConfig holds the canonical environment fingerprint.
type EnvConfig struct {
	Platform              string `json:"platform"`
	PlatformRaw           string `json:"platform_raw"`
	Arch                  string `json:"arch"`
	NodeVersion           string `json:"node_version"`
	Terminal              string `json:"terminal"`
	PackageManagers      string `json:"package_managers"`
	Runtimes              string `json:"runtimes"`
	IsRunningWithBun      bool   `json:"is_running_with_bun"`
	IsCI                  bool   `json:"is_ci"`
	IsClaudeAIAuth        bool   `json:"is_claude_ai_auth"`
	Version               string `json:"version"`
	VersionBase           string `json:"version_base"`
	BuildTime             string `json:"build_time"`
	DeploymentEnvironment string `json:"deployment_environment"`
	VCS                   string `json:"vcs"`
}

// ProcessConfig holds process metric ranges.
type ProcessConfig struct {
	ConstrainedMemory int64   `json:"constrained_memory"`
	RSSRange          [2]int64 `json:"rss_range"`
	HeapTotalRange    [2]int64 `json:"heap_total_range"`
	HeapUsedRange     [2]int64 `json:"heap_used_range"`
}

// DefaultEnv returns the default environment configuration.
func DefaultEnv() *EnvConfig {
	return &EnvConfig{
		Platform:              "darwin",
		PlatformRaw:           "darwin",
		Arch:                  "arm64",
		NodeVersion:           "v24.3.0",
		Terminal:              "iTerm2.app",
		PackageManagers:      "npm,pnpm",
		Runtimes:              "node",
		IsRunningWithBun:      false,
		IsCI:                  false,
		IsClaudeAIAuth:        true,
		Version:               "2.1.81",
		VersionBase:           "2.1.81",
		BuildTime:             "2026-03-20T21:26:18Z",
		DeploymentEnvironment: "unknown-darwin",
		VCS:                   "git",
	}
}

// DefaultProcess returns the default process configuration.
func DefaultProcess() *ProcessConfig {
	return &ProcessConfig{
		ConstrainedMemory: 34359738368, // 32GB
		RSSRange:          [2]int64{300000000, 500000000},
		HeapTotalRange:    [2]int64{40000000, 80000000},
		HeapUsedRange:     [2]int64{100000000, 200000000},
	}
}

// BuildCanonicalEnv builds the canonical environment map for event logging.
func BuildCanonicalEnv(config *EnvConfig) map[string]interface{} {
	if config == nil {
		config = DefaultEnv()
	}

	return map[string]interface{}{
		"platform":                config.Platform,
		"platform_raw":            config.PlatformRaw,
		"arch":                    config.Arch,
		"node_version":            config.NodeVersion,
		"terminal":                config.Terminal,
		"package_managers":        config.PackageManagers,
		"runtimes":                config.Runtimes,
		"is_running_with_bun":     config.IsRunningWithBun,
		"is_ci":                   false,
		"is_claubbit":             false,
		"is_claude_code_remote":   false,
		"is_local_agent_mode":     false,
		"is_conductor":            false,
		"is_github_action":        false,
		"is_claude_code_action":   false,
		"is_claude_ai_auth":       config.IsClaudeAIAuth,
		"version":                 config.Version,
		"version_base":            config.VersionBase,
		"build_time":              config.BuildTime,
		"deployment_environment":  config.DeploymentEnvironment,
		"vcs":                     config.VCS,
	}
}

// BuildCanonicalProcess generates realistic process metrics.
func BuildCanonicalProcess(original interface{}, config *ProcessConfig) interface{} {
	if config == nil {
		config = DefaultProcess()
	}

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Handle base64-encoded process data
	if _, ok := original.(string); ok {
		// Try to decode base64
		// For now, just generate new metrics
		return generateProcessMetrics(config)
	}

	// Handle JSON object
	if m, ok := original.(map[string]interface{}); ok {
		metrics := generateProcessMetrics(config)
		for k, v := range metrics {
			m[k] = v
		}
		return m
	}

	return original
}

func generateProcessMetrics(config *ProcessConfig) map[string]interface{} {
	return map[string]interface{}{
		"constrainedMemory": config.ConstrainedMemory,
		"rss":               randomInRange(config.RSSRange[0], config.RSSRange[1]),
		"heapTotal":         randomInRange(config.HeapTotalRange[0], config.HeapTotalRange[1]),
		"heapUsed":          randomInRange(config.HeapUsedRange[0], config.HeapUsedRange[1]),
	}
}

func randomInRange(min, max int64) int64 {
	return min + rand.Int63n(max-min)
}

// RewriteEventBatch rewrites event logging batch payload.
func RewriteEventBatch(body []byte, accountIdentity string, envConfig *EnvConfig, processConfig *ProcessConfig) ([]byte, error) {
	var bodyObj map[string]interface{}
	if err := json.Unmarshal(body, &bodyObj); err != nil {
		return body, nil
	}

	events, ok := bodyObj["events"].([]interface{})
	if !ok {
		return body, nil
	}

	// Generate identity from account
	identity := GenerateIdentityFromAccount(accountIdentity)

	for _, event := range events {
		eventMap, ok := event.(map[string]interface{})
		if !ok {
			continue
		}

		eventData, ok := eventMap["event_data"].(map[string]interface{})
		if !ok {
			continue
		}

		// Rewrite identity fields
		if _, exists := eventData["device_id"]; exists {
			eventData["device_id"] = identity.DeviceID
		}

		// Rewrite environment fingerprint
		if _, exists := eventData["env"]; exists {
			eventData["env"] = BuildCanonicalEnv(envConfig)
		}

		// Rewrite process metrics
		if _, exists := eventData["process"]; exists {
			eventData["process"] = BuildCanonicalProcess(eventData["process"], processConfig)
		}

		// Strip gateway-related fields
		delete(eventData, "baseUrl")
		delete(eventData, "base_url")
		delete(eventData, "gateway")
	}

	return json.Marshal(bodyObj)
}

// RewriteAdditionalMetadata cleans sensitive fields from additional_metadata.
func RewriteAdditionalMetadata(original string) string {
	// Try to decode base64
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(original), &decoded); err != nil {
		return original
	}

	// Remove gateway-related fields
	delete(decoded, "baseUrl")
	delete(decoded, "base_url")
	delete(decoded, "gateway")

	// Re-encode
	result, err := json.Marshal(decoded)
	if err != nil {
		return original
	}

	return string(result)
}