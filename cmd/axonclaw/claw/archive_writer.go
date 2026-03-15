package claw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"

	axoncontext "github.com/looplj/axonhub/axon/context"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

func AppendArchiveMessage(ctx context.Context, workspace string, msg agent.Message) error {
	threadID := resolveArchiveThreadID(ctx)
	if threadID == "" {
		threadID = "unknown-thread"
	}

	now := time.Now()

	path := filepath.Join(
		workspace,
		conf.DefaultDir,
		"messages",
		"archives",
		fmt.Sprintf("%s_%s.md", now.Format("2006-01-02"), sanitizeArchiveThreadID(threadID)),
	)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open archive file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(renderArchiveMessage(now.UTC(), msg)); err != nil {
		return fmt.Errorf("append archive message: %w", err)
	}

	return nil
}

func resolveArchiveThreadID(ctx context.Context) string {
	if meta, ok := bus.MetadataFromContext(ctx); ok && strings.TrimSpace(meta.ThreadID) != "" {
		return strings.TrimSpace(meta.ThreadID)
	}

	return strings.TrimSpace(axoncontext.ThreadID(ctx))
}

func sanitizeArchiveThreadID(threadID string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")

	cleaned := replacer.Replace(strings.TrimSpace(threadID))
	if cleaned == "" {
		return "unknown-thread"
	}

	return cleaned
}

func renderArchiveMessage(now time.Time, msg agent.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s %s\n\n", now.Format(time.RFC3339), msg.Role)

	if msg.RoundIndex != 0 {
		fmt.Fprintf(&b, "- Round index: %d\n", msg.RoundIndex)
	}

	if msg.ToolUse != nil {
		fmt.Fprintf(&b, "- Tool use: `%s` (id: `%s`)\n", msg.ToolUse.Name, msg.ToolUse.ID)
	}

	if msg.ToolUseID != nil {
		fmt.Fprintf(&b, "- Tool call id: `%s`\n", *msg.ToolUseID)
	}

	if msg.IsError != nil && *msg.IsError {
		b.WriteString("- Is error: true\n")
	}

	b.WriteString("\n")

	if msg.ToolUse != nil {
		fmt.Fprintf(&b, "```json\n%s\n```\n\n", strings.TrimSpace(msg.ToolUse.Input))
		return b.String()
	}

	if msg.Content != nil {
		renderArchiveContent(&b, msg.Content)
	}

	b.WriteString("\n")

	return b.String()
}

func renderArchiveContent(b *strings.Builder, c *agent.Content) {
	if c.Text != nil && strings.TrimSpace(*c.Text) != "" {
		fmt.Fprintf(b, "```\n%s\n```\n", strings.TrimSpace(*c.Text))
		return
	}

	for _, part := range c.Parts {
		//nolint:exhaustive // Archive rendering only persists text/thinking parts; other part types are intentionally skipped.
		switch part.Type {
		case agent.ContentPartText:
			if strings.TrimSpace(part.Text) != "" {
				fmt.Fprintf(b, "```\n%s\n```\n", strings.TrimSpace(part.Text))
			}
		case agent.ContentPartThinking:
			if strings.TrimSpace(part.Thinking) != "" {
				fmt.Fprintf(b, "**Thinking:**\n```\n%s\n```\n", strings.TrimSpace(part.Thinking))
			}
		}
	}
}
