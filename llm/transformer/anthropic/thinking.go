package anthropic

import "github.com/looplj/axonhub/llm"

func supportsAdaptiveThinking(config *Config) bool {
	if config == nil {
		return true
	}

	//nolint:exhaustive // Checked.
	switch config.Type {
	case PlatformDirect, PlatformClaudeCode, PlatformBedrock, PlatformVertex:
		return true
	default:
		return false
	}
}

// supportsOutputConfig returns true if the platform supports the output_config field
// with effort control. DeepSeek supports output_config.effort but does NOT support
// thinking.type = "adaptive".
func supportsOutputConfig(config *Config) bool {
	if config == nil {
		return true
	}

	//nolint:exhaustive // Checked.
	switch config.Type {
	case PlatformDirect, PlatformClaudeCode, PlatformBedrock, PlatformVertex, PlatformDeepSeek:
		return true
	default:
		return false
	}
}

// thinkingBudgetToReasoningEffort converts thinking budget tokens to reasoning effort string.
// This is only used when a native budget_tokens request must be expressed in a
// protocol that has no budget field (e.g. OpenAI reasoning_effort); the original
// budget always travels in llm.Request.ReasoningBudget. Levels are monotonic so
// larger budgets never flatten into a lower tier.
func thinkingBudgetToReasoningEffort(budgetTokens int64) string {
	switch {
	case budgetTokens <= 5000:
		return llm.ReasoningEffortLow
	case budgetTokens <= 15000:
		return llm.ReasoningEffortMedium
	case budgetTokens <= 30000:
		return llm.ReasoningEffortHigh
	default:
		return llm.ReasoningEffortXHigh
	}
}

// normalizeAnthropicEffort maps a unified effort level onto the levels accepted by
// output_config.effort. "minimal" is an OpenAI-only level with no Anthropic
// equivalent; the closest (and the mapping mandated by the effort table) is "low".
// All other levels, including "xhigh" and "max", pass through unchanged.
func normalizeAnthropicEffort(effort string) string {
	if effort == llm.ReasoningEffortMinimal {
		return llm.ReasoningEffortLow
	}

	return effort
}

// getDefaultReasoningEffortMapping returns the default mapping from ReasoningEffort to thinking budget tokens.
// Only used as the compatibility fallback for platforms without output_config.effort
// support, where a budget is the only way to express an effort level. Channels can
// override individual entries via Config.ReasoningEffortToBudget.
var defaultReasoningEffortMapping = map[string]int64{
	llm.ReasoningEffortMinimal: 5000,
	llm.ReasoningEffortLow:     5000,
	llm.ReasoningEffortMedium:  15000,
	llm.ReasoningEffortHigh:    30000,
	llm.ReasoningEffortXHigh:   30000,
	llm.ReasoningEffortMax:     30000,
}

// getThinkingBudgetTokensWithConfig returns the thinking budget tokens for a given reasoning effort with config.
func getThinkingBudgetTokensWithConfig(reasoningEffort string, config *Config) int64 {
	if config != nil && config.ReasoningEffortToBudget != nil {
		if budget, exists := config.ReasoningEffortToBudget[reasoningEffort]; exists {
			return budget
		}
	}

	// Fall back to default mapping
	if budget, exists := defaultReasoningEffortMapping[reasoningEffort]; exists {
		return budget
	}

	// Default to medium if not found
	return 15000
}
