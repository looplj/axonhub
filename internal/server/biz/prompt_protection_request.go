package biz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

var ErrPromptProtectionRejected = errors.New("prompt protection rejected request")

type PromptProtectionResult struct {
	Request      *llm.Request
	MatchedRules []*ent.PromptProtectionRule
	Rejected     bool
}

// ApplyPromptProtectionRules applies prompt protection rules to a request.
func ApplyPromptProtectionRules(req *llm.Request, rules []*ent.PromptProtectionRule) PromptProtectionResult {
	if req != nil && req.Decisions != nil && len(rules) > 0 {
		return applyDecisionsPromptProtectionRules(req, rules)
	}
	if req != nil && req.Compact != nil && len(rules) > 0 {
		return applyCompactPromptProtectionRules(req, rules)
	}
	if req == nil || len(req.Messages) == 0 || len(rules) == 0 {
		return PromptProtectionResult{Request: req}
	}

	messages := req.Messages

	var matchedRules []*ent.PromptProtectionRule

	for _, rule := range rules {
		if rule == nil || rule.Settings == nil {
			continue
		}

		var ruleMatches bool

		for i, msg := range messages {
			if !promptProtectionRuleAppliesToRole(rule.Settings.Scopes, msg.Role) {
				continue
			}

			updatedMsg, msgApplied := applyPromptProtectionRuleToMessage(msg, rule)
			if msgApplied {
				if rule.Settings.Action == objects.PromptProtectionActionReject {
					return PromptProtectionResult{
						MatchedRules: []*ent.PromptProtectionRule{rule},
						Rejected:     true,
					}
				}

				messages[i] = updatedMsg
				ruleMatches = true
			}
		}

		if !ruleMatches {
			continue
		}

		matchedRules = append(matchedRules, rule)
	}

	req.Messages = messages

	return PromptProtectionResult{
		Request:      req,
		MatchedRules: matchedRules,
	}
}

func applyDecisionsPromptProtectionRules(req *llm.Request, rules []*ent.PromptProtectionRule) PromptProtectionResult {
	body := append([]byte(nil), req.Decisions.Body...)
	var matchedRules []*ent.PromptProtectionRule
	for _, rule := range rules {
		if rule == nil || rule.Settings == nil {
			continue
		}
		matched := false
		for _, field := range decisionsPromptFields(body) {
			if !promptProtectionRuleAppliesToRole(rule.Settings.Scopes, field.role) || !MatchPromptProtectionRule(rule.Pattern, field.text) {
				continue
			}
			matched = true
			if rule.Settings.Action == objects.PromptProtectionActionReject {
				return PromptProtectionResult{Request: req, MatchedRules: []*ent.PromptProtectionRule{rule}, Rejected: true}
			}
			masked := ReplacePromptProtectionRule(rule.Pattern, field.text, rule.Settings.Replacement)
			var err error
			body, err = sjson.SetBytes(body, field.path, masked)
			if err != nil {
				return PromptProtectionResult{Request: req}
			}
		}
		if matched {
			matchedRules = append(matchedRules, rule)
		}
	}
	req.Decisions.Body = body
	return PromptProtectionResult{Request: req, MatchedRules: matchedRules}
}

type decisionsPromptField struct {
	path string
	role string
	text string
}

func decisionsPromptFields(body []byte) []decisionsPromptField {
	if gjson.GetBytes(body, "input").Type == gjson.String {
		return []decisionsPromptField{{path: "input", role: "user", text: gjson.GetBytes(body, "input").String()}}
	}
	var fields []decisionsPromptField
	for i := range int(gjson.GetBytes(body, "input.#").Int()) {
		itemPath := fmt.Sprintf("input.%d", i)
		role := gjson.GetBytes(body, itemPath+".role").String()
		if role == "" {
			role = "user"
		}
		for j := range int(gjson.GetBytes(body, itemPath+".content.#").Int()) {
			partPath := fmt.Sprintf("%s.content.%d", itemPath, j)
			if !strings.EqualFold(gjson.GetBytes(body, partPath+".type").String(), "input_text") {
				continue
			}
			fields = append(fields, decisionsPromptField{path: partPath + ".text", role: role, text: gjson.GetBytes(body, partPath+".text").String()})
		}
	}
	return fields
}

// applyCompactPromptProtectionRules protects Compact's separate input and
// instructions; its Messages slice is empty and cannot carry these masks.
func applyCompactPromptProtectionRules(req *llm.Request, rules []*ent.PromptProtectionRule) PromptProtectionResult {
	view := *req
	view.Compact = nil
	view.Messages = make([]llm.Message, 0, len(req.Compact.Input)+1)
	inputOffset := 0
	if req.Compact.Instructions != "" {
		view.Messages = append(view.Messages, llm.Message{
			Role: "system",
			Content: llm.MessageContent{
				Content: &req.Compact.Instructions,
			},
		})
		inputOffset = 1
	}
	view.Messages = append(view.Messages, req.Compact.Input...)

	result := ApplyPromptProtectionRules(&view, rules)
	if result.Rejected {
		return result
	}
	if inputOffset > 0 {
		req.Compact.Instructions = *view.Messages[0].Content.Content
	}
	req.Compact.Input = view.Messages[inputOffset:]
	result.Request = req

	return result
}

// ProtectWithResult applies enabled prompt-protection rules and preserves the
// matched rules for callers that must patch a provider-native request body.
func (svc *PromptProtectionRuleService) ProtectWithResult(ctx context.Context, req *llm.Request) (PromptProtectionResult, error) {
	rules, err := svc.ListEnabledRules(ctx)
	if err != nil {
		log.Warn(ctx, "failed to load enabled prompt protection rules", log.Cause(err))
		return PromptProtectionResult{}, err
	}

	if len(rules) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "no enabled prompt protection rules")
		}
		return PromptProtectionResult{Request: req}, nil
	}

	result := ApplyPromptProtectionRules(req, rules)
	if len(result.MatchedRules) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "prompt protection passed without rule match", log.Int("rule_count", len(rules)))
		}
		return result, nil
	}

	if result.Rejected {
		log.Warn(ctx, "prompt protection rejected request",
			log.String("rule_name", result.MatchedRules[0].Name),
		)

		return result, ErrPromptProtectionRejected
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "prompt protection masked request", log.Any("rules", result.MatchedRules))
	}

	return result, nil
}

// Protect applies enabled prompt-protection rules and returns the protected request.
func (svc *PromptProtectionRuleService) Protect(ctx context.Context, req *llm.Request) (*llm.Request, error) {
	result, err := svc.ProtectWithResult(ctx, req)

	return result.Request, err
}

func applyPromptProtectionRuleToMessage(msg llm.Message, rule *ent.PromptProtectionRule) (llm.Message, bool) {
	matched := false

	if msg.Content.Content != nil && *msg.Content.Content != "" && MatchPromptProtectionRule(rule.Pattern, *msg.Content.Content) {
		if rule.Settings.Action == objects.PromptProtectionActionMask {
			masked := ReplacePromptProtectionRule(rule.Pattern, *msg.Content.Content, rule.Settings.Replacement)
			msg.Content = llm.MessageContent{Content: &masked}
		}

		matched = true
	}

	for i, part := range msg.Content.MultipleContent {
		if !strings.EqualFold(part.Type, "text") || part.Text == nil || *part.Text == "" {
			continue
		}

		if !MatchPromptProtectionRule(rule.Pattern, *part.Text) {
			continue
		}

		if rule.Settings.Action == objects.PromptProtectionActionMask {
			masked := ReplacePromptProtectionRule(rule.Pattern, *part.Text, rule.Settings.Replacement)
			msg.Content.MultipleContent[i].Text = &masked
		}

		matched = true
	}

	return msg, matched
}

func promptProtectionRuleAppliesToRole(scopes []objects.PromptProtectionScope, role string) bool {
	if len(scopes) == 0 {
		return true
	}

	roleScope := objects.PromptProtectionScope(strings.ToLower(role))

	return slices.Contains(scopes, roleScope)
}
