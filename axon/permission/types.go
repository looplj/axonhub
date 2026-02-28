package permission

import (
	"encoding/json"
	"time"
)

type Effect string

const (
	EffectAllow           Effect = "allow"
	EffectDeny            Effect = "deny"
	EffectRequireApproval Effect = "require_approval"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type ToolRequest struct {
	ThreadID   string
	Workspace  string
	ToolCallID string
	ToolName   string
	ToolInput  json.RawMessage
	StartedAt  time.Time
}

type ResourceType string

const (
	ResourcePath    ResourceType = "path"
	ResourceCommand ResourceType = "command"
	ResourceURL     ResourceType = "url"
	ResourceDomain  ResourceType = "domain"
)

type Resource struct {
	Type ResourceType `json:"type"`

	// Path
	Path             string `json:"path,omitempty"` // absolute, cleaned
	WorkspaceRel     string `json:"workspace_rel,omitempty"`
	OutsideWorkspace bool   `json:"outside_workspace,omitempty"`

	// Command
	Command string `json:"command,omitempty"`
	Cwd     string `json:"cwd,omitempty"`

	// Network
	URL    string `json:"url,omitempty"`    // redacted
	Domain string `json:"domain,omitempty"` // host
	Scheme string `json:"scheme,omitempty"`
}

type DecisionDisplay struct {
	Summary   string     `json:"summary"`
	Details   []string   `json:"details,omitempty"`
	Resources []Resource `json:"resources,omitempty"`
}

type ToolDecision struct {
	Effect    Effect          `json:"effect"`
	RuleID    string          `json:"rule_id,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	RiskLevel RiskLevel       `json:"risk_level,omitempty"`
	Display   DecisionDisplay `json:"display,omitempty"`

	Resources []Resource `json:"resources,omitempty"`
}
