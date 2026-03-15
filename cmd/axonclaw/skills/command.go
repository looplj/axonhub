package skills

import (
	"context"
	"os"
	"path/filepath"

	"github.com/looplj/skills/skillscmd"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	skilllib "github.com/looplj/skills"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

const defaultSkillWorkspaceDir = ".axonclaw"

func NewCommand(workspaceDir string) *cobra.Command {
	return skillscmd.NewRootCommand(skillscmd.RootOptions{
		Use:          "skills",
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		WorkspaceDir: filepath.Join(workspaceDir, defaultSkillWorkspaceDir, "skills"),
		BundledSkillsFunc: func(ctx context.Context) []skilllib.Skill {
			var configs []Config

			items, err := conf.LoadBuiltinSkills()
			if err != nil {
				configs = DefaultConfigs()
			} else {
				configs = lo.Map(items, func(item conf.BuiltinSkill, _ int) Config {
					return Config{
						Name:    item.Name,
						Enabled: item.Enabled,
						Order:   item.Order,
					}
				})
			}

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
