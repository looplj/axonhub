package anthropic

// thinkingBudgetToReasoningEffort converts thinking budget tokens to reasoning effort string.
func thinkingBudgetToReasoningEffort(budgetTokens int64) string {
	// Map budget tokens to reasoning effort based on the same logic used in outbound
	if budgetTokens <= 5000 {
		return "low"
	} else if budgetTokens <= 15000 {
		return "medium"
	} else {
		return "high"
	}
}

// getDefaultReasoningEffortMapping returns the default mapping from ReasoningEffort to thinking budget tokens.
var defaultReasoningEffortMapping = map[string]int64{
	"low":    5000,
	"medium": 15000,
	"high":   30000,
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
