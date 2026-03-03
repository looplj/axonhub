package cmds

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

type ConfOptions struct {
	Dir    string
	Stdout *os.File
	Stderr *os.File
}

func NewConfCommand(opts ConfOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var dir string
	defaultDir := opts.Dir
	if defaultDir == "" {
		defaultDir = ".axonclaw"
	}

	root := &cobra.Command{
		Use:           "conf",
		Short:         "Read and update axonclaw config",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Manage axonclaw configuration.

Keys:
  base_url, api_key, instance_id, name, poll_interval, heartbeat_interval

Notes:
  - api_key is always masked in output.
  - By default, axonclaw loads config.yml from .axonclaw directory.`,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&dir, "dir", defaultDir, "Config directory (default: .axonclaw)")

	root.AddCommand(newConfPathCmd(stdout, &dir))
	root.AddCommand(newConfGetCmd(stdout, &dir))
	root.AddCommand(newConfListCmd(stdout, &dir))
	root.AddCommand(newConfSetCmd(stdout, stderr, &dir))

	return root
}

var confKeys = []string{"base_url", "api_key", "instance_id", "name", "poll_interval", "heartbeat_interval", "auto_sync_config", "auto_sync_config_interval", "debug"}

func isValidConfKey(k string) bool {
	return lo.Contains(confKeys, k)
}

func newConfPathCmd(out *os.File, dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show which config file will be used",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(*dir, conf.FileName)
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("config file not found: %s", path)
			}
			fmt.Fprintln(out, path)
			return nil
		},
	}
}

func newConfGetCmd(out *os.File, dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(`
axonclaw conf get base_url
axonclaw conf get api_key
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if !isValidConfKey(key) {
				return fmt.Errorf("unknown key %q (allowed: %s)", key, strings.Join(confKeys, ", "))
			}

			path := filepath.Join(*dir, conf.FileName)
			val, _, err := conf.GetYAMLString(path, key)
			if err != nil {
				return err
			}
			if key == "api_key" {
				val = maskSecret(val)
			}
			fmt.Fprintln(out, val)
			return nil
		},
	}
}

func newConfListCmd(out *os.File, dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List config keys (supports environment variables)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := conf.LoadConfig()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			values := map[string]string{
				"base_url":                  cfg.BaseURL,
				"api_key":                   maskSecret(cfg.APIKey),
				"name":                      cfg.Name,
				"poll_interval":             cfg.PollInterval.String(),
				"heartbeat_interval":        cfg.HeartbeatInterval.String(),
				"auto_sync_config":          fmt.Sprintf("%v", cfg.AutoSyncConfig),
				"auto_sync_config_interval": cfg.AutoSyncConfigInterval.String(),
				"debug":                     fmt.Sprintf("%v", cfg.Debug),
			}

			for _, key := range confKeys {
				if key == "instance_id" {
					fmt.Fprintf(out, "%s\t%s\n", key, "")
					continue
				}
				val, ok := values[key]
				if !ok {
					val = ""
				}
				fmt.Fprintf(out, "%s\t%s\n", key, val)
			}
			return nil
		},
	}
}

func newConfSetCmd(out *os.File, errOut *os.File, dir *string) *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.MinimumNArgs(2),
		Example: strings.TrimSpace(`
axonclaw conf set base_url http://localhost:8090
axonclaw conf set name my-agent
axonclaw conf set api_key sk-***
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if !isValidConfKey(key) {
				return fmt.Errorf("unknown key %q (allowed: %s)", key, strings.Join(confKeys, ", "))
			}

			var val string
			val = strings.Join(args[1:], " ")
			val = strings.TrimSpace(val)

			if val == "" {
				return fmt.Errorf("value is required")
			}

			path := strings.TrimSpace(file)
			if path == "" {
				path = filepath.Join(*dir, conf.FileName)
			}

			if err := conf.SetYAMLKey(path, key, val); err != nil {
				return err
			}
			fmt.Fprintf(errOut, "config\t%s\n", path)

			display := val
			if key == "api_key" {
				display = maskSecret(val)
				fmt.Fprintln(errOut, "api_key saved (masked)")
			}
			fmt.Fprintf(out, "%s\t%s\n", key, display)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Explicit config file path to update")
	return cmd
}

func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
