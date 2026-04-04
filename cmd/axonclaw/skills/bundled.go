package skills

import (
	"fmt"
	"strings"

	_ "embed"

	skilllib "github.com/looplj/skills"
)

const (
	MemoryManagementSkillName = "memory-management"
)

type Config struct {
	Name    string
	Enabled bool
	Order   int
}

//go:embed memory-management/SKILL.md
var memoryManagementSkill string

func DefaultConfigs() []Config {
	return []Config{
		{
			Name:    MemoryManagementSkillName,
			Enabled: true,
			Order:   0,
		},
	}
}

func BundledSkills(configs []Config) ([]skilllib.Skill, error) {
	enabled := enabledSet(configs)
	if !enabled[MemoryManagementSkillName] {
		return []skilllib.Skill{}, nil
	}

	skill, err := skilllib.ParseSkillMarkdown(memoryManagementSkill)
	if err != nil {
		return nil, fmt.Errorf("parse %s bundled skill: %w", MemoryManagementSkillName, err)
	}

	return []skilllib.Skill{skill}, nil
}

func enabledSet(configs []Config) map[string]bool {
	items := map[string]bool{}
	for _, cfg := range DefaultConfigs() {
		items[cfg.Name] = cfg.Enabled
	}

	for _, cfg := range configs {
		if strings.TrimSpace(cfg.Name) == "" {
			continue
		}

		items[cfg.Name] = cfg.Enabled
	}

	return items
}
