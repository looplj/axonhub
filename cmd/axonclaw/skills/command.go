package skills

import (
	"context"
	"os"
	"path/filepath"

	"github.com/looplj/skills/skillscmd"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	skilllib "github.com/looplj/skills"
)

const defaultSkillWorkspaceDir = ".axonclaw"

type BuiltinSkillConfig struct {
	Name    string
	Enabled bool
	Order   int
}

type LoadBuiltinSkillsFunc func() ([]BuiltinSkillConfig, error)

func NewCommand(workspaceDir string, loadBuiltinSkills LoadBuiltinSkillsFunc) *cobra.Command {
	return skillscmd.NewRootCommand(skillscmd.RootOptions{
		Use:          "skills",
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		WorkspaceDir: filepath.Join(workspaceDir, defaultSkillWorkspaceDir, "skills"),
		BundledSkillsFunc: func(ctx context.Context) []skilllib.Skill {
			var configs []Config

			if loadBuiltinSkills != nil {
				items, err := loadBuiltinSkills()
				if err == nil {
					configs = lo.Map(items, func(item BuiltinSkillConfig, _ int) Config {
						return Config(item)
					})

					bundled, err := BundledSkills(configs)
					if err == nil {
						return bundled
					}
				}
			}

			configs = DefaultConfigs()
			bundled, err := BundledSkills(configs)
			if err != nil {
				return nil
			}

			return bundled
		},
		Commands:             []string{"search", "list", "add", "remove"},
		EnableAgentDiscovery: false,
		EnableAgentFlags:     false,
	})
}
