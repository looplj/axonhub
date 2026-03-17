package cmds

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	clawmemory "github.com/looplj/axonhub/cmd/axonclaw/memory"
	"github.com/looplj/axonhub/cmd/axonclaw/plugins"
)

func NewMemoryCommand(opts StdioOptions, workspaceDir string) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	layout := clawmemory.NewLayout(workspaceDir)
	store := clawmemory.NewStore(clawmemory.StoreOptions{
		Layout:   layout,
		Embedder: clawmemory.NewEmbedderFromConfig(),
		Hooks: plugins.NewManager(plugins.ManagerOptions{
			Dir: layout.PluginsDir(),
		}),
	})

	root := &cobra.Command{
		Use:   "memory",
		Short: "Manage local memory files",
		Long: `Memory is stored as Markdown files under the workspace:
- .axonclaw/MEMORY.md for curated long-term memory
- .axonclaw/memory/YYYY-MM-DD.md for daily append-only notes`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newMemoryAddCmd(stdout, store, layout))
	root.AddCommand(newMemoryGetCmd(stdout, store))
	root.AddCommand(newMemoryListCmd(stdout, store))
	root.AddCommand(newMemorySearchCmd(stdout, store))
	root.AddCommand(newMemoryRewriteCmd(stdout, store))
	root.AddCommand(newMemoryDeleteCmd(stdout, store, layout))

	return root
}

func newMemoryAddCmd(out *os.File, store *clawmemory.Store, layout clawmemory.Layout) *cobra.Command {
	var (
		content  string
		longTerm bool
	)

	cmd := &cobra.Command{
		Use:   "add [content]",
		Args:  cobra.ArbitraryArgs,
		Short: "Append memory",
		Example: strings.TrimSpace(`
axonclaw memory add "Finished migration for billing retries"
axonclaw memory add --longterm "User prefers concise status updates"
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if content == "" && len(args) > 1 {
				content = strings.Join(args, " ")
			}

			if content == "" && len(args) == 1 {
				content = args[0]
			}
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("content is required (use --content or provide as args)")
			}

			targets, autoPromoted, err := store.Add(context.Background(), content, longTerm)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Appended memory to %s", formatMemoryTargets(targets, layout))

			if autoPromoted {
				fmt.Fprint(out, " (auto-promoted to long-term)")
			}

			fmt.Fprintln(out)

			return nil
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "Memory content")
	cmd.Flags().BoolVar(&longTerm, "longterm", false, "Also append to .axonclaw/MEMORY.md")
	return cmd
}

func newMemoryGetCmd(out *os.File, store *clawmemory.Store) *cobra.Command {
	var (
		date      string
		longTerm  bool
		yesterday bool
	)

	cmd := &cobra.Command{
		Use:   "get",
		Args:  cobra.NoArgs,
		Short: "Read memory content",
		Example: strings.TrimSpace(`
axonclaw memory get
axonclaw memory get --longterm
axonclaw memory get --date 2026-03-15
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := store.Get(date, longTerm, yesterday)
			if err != nil {
				return err
			}
			if content == "" {
				fmt.Fprintln(out, "No memories found.")
				return nil
			}
			fmt.Fprintln(out, content)
			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Read a specific daily memory file (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&longTerm, "longterm", false, "Read .axonclaw/MEMORY.md")
	cmd.Flags().BoolVar(&yesterday, "yesterday", false, "Read yesterday's daily memory file")
	return cmd
}

func newMemoryListCmd(out *os.File, store *clawmemory.Store) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List memory files",
		Example: strings.TrimSpace(`
axonclaw memory list
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := store.List()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(out, "No memories found.")
				return nil
			}

			for _, entry := range entries {
				fmt.Fprintf(out, "%s\t%d bytes\n", entry.Label, entry.Size)
			}
			return nil
		},
	}
	return cmd
}

func newMemorySearchCmd(out *os.File, store *clawmemory.Store) *cobra.Command {
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
			query := strings.Join(args, " ")

			matches, err := store.Search(context.Background(), query, limit)
			if err != nil {
				return err
			}

			if len(matches) == 0 {
				fmt.Fprintln(out, "No matching memories found.")
				return nil
			}

			for _, match := range matches {
				fmt.Fprintf(out, "%s:%d\t%s\n", match.Label, match.Line, match.Text)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results")
	return cmd
}

func newMemoryRewriteCmd(out *os.File, store *clawmemory.Store) *cobra.Command {
	var (
		content  string
		longTerm bool
	)

	cmd := &cobra.Command{
		Use:   "rewrite --longterm --content <content>",
		Args:  cobra.NoArgs,
		Short: "Rewrite long-term memory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !longTerm {
				return fmt.Errorf("rewrite currently supports only --longterm")
			}

			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("content is required")
			}

			if err := store.RewriteLongTerm(context.Background(), content); err != nil {
				return err
			}

			fmt.Fprintln(out, "Rewrote .axonclaw/MEMORY.md (old content archived to today's daily memory)")

			return nil
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "Replacement content")
	cmd.Flags().BoolVar(&longTerm, "longterm", false, "Rewrite .axonclaw/MEMORY.md")

	return cmd
}

func newMemoryDeleteCmd(out *os.File, store *clawmemory.Store, layout clawmemory.Layout) *cobra.Command {
	var (
		date     string
		longTerm bool
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Args:  cobra.NoArgs,
		Short: "Delete a memory file",
		Example: strings.TrimSpace(`
axonclaw memory delete --date 2026-03-15
axonclaw memory delete --longterm
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, deleted, err := store.Delete(context.Background(), date, longTerm)
			if err != nil {
				return err
			}

			if !deleted {
				fmt.Fprintln(out, "Memory file does not exist.")
				return nil
			}

			fmt.Fprintf(out, "Deleted %s\n", layout.DisplayPath(path))

			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Delete a specific daily memory file (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&longTerm, "longterm", false, "Delete .axonclaw/MEMORY.md")
	return cmd
}
func formatMemoryTargets(targets []string, layout clawmemory.Layout) string {
	labels := make([]string, 0, len(targets))
	for _, target := range targets {
		labels = append(labels, layout.DisplayPath(target))
	}

	return strings.Join(labels, ", ")
}
