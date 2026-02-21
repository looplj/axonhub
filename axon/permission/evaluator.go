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

	caps := e.extractor.Capabilities(req.ToolName)
	if len(caps) == 0 {
		caps = []string{"unknown"}
	}

	extracted, err := e.extractor.Extract(req.Workspace, req.ToolName, req.ToolInput)
	if err != nil {
		e.logger.Warn("permission: resource extraction failed, forcing approval",
			"tool", req.ToolName,
			"err", err,
		)
		decision := ToolDecision{
			Effect:       EffectRequireApproval,
			RuleID:       "extract.failed",
			Reason:       fmt.Sprintf("cannot extract resources safely: %v; approval required", err),
			RiskLevel:    RiskHigh,
			Capabilities: caps,
		}
		decision.Display = DecisionDisplay{
			Summary: fmt.Sprintf("%s (%s)", req.ToolName, decision.Effect),
			Details: []string{
				"rule: " + decision.RuleID,
				"reason: " + decision.Reason,
				"risk: " + string(decision.RiskLevel),
			},
		}
		return e.handleDecision(ctx, req, decision, caps)
	}

	resources := fromExtractorResources(extracted)

	if dec, ok := HardDeny(caps, resources, req.Workspace); ok {
		e.auditDecision(req, dec)
		return fmt.Errorf("%w: %s: %s", ErrToolCallBlocked, dec.RuleID, dec.Reason)
	}

	for _, c := range caps {
		if e.grants.Match(toGrantRequest(req), c, toGrantResources(resources)) {
			dec := ToolDecision{
				Effect:       EffectAllow,
				RuleID:       "grant.match",
				Reason:       "allowed by previously granted approval",
				RiskLevel:    RiskLow,
				Capabilities: caps,
				Resources:    resources,
			}
			e.auditDecision(req, dec)
			return nil
		}
	}

	pres := toPolicyResources(resources)
	pd := e.policy.Evaluate(caps, pres)
	decision := ToolDecision{
		Effect:       Effect(pd.Effect),
		RuleID:       pd.RuleID,
		Reason:       pd.Reason,
		RiskLevel:    RiskLevel(pd.RiskLevel),
		Capabilities: caps,
		Resources:    resources,
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

	return e.handleDecision(ctx, req, decision, caps)
}

func (e *Evaluator) handleDecision(ctx context.Context, req ToolRequest, decision ToolDecision, caps []string) error {
	switch decision.Effect {
	case EffectAllow:
		e.auditDecision(req, decision)
		return nil
	case EffectDeny:
		e.auditDecision(req, decision)
		return fmt.Errorf("%w: %s: %s", ErrToolCallDenied, decision.RuleID, decision.Reason)
	case EffectRequireApproval:
		resp, err := e.approver.Request(ctx, approval.Request{
			ID:           uuid.NewString(),
			ThreadID:     req.ThreadID,
			Workspace:    req.Workspace,
			ToolCallID:   req.ToolCallID,
			ToolName:     req.ToolName,
			Capabilities: caps,
			Summary:      decision.Display.Summary,
			Reason:       decision.Reason,
			RiskLevel:    string(decision.RiskLevel),
			Resources:    json.RawMessage(mustJSON(decision.Resources)),
		})
		if err != nil {
			e.auditDecision(req, decision)
			return err
		}

		if resp.Granted {
			for _, c := range caps {
				e.grants.Add(toGrantRequest(req), resp.Scope, c, toGrantResources(decision.Resources))
			}
			if resp.Scope == grant.ScopeWorkspace {
				_ = e.grants.SaveWorkspace(req.Workspace)
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

func fromExtractorResources(in []extractor.Resource) []Resource {
	out := make([]Resource, 0, len(in))
	for _, r := range in {
		pr := Resource{
			Path:             r.Path,
			WorkspaceRel:     r.WorkspaceRel,
			OutsideWorkspace: r.OutsideWorkspace,
			Command:          r.Command,
			Cwd:              r.Cwd,
			URL:              r.URL,
			Domain:           r.Domain,
			Scheme:           r.Scheme,
		}
		switch r.Type {
		case extractor.ResourcePath:
			pr.Type = ResourcePath
		case extractor.ResourceCommand:
			pr.Type = ResourceCommand
		case extractor.ResourceURL:
			pr.Type = ResourceURL
		case extractor.ResourceDomain:
			pr.Type = ResourceDomain
		}
		out = append(out, pr)
	}
	return out
}

func toPolicyResources(in []Resource) []policy.Resource {
	out := make([]policy.Resource, 0, len(in))
	for _, r := range in {
		pr := policy.Resource{
			Path:             r.Path,
			WorkspaceRel:     r.WorkspaceRel,
			OutsideWorkspace: r.OutsideWorkspace,
			Command:          r.Command,
			Cwd:              r.Cwd,
			URL:              r.URL,
			Domain:           r.Domain,
			Scheme:           r.Scheme,
		}
		switch r.Type {
		case ResourcePath:
			pr.Type = policy.ResourcePath
		case ResourceCommand:
			pr.Type = policy.ResourceCommand
		case ResourceURL:
			pr.Type = policy.ResourceURL
		case ResourceDomain:
			pr.Type = policy.ResourceDomain
		}
		out = append(out, pr)
	}
	return out
}

func toGrantRequest(req ToolRequest) grant.Request {
	return grant.Request{
		ToolCallID: req.ToolCallID,
		ThreadID:   req.ThreadID,
		Workspace:  req.Workspace,
		ToolName:   req.ToolName,
	}
}

func toGrantResources(in []Resource) []grant.Resource {
	out := make([]grant.Resource, 0, len(in))
	for _, r := range in {
		gr := grant.Resource{
			Path:             r.Path,
			WorkspaceRel:     r.WorkspaceRel,
			OutsideWorkspace: r.OutsideWorkspace,
			Domain:           r.Domain,
			Command:          r.Command,
		}
		switch r.Type {
		case ResourcePath:
			gr.Type = grant.ResourcePath
		case ResourceDomain:
			gr.Type = grant.ResourceDomain
		case ResourceCommand:
			gr.Type = grant.ResourceCommand
		default:
			continue
		}
		out = append(out, gr)
	}
	return out
}
