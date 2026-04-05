package cmds

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/looplj/axonhub/cmd/axoncli/conf"
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
		if home, err := os.UserHomeDir(); err == nil {
			defaultDir = filepath.Join(home, ".axoncli")
		}
	}

	root := &cobra.Command{
		Use:           "conf",
		Short:         "Read and update axoncli config",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Manage axoncli configuration.

Keys:
  base_url, api_key, model, trace_header, thread_header, reasoning_effort

Notes:
  - api_key is always masked in output.
  - By default, axoncli loads config.yml from ~/.axoncli or current directory.`,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&dir, "dir", defaultDir, "Config directory (default: ~/.axoncli)")

	root.AddCommand(newConfPathCmd(stdout, &dir))
	root.AddCommand(newConfGetCmd(stdout, &dir))
	root.AddCommand(newConfListCmd(stdout, &dir))
	root.AddCommand(newConfSetCmd(stdout, stderr, &dir))

	return root
}

var confKeys = []string{"base_url", "api_key", "model", "trace_header", "thread_header", "reasoning_effort"}

func isValidConfKey(k string) bool {
	return lo.Contains(confKeys, k)
}

func newConfPathCmd(out *os.File, dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show which config file will be used",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := conf.FindConfigFile(*dir)
			if err != nil {
				return err
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
axoncli conf get base_url
axoncli conf get api_key
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if !isValidConfKey(key) {
				return fmt.Errorf("unknown key %q (allowed: %s)", key, strings.Join(confKeys, ", "))
			}

			path, err := conf.FindConfigFile(*dir)
			if err != nil {
				return err
			}
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
		Short: "List config keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := conf.FindConfigFile(*dir)
			if err != nil {
				return err
			}
			for _, key := range confKeys {
				val, _, err := conf.GetYAMLString(path, key)
				if err != nil {
					return err
				}
				if key == "api_key" {
					val = maskSecret(val)
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
axoncli conf set base_url http://localhost:8090
axoncli conf set model deepseek-chat
axoncli conf set api_key sk-***
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
				p, err := conf.FindConfigFile(*dir)
				if err != nil {
					return err
				}
				path = p
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
