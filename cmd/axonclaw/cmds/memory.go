package cmds

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/looplj/axonhub/axon/pkg/grep"
	"github.com/spf13/cobra"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

const longTermMemoryFileName = "MEMORY.md"

func NewMemoryCommand(opts StdioOptions, workspaceDir string) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	layout := newMemoryLayout(workspaceDir)

	root := &cobra.Command{
		Use:   "memory",
		Short: "Manage local memory files",
		Hidden: true,
		Long: `Memory is stored as Markdown files under the workspace:
- .axonclaw/MEMORY.md for curated long-term memory
- .axonclaw/memory/YYYY-MM-DD.md for daily append-only notes`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newMemoryAddCmd(stdout, layout))
	root.AddCommand(newMemoryGetCmd(stdout, layout))
	root.AddCommand(newMemoryListCmd(stdout, layout))
	root.AddCommand(newMemorySearchCmd(stdout, layout))
	root.AddCommand(newMemoryRewriteCmd(stdout, layout))
	root.AddCommand(newMemoryDeleteCmd(stdout, layout))

	return root
}

func newMemoryAddCmd(out *os.File, layout memoryLayout) *cobra.Command {
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

			content = strings.TrimSpace(content)

			targets := []string{layout.todayPath()}
			if longTerm || shouldPromoteToLongTerm(content) {
				targets = append(targets, layout.longTermPath())
			}

			for _, target := range targets {
				if err := appendMemoryFile(target, formatMemoryEntry(content)); err != nil {
					return err
				}
			}

			fmt.Fprintf(out, "Appended memory to %s", formatMemoryTargets(targets, layout))

			if !longTerm && len(targets) > 1 {
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

func newMemoryGetCmd(out *os.File, layout memoryLayout) *cobra.Command {
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
			path, err := resolveMemoryTarget(layout, date, longTerm, yesterday)
			if err != nil {
				return err
			}

			content, err := readMemoryFile(path)
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

func newMemoryListCmd(out *os.File, layout memoryLayout) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List memory files",
		Example: strings.TrimSpace(`
axonclaw memory list
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := listMemoryFiles(layout)
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

func newMemorySearchCmd(out *os.File, layout memoryLayout) *cobra.Command {
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

			matches, err := searchMemoryFiles(query, limit, layout)
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

func newMemoryRewriteCmd(out *os.File, layout memoryLayout) *cobra.Command {
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

			oldContent, err := readMemoryFile(layout.longTermPath())
			if err != nil {
				return err
			}

			if oldContent != "" {
				archiveHeader := fmt.Sprintf("\n---\n\n## Archived from MEMORY.md (%s)\n\n", time.Now().Format("2006-01-02 15:04"))
				if err := appendMemoryFile(layout.todayPath(), archiveHeader+oldContent+"\n"); err != nil {
					return fmt.Errorf("archive old long-term memory: %w", err)
				}
			}

			if err := writeMemoryFile(layout.longTermPath(), strings.TrimSpace(content)+"\n"); err != nil {
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

func newMemoryDeleteCmd(out *os.File, layout memoryLayout) *cobra.Command {
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
			path, err := resolveDeleteTarget(layout, date, longTerm)
			if err != nil {
				return err
			}

			if err := os.Remove(path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Fprintln(out, "Memory file does not exist.")
					return nil
				}

				return err
			}

			fmt.Fprintf(out, "Deleted %s\n", displayMemoryPath(path, layout))

			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Delete a specific daily memory file (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&longTerm, "longterm", false, "Delete .axonclaw/MEMORY.md")
	return cmd
}

type memoryLayout struct {
	workspace string
}

type memoryFileEntry struct {
	Label string
	Size  int64
}

type memorySearchMatch struct {
	Label string
	Line  int
	Text  string
}

func newMemoryLayout(workspace string) memoryLayout {
	return memoryLayout{workspace: workspace}
}

func (m memoryLayout) rootDir() string {
	return filepath.Join(m.workspace, conf.DefaultDir)
}

func (m memoryLayout) longTermPath() string {
	return filepath.Join(m.rootDir(), longTermMemoryFileName)
}

func (m memoryLayout) dailyDir() string {
	return filepath.Join(m.rootDir(), "memory")
}

func (m memoryLayout) dailyPath(date time.Time) string {
	return filepath.Join(m.dailyDir(), date.Format("2006-01-02")+".md")
}

func (m memoryLayout) todayPath() string {
	return m.dailyPath(time.Now())
}

func appendMemoryFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open memory file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("write memory: %w", err)
	}

	return nil
}

func writeMemoryFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}

	return os.WriteFile(path, []byte(content), 0o600) //nolint:gosec // Memory files are user-scoped content written under the workspace.
}

func readMemoryFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}

		return "", fmt.Errorf("read memory file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

func listMemoryFiles(layout memoryLayout) ([]memoryFileEntry, error) {
	var out []memoryFileEntry
	if info, err := os.Stat(layout.longTermPath()); err == nil {
		out = append(out, memoryFileEntry{Label: ".axonclaw/MEMORY.md", Size: info.Size()})
	}

	entries, err := os.ReadDir(layout.dailyDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read memory directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat memory file: %w", err)
		}

		out = append(out, memoryFileEntry{
			Label: filepath.ToSlash(filepath.Join(".axonclaw", "memory", entry.Name())),
			Size:  info.Size(),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Label < out[j].Label
	})

	return out, nil
}

func searchMemoryFiles(query string, limit int, layout memoryLayout) ([]memorySearchMatch, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	if limit <= 0 {
		limit = 10
	}

	searcher := grep.NewSearcher(layout.workspace)

	opts := grep.Options{
		Pattern:    query,
		Path:       filepath.ToSlash(filepath.Join(conf.DefaultDir, "memory")),
		OutputMode: "content",
		IgnoreCase: new(bool),
		LineNumber: new(bool),
		HeadLimit:  limit,
		Literal:    true,
	}
	*opts.IgnoreCase = true
	*opts.LineNumber = true

	result, err := searcher.Search(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("search memory: %w", err)
	}

	if result.Text == "No results found." {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSuffix(result.Text, "\n"), "\n")
	matches := make([]memorySearchMatch, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		label := parts[0]

		var lineNum int

		_, err := fmt.Sscanf(parts[1], "%d", &lineNum)
		if err != nil {
			continue
		}

		text := strings.TrimSpace(parts[2])

		matches = append(matches, memorySearchMatch{
			Label: label,
			Line:  lineNum,
			Text:  text,
		})
	}

	return matches, nil
}

func resolveMemoryTarget(layout memoryLayout, date string, longTerm, yesterday bool) (string, error) {
	if longTerm {
		if date != "" || yesterday {
			return "", fmt.Errorf("--longterm cannot be combined with daily selectors")
		}

		return layout.longTermPath(), nil
	}

	switch {
	case date != "" && yesterday:
		return "", fmt.Errorf("--date and --yesterday cannot be used together")
	case date != "":
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			return "", fmt.Errorf("invalid --date %q, expected YYYY-MM-DD", date)
		}

		return layout.dailyPath(parsed), nil
	case yesterday:
		return layout.dailyPath(time.Now().AddDate(0, 0, -1)), nil
	default:
		return layout.todayPath(), nil
	}
}

func resolveDeleteTarget(layout memoryLayout, date string, longTerm bool) (string, error) {
	if longTerm {
		if date != "" {
			return "", fmt.Errorf("--longterm cannot be combined with --date")
		}

		return layout.longTermPath(), nil
	}

	if date == "" {
		return "", fmt.Errorf("delete requires --date or --longterm")
	}

	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", fmt.Errorf("invalid --date %q, expected YYYY-MM-DD", date)
	}

	return layout.dailyPath(parsed), nil
}

func displayMemoryPath(path string, layout memoryLayout) string {
	relative, err := filepath.Rel(layout.workspace, path)
	if err != nil {
		return path
	}

	return filepath.ToSlash(relative)
}

func formatMemoryTargets(targets []string, layout memoryLayout) string {
	labels := make([]string, 0, len(targets))
	for _, target := range targets {
		labels = append(labels, displayMemoryPath(target, layout))
	}

	return strings.Join(labels, ", ")
}

func formatMemoryEntry(content string) string {
	return fmt.Sprintf("- [%s] %s\n", time.Now().Format("15:04"), content)
}

func shouldPromoteToLongTerm(content string) bool {
	normalized := strings.ToLower(strings.TrimSpace(content))
	if normalized == "" {
		return false
	}

	keywords := []string{
		"user prefers", "prefer", "preference", "always", "never", "decision", "decided",
		"rule", "policy", "remember", "long-term", "durable", "stable", "lesson learned",
		"偏好", "习惯", "规则", "决策", "长期", "记住", "以后都",
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}

	return strings.HasPrefix(normalized, "remember:")
}
