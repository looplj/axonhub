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

	path := filepath.Join(
		workspace,
		conf.DefaultDir,
		"messages",
		"archives",
		fmt.Sprintf("%s_%s.md", time.Now().Format("2006-01-02"), sanitizeArchiveThreadID(threadID)),
	)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open archive file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(renderArchiveMessage(time.Now().UTC(), msg)); err != nil {
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
	timeStr := now.Format("15:04:05")

	switch {
	case msg.ToolCall != nil:
		return fmt.Sprintf("[%s] tool:%s(id:%s): %s\n", timeStr, msg.ToolCall.Name, msg.ToolCall.ID, strings.TrimSpace(msg.ToolCall.Input))

	case msg.ToolUseID != nil:
		prefix := "tool_result"
		if msg.IsError != nil && *msg.IsError {
			prefix = "tool_error"
		}

		content := renderArchiveContentInline(msg.Content)

		return fmt.Sprintf("[%s] %s(%s): %s\n", timeStr, prefix, *msg.ToolUseID, content)

	default:
		content := renderArchiveContentInline(msg.Content)
		return fmt.Sprintf("[%s] %s: %s\n", timeStr, msg.Role, content)
	}
}

func renderArchiveContentInline(c *agent.Content) string {
	if c == nil {
		return ""
	}

	if c.Text != nil && strings.TrimSpace(*c.Text) != "" {
		return strings.TrimSpace(*c.Text)
	}

	var parts []string
	for _, part := range c.Parts {
		switch part.Type {
		case agent.ContentPartText:
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, strings.TrimSpace(part.Text))
			}
		case agent.ContentPartThinking:
			if strings.TrimSpace(part.Thinking) != "" {
				parts = append(parts, "[thinking] "+strings.TrimSpace(part.Thinking))
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " | ")
}
