package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/looplj/axonhub/axon/permission/grant"
)

type approvalChoice struct {
	label string
	scope grant.Scope
	deny  bool
}

type approvalSelector struct {
	choices     []approvalChoice
	selectedIdx int
}

var (
	approvalItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				PaddingLeft(2)

	approvalSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("205")).
				PaddingLeft(2)

	approvalDenyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				PaddingLeft(2)

	approvalDenySelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("15")).
					Background(lipgloss.Color("196")).
					PaddingLeft(2)
)

func newApprovalSelector() *approvalSelector {
	return &approvalSelector{
		choices: []approvalChoice{
			{label: "Approve (once)", scope: grant.ScopeOnce, deny: false},
			{label: "Approve (thread)", scope: grant.ScopeThread, deny: false},
			{label: "Approve (workspace)", scope: grant.ScopeWorkspace, deny: false},
			{label: "Deny", scope: "", deny: true},
		},
		selectedIdx: 0,
	}
}

func (as *approvalSelector) reset() {
	as.selectedIdx = 0
}

func (as *approvalSelector) moveSelection(delta int) {
	n := len(as.choices)
	as.selectedIdx = (as.selectedIdx + delta + n) % n
}

func (as *approvalSelector) handleKey(key string) (bool, tea.Cmd) {
	switch key {
	case "up":
		as.moveSelection(-1)
		return true, nil
	case "down":
		as.moveSelection(1)
		return true, nil
	case "enter":
		return true, nil
	}
	return false, nil
}

func (as *approvalSelector) currentChoice() approvalChoice {
	if as.selectedIdx >= 0 && as.selectedIdx < len(as.choices) {
		return as.choices[as.selectedIdx]
	}
	return as.choices[0]
}

func (as *approvalSelector) render() string {
	var result string
	for i, choice := range as.choices {
		line := "● " + choice.label
		if i == as.selectedIdx {
			if choice.deny {
				result += approvalDenySelectedStyle.Render(line) + "\n"
			} else {
				result += approvalSelectedStyle.Render(line) + "\n"
			}
		} else {
			if choice.deny {
				result += approvalDenyStyle.Render(line) + "\n"
			} else {
				result += approvalItemStyle.Render(line) + "\n"
			}
		}
	}
	return result
}
