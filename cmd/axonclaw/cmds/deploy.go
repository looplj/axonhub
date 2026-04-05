package cmds

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/looplj/axonhub/axon/api"
	"github.com/spf13/cobra"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

func NewDeployCommand(opts StdioOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var name string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a new AxonClaw instance",
		Long: `Deploy a new AxonClaw instance to the same host as the current instance.

This command allows an agent to spawn a new instance of itself on the same host.
The new instance will inherit the host and base URL from the current instance.

Required:
  --name      Name for the new instance

Examples:
  axonclaw deploy --name worker-1
`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name = strings.TrimSpace(name)
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			cfg, err := conf.LoadConfig()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.BaseURL == "" || cfg.APIKey == "" {
				return fmt.Errorf("base_url and api_key must be configured")
			}

			client := api.NewClient(cfg.BaseURL, cfg.APIKey)

			input := &api.DeployAxonClawInput{
				Name: name,
			}

			resp, err := api.DeployAxonClaw(context.Background(), client, input)
			if err != nil {
				return fmt.Errorf("deploy failed: %w", err)
			}

			if !resp.DeployAxonClaw.Success {
				errMsg := "unknown error"
				if resp.DeployAxonClaw.Error != nil {
					errMsg = *resp.DeployAxonClaw.Error
				}

				return fmt.Errorf("deploy failed: %s", errMsg)
			}

			instance := resp.DeployAxonClaw.Instance
			if instance != nil {
				fmt.Fprintf(stdout, "instance deployed:\n")
				fmt.Fprintf(stdout, "  id: %s\n", instance.Id)
				fmt.Fprintf(stdout, "  agent_id: %s\n", instance.AgentID)
			} else {
				fmt.Fprintln(stdout, "instance deployed successfully")
			}

			return nil
		},
	}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&name, "name", "", "Name for the new instance (required)")

	return cmd
}

var _ graphql.Client = (graphql.Client)(nil)
