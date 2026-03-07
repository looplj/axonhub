package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/looplj/axonhub/axon/permission/policy"
)

var (
	approvalBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("205")).
				Padding(1, 2)

	approvalTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)

	approvalDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	approvalWarnStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Bold(true)
)

func (m Model) renderApprovalModal() string {
	req := m.approvalReq
	var b strings.Builder

	title := fmt.Sprintf("Permission Approval Required (%s)", strings.ToUpper(req.RiskLevel))
	b.WriteString(approvalTitleStyle.Render(title))
	b.WriteString("\n")

	fmt.Fprintf(&b, "Tool: %s\n", req.ToolName)
	if req.Reason != "" {
		fmt.Fprintf(&b, "Reason: %s\n", req.Reason)
	}

	if len(req.Resources) > 0 {
		if res := formatApprovalResources(req.Resources); res != "" {
			b.WriteString("\n")
			b.WriteString(approvalDimStyle.Render("Resources:"))
			b.WriteString("\n")
			b.WriteString(res)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(approvalWarnStyle.Render("Select action:"))
	b.WriteString("\n")
	b.WriteString(m.approvalSelector.render())
	b.WriteString(approvalDimStyle.Render("  ↑/↓: navigate  Enter: confirm  Esc: deny"))

	content := approvalBoxStyle.Width(min(m.width-4, 100)).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func formatApprovalResources(raw json.RawMessage) string {
	var resources []policy.Resource
	if err := json.Unmarshal(raw, &resources); err != nil {
		return approvalDimStyle.Render("  (unparseable resources)")
	}
	if len(resources) == 0 {
		return ""
	}

	var lines []string
	for _, r := range resources {
		switch r.Type {
		case policy.ResourcePath:
			p := r.Path
			if r.WorkspaceRel != "" {
				p = r.WorkspaceRel
			}
			extra := ""
			if r.OutsideWorkspace {
				extra = " (outside workspace)"
			}
			lines = append(lines, fmt.Sprintf("  - path: %s%s", p, extra))
		case policy.ResourceURL:
			lines = append(lines, fmt.Sprintf("  - url: %s", r.URL))
		case policy.ResourceDomain:
			lines = append(lines, fmt.Sprintf("  - domain: %s", r.Domain))
		case policy.ResourceCommand:
			cmd := truncateStr(r.Command, 120)
			lines = append(lines, fmt.Sprintf("  - command: %s", cmd))
			if r.Cwd != "" {
				lines = append(lines, fmt.Sprintf("    cwd: %s", r.Cwd))
			}
		case policy.ResourceDir:
			p := r.Path
			if r.WorkspaceRel != "" {
				p = r.WorkspaceRel
			}
			extra := ""
			if r.OutsideWorkspace {
				extra = " (outside workspace)"
			}
			lines = append(lines, fmt.Sprintf("  - dir: %s%s", p, extra))
		default:
			// ignore unknown for now
		}
	}
	return approvalDimStyle.Render(strings.Join(lines, "\n"))
}
