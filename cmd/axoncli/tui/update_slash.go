package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/samber/lo"
)

type slashCommand struct {
	Command     string
	Description string
}

const maxSlashVisible = 8

var slashCommands = []slashCommand{
	{Command: "/help", Description: "Show this help"},
	{Command: "/clear", Description: "Clear the screen"},
	{Command: "/reload", Description: "Reload config"},
	{Command: "/messages", Description: "Show conversation history"},
	{Command: "/models", Description: "Switch model"},
	{Command: "/quit", Description: "Exit the agent"},
	{Command: "/exit", Description: "Exit the agent"},
}

func (m *Model) handleSlashKey(key string) (bool, tea.Cmd) {
	if !m.slashActive || len(m.slashMatches) == 0 {
		return false, nil
	}

	switch key {
	case "up":
		m.moveSlashSelection(-1)
		return true, nil
	case "down":
		m.moveSlashSelection(1)
		return true, nil
	case "enter":
		cmd := m.applyAndExecuteSlashCompletion()
		return true, cmd
	case "esc":
		if m.processing {
			return false, nil
		}
		m.closeSlashSuggestions()
		m.applyLayout()
		return true, nil
	}
	return false, nil
}

func (m *Model) refreshSlashSuggestions() {
	value := m.textarea.Value()
	if strings.Contains(value, "\n") {
		m.closeSlashSuggestions()
		return
	}

	if !strings.HasPrefix(value, "/") {
		m.closeSlashSuggestions()
		return
	}

	query := value
	prevSelected := ""
	if m.slashActive && m.slashIndex >= 0 && m.slashIndex < len(m.slashMatches) {
		prevSelected = m.slashMatches[m.slashIndex].Command
	}

	matches := lo.Filter(slashCommands, func(c slashCommand, _ int) bool {
		return strings.HasPrefix(c.Command, query)
	})
	if len(matches) == 0 {
		m.closeSlashSuggestions()
		return
	}

	m.slashActive = true
	m.slashMatches = matches

	if prevSelected != "" {
		_, idx, ok := lo.FindIndexOf(matches, func(c slashCommand) bool { return c.Command == prevSelected })
		if ok {
			m.slashIndex = idx
		} else {
			m.slashIndex = 0
		}
	} else {
		m.slashIndex = 0
	}

	m.ensureSlashVisible()
}

func (m *Model) closeSlashSuggestions() {
	m.slashActive = false
	m.slashMatches = nil
	m.slashIndex = 0
	m.slashOffset = 0
}

func (m *Model) moveSlashSelection(delta int) {
	if len(m.slashMatches) == 0 {
		return
	}
	n := len(m.slashMatches)
	m.slashIndex = (m.slashIndex + delta + n) % n
	m.ensureSlashVisible()
}

func (m *Model) ensureSlashVisible() {
	visible := m.slashVisibleCount()
	if visible == 0 {
		m.slashOffset = 0
		return
	}

	if m.slashIndex < m.slashOffset {
		m.slashOffset = m.slashIndex
		return
	}

	if m.slashIndex >= m.slashOffset+visible {
		m.slashOffset = m.slashIndex - visible + 1
		return
	}
}

func (m *Model) applySlashCompletion() {
	if !m.slashActive || len(m.slashMatches) == 0 || m.slashIndex < 0 || m.slashIndex >= len(m.slashMatches) {
		return
	}

	selected := m.slashMatches[m.slashIndex].Command
	value := m.textarea.Value()
	if strings.Contains(value, "\n") || !strings.HasPrefix(value, "/") {
		return
	}

	m.textarea.SetValue(selected)
	m.textarea.CursorEnd()
	m.closeSlashSuggestions()
}

func (m *Model) applyAndExecuteSlashCompletion() tea.Cmd {
	if !m.slashActive || len(m.slashMatches) == 0 || m.slashIndex < 0 || m.slashIndex >= len(m.slashMatches) {
		return nil
	}

	selected := m.slashMatches[m.slashIndex].Command
	value := m.textarea.Value()
	if strings.Contains(value, "\n") || !strings.HasPrefix(value, "/") {
		return nil
	}

	m.textarea.SetValue(selected)
	m.textarea.CursorEnd()
	m.closeSlashSuggestions()
	m.applyLayout()

	// Execute the command directly
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()
	m.applyLayout()

	if cmd, handled := m.handleCommand(input); handled {
		return cmd
	}

	return nil
}
