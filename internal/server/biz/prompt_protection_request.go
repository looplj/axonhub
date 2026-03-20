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

func ApplyPromptProtectionRules(req *llm.Request, rules []*ent.PromptProtectionRule) (*llm.Request, *ent.PromptProtectionRule, bool) {
	if req == nil || len(req.Messages) == 0 || len(rules) == 0 {
		return req, nil, false
	}

	newReq := *req
	newReq.Messages = clonePromptProtectionMessages(req.Messages)

	for _, rule := range rules {
		applied := false

		for i, msg := range newReq.Messages {
			if !promptProtectionRuleAppliesToRole(rule.Settings.Scopes, msg.Role) {
				continue
			}

			updatedMsg, msgApplied := applyPromptProtectionRuleToMessage(msg, rule)
			if msgApplied {
				newReq.Messages[i] = updatedMsg
				applied = true
			}
		}

		if applied {
			return &newReq, rule, true
		}
	}

	return req, nil, false
}

func (svc *PromptProtectionRuleService) Protect(ctx context.Context, req *llm.Request) (*llm.Request, error) {
	rules, err := svc.ListEnabledRules(ctx)
	if err != nil {
		log.Warn(ctx, "failed to load enabled prompt protection rules", log.Cause(err))
		return nil, err
	}

	if len(rules) == 0 {
		log.Debug(ctx, "no enabled prompt protection rules")
		return req, nil
	}

	protected, matchedRule, matched := ApplyPromptProtectionRules(req, rules)
	if !matched || matchedRule == nil || matchedRule.Settings == nil {
		log.Debug(ctx, "prompt protection passed without rule match", log.Int("rule_count", len(rules)))
		return req, nil
	}

	if matchedRule.Settings.Action == objects.PromptProtectionActionReject {
		log.Warn(ctx, "prompt protection rejected request",
			log.String("rule_name", matchedRule.Name),
			log.String("action", string(matchedRule.Settings.Action)),
		)

		return protected, ErrPromptProtectionRejected
	}

	log.Debug(ctx, "prompt protection masked request",
		log.String("rule_name", matchedRule.Name),
		log.String("action", string(matchedRule.Settings.Action)),
	)

	return protected, nil
}

func applyPromptProtectionRuleToMessage(msg llm.Message, rule *ent.PromptProtectionRule) (llm.Message, bool) {
	applied := false

	if msg.Content.Content != nil && *msg.Content.Content != "" && MatchPromptProtectionRule(rule.Pattern, *msg.Content.Content) {
		if rule.Settings.Action == objects.PromptProtectionActionMask {
			masked := ReplacePromptProtectionRule(rule.Pattern, *msg.Content.Content, rule.Settings.Replacement)
			msg.Content = llm.MessageContent{Content: &masked}
		}

		applied = true
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

		applied = true
	}

	return msg, applied
}

func clonePromptProtectionMessages(messages []llm.Message) []llm.Message {
	cloned := make([]llm.Message, len(messages))
	for i, msg := range messages {
		cloned[i] = msg
		if len(msg.Content.MultipleContent) > 0 {
			cloned[i].Content.MultipleContent = append([]llm.MessageContentPart(nil), msg.Content.MultipleContent...)
		}
	}

	return cloned
}

func promptProtectionRuleAppliesToRole(scopes []objects.PromptProtectionScope, role string) bool {
	if len(scopes) == 0 {
		return true
	}

	roleScope := objects.PromptProtectionScope(strings.ToLower(role))

	return slices.Contains(scopes, roleScope)
}
