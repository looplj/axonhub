package permission

import (
	"encoding/json"
	"time"

	"github.com/looplj/axonhub/axon/permission/policy"
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

type DecisionDisplay struct {
	Summary   string            `json:"summary"`
	Details   []string          `json:"details,omitempty"`
	Resources []policy.Resource `json:"resources,omitempty"`
}

type ToolDecision struct {
	Effect    Effect          `json:"effect"`
	RuleID    string          `json:"rule_id,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	RiskLevel RiskLevel       `json:"risk_level,omitempty"`
	Display   DecisionDisplay `json:"display,omitempty"`

	Resources []policy.Resource `json:"resources,omitempty"`
}
