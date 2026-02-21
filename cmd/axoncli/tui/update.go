package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyWindowSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		key := msg.String()

		// Handle approval modal keys first if active.
		if m.approvalActive {
			return m.handleApprovalKey(key)
		}

		// Handle model selector keys first if active
		if m.modelSelector.active {
			if handled, cmd := m.modelSelector.handleKey(key); handled {
				m.applyLayout()
				return m, cmd
			}
		}

		if handled, cmd := m.handleSlashKey(key); handled {
			return m, cmd
		}

		switch key {
		case "ctrl+c":
			return m.handleCtrlC()
		case "esc":
			if m.modelSelector.active {
				m.modelSelector.close()
				m.applyLayout()
				return m, nil
			}
			if m.hasSelection() {
				m.selectionStart = selectionPos{}
				m.selectionEnd = selectionPos{}
				m.syncViewport()
				return m, nil
			}
			if m.processing && m.processCancel != nil {
				m.processCancel()
				m.processing = false
				m.streamEvents = nil
				m.processCancel = nil
				m.streamText.Reset()
				m.streamingStartLineIndex = -1
				m.streamingLineCount = 0
				m.activeThinking = nil
				m.appendLine("⚠ Cancelled")
				m.syncViewport()
			}
			return m, nil
		case "enter":
			if m.processing {
				return m.handleSteer()
			}
			return m.handleSubmit()
		case "ctrl+enter":
			if m.processing {
				return m.handleSteer()
			}
			return m.handleSubmit()
		// Viewport scrolling keys (when not in textarea or when textarea has single line)
		case "up":
			// If model selector is active, let it handle the key
			if m.modelSelector.active {
				return m, nil
			}
			// If textarea has multiple lines and cursor is not at first line, move cursor up
			if m.textarea.Line() > 0 {
				m.textarea.CursorUp()
			} else {
				m.viewport.ScrollUp(1)
			}
			return m, nil
		case "down":
			// If model selector is active, let it handle the key
			if m.modelSelector.active {
				return m, nil
			}
			// If textarea has multiple lines and cursor is not at last line, move cursor down
			if m.textarea.Line() < m.textarea.LineCount()-1 {
				m.textarea.CursorDown()
			} else {
				m.viewport.ScrollDown(1)
			}
			return m, nil
		case "pgup":
			m.viewport.HalfPageUp()
			return m, nil
		case "pgdown":
			m.viewport.HalfPageDown()
			return m, nil
		case "ctrl+u":
			// Clear current line in textarea
			m.textarea.SetValue("")
			return m, nil
		case "home":
			m.viewport.GotoTop()
			return m, nil
		case "end":
			m.viewport.GotoBottom()
			return m, nil
		case "ctrl+shift+c":
			if m.hasSelection() {
				return m, tea.SetClipboard(m.getSelectedText())
			}
			return m, nil
		case "ctrl+y":
			if m.hasSelection() {
				return m, tea.SetClipboard(m.getSelectedText())
			}
			return m, nil
		case "t":
			if len(m.thinkingBlocks) > 0 && m.thinkingBlocks[len(m.thinkingBlocks)-1] != nil {
				block := m.thinkingBlocks[len(m.thinkingBlocks)-1]
				block.expanded = !block.expanded
				m.updateThinkingBlock(block)
				m.syncViewport()
			}
			return m, nil
		}

		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
		m.refreshSlashSuggestions()
		m.applyLayout()
		return m, tea.Batch(cmds...)

	case tea.MouseClickMsg:
		mouse := tea.Mouse(msg)
		if mouse.Button != tea.MouseLeft {
			return m, nil
		}
		if idx, ok := m.thinkingBlockIndexAtMouse(mouse.X, mouse.Y); ok {
			if idx >= 0 && idx < len(m.thinkingBlocks) && m.thinkingBlocks[idx] != nil {
				block := m.thinkingBlocks[idx]
				block.expanded = !block.expanded
				m.updateThinkingBlock(block)
				m.syncViewport()
				return m, nil
			}
		}
		if pos, ok := m.viewportPosAtMouse(mouse.X, mouse.Y); ok {
			m.selecting = true
			m.selectionStart = pos
			m.selectionEnd = pos
			m.syncViewport()
		}
		return m, nil

	case tea.MouseMotionMsg:
		mouse := tea.Mouse(msg)
		if m.selecting && mouse.Button == tea.MouseLeft {
			if pos, ok := m.viewportPosAtMouse(mouse.X, mouse.Y); ok {
				m.selectionEnd = pos
				m.syncViewport()
			}
		}
		return m, nil

	case tea.MouseReleaseMsg:
		mouse := tea.Mouse(msg)
		if mouse.Button == tea.MouseLeft {
			m.selecting = false
			m.syncViewport()
			if m.hasSelection() {
				return m, tea.SetClipboard(m.getSelectedText())
			}
		}
		return m, nil

	case tea.MouseWheelMsg:
		mouse := tea.Mouse(msg)
		viewportTopY := m.viewportTopY()
		viewportBottomY := viewportTopY + m.viewport.Height() - 1
		if mouse.Y < viewportTopY || mouse.Y > viewportBottomY {
			return m, nil
		}
		switch mouse.Button {
		case tea.MouseWheelUp:
			m.viewport.ScrollUp(3)
		case tea.MouseWheelDown:
			m.viewport.ScrollDown(3)
		}
		return m, nil

	case tea.PasteMsg:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
		m.refreshSlashSuggestions()
		m.applyLayout()
		return m, tea.Batch(cmds...)

	case agentEventMsg:
		if m.streamEvents != nil {
			return m, waitForAgentEvent(m.agentEvents)
		}
		return m.handleAgentEvent(msg.event)

	case approvalReqMsg:
		m.approvalActive = true
		m.approvalReq = msg.req
		m.approvalSelector.reset()
		m.syncViewport()
		return m, waitForApprovalRequest(m.approvalReqCh)

	case confEventMsg:
		return m.handleConfEvent(msg.event)

	case agentDoneMsg:
		m.processing = false
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			errStr := fmt.Sprintf("%v", msg.err)
			lines := strings.Split(errStr, "\n")
			for i, line := range lines {
				if i == 0 {
					m.appendLine(fmt.Sprintf("✗ Process error: %s", line))
				} else if line != "" {
					m.appendLine(fmt.Sprintf("  %s", line))
				}
			}
			m.syncViewport()
		}
		// Refocus textarea and reset cursor to ensure input method is positioned correctly
		m.textarea.Focus()
		m.textarea.CursorStart()
		return m, nil

	case streamEventMsg:
		return m.handleStreamEvent(msg.event)

	case streamDoneMsg:
		m.processing = false
		m.streamEvents = nil
		m.streamingStartLineIndex = -1
		m.streamingLineCount = 0
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			errStr := fmt.Sprintf("%v", msg.err)
			lines := strings.Split(errStr, "\n")
			for i, line := range lines {
				if i == 0 {
					m.appendLine(fmt.Sprintf("✗ Process error: %s", line))
				} else if line != "" {
					m.appendLine(fmt.Sprintf("  %s", line))
				}
			}
		}
		m.syncViewport()
		// Refocus textarea and reset cursor to ensure input method is positioned correctly
		m.textarea.Focus()
		m.textarea.CursorStart()
		return m, nil

	case confReloadDoneMsg:
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			errStr := fmt.Sprintf("%v", msg.err)
			lines := strings.Split(errStr, "\n")
			for i, line := range lines {
				if i == 0 {
					m.appendLine(fmt.Sprintf("✗ Reload error: %s", line))
				} else if line != "" {
					m.appendLine(fmt.Sprintf("  %s", line))
				}
			}
			m.syncViewport()
		}
		return m, nil

	case modelListMsg:
		if handled, cmd := m.modelSelector.handleMsg(msg); handled {
			m.applyLayout()
			return m, cmd
		}
		return m, nil

	case modelSelectMsg:
		if handled, cmd := m.modelSelector.handleMsg(msg); handled {
			// Update the model in the UI and save to config
			m.model = msg.modelID
			m.appendLine(fmt.Sprintf("✓ Model switched to: %s", msg.modelID))
			m.syncViewport()
			m.applyLayout()
			// Save the model to config file
			return m, tea.Batch(cmd, m.saveModelToConfig(msg.modelID))
		}
		return m, nil

	case processMsg:
		cmd := m.startProcess(msg.content)
		return m, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

const ctrlCInterval = 500 * time.Millisecond

func (m Model) handleCtrlC() (tea.Model, tea.Cmd) {
	now := time.Now()
	if now.Sub(m.lastCtrlC) < ctrlCInterval {
		m.cancel()
		return m, tea.Quit
	}
	m.lastCtrlC = now
	if m.processing && m.processCancel != nil {
		m.processCancel()
		m.processing = false
		m.streamEvents = nil
		m.processCancel = nil
		m.streamText.Reset()
		m.streamingStartLineIndex = -1
		m.streamingLineCount = 0
		m.activeThinking = nil
		m.appendLine("⚠ Cancelled (press Ctrl+C again to quit)")
	} else {
		m.appendLine("Press Ctrl+C again to quit")
	}
	m.syncViewport()
	return m, nil
}
