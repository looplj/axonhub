package biz

import (
	"sort"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
	"github.com/looplj/axonhub/llm"
)

// PromptMatcher evaluates whether a prompt's activation conditions are satisfied.
type PromptMatcher struct{}

// NewPromptMatcher creates a new PromptMatcher instance.
func NewPromptMatcher() *PromptMatcher {
	return &PromptMatcher{}
}

// MatchPrompt checks if a prompt's conditions are satisfied for the given model.
// Returns true if no conditions are defined (always match) or all conditions are met.
func (m *PromptMatcher) MatchPrompt(prompt *ent.Prompt, model string) bool {
	if prompt == nil {
		return false
	}

	return m.MatchConditions(prompt.Settings.Conditions, model)
}

// MatchConditions checks if all composite conditions are satisfied.
// Returns true if no conditions are defined (always match) or all conditions are met.
func (m *PromptMatcher) MatchConditions(conditions []objects.PromptActivationConditionComposite, model string) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, composite := range conditions {
		if !m.matchCompositeCondition(composite, model) {
			return false
		}
	}

	return true
}

// matchCompositeCondition checks if at least one condition in the composite is satisfied.
// Returns true if conditions list is empty or at least one condition matches.
func (m *PromptMatcher) matchCompositeCondition(composite objects.PromptActivationConditionComposite, model string) bool {
	if len(composite.Conditions) == 0 {
		return true
	}

	for _, condition := range composite.Conditions {
		if m.matchCondition(condition, model) {
			return true
		}
	}

	return false
}

// matchCondition checks if a single condition is satisfied.
func (m *PromptMatcher) matchCondition(condition objects.PromptActivationCondition, model string) bool {
	switch condition.Type {
	case objects.PromptActivationConditionTypeModelID:
		return m.matchModelID(condition, model)
	case objects.PromptActivationConditionTypeModelPattern:
		return m.matchModelPattern(condition, model)
	default:
		return false
	}
}

// matchModelID checks if the model ID exactly matches.
func (m *PromptMatcher) matchModelID(condition objects.PromptActivationCondition, model string) bool {
	if condition.ModelID == nil {
		return false
	}

	return *condition.ModelID == model
}

// matchModelPattern checks if the model matches the pattern.
func (m *PromptMatcher) matchModelPattern(condition objects.PromptActivationCondition, model string) bool {
	if condition.ModelPattern == nil || *condition.ModelPattern == "" {
		return false
	}

	return xregexp.MatchString(*condition.ModelPattern, model)
}

// FilterMatchingPrompts filters prompts that match the given model.
func (m *PromptMatcher) FilterMatchingPrompts(prompts []*ent.Prompt, model string) []*ent.Prompt {
	return lo.Filter(prompts, func(p *ent.Prompt, _ int) bool {
		return m.MatchPrompt(p, model)
	})
}

// ApplyPrompts applies matching prompts to the llm.Request based on their action settings.
// Prompts with action type "prepend" are added before existing messages.
// Prompts with action type "append" are added after existing messages.
// Prompts are sorted by their Order field (ascending), with CreatedAt as tiebreaker.
func (m *PromptMatcher) ApplyPrompts(request *llm.Request, prompts []*ent.Prompt) *llm.Request {
	if len(prompts) == 0 {
		return request
	}

	sort.SliceStable(prompts, func(i, j int) bool {
		if prompts[i].Order != prompts[j].Order {
			return prompts[i].Order < prompts[j].Order
		}

		return prompts[i].CreatedAt.Before(prompts[j].CreatedAt)
	})

	var (
		prependMessages []llm.Message
		appendMessages  []llm.Message
	)

	for _, prompt := range prompts {
		msg := llm.Message{
			Role: prompt.Role,
			Content: llm.MessageContent{
				Content: &prompt.Content,
			},
		}

		switch prompt.Settings.Action.Type {
		case objects.PromptActionTypePrepend:
			prependMessages = append(prependMessages, msg)
		case objects.PromptActionTypeAppend:
			appendMessages = append(appendMessages, msg)
		default:
			prependMessages = append(prependMessages, msg)
		}
	}

	newMessages := make([]llm.Message, 0, len(prependMessages)+len(request.Messages)+len(appendMessages))
	newMessages = append(newMessages, prependMessages...)
	newMessages = append(newMessages, request.Messages...)
	newMessages = append(newMessages, appendMessages...)

	request.Messages = newMessages

	return request
}
