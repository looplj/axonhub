package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/looplj/axonhub/axon/agent"
)

var (
	toolStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	toolDimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	assistantStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	thinkingHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	thinkingDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// formatToolStart formats a tool start event into a display line.
func formatToolStart(ev agent.AgentEvent) string {
	var summary string
	switch ev.ToolName {
	case "Bash":
		var input struct{ Command string }
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = truncateStr(input.Command, 80)
		}
	case "Read":
		var input struct{ Path string }
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Path
		}
	case "Write":
		var input struct{ Path string }
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Path
		}
	case "Edit":
		var input struct{ Path string }
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Path
		}
	case "Grep":
		var input struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path,omitempty"`
		}
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			if input.Path != "" {
				summary = fmt.Sprintf("%s in %s", input.Pattern, input.Path)
			} else {
				summary = input.Pattern
			}
		}
	case "Glob":
		var input struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path,omitempty"`
		}
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			if input.Path != "" {
				summary = fmt.Sprintf("%s in %s", input.Pattern, input.Path)
			} else {
				summary = input.Pattern
			}
		}
	case "Skill":
		var input struct {
			Skill string `json:"skill"`
			Args  string `json:"args,omitempty"`
		}
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			if input.Args != "" {
				summary = fmt.Sprintf("%s %s", input.Skill, truncateStr(input.Args, 40))
			} else {
				summary = input.Skill
			}
		}
	case "MemoryAdd":
		var input struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Path
		}
	case "MemoryGet":
		var input struct{ Path string }
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Path
		}
	case "MemorySearch":
		var input struct{ Query string }
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Query
		}
	case "MemoryList":
		summary = "list memories"
	case "MemoryDelete":
		var input struct{ Path string }
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Path
		}
	case "WebSearch":
		var input struct {
			Query          string   `json:"query"`
			AllowedDomains []string `json:"allowed_domains,omitempty"`
			BlockedDomains []string `json:"blocked_domains,omitempty"`
		}
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Query
			if len(input.AllowedDomains) > 0 {
				summary = fmt.Sprintf("%s (site: %s)", input.Query, strings.Join(input.AllowedDomains, ", "))
			}
		}
	case "WebFetch":
		var input struct{ Query string }
		if json.Unmarshal([]byte(ev.ToolInput), &input) == nil {
			summary = input.Query
		}
	default:
		summary = truncateStr(ev.ToolInput, 80)
	}

	line := toolStyle.Render(fmt.Sprintf("⚡ %s", ev.ToolName))
	if summary != "" {
		line += " " + toolDimStyle.Render(summary)
	}
	return line
}

// formatToolEnd formats a tool end event into a display line.
func formatToolEnd(ev agent.AgentEvent) string {
	if ev.Result == nil {
		return ""
	}
	if ev.Result.Error != nil {
		errStr := fmt.Sprintf("%v", ev.Result.Error)
		lines := strings.Split(errStr, "\n")
		var formatted strings.Builder
		for i, line := range lines {
			if i == 0 {
				formatted.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ %s", line)))
			} else {
				formatted.WriteString("\n")
				formatted.WriteString(errorStyle.Render(fmt.Sprintf("    %s", line)))
			}
		}
		return formatted.String()
	}
	return ""
}

// formatAssistantMessage formats an assistant message for display with word wrap.
func formatAssistantMessage(msg agent.Message, width int) string {
	text := msg.Content.String()
	if text == "" {
		return ""
	}
	if width > 0 {
		return assistantStyle.Width(width).Render(text)
	}
	return assistantStyle.Render(text)
}

func formatStreamingText(text string, width int) string {
	if text == "" {
		return ""
	}
	if width > 0 {
		return assistantStyle.Width(width).Render(text)
	}
	return assistantStyle.Render(text)
}

// truncateStr truncates a string to max characters, collapsing newlines.
func truncateStr(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// formatThinkingHeader formats the thinking header line with expand/collapse indicator.
func formatThinkingHeader(expanded, complete bool, width int) string {
	indicator := "▶"
	if expanded {
		indicator = "▼"
	}
	status := "Thinking..."
	if complete {
		status = "Thought"
	}
	header := fmt.Sprintf("%s %s", indicator, status)
	if width > 0 {
		return thinkingHeaderStyle.Width(width).Render(header)
	}
	return thinkingHeaderStyle.Render(header)
}

// formatThinkingContent formats the thinking content for display.
func formatThinkingContent(content string, width int) string {
	if width > 0 {
		return thinkingDimStyle.Width(width).Render(content)
	}
	return thinkingDimStyle.Render(content)
}
