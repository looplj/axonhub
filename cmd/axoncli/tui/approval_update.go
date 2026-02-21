package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/looplj/axonhub/axon/permission/approval"
)

func (m Model) handleApprovalKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "down":
		m.approvalSelector.handleKey(key)
		return m, nil
	case "enter":
		choice := m.approvalSelector.currentChoice()
		if choice.deny {
			if m.approvalSvc != nil {
				_ = m.approvalSvc.Deny(m.approvalReq)
			}
			m.appendLine(fmt.Sprintf("✗ Denied %s", m.approvalReq.ToolName))
		} else {
			if m.approvalSvc != nil {
				_ = m.approvalSvc.Grant(m.approvalReq, choice.scope)
			}
			m.appendLine(fmt.Sprintf("✓ Approved %s (scope: %s)", m.approvalReq.ToolName, choice.scope))
		}
		m.approvalActive = false
		m.approvalReq = approval.Request{}
		m.approvalSelector.reset()
		m.syncViewport()
		return m, waitForApprovalRequest(m.approvalReqCh)
	case "esc":
		if m.approvalSvc != nil {
			_ = m.approvalSvc.Deny(m.approvalReq)
		}
		m.appendLine(fmt.Sprintf("✗ Denied %s", m.approvalReq.ToolName))
		m.approvalActive = false
		m.approvalReq = approval.Request{}
		m.approvalSelector.reset()
		m.syncViewport()
		return m, waitForApprovalRequest(m.approvalReqCh)
	}
	return m, nil
}
