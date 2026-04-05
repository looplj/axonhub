package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Effect string

const (
	EffectAllow           Effect = "allow"
	EffectDeny            Effect = "deny"
	EffectRequireApproval Effect = "require_approval"
)

// Defaults controls how the policy engine behaves when no rule matches.
//
// Mode supports:
// - "require_approval_by_default" (default): no match => require approval
// - "deny_by_default": no match => deny
// - "allow_by_default": no match => allow
type Defaults struct {
	Mode string `yaml:"mode,omitempty"`
}

// Document is the top-level policy YAML schema.
//
// Multiple policy files can be loaded and merged. Rules are evaluated in the
// final merged order; deny rules short-circuit immediately, while allow /
// require_approval candidates may be overridden by later rules.
type Document struct {
	// Version is the policy schema version. Only version 1 is supported.
	Version int `yaml:"version"`

	// Defaults controls the engine's fallback decision when no rule matches.
	Defaults Defaults `yaml:"defaults,omitempty"`

	// Allow is a low-precedence tool allowlist. It is evaluated after all
	// rules. Prefer rules when you need resource constraints.
	Allow []AllowEntry `yaml:"allow,omitempty"`

	// Rules are evaluated top-to-bottom. Rules can match on tool name and
	// resource constraints (paths/domains/schemes/command regexes).
	Rules []Rule `yaml:"rules,omitempty"`
}

// AllowEntry allows a tool without requiring approval. This is a
// low-precedence fallback and does not support resource constraints.
type AllowEntry struct {
	Tool string `yaml:"tool"`
}

// Rule is a single policy rule.
type Rule struct {
	// ID is a stable identifier used for audit and UI display.
	ID        string `yaml:"id"`
	// Effect is one of: allow, deny, require_approval.
	Effect    Effect `yaml:"effect"`
	// RiskLevel is a UI hint (e.g. low/medium/high/critical).
	RiskLevel string `yaml:"risk_level,omitempty"`
	// Reason is a human-readable explanation shown in approval UI / logs.
	Reason    string `yaml:"reason,omitempty"`
	// When describes the match conditions for this rule.
	When      When   `yaml:"when"`
}

// When describes rule match conditions.
type When struct {
	// ToolIn matches if the tool name is in this list.
	ToolIn []string `yaml:"tool_in,omitempty"`

	// Resource matches against extracted resources (paths, domains, commands, URLs).
	Resource ResourceWhen `yaml:"resource,omitempty"`
}

// ResourceWhen matches against extracted resources.
//
// Multiple matchers are AND-ed together. For list matchers (e.g. domain_in),
// the rule matches if any resource satisfies the matcher.
type ResourceWhen struct {
	// OutsideWorkspace matches file/path/dir resources based on whether they are
	// outside the current workspace root.
	OutsideWorkspace *bool `yaml:"outside_workspace,omitempty"`

	// PathMatches is a glob pattern list matched against workspace-relative path
	// when available (otherwise absolute path). Supported globs:
	// - `*` matches any chars except `/`
	// - `?` matches any single char except `/`
	// - `**` matches any chars including `/`
	PathMatches    []string `yaml:"path_matches,omitempty"`
	// DirMatches is a glob pattern list matched against directory resources
	// (workspace-relative when available, otherwise absolute path).
	DirMatches     []string `yaml:"dir_matches,omitempty"`
	// DomainIn matches URL/domain resources by exact domain string.
	DomainIn       []string `yaml:"domain_in,omitempty"`
	// SchemeIn matches URL resources by scheme (e.g. "https").
	SchemeIn       []string `yaml:"scheme_in,omitempty"`
	// CommandMatches is a regexp list matched against command resources.
	CommandMatches []string `yaml:"command_matches,omitempty"`
	// SkillIn matches skill resources by exact skill name.
	SkillIn        []string `yaml:"skill_in,omitempty"`
}

func LoadFiles(paths ...string) (Document, error) {
	var merged Document
	merged.Version = 1

	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Document{}, fmt.Errorf("policy: read %s: %w", p, err)
		}
		var d Document
		if err := yaml.Unmarshal(data, &d); err != nil {
			return Document{}, fmt.Errorf("policy: parse %s: %w", p, err)
		}
		if d.Version == 0 {
			d.Version = 1
		}
		if d.Version != 1 {
			return Document{}, fmt.Errorf("policy: unsupported version %d in %s", d.Version, p)
		}

		// Later files override defaults.
		if strings.TrimSpace(d.Defaults.Mode) != "" {
			merged.Defaults.Mode = d.Defaults.Mode
		}
		merged.Allow = append(merged.Allow, d.Allow...)
		merged.Rules = append(merged.Rules, d.Rules...)
	}

	return merged, nil
}

func DefaultPaths(configDir, workspace string) []string {
	var out []string
	if workspace != "" {
		out = append(out, filepath.Join(workspace, ".agent/policy.yml"))
	}
	if configDir != "" {
		out = append(out, filepath.Join(configDir, "policy.yml"))
	}
	return out
}
