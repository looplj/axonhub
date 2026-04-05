package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/looplj/axonhub/axon/agent"
	axonconf "github.com/looplj/axonhub/axon/conf"
)

func (m Model) handleConfEvent(ev axonconf.ReloadEvent) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case axonconf.ReloadEventRequested:
		m.appendLine("↻ Reload requested")
	case axonconf.ReloadEventStarted:
		if ev.ConfigFile != "" {
			m.appendLine(fmt.Sprintf("↻ Reloading config (%s)...", ev.ConfigFile))
		} else {
			m.appendLine("↻ Reloading config...")
		}
	case axonconf.ReloadEventNoop:
		m.appendLine("✓ Config unchanged")
	case axonconf.ReloadEventFailed:
		if ev.Error != "" {
			lines := strings.Split(ev.Error, "\n")
			for i, line := range lines {
				if i == 0 {
					m.appendLine(fmt.Sprintf("✗ Reload failed: %s", line))
				} else if line != "" {
					m.appendLine(fmt.Sprintf("  %s", line))
				}
			}
		} else {
			m.appendLine("✗ Reload failed")
		}
	case axonconf.ReloadEventApplied:
		if v, ok := ev.Attributes["model"]; ok && v != "" {
			m.model = v
		}
		if len(ev.ChangedKeys) > 0 {
			m.appendLine(fmt.Sprintf("✓ Config applied (%s)", strings.Join(ev.ChangedKeys, ", ")))
		} else {
			m.appendLine("✓ Config applied")
		}
	}

	m.syncViewport()
	return m, waitForConfEvent(m.confEvents)
}

func (m Model) handleAgentEvent(ev agent.AgentEvent) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case agent.EventAgentStart:
		m.processing = true
	case agent.EventAgentEnd:
		m.processing = false
	case agent.EventToolStart:
		m.appendLine(formatToolStart(ev))
	case agent.EventToolEnd:
		if line := formatToolEnd(ev); line != "" {
			m.appendLine(line)
		}
	case agent.EventMessageAdded:
		if ev.Message != nil {
			m.threadMgr.AddMessage(m.threadID, *ev.Message)
			if ev.Message.Role == agent.RoleAssistant && ev.Message.Content != nil {
				if line := formatAssistantMessage(*ev.Message, m.width); line != "" {
					m.appendLine(line)
				}
			}
		}
	case agent.EventToolSkipped:
		m.appendLine(fmt.Sprintf("  ⏭ Skipped tool: %s", ev.ToolName))
	case agent.EventSteeringApplied:
		m.appendLine("  ⤷ Steering applied")
	case agent.EventError:
		if ev.Error != nil && !errors.Is(ev.Error, context.Canceled) {
			errStr := fmt.Sprintf("%v", ev.Error)
			lines := strings.Split(errStr, "\n")
			for i, line := range lines {
				if i == 0 {
					m.appendLine(fmt.Sprintf("✗ Error: %s", line))
				} else if line != "" {
					m.appendLine(fmt.Sprintf("  %s", line))
				}
			}
		}
	}

	m.syncViewport()
	return m, waitForAgentEvent(m.agentEvents)
}

func (m *Model) handleStreamEvent(ev agent.AgentEvent) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case agent.EventTextDelta:
		m.streamText.WriteString(ev.Delta)
		m.updateStreamingLines()
	case agent.EventTextComplete:
		m.streamText.Reset()
		m.streamingStartLineIndex = -1
		m.streamingLineCount = 0
	case agent.EventThinkingDelta:
		if m.activeThinking == nil {
			block := &thinkingBlock{
				expanded:              true,
				complete:              false,
				headerLineIndex:       -1,
				contentStartLineIndex: -1,
				contentLineCount:      0,
			}
			m.thinkingBlocks = append(m.thinkingBlocks, block)
			m.activeThinking = block
		}
		m.activeThinking.content.WriteString(ev.Thinking)
		m.activeThinking.complete = false
		m.updateThinkingBlock(m.activeThinking)
	case agent.EventThinkingComplete:
		if m.activeThinking != nil {
			m.activeThinking.complete = true
			m.updateThinkingBlock(m.activeThinking)
			m.activeThinking = nil
		}
	case agent.EventToolStart:
		m.appendLine(formatToolStart(ev))
	case agent.EventToolEnd:
		if line := formatToolEnd(ev); line != "" {
			m.appendLine(line)
		}
	case agent.EventToolCallDelta:
	case agent.EventToolCallComplete:
	case agent.EventMessageAdded:
		if ev.Message != nil {
			m.threadMgr.AddMessage(m.threadID, *ev.Message)
		}
	case agent.EventToolSkipped:
		m.appendLine(fmt.Sprintf("  ⏭ Skipped tool: %s", ev.ToolName))
	case agent.EventSteeringApplied:
		m.appendLine("  ⤷ Steering applied")
	case agent.EventError:
		if ev.Error != nil && !errors.Is(ev.Error, context.Canceled) {
			errStr := fmt.Sprintf("%v", ev.Error)
			lines := strings.Split(errStr, "\n")
			for i, line := range lines {
				if i == 0 {
					m.appendLine(fmt.Sprintf("✗ Error: %s", line))
				} else if line != "" {
					m.appendLine(fmt.Sprintf("  %s", line))
				}
			}
		}
		return m, nil
	case agent.EventAgentEnd:
		m.processing = false
		m.streamEvents = nil
		m.streamText.Reset()
		m.streamingStartLineIndex = -1
		m.streamingLineCount = 0
		// Reset active thinking for next message (keep existing blocks in history)
		m.activeThinking = nil
		m.syncViewport()
		return m, nil
	}

	m.syncViewport()
	if m.streamEvents != nil {
		return m, waitForStreamEvent(m.streamEvents)
	}
	return m, nil
}

func (m *Model) updateStreamingLines() {
	text := m.streamText.String()
	if text == "" {
		return
	}
	formatted := formatStreamingText(text, m.width)
	lines := strings.Split(formatted, "\n")

	if m.streamingStartLineIndex >= 0 && m.streamingLineCount > 0 {
		m.spliceLines(m.streamingStartLineIndex, m.streamingLineCount, lines)
		m.streamingLineCount = len(lines)
		return
	}

	start := len(m.lines)
	m.spliceLines(start, 0, lines)
	m.streamingStartLineIndex = start
	m.streamingLineCount = len(lines)
}

func (m *Model) updateThinkingBlock(block *thinkingBlock) {
	if block == nil {
		return
	}
	content := block.content.String()
	if content == "" {
		return
	}

	// Ensure header exists.
	if block.headerLineIndex < 0 || block.headerLineIndex >= len(m.lines) {
		m.appendLine(formatThinkingHeader(block.expanded, block.complete, m.width))
		block.headerLineIndex = len(m.lines) - 1
	}

	// Always keep header up-to-date (indicator + status).
	if block.headerLineIndex >= 0 && block.headerLineIndex < len(m.lines) {
		m.lines[block.headerLineIndex] = formatThinkingHeader(block.expanded, block.complete, m.width)
	}

	if !block.expanded {
		// Remove content lines if collapsed.
		if block.contentStartLineIndex >= 0 && block.contentLineCount > 0 {
			m.spliceLines(block.contentStartLineIndex, block.contentLineCount, nil)
		}
		block.contentStartLineIndex = -1
		block.contentLineCount = 0
		return
	}

	formatted := formatThinkingContent(content, m.width)
	contentLines := strings.Split(formatted, "\n")

	// Insert/update content lines below header.
	if block.contentStartLineIndex >= 0 && block.contentLineCount > 0 {
		m.spliceLines(block.contentStartLineIndex, block.contentLineCount, contentLines)
		block.contentLineCount = len(contentLines)
		return
	}

	insertAt := block.headerLineIndex + 1
	m.spliceLines(insertAt, 0, contentLines)
	block.contentStartLineIndex = insertAt
	block.contentLineCount = len(contentLines)
}
