package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/safeguards"
)

const SystemKeyClaudeCodeSafeguards = "claude_code_safeguards"

// ClaudeCodeSafeguardsConfig uses seconds at the settings API boundary.
type ClaudeCodeSafeguardsConfig struct {
	ClassifierModel       string   `json:"classifierModel"`
	Policy                string   `json:"policy"`
	FallbackModels        []string `json:"fallbackModels"`
	EscalateOnDenial      bool     `json:"escalateOnDenial"`
	AttemptTimeoutSeconds float64  `json:"attemptTimeoutSeconds"`
	TimeoutSeconds        float64  `json:"timeoutSeconds"`
	MaxTokens             int      `json:"maxTokens"`
	TranscriptCharBudget  int      `json:"transcriptCharBudget"`
}

type ClaudeCodeSafeguardsSettings struct {
	Config              ClaudeCodeSafeguardsConfig `json:"config"`
	UsingServerDefaults bool                       `json:"usingServerDefaults"`
	DefaultPolicy       string                     `json:"defaultPolicy"`
}

func claudeCodeConfig(c safeguards.Config) ClaudeCodeSafeguardsConfig {
	fallback := append([]string{}, c.FallbackModels...)
	return ClaudeCodeSafeguardsConfig{
		ClassifierModel: c.ClassifierModel, Policy: c.Policy, FallbackModels: fallback,
		EscalateOnDenial: c.EscalateOnDenial, AttemptTimeoutSeconds: c.AttemptTimeout.Seconds(),
		TimeoutSeconds: c.Timeout.Seconds(), MaxTokens: c.MaxTokens, TranscriptCharBudget: c.TranscriptCharBudget,
	}
}

func (c ClaudeCodeSafeguardsConfig) runtimeConfig() (safeguards.Config, error) {
	if math.IsNaN(c.TimeoutSeconds) || math.IsInf(c.TimeoutSeconds, 0) || c.TimeoutSeconds <= 0 || c.TimeoutSeconds > 120 {
		return safeguards.Config{}, xerrors.ValidationError("overall timeout must be greater than zero and at most 120 seconds")
	}
	if math.IsNaN(c.AttemptTimeoutSeconds) || math.IsInf(c.AttemptTimeoutSeconds, 0) || c.AttemptTimeoutSeconds < 0 || c.AttemptTimeoutSeconds > 120 {
		return safeguards.Config{}, xerrors.ValidationError("attempt timeout must be between zero and 120 seconds")
	}
	if c.MaxTokens < 1 || c.MaxTokens > 16384 {
		return safeguards.Config{}, xerrors.ValidationError("max tokens must be between 1 and 16384")
	}
	if c.TranscriptCharBudget < 1 || c.TranscriptCharBudget > 1000000 {
		return safeguards.Config{}, xerrors.ValidationError("transcript budget must be between 1 and 1000000 UTF-8 bytes")
	}
	model := strings.TrimSpace(c.ClassifierModel)
	fallback := make([]string, 0, len(c.FallbackModels))
	seen := map[string]bool{model: true}
	for _, raw := range c.FallbackModels {
		name := strings.TrimSpace(raw)
		if name == "" || seen[name] {
			return safeguards.Config{}, xerrors.ValidationError("fallback models must be nonempty, distinct and different from the primary model")
		}
		seen[name] = true
		fallback = append(fallback, name)
	}
	timeout := time.Duration(c.TimeoutSeconds * float64(time.Second))
	if timeout <= 0 {
		return safeguards.Config{}, xerrors.ValidationError("overall timeout must be at least one nanosecond")
	}
	return safeguards.Config{
		ClassifierModel: model, Policy: c.Policy, FallbackModels: fallback,
		EscalateOnDenial: c.EscalateOnDenial,
		AttemptTimeout:   time.Duration(c.AttemptTimeoutSeconds * float64(time.Second)),
		Timeout:          timeout, MaxTokens: c.MaxTokens, TranscriptCharBudget: c.TranscriptCharBudget,
	}, nil
}

func (s *SystemService) claudeCodeSafeguardsSettings(ctx context.Context) (*ClaudeCodeSafeguardsSettings, error) {
	// Read directly so an older in-flight cache fill, or another process's local
	// cache, cannot restore stale policy after an administrator saves or resets it.
	record, err := s.entFromContext(ctx).System.Query().Where(system.KeyEQ(SystemKeyClaudeCodeSafeguards)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("read Claude Code safeguards settings: %w", err)
	}
	defaults := s.safeguardsDefaults
	if defaults.Timeout == 0 {
		defaults.Timeout = safeguards.DefaultTimeout
	}
	if defaults.MaxTokens == 0 {
		defaults.MaxTokens = safeguards.MaxTokens
	}
	if defaults.TranscriptCharBudget == 0 {
		defaults.TranscriptCharBudget = 24000
	}
	result := &ClaudeCodeSafeguardsSettings{Config: claudeCodeConfig(defaults), UsingServerDefaults: true, DefaultPolicy: safeguards.DefaultPolicy}
	if ent.IsNotFound(err) || record.Value == "null" {
		return result, nil
	}
	var stored ClaudeCodeSafeguardsConfig
	if err := json.Unmarshal([]byte(record.Value), &stored); err != nil {
		return nil, fmt.Errorf("decode Claude Code safeguards settings: %w", err)
	}
	config, err := stored.runtimeConfig()
	if err != nil {
		return nil, err
	}
	result.Config = claudeCodeConfig(config)
	result.UsingServerDefaults = false
	return result, nil
}

func (s *SystemService) ClaudeCodeSafeguardsSettings(ctx context.Context) (*ClaudeCodeSafeguardsSettings, error) {
	if err := authz.RequireScope(ctx, scopes.ScopeReadSettings); err != nil {
		return nil, err
	}
	return s.claudeCodeSafeguardsSettings(ctx)
}

// ClaudeCodeSafeguardsConfig reads only server-owned settings; reviewer model calls
// still run under the original caller's model access and quota restrictions.
func (s *SystemService) ClaudeCodeSafeguardsConfig(ctx context.Context) (safeguards.Config, error) {
	settings, err := authz.RunWithSystemBypass(ctx, "claude-code-safeguards-config", func(ctx context.Context) (*ClaudeCodeSafeguardsSettings, error) {
		return s.claudeCodeSafeguardsSettings(ctx)
	})
	if err != nil {
		return safeguards.Config{}, err
	}
	return settings.Config.runtimeConfig()
}

func (s *SystemService) SetClaudeCodeSafeguardsSettings(ctx context.Context, input ClaudeCodeSafeguardsConfig) error {
	if err := authz.RequireScope(ctx, scopes.ScopeWriteSettings); err != nil {
		return err
	}
	config, err := input.runtimeConfig()
	if err != nil {
		return err
	}
	value, err := json.Marshal(claudeCodeConfig(config))
	if err != nil {
		return fmt.Errorf("encode Claude Code safeguards settings: %w", err)
	}
	return s.setSystemValue(ctx, SystemKeyClaudeCodeSafeguards, string(value))
}

func (s *SystemService) ResetClaudeCodeSafeguardsSettings(ctx context.Context) error {
	if err := authz.RequireScope(ctx, scopes.ScopeWriteSettings); err != nil {
		return err
	}
	// A null override keeps reset on the standard atomic upsert/cache invalidation path.
	return s.setSystemValue(ctx, SystemKeyClaudeCodeSafeguards, "null")
}
