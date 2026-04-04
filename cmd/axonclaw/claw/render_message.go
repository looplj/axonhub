package claw

import (
	"fmt"
	"strings"
	"time"

	"github.com/looplj/axonhub/axon/agent"
)

type MessageRenderOptions struct {
	TimePrefix bool
	Separator  string
}

func RenderMessage(ts time.Time, msg agent.Message, opts MessageRenderOptions) string {
	var sb strings.Builder

	if opts.TimePrefix {
		fmt.Fprintf(&sb, "[%s] ", ts.Format("15:04:05"))
	}

	switch {
	case msg.ToolCall != nil:
		fmt.Fprintf(&sb, "tool:%s(id:%s): %s", msg.ToolCall.Name, msg.ToolCall.ID, strings.TrimSpace(msg.ToolCall.Input))

	case msg.ToolUseID != nil:
		prefix := "tool_result"
		if msg.IsError != nil && *msg.IsError {
			prefix = "tool_error"
		}

		fmt.Fprintf(&sb, "%s(%s): %s", prefix, *msg.ToolUseID, renderContentInline(msg.Content))

	default:
		fmt.Fprintf(&sb, "%s: %s", msg.Role, renderContentInline(msg.Content))
	}

	sep := opts.Separator
	if sep == "" {
		sep = "\n"
	}

	sb.WriteString(sep)

	return sb.String()
}

func RenderMessages(messages []agent.Message, opts MessageRenderOptions) string {
	var sb strings.Builder

	now := time.Now().UTC()

	for _, msg := range messages {
		sb.WriteString(RenderMessage(now, msg, opts))
	}

	return sb.String()
}

func renderContentInline(c *agent.Content) string {
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
		case agent.ContentPartRedactedThinking:
		case agent.ContentPartImage:
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " | ")
}
