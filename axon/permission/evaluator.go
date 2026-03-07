package permission

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/axon/permission/approval"
	"github.com/looplj/axonhub/axon/permission/extractor"
	"github.com/looplj/axonhub/axon/permission/grant"
	"github.com/looplj/axonhub/axon/permission/policy"
)

type Evaluator struct {
	logger    *slog.Logger
	extractor extractor.Extractor
	policy    *policy.Engine
	approver  approval.Service
	grants    grant.Store
}

type EvaluatorOptions struct {
	Logger    *slog.Logger
	Extractor extractor.Extractor
	Policy    *policy.Engine
	Approver  approval.Service
	Grants    grant.Store
}

func NewEvaluator(opts EvaluatorOptions) *Evaluator {
	l := opts.Logger
	if l == nil {
		l = slog.Default()
	}
	ex := opts.Extractor
	if ex == nil {
		ex = extractor.DefaultExtractor{}
	}
	return &Evaluator{
		logger:    l,
		extractor: ex,
		policy:    opts.Policy,
		approver:  opts.Approver,
		grants:    opts.Grants,
	}
}

func (e *Evaluator) Evaluate(ctx context.Context, req ToolRequest) error {
	if req.StartedAt.IsZero() {
		req.StartedAt = time.Now()
	}

	extracted, err := e.extractor.Extract(req.Workspace, req.ToolName, req.ToolInput)
	if err != nil {
		e.logger.Warn("permission: resource extraction failed, forcing approval",
			"tool", req.ToolName,
			"err", err,
		)
		decision := ToolDecision{
			Effect:    EffectRequireApproval,
			RuleID:    "extract.failed",
			Reason:    fmt.Sprintf("cannot extract resources safely: %v; approval required", err),
			RiskLevel: RiskHigh,
		}
		decision.Display = DecisionDisplay{
			Summary: fmt.Sprintf("%s (%s)", req.ToolName, decision.Effect),
			Details: []string{
				"rule: " + decision.RuleID,
				"reason: " + decision.Reason,
				"risk: " + string(decision.RiskLevel),
			},
		}
		return e.handleDecision(ctx, req, decision)
	}

	// Extractor returns the shared policy.Resource type used throughout the
	// permission stack. Keep resources as-is to avoid drift between layers.
	resources := extracted

	if dec, ok := HardDeny(req.ToolName, resources, req.Workspace); ok {
		e.auditDecision(req, dec)
		return fmt.Errorf("%w: %s: %s", ErrToolCallBlocked, dec.RuleID, dec.Reason)
	}

	pd := e.policy.Evaluate(req.ToolName, resources)
	if pd.Effect == policy.EffectDeny {
		decision := ToolDecision{
			Effect:    EffectDeny,
			RuleID:    pd.RuleID,
			Reason:    pd.Reason,
			RiskLevel: RiskLevel(pd.RiskLevel),
			Resources: resources,
		}
		decision.Display = DecisionDisplay{
			Summary:   fmt.Sprintf("%s (%s)", req.ToolName, decision.Effect),
			Resources: resources,
			Details: []string{
				"rule: " + decision.RuleID,
				"reason: " + decision.Reason,
				"risk: " + string(decision.RiskLevel),
			},
		}

		return e.handleDecision(ctx, req, decision)
	}

	if e.grants.Match(toGrantRequest(req), toGrantResources(resources)) {
		dec := ToolDecision{
			Effect:    EffectAllow,
			RuleID:    "grant.match",
			Reason:    "allowed by previously granted approval",
			RiskLevel: RiskLow,
			Resources: resources,
		}
		e.auditDecision(req, dec)
		return nil
	}

	decision := ToolDecision{
		Effect:    Effect(pd.Effect),
		RuleID:    pd.RuleID,
		Reason:    pd.Reason,
		RiskLevel: RiskLevel(pd.RiskLevel),
		Resources: resources,
	}

	decision.Display = DecisionDisplay{
		Summary:   fmt.Sprintf("%s (%s)", req.ToolName, decision.Effect),
		Resources: resources,
		Details: []string{
			"rule: " + decision.RuleID,
			"reason: " + decision.Reason,
			"risk: " + string(decision.RiskLevel),
		},
	}

	return e.handleDecision(ctx, req, decision)
}

func (e *Evaluator) handleDecision(ctx context.Context, req ToolRequest, decision ToolDecision) error {
	switch decision.Effect {
	case EffectAllow:
		e.auditDecision(req, decision)
		return nil
	case EffectDeny:
		e.auditDecision(req, decision)
		return fmt.Errorf("%w: %s: %s", ErrToolCallDenied, decision.RuleID, decision.Reason)
	case EffectRequireApproval:
		resp, err := e.approver.Request(ctx, approval.Request{
			ID:         uuid.NewString(),
			ThreadID:   req.ThreadID,
			Workspace:  req.Workspace,
			ToolCallID: req.ToolCallID,
			ToolName:   req.ToolName,
			Summary:    decision.Display.Summary,
			Reason:     decision.Reason,
			RiskLevel:  string(decision.RiskLevel),
			Resources:  json.RawMessage(mustJSON(decision.Resources)),
		})
		if err != nil {
			e.auditDecision(req, decision)
			return err
		}

		if resp.Granted {
			resourcesToGrant := decision.Resources
			if len(resp.Resources) > 0 {
				resourcesToGrant = parseSelectedResources(resp.Resources)
			}
			e.grants.Add(toGrantRequest(req), resp.Scope, toGrantResources(resourcesToGrant))

			//nolint:exhaustive // ScopeThread is not supported.
			switch resp.Scope {
			case grant.ScopeWorkspace:
				_ = e.grants.SaveWorkspace(req.Workspace)
			case grant.ScopeGlobal:
				_ = e.grants.SaveGlobal()
			}
			decision.RuleID = "approval.granted"
			decision.Reason = "approved by user"
			decision.Effect = EffectAllow
			decision.RiskLevel = RiskLow
		} else {
			decision.RuleID = "approval.denied"
			decision.Reason = "denied by user"
			decision.Effect = EffectDeny
		}
		e.auditDecision(req, decision)
		if decision.Effect == EffectDeny {
			return fmt.Errorf("%w: %s: %s", ErrToolCallDenied, decision.RuleID, decision.Reason)
		}
		return nil
	default:
		e.auditDecision(req, decision)
		return fmt.Errorf("unknown decision effect: %s", decision.Effect)
	}
}

func (e *Evaluator) LogToolResult(req ToolRequest, toolErr error) {
	if toolErr != nil {
		e.logger.Info("permission: tool execution error",
			"tool_call_id", req.ToolCallID,
			"tool", req.ToolName,
			"thread_id", req.ThreadID,
			"err", toolErr.Error(),
		)
		return
	}
	e.logger.Info("permission: tool execution ok",
		"tool_call_id", req.ToolCallID,
		"tool", req.ToolName,
		"thread_id", req.ThreadID,
	)
}

func (e *Evaluator) auditDecision(req ToolRequest, dec ToolDecision) {
	e.logger.Info("permission: decision",
		"tool_call_id", req.ToolCallID,
		"tool", req.ToolName,
		"thread_id", req.ThreadID,
		"workspace", req.Workspace,
		"effect", dec.Effect,
		"rule_id", dec.RuleID,
		"risk", dec.RiskLevel,
		"reason", dec.Reason,
	)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func toGrantRequest(req ToolRequest) grant.Request {
	return grant.Request{
		ToolCallID: req.ToolCallID,
		ThreadID:   req.ThreadID,
		Workspace:  req.Workspace,
		ToolName:   req.ToolName,
	}
}

func toGrantResources(in []policy.Resource) []grant.Resource {
	out := make([]grant.Resource, 0, len(in))
	for _, r := range in {
		gr := grant.Resource{
			Path:             r.Path,
			WorkspaceRel:     r.WorkspaceRel,
			OutsideWorkspace: r.OutsideWorkspace,
			Domain:           r.Domain,
			Command:          r.Command,
			Skill:            r.Skill,
		}
		switch r.Type {
		case policy.ResourcePath:
			gr.Type = grant.ResourcePath
		case policy.ResourceDir:
			gr.Type = grant.ResourceDir
		case policy.ResourceDomain:
			gr.Type = grant.ResourceDomain
		case policy.ResourceCommand:
			gr.Type = grant.ResourceCommand
		case policy.ResourceSkill:
			gr.Type = grant.ResourceSkill
		default:
			continue
		}
		out = append(out, gr)
	}
	return out
}

func parseSelectedResources(raw []json.RawMessage) []policy.Resource {
	out := make([]policy.Resource, 0, len(raw))
	for _, r := range raw {
		var res policy.Resource
		if err := json.Unmarshal(r, &res); err == nil {
			out = append(out, res)
		}
	}
	return out
}
