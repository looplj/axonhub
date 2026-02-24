package cmds

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/looplj/axonhub/axon/memory"
	"github.com/spf13/cobra"
)

type MemoryOptions struct {
	Dir    string
	Stdout *os.File
	Stderr *os.File
}

func NewMemoryCommand(opts MemoryOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var dir string
	defaultDir := filepath.Join(opts.Dir, "memories")
	var store memory.Store

	root := &cobra.Command{
		Use:   "memory",
		Short: "Manage local memories",
		Long: `Memory stores small text snippets on disk for later retrieval.

Each entry is stored under a logical path (like a file path), and can be listed, searched, and deleted.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir = defaultDir
			}
			if store == nil {
				store = memory.NewFileStore(dir)
			}
			return nil
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&dir, "dir", defaultDir, "Memory store directory")

	storeGetter := func() memory.Store { return store }
	root.AddCommand(newMemoryAddCmd(stdout, storeGetter))
	root.AddCommand(newMemoryGetCmd(stdout, storeGetter))
	root.AddCommand(newMemoryListCmd(stdout, storeGetter))
	root.AddCommand(newMemorySearchCmd(stdout, storeGetter))
	root.AddCommand(newMemoryDeleteCmd(stdout, storeGetter))

	return root
}

func newMemoryAddCmd(_ *os.File, store func() memory.Store) *cobra.Command {
	var source string
	var content string

	cmd := &cobra.Command{
		Use:   "add <path> [content]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Add a memory entry",
		Example: strings.TrimSpace(`
axonclaw memory add internal/pkg/errors "Use unified error wrapper"
axonclaw memory add internal/pkg/errors --content "Use unified error wrapper" --source docs
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			path := args[0]
			if content == "" && len(args) > 1 {
				content = strings.Join(args[1:], " ")
			}
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("content is required (use --content or provide as args)")
			}
			return store().Add(ctx, path, content, source)
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "Memory content")
	cmd.Flags().StringVar(&source, "source", "", "Optional source identifier")
	return cmd
}

func newMemoryGetCmd(out *os.File, store func() memory.Store) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <path>",
		Args:  cobra.ExactArgs(1),
		Short: "Get memory content by path",
		Example: strings.TrimSpace(`
axonclaw memory get internal/pkg/errors
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			content, err := store().Get(ctx, args[0])
			if err != nil {
				return err
			}
			if content == "" {
				fmt.Fprintln(out, "No memories found at this path.")
				return nil
			}
			fmt.Fprintln(out, content)
			return nil
		},
	}
	return cmd
}

func newMemoryListCmd(out *os.File, store func() memory.Store) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List memory entries",
		Example: strings.TrimSpace(`
axonclaw memory list
axonclaw memory list --limit 100
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			entries, err := store().List(ctx, limit)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(out, "No memories found.")
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", e.ID, e.Path, e.Source, e.CreatedAt.Format("2006-01-02 15:04:05"), e.Content)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

func newMemorySearchCmd(out *os.File, store func() memory.Store) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Args:  cobra.MinimumNArgs(1),
		Short: "Search memory entries",
		Example: strings.TrimSpace(`
axonclaw memory search jwt
axonclaw memory search "quota exceeded" --limit 20
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			query := strings.Join(args, " ")
			entries, err := store().Search(ctx, query, limit)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(out, "No matching memories found.")
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", e.ID, e.Path, e.Source, e.CreatedAt.Format("2006-01-02 15:04:05"), e.Content)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results")
	return cmd
}

func newMemoryDeleteCmd(_ *os.File, store func() memory.Store) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <path>",
		Args:  cobra.ExactArgs(1),
		Short: "Delete memory by path",
		Example: strings.TrimSpace(`
axonclaw memory delete internal/pkg/errors
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return store().Delete(ctx, args[0])
		},
	}
	return cmd
}
