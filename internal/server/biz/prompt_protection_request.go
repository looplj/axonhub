package biz

import (
	"context"
	"errors"
	"slices"
	"strings"

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

func (svc *PromptProtectionRuleService) Protect(ctx context.Context, req *llm.Request) (*llm.Request, error) {
	rules, err := svc.ListEnabledRules(ctx)
	if err != nil {
		log.Warn(ctx, "failed to load enabled prompt protection rules", log.Cause(err))
		return nil, err
	}

	if len(rules) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "no enabled prompt protection rules")
		}
		return req, nil
	}

	result := ApplyPromptProtectionRules(req, rules)
	if len(result.MatchedRules) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "prompt protection passed without rule match", log.Int("rule_count", len(rules)))
		}
		return req, nil
	}

	if result.Rejected {
		log.Warn(ctx, "prompt protection rejected request",
			log.String("rule_name", result.MatchedRules[0].Name),
		)

		return result.Request, ErrPromptProtectionRejected
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "prompt protection masked request", log.Any("rules", result.MatchedRules))
	}

	return result.Request, nil
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
