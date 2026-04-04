package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/agent"

	tea "charm.land/bubbletea/v2"
	axoncontext "github.com/looplj/axonhub/axon/context"

	"github.com/looplj/axonhub/cmd/axoncli/conf"
)

func (m Model) handleSteer() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" {
		return m, nil
	}

	m.textarea.Reset()
	m.closeSlashSuggestions()
	m.applyLayout()

	m.appendLine(fmt.Sprintf("⤷ Steering: %s", input))
	m.syncViewport()

	m.agent.Steer(agent.Message{
		Role:    agent.RoleUser,
		Content: &agent.Content{Text: &input},
	})

	return m, nil
}

func (m Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" {
		return m, nil
	}

	// Hide logo when user submits first message
	if m.showLogo {
		m.showLogo = false
	}

	m.textarea.Reset()
	m.closeSlashSuggestions()
	m.applyLayout()

	if cmd, handled := m.handleCommand(input); handled {
		return m, cmd
	}

	m.appendLine(fmt.Sprintf("❯ %s", input))
	m.syncViewport()

	return m, func() tea.Msg {
		return processMsg{content: agent.Content{Text: &input}}
	}
}

func (m *Model) handleCommand(input string) (tea.Cmd, bool) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return nil, false
	}

	switch fields[0] {
	case "/quit", "/exit", "/q":
		m.cancel()
		return tea.Quit, true
	case "/clear":
		m.lines = nil
		m.streamingStartLineIndex = -1
		m.streamingLineCount = 0
		m.thinkingBlocks = nil
		m.activeThinking = nil
		m.thinkingHeaderViewportLine = nil
		m.syncViewport()
		return nil, true
	case "/help":
		m.appendLine("Commands:")
		m.appendLine("  /help       Show this help")
		m.appendLine("  /clear      Clear the screen")
		m.appendLine("  /conf reload Reload config")
		m.appendLine("  /reload     Alias for /conf reload")
		m.appendLine("  /messages   Show conversation history")
		m.appendLine("  /models     Switch model")
		m.appendLine("  /quit       Exit the agent")
		m.appendLine("Shortcuts:")
		m.appendLine("  Enter       Send message")
		m.appendLine("  Shift+Enter New line")
		m.appendLine("  Ctrl+Enter  Send message")
		m.appendLine("  Ctrl+J      New line")
		m.appendLine("  Ctrl+U      Clear input line")
		m.appendLine("  Esc         Cancel processing")
		m.appendLine("  Ctrl+C x2   Quit")
		m.appendLine("Scroll:")
		m.appendLine("  ↑/↓         Scroll one line")
		m.appendLine("  PgUp/PgDn   Scroll half page")
		m.appendLine("  Home/End    Jump to top/bottom")
		m.appendLine("Selection:")
		m.appendLine("  drag select Release to copy")
		m.appendLine("  Esc         Clear selection")
		m.syncViewport()
		return nil, true
	case "/messages":
		msgs := m.agent.Messages()
		if len(msgs) == 0 {
			m.appendLine("(no messages)")
		} else {
			for i, msg := range msgs {
				content := ""
				if msg.Content != nil {
					content = truncateStr(msg.Content.String(), 100)
				}

				if msg.ToolCall != nil {
					content = fmt.Sprintf("[tool_call: %s]", msg.ToolCall.Name)
				}
				m.appendLine(fmt.Sprintf("  %3d. [%s] %s", i+1, msg.Role, content))
			}
		}
		m.syncViewport()
		return nil, true
	case "/conf":
		if len(fields) >= 2 && fields[1] == "reload" {
			if m.processing && m.processCancel != nil {
				m.processCancel()
				m.processing = false
				m.appendLine("⚠ Cancelled")
			}
			m.appendLine("↻ Reloading config...")
			m.syncViewport()
			return m.startConfReload(), true
		}
		return nil, false
	case "/reload":
		if m.processing && m.processCancel != nil {
			m.processCancel()
			m.processing = false
			m.appendLine("⚠ Cancelled")
		}
		m.appendLine("↻ Reloading config...")
		m.syncViewport()
		return m.startConfReload(), true
	case "/models":
		// Load config to get baseURL and apiKey
		if err := m.modelSelector.loadConfig(); err != nil {
			m.appendLine(fmt.Sprintf("✗ Failed to load config: %v", err))
			m.syncViewport()
			return nil, true
		}
		m.appendLine("Loading models...")
		m.syncViewport()
		return m.modelSelector.open(), true
	}
	return nil, false
}

func (m *Model) startConfReload() tea.Cmd {
	if m.reloadConf == nil {
		return func() tea.Msg {
			return confReloadDoneMsg{err: fmt.Errorf("reload not supported")}
		}
	}
	return func() tea.Msg {
		err := m.reloadConf(m.ctx)
		return confReloadDoneMsg{err: err}
	}
}

func (m Model) saveModelToConfig(modelID string) tea.Cmd {
	return func() tea.Msg {
		path, err := conf.FindConfigFile(m.configDir)
		if err != nil {
			return confReloadDoneMsg{err: fmt.Errorf("failed to find config file: %w", err)}
		}
		if err := conf.SetYAMLKey(path, "model", modelID); err != nil {
			return confReloadDoneMsg{err: fmt.Errorf("failed to save model: %w", err)}
		}
		// Reload config to apply changes
		if m.reloadConf != nil {
			if err := m.reloadConf(m.ctx); err != nil {
				return confReloadDoneMsg{err: fmt.Errorf("failed to reload config: %w", err)}
			}
		}
		return confReloadDoneMsg{}
	}
}

func (m *Model) startProcess(content agent.Content) tea.Cmd {
	processCtx, processCancel := context.WithCancel(m.ctx)
	m.processCancel = processCancel
	m.processing = true
	m.streamText.Reset()
	m.streamingStartLineIndex = -1
	m.streamingLineCount = 0
	m.activeThinking = nil

	traceID := uuid.New().String()
	traceCtx := axoncontext.WithTraceID(processCtx, traceID)
	m.streamEvents = m.agent.ProcessStream(traceCtx, content)

	return waitForStreamEvent(m.streamEvents)
}
