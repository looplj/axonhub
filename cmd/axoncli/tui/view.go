package tui

import (
	"fmt"
	"math/rand"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Logo text - AXON
var logoLines = []string{
	`  █████   ██   ██   ██████   ███    ██ `,
	` ██   ██   ██ ██   ██    ██  ████   ██ `,
	` ███████    ███    ██    ██  ██ ██  ██ `,
	` ██   ██   ██ ██   ██    ██  ██  ██ ██ `,
	` ██   ██  ██   ██   ██████   ██   ████ `,
}

// Help tips to display below logo
var helpTips = []string{
	"Press Enter to start chatting with Axon",
	"Use /help to see all available commands",
	"Press Ctrl+C twice to exit",
	"Use Shift+Enter to insert a new line",
	"Type /models to switch between AI models",
	"Use /clear to clear the conversation",
	"Drag to select text, release to copy",
	"Use ↑/↓ or PgUp/PgDn to scroll through history",
	"Type /messages to view conversation history",
}

func getRandomTip() string {
	return helpTips[rand.Intn(len(helpTips))]
}

var (
	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	tipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	welcomeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)
)

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(1)

	processingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("241")).
			Padding(0, 0)
)

// View renders the full TUI layout.
func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	if !m.ready {
		v.SetContent("Initializing...")
		return v
	}

	if m.approvalActive {
		v.SetContent(m.renderApprovalModal())
		return v
	}

	// Show logo screen if enabled and no messages yet
	if m.showLogo && len(m.lines) == 0 {
		v.SetContent(m.renderLogoScreen())
		return v
	}

	var b strings.Builder

	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")
	if m.slashActive {
		b.WriteString(m.renderSlashDropdown())
		b.WriteString("\n")
	}
	if m.modelSelector.active {
		b.WriteString(m.modelSelector.render(m.width))
		b.WriteString("\n")
	}
	// Render textarea inside a bordered box
	b.WriteString(inputBoxStyle.Render(m.textarea.View()))

	v.SetContent(b.String())
	return v
}

func (m Model) renderHeader() string {
	var b strings.Builder
	b.WriteString(infoStyle.Render(fmt.Sprintf("Model: %s", m.model)))
	b.WriteString(" · ")
	b.WriteString(infoStyle.Render(fmt.Sprintf("Workspace: %s", m.workspace)))
	b.WriteString(" · ")
	b.WriteString(infoStyle.Render(fmt.Sprintf("Thread: thread-%s", m.threadID)))
	return b.String()
}

func (m Model) renderLogoScreen() string {
	var b strings.Builder

	// Calculate vertical centering
	logoHeight := len(logoLines) + 8 // logo + info(3 lines) + welcome + tip + spacing
	availableHeight := m.height - 3  // Reserve space for input box
	if m.slashActive {
		availableHeight -= m.slashExtraHeight()
	}
	topPadding := (availableHeight - logoHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	// Add top padding
	for i := 0; i < topPadding; i++ {
		b.WriteString("\n")
	}

	// Render logo centered using lipgloss for proper ANSI handling
	for _, line := range logoLines {
		centeredLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, logoStyle.Render(line))
		b.WriteString(centeredLine)
		b.WriteString("\n")
	}

	// Add spacing
	b.WriteString("\n")

	// Render info lines (model, workspace, thread) - each on its own line
	infoLines := []string{
		fmt.Sprintf("Model: %s", m.model),
		fmt.Sprintf("Workspace: %s", m.workspace),
		fmt.Sprintf("Thread: thread-%s", m.threadID),
	}
	for _, line := range infoLines {
		centeredLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, infoStyle.Render(line))
		b.WriteString(centeredLine)
		b.WriteString("\n")
	}

	// Add spacing
	b.WriteString("\n")

	// Render welcome message
	welcomeMsg := "Welcome to Axon"
	centeredWelcome := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, welcomeStyle.Render(welcomeMsg))
	b.WriteString(centeredWelcome)
	b.WriteString("\n")

	// Add spacing
	b.WriteString("\n")

	// Render random tip
	centeredTip := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, tipStyle.Render(m.logoTip))
	b.WriteString(centeredTip)
	b.WriteString("\n")

	// Fill remaining space and render input box at bottom
	for i := topPadding + logoHeight; i < availableHeight-1; i++ {
		b.WriteString("\n")
	}

	// Render slash dropdown if active
	if m.slashActive {
		b.WriteString(m.renderSlashDropdown())
		b.WriteString("\n")
	}

	// Render textarea at the bottom
	b.WriteString(inputBoxStyle.Render(m.textarea.View()))

	return b.String()
}

func (m Model) renderStatusBar() string {
	if m.processing {
		return processingStyle.Render(m.spinner.View() + " Processing...")
	}
	if m.hasSelection() {
		return statusBarStyle.Render("Release: copied to clipboard · Esc: clear selection · ↑↓/PgUp/PgDn: scroll")
	}
	if len(m.thinkingBlocks) > 0 {
		return statusBarStyle.Render("Enter: send · Shift+Enter/Ctrl+J: newline · drag: select (release to copy) · ↑↓/PgUp/PgDn: scroll · t: toggle thinking · click ▶/▼: toggle thinking · Esc: cancel · /help: commands")
	}
	return statusBarStyle.Render("Enter: send · Shift+Enter/Ctrl+J: newline · drag: select (release to copy) · ↑↓/PgUp/PgDn: scroll · Esc: cancel · /help: commands")
}

var (
	slashItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(1)

	slashSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("205")).
				PaddingLeft(1)
)

func (m Model) renderSlashDropdown() string {
	if !m.slashActive || len(m.slashMatches) == 0 {
		return ""
	}

	start := m.slashOffset
	end := start + m.slashVisibleCount()
	if end > len(m.slashMatches) {
		end = len(m.slashMatches)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			b.WriteString("\n")
		}
		item := m.slashMatches[i]
		line := fmt.Sprintf("%-14s %s", item.Command, item.Description)
		if i == m.slashIndex {
			b.WriteString(slashSelectedStyle.Render(line))
		} else {
			b.WriteString(slashItemStyle.Render(line))
		}
	}
	return b.String()
}
