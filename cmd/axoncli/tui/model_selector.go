package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/looplj/axonhub/cmd/axoncli/conf"
)

// ModelInfo holds information about an AI model
type ModelInfo struct {
	ID          string
	DisplayName string
	CreatedAt   string
}

// modelSelector manages the model selection UI
type modelSelector struct {
	active      bool
	models      []ModelInfo
	filtered    []ModelInfo
	selectedIdx int
	offset      int
	searchQuery string
	width       int
	height      int
	configDir   string
	baseURL     string
	apiKey      string
	err         error
	loading     bool
}

const maxModelVisible = 10

var (
	modelHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)

	modelItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(2)

	modelSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("205")).
				PaddingLeft(2)

	modelSearchStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)

	modelGroupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Bold(true).
			PaddingLeft(1)

	modelErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
)

// modelListMsg is sent when models are loaded
type modelListMsg struct {
	models []ModelInfo
	err    error
}

// modelSelectMsg is sent when a model is selected
type modelSelectMsg struct {
	modelID string
}

func newModelSelector(configDir, baseURL, apiKey string, onSelect func(modelID string)) *modelSelector {
	return &modelSelector{
		configDir: configDir,
		baseURL:   baseURL,
		apiKey:    apiKey,
		models:    make([]ModelInfo, 0),
		filtered:  make([]ModelInfo, 0),
	}
}

func (ms *modelSelector) open() tea.Cmd {
	ms.active = true
	ms.loading = true
	ms.err = nil
	ms.searchQuery = ""
	ms.selectedIdx = 0
	ms.offset = 0
	return ms.fetchModels()
}

func (ms *modelSelector) close() {
	ms.active = false
	ms.models = nil
	ms.filtered = nil
	ms.err = nil
	ms.loading = false
}

func (ms *modelSelector) fetchModels() tea.Cmd {
	return func() tea.Msg {
		client := anthropic.NewClient(
			option.WithBaseURL(ms.baseURL),
			option.WithAPIKey(ms.apiKey),
		)

		ctx := context.Background()
		page, err := client.Models.List(ctx, anthropic.ModelListParams{})
		if err != nil {
			return modelListMsg{err: fmt.Errorf("failed to fetch models: %w", err)}
		}

		models := make([]ModelInfo, 0, len(page.Data))
		for _, m := range page.Data {
			models = append(models, ModelInfo{
				ID:          m.ID,
				DisplayName: m.DisplayName,
				CreatedAt:   m.CreatedAt.Format("2006-01-02"),
			})
		}

		return modelListMsg{models: models}
	}
}

func (ms *modelSelector) handleMsg(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case modelListMsg:
		ms.loading = false
		if msg.err != nil {
			ms.err = msg.err
			return true, nil
		}
		ms.models = msg.models
		ms.applyFilter()
		return true, nil

	case modelSelectMsg:
		ms.close()
		return true, nil
	}
	return false, nil
}

func (ms *modelSelector) handleKey(key string) (bool, tea.Cmd) {
	if !ms.active {
		return false, nil
	}

	switch key {
	case "esc":
		ms.close()
		return true, nil
	case "up":
		ms.moveSelection(-1)
		return true, nil
	case "down":
		ms.moveSelection(1)
		return true, nil
	case "enter":
		if len(ms.filtered) > 0 && ms.selectedIdx >= 0 && ms.selectedIdx < len(ms.filtered) {
			selected := ms.filtered[ms.selectedIdx]
			return true, func() tea.Msg {
				return modelSelectMsg{modelID: selected.ID}
			}
		}
		return true, nil
	case "backspace":
		if len(ms.searchQuery) > 0 {
			ms.searchQuery = ms.searchQuery[:len(ms.searchQuery)-1]
			ms.applyFilter()
		}
		return true, nil
	}

	// Handle character input for search
	if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
		ms.searchQuery += key
		ms.applyFilter()
		return true, nil
	}

	return false, nil
}

func (ms *modelSelector) moveSelection(delta int) {
	if len(ms.filtered) == 0 {
		return
	}
	n := len(ms.filtered)
	ms.selectedIdx = (ms.selectedIdx + delta + n) % n
	ms.ensureVisible()
}

func (ms *modelSelector) ensureVisible() {
	visible := ms.visibleCount()
	if visible == 0 {
		ms.offset = 0
		return
	}

	if ms.selectedIdx < ms.offset {
		ms.offset = ms.selectedIdx
		return
	}

	if ms.selectedIdx >= ms.offset+visible {
		ms.offset = ms.selectedIdx - visible + 1
		return
	}
}

func (ms *modelSelector) visibleCount() int {
	if !ms.active || len(ms.filtered) == 0 {
		return 0
	}
	if len(ms.filtered) < maxModelVisible {
		return len(ms.filtered)
	}
	return maxModelVisible
}

func (ms *modelSelector) extraHeight() int {
	if !ms.active {
		return 0
	}
	// Header (1) + search box (1) + separator (1) + items + padding (1)
	return 4 + ms.visibleCount()
}

func (ms *modelSelector) applyFilter() {
	if ms.searchQuery == "" {
		ms.filtered = make([]ModelInfo, len(ms.models))
		copy(ms.filtered, ms.models)
	} else {
		query := strings.ToLower(ms.searchQuery)
		ms.filtered = make([]ModelInfo, 0, len(ms.models))
		for _, m := range ms.models {
			if strings.Contains(strings.ToLower(m.ID), query) ||
				strings.Contains(strings.ToLower(m.DisplayName), query) {
				ms.filtered = append(ms.filtered, m)
			}
		}
	}
	ms.selectedIdx = 0
	ms.offset = 0
	ms.ensureVisible()
}

func (ms *modelSelector) render(width int) string {
	if !ms.active {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(modelHeaderStyle.Render("Select model"))
	b.WriteString("\n")

	// Search box
	searchDisplay := ms.searchQuery
	if searchDisplay == "" {
		searchDisplay = "Search..."
	}
	b.WriteString(modelSearchStyle.Render(searchDisplay))
	b.WriteString("\n")

	// Separator
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")

	// Content
	if ms.loading {
		b.WriteString("  Loading models...")
	} else if ms.err != nil {
		b.WriteString(modelErrorStyle.Render(fmt.Sprintf("  Error: %v", ms.err)))
	} else if len(ms.filtered) == 0 {
		b.WriteString("  No models found")
	} else {
		start := ms.offset
		end := start + ms.visibleCount()
		if end > len(ms.filtered) {
			end = len(ms.filtered)
		}

		for i := start; i < end; i++ {
			model := ms.filtered[i]
			line := fmt.Sprintf("● %s", model.DisplayName)
			if model.DisplayName != model.ID {
				line = fmt.Sprintf("● %s (%s)", model.DisplayName, model.ID)
			}

			if i == ms.selectedIdx {
				b.WriteString(modelSelectedStyle.Render(line))
			} else {
				b.WriteString(modelItemStyle.Render(line))
			}
			if i < end-1 {
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

// getCurrentModelID returns the currently selected model ID
func (ms *modelSelector) getCurrentModelID() string {
	if len(ms.filtered) > 0 && ms.selectedIdx >= 0 && ms.selectedIdx < len(ms.filtered) {
		return ms.filtered[ms.selectedIdx].ID
	}
	return ""
}

// loadConfig loads the current config to get baseURL and apiKey
func (ms *modelSelector) loadConfig() error {
	cfg, err := conf.LoadEffectiveConfig(ms.configDir)
	if err != nil {
		return err
	}
	ms.baseURL = cfg.BaseURL
	ms.apiKey = cfg.APIKey
	return nil
}
