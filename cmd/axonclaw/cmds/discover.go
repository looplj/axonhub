package cmds

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
	"github.com/spf13/cobra"
)

type DiscoverOptions struct {
	ConfigDir string
	Stdout    *os.File
	Stderr    *os.File
}

func NewDiscoverCommand(opts DiscoverOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var configDir string
	defaultConfigDir := opts.ConfigDir
	if defaultConfigDir == "" {
		defaultConfigDir = ".axonclaw"
	}

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover peer agents in the project",
		Long: `Discover other available agents in the same project.

Returns a list of peer agents with their:
- Agent ID and name
- Description
- Status
- Instance ID

Use this information to communicate with other agents via the SendMessage tool.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := filepath.Join(configDir, conf.FileName)
			cfg, err := conf.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.BaseURL == "" || cfg.APIKey == "" {
				return fmt.Errorf("base_url and api_key must be configured (use 'axonclaw conf set')")
			}

			client := api.NewClient(cfg.BaseURL, cfg.APIKey)

			ctx := cmd.Context()
			resp, err := api.PeerAgents(ctx, client)
			if err != nil {
				return err
			}

			peers := resp.PeerAgents
			if len(peers) == 0 {
				fmt.Fprintln(stdout, "No peer agents found in this project.")
				return nil
			}

			fmt.Fprintf(stdout, "Found %d peer agent instance(s):\n\n", len(peers))
			for _, p := range peers {
				fmt.Fprintf(stdout, "- Agent: %s (ID: %s)\n", p.Name, p.AgentID)
				fmt.Fprintf(stdout, "  Status: %s\n", p.Status)
				fmt.Fprintf(stdout, "  InstanceID: %s\n\n", p.InstanceID)
			}

			return nil
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.PersistentFlags().StringVar(&configDir, "dir", defaultConfigDir, "Config directory")

	return cmd
}
