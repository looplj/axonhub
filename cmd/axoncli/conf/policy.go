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
	Rules: []policy.Rule{
		{
			ID:     "allow_workspace_read",
			Effect: policy.EffectAllow,
			When: policy.When{
				CapabilityIn: []string{"fs.read", "fs.write", "fs.edit"},
				Resource: policy.ResourceWhen{
					OutsideWorkspace: lo.ToPtr(false),
				},
			},
		},
		{
			ID:     "allow_axoncli_commands",
			Effect: policy.EffectAllow,
			When: policy.When{
				Resource: policy.ResourceWhen{
					CommandMatches: []string{"axoncli.*"},
				},
			},
		},
	},
}

func LoadOrCreatePolicy(configDir, workspace string) (policy.Document, error) {
	paths := policy.DefaultPaths(configDir, workspace)

	var existingPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			existingPath = p
			break
		}
	}

	if existingPath != "" {
		return policy.LoadFiles(existingPath)
	}

	defaultPath := filepath.Join(configDir, PolicyFileName)
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
