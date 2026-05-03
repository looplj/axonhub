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
	Request         *llm.Request
	MatchedRules    []*ent.PromptProtectionRule
	Rejected        bool
	RulesEvaluated  bool
	RulesEnabled    bool
	FragmentResults []PromptProtectionFragmentResult
}

type PromptProtectionFragmentResult struct {
	Path            string
	Scope           string
	Text            string
	Matched         bool
	Changed         bool
	Rejected        bool
	ReplacementText string
	ReplayAllowed   bool
	DropRequired    bool
	RejectRequired  bool
	NoRules         bool
}

// ApplyPromptProtectionRules applies prompt protection rules to a request.
func ApplyPromptProtectionRules(req *llm.Request, rules []*ent.PromptProtectionRule) PromptProtectionResult {
	result := PromptProtectionResult{
		Request:        req,
		RulesEvaluated: true,
		RulesEnabled:   len(rules) > 0,
	}

	if req == nil {
		return result
	}

	if len(rules) == 0 {
		result.FragmentResults = evaluateProtectableFragmentsWithoutRules(req)
		return result
	}

	messages := req.Messages

	var matchedRules []*ent.PromptProtectionRule
	fragmentResults := initialProtectableFragmentResults(req)

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
						MatchedRules:    []*ent.PromptProtectionRule{rule},
						Rejected:        true,
						RulesEvaluated:  true,
						RulesEnabled:    true,
						FragmentResults: fragmentResults,
					}
				}

				messages[i] = updatedMsg
				ruleMatches = true
			}
		}

		fragmentApplied, fragmentRejected := applyPromptProtectionRuleToFragments(fragmentResults, rule)
		if fragmentRejected {
			return PromptProtectionResult{
				Request:         req,
				MatchedRules:    []*ent.PromptProtectionRule{rule},
				Rejected:        true,
				RulesEvaluated:  true,
				RulesEnabled:    true,
				FragmentResults: fragmentResults,
			}
		}

		if fragmentApplied {
			ruleMatches = true
		}

		if !ruleMatches {
			continue
		}

		matchedRules = append(matchedRules, rule)
	}

	req.Messages = messages

	return PromptProtectionResult{
		Request:         req,
		MatchedRules:    matchedRules,
		RulesEvaluated:  true,
		RulesEnabled:    true,
		FragmentResults: fragmentResults,
	}
}

func evaluateProtectableFragmentsWithoutRules(req *llm.Request) []PromptProtectionFragmentResult {
	fragments := openAIResponsesProtectableFragments(req)
	if len(fragments) == 0 {
		return nil
	}

	results := make([]PromptProtectionFragmentResult, 0, len(fragments))
	for _, fragment := range fragments {
		results = append(results, PromptProtectionFragmentResult{
			Path:          fragment.Path,
			Scope:         fragment.Scope,
			Text:          fragment.Text,
			ReplayAllowed: true,
			NoRules:       true,
		})
	}

	return results
}

func initialProtectableFragmentResults(req *llm.Request) []PromptProtectionFragmentResult {
	fragments := openAIResponsesProtectableFragments(req)
	if len(fragments) == 0 {
		return nil
	}

	results := make([]PromptProtectionFragmentResult, 0, len(fragments))
	for _, fragment := range fragments {
		results = append(results, PromptProtectionFragmentResult{
			Path:          fragment.Path,
			Scope:         fragment.Scope,
			Text:          fragment.Text,
			ReplayAllowed: true,
		})
	}

	return results
}

func openAIResponsesProtectableFragments(req *llm.Request) []llm.OpenAIResponsesProtectableFragment {
	if req == nil || req.ProviderExtensions == nil || req.ProviderExtensions.OpenAIResponses == nil ||
		req.ProviderExtensions.OpenAIResponses.Request == nil {
		return nil
	}

	return req.ProviderExtensions.OpenAIResponses.Request.ProtectableFragments
}

func applyPromptProtectionRuleToFragments(results []PromptProtectionFragmentResult, rule *ent.PromptProtectionRule) (bool, bool) {
	if len(results) == 0 || rule == nil || rule.Settings == nil {
		return false, false
	}

	var applied bool
	for i := range results {
		fragment := &results[i]
		if fragment.Path == "" {
			continue
		}

		if fragment.Text == "" {
			continue
		}

		if !promptProtectionRuleAppliesToRole(rule.Settings.Scopes, fragment.Scope) {
			continue
		}

		if !MatchPromptProtectionRule(rule.Pattern, fragment.Text) {
			continue
		}

		fragment.Matched = true
		applied = true

		if rule.Settings.Action == objects.PromptProtectionActionReject {
			fragment.Rejected = true
			fragment.RejectRequired = true
			fragment.ReplayAllowed = false
			return true, true
		}

		if rule.Settings.Action != objects.PromptProtectionActionMask {
			continue
		}

		replacement := ReplacePromptProtectionRule(rule.Pattern, fragment.Text, rule.Settings.Replacement)
		fragment.ReplacementText = replacement
		fragment.Changed = replacement != fragment.Text
		if fragment.Changed {
			fragment.DropRequired = true
			fragment.ReplayAllowed = false
		}
	}

	return applied, false
}

func (svc *PromptProtectionRuleService) Protect(ctx context.Context, req *llm.Request) (*llm.Request, error) {
	result, err := svc.ProtectWithResult(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.Request, nil
}

func (svc *PromptProtectionRuleService) ProtectWithResult(ctx context.Context, req *llm.Request) (PromptProtectionResult, error) {
	rules, err := svc.ListEnabledRules(ctx)
	if err != nil {
		log.Warn(ctx, "failed to load enabled prompt protection rules", log.Cause(err))
		return PromptProtectionResult{Request: req}, err
	}

	result := ApplyPromptProtectionRules(req, rules)
	if len(rules) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "no enabled prompt protection rules")
		}
		return result, nil
	}

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
