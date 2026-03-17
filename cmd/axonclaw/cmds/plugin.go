package cmds

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	clawmemory "github.com/looplj/axonhub/cmd/axonclaw/memory"
	"github.com/looplj/axonhub/cmd/axonclaw/plugins"
)

func NewPluginCommand(opts StdioOptions, workspaceDir string) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	layout := clawmemory.NewLayout(workspaceDir)
	mgr := plugins.NewManager(plugins.ManagerOptions{Dir: layout.PluginsDir()})

	root := &cobra.Command{
		Use:           "plugin",
		Short:         "Manage AxonClaw WASM plugins",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newPluginListCmd(stdout, mgr))
	root.AddCommand(newPluginInspectCmd(stdout, mgr))

	return root
}

func newPluginListCmd(out *os.File, mgr *plugins.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List installed WASM plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := mgr.List()
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Fprintln(out, "No WASM plugins found.")
				return nil
			}

			for _, item := range items {
				hooks := make([]string, 0, len(item.Manifest.Hooks))
				for hook := range item.Manifest.Hooks {
					hooks = append(hooks, hook)
				}
				sort.Strings(hooks)
				fmt.Fprintf(out, "%s\t%s\t%s\n", item.Manifest.Name, item.WasmPath, hooks)
			}
			return nil
		},
	}
}

func newPluginInspectCmd(out *os.File, mgr *plugins.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Inspect one WASM plugin manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := mgr.List()
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.Manifest.Name != args[0] {
					continue
				}

				fmt.Fprintf(out, "name\t%s\n", item.Manifest.Name)
				fmt.Fprintf(out, "description\t%s\n", item.Manifest.Description)
				fmt.Fprintf(out, "manifest\t%s\n", item.ManifestPath)
				fmt.Fprintf(out, "wasm\t%s\n", item.WasmPath)
				hooks := make([]string, 0, len(item.Manifest.Hooks))
				for hook, export := range item.Manifest.Hooks {
					hooks = append(hooks, fmt.Sprintf("%s=%s", hook, export))
				}
				sort.Strings(hooks)
				for _, hook := range hooks {
					fmt.Fprintf(out, "hook\t%s\n", hook)
				}
				return nil
			}

			return fmt.Errorf("plugin %q not found", args[0])
		},
	}
}
