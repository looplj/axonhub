package conf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/looplj/axonhub/axon/permission/policy"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

const PolicyFileName = "policy.yml"

var DefaultPolicy = policy.Document{
	Version: 1,
	Defaults: policy.Defaults{
		Mode: "allow_by_default",
	},
	Rules: []policy.Rule{
		{
			ID:     "deny_config_yml",
			Effect: policy.EffectDeny,
			Reason: "deny reading sensitive config file",
			When: policy.When{
				ToolIn: []string{"Read", "Glob", "Grep"},
				Resource: policy.ResourceWhen{
					PathMatches: []string{"**/.axonclaw/config.yml"},
				},
			},
		},
		{
			ID:     "deny_cat_command",
			Effect: policy.EffectDeny,
			Reason: "deny using cat command to read files",
			When: policy.When{
				ToolIn: []string{"Bash"},
				Resource: policy.ResourceWhen{
					CommandMatches: []string{"^cat\\s+.*"},
				},
			},
		},
		{
			ID:     "allow_workspace_fs",
			Effect: policy.EffectAllow,
			Reason: "allow workspace file access",
			When: policy.When{
				ToolIn: []string{"Read", "Write", "Edit", "Glob", "Grep"},
				Resource: policy.ResourceWhen{
					OutsideWorkspace: lo.ToPtr(false),
				},
			},
		},
		{
			ID:     "allow_all_commands",
			Effect: policy.EffectAllow,
			Reason: "allow executing all commands",
			When: policy.When{
				ToolIn: []string{"Bash"},
			},
		},
		{
			ID:     "allow_web_fetch",
			Effect: policy.EffectAllow,
			Reason: "allow web fetch",
			When: policy.When{
				ToolIn: []string{"WebFetch"},
			},
		},
		{
			ID:     "allow_web_search",
			Effect: policy.EffectAllow,
			Reason: "allow web search",
			When: policy.When{
				ToolIn: []string{"WebSearch"},
			},
		},
	},
}

func LoadOrCreatePolicy(workspace string) (policy.Document, error) {
	defaultPath := filepath.Join(workspace, ".axonclaw", PolicyFileName)
	if _, err := os.Stat(defaultPath); err == nil {
		return policy.LoadFiles(defaultPath)
	}

	if err := createDefaultPolicyFile(defaultPath); err != nil {
		return policy.Document{}, fmt.Errorf("policy: create default file: %w", err)
	}

	return DefaultPolicy, nil
}

func createDefaultPolicyFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	data, err := yaml.Marshal(DefaultPolicy)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func GetPolicyFilePath(configDir string) string {
	return filepath.Join(configDir, PolicyFileName)
}
