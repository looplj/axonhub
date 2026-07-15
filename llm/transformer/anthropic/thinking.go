package anthropic

import (
	"fmt"
	"strings"

	"github.com/looplj/axonhub/llm"
)

const minimumManualThinkingBudget int64 = 1024

type ThinkingCapability string

const (
	ThinkingCapabilityUnknown           ThinkingCapability = "unknown"
	ThinkingCapabilityAdaptiveOnly      ThinkingCapability = "adaptive_only"
	ThinkingCapabilityAdaptivePreferred ThinkingCapability = "adaptive_preferred"
	ThinkingCapabilityManualSupported   ThinkingCapability = "manual_supported"
)

type thinkingRequestPlan struct {
	thinking                   *Thinking
	outputConfig               *OutputConfig
	unsupportedReasoningEffort bool
	validationErr              error
}

func resolveThinkingRequestPlan(chatReq *llm.Request, config *Config) thinkingRequestPlan {
	plan := thinkingRequestPlan{}
	if chatReq == nil {
		return plan
	}

	if thinkingTypeFromMetadata(chatReq) == "disabled" || chatReq.ReasoningEffort == "none" {
		plan.thinking = &Thinking{Type: "disabled"}
		return plan
	}

	if config != nil && config.Type == PlatformDeepSeek {
		return resolveDeepSeekThinkingPlan(chatReq)
	}

	if thinkingTypeFromMetadata(chatReq) == "adaptive" {
		plan.thinking = &Thinking{Type: "adaptive"}
		if effort := outputConfigEffortFromMetadata(chatReq); effort != "" && supportsOutputConfig(config) {
			plan.outputConfig = &OutputConfig{Effort: effort}
		}
		return plan
	}

	if effort := outputConfigEffortFromMetadata(chatReq); effort != "" {
		if supportsOutputConfig(config) {
			plan.outputConfig = &OutputConfig{Effort: effort}
			return plan
		}

		plan.unsupportedReasoningEffort = true
		return plan
	}

	capability := resolveThinkingCapability(chatReq.Model, config)

	if chatReq.ReasoningBudget != nil {
		if capability != ThinkingCapabilityManualSupported {
			plan.validationErr = fmt.Errorf("manual thinking is not supported for this target capability")
			return plan
		}

		return manualThinkingPlan(*chatReq.ReasoningBudget, resolveMaxTokens(chatReq))
	}

	if chatReq.ReasoningEffort == "" {
		return plan
	}

	switch capability {
	case ThinkingCapabilityAdaptiveOnly, ThinkingCapabilityAdaptivePreferred:
		effort, ok := normalizeAnthropicEffort(chatReq.ReasoningEffort)
		if !ok {
			plan.unsupportedReasoningEffort = true
			return plan
		}

		plan.thinking = &Thinking{Type: "adaptive"}
		plan.outputConfig = &OutputConfig{Effort: effort}
		return plan
	case ThinkingCapabilityManualSupported:
		plan.unsupportedReasoningEffort = true
		return plan
	default:
		plan.unsupportedReasoningEffort = true
		return plan
	}
}

func resolveDeepSeekThinkingPlan(chatReq *llm.Request) thinkingRequestPlan {
	plan := thinkingRequestPlan{}

	if effort := outputConfigEffortFromMetadata(chatReq); effort != "" {
		plan.outputConfig = &OutputConfig{Effort: effort}
		return plan
	}

	if chatReq.ReasoningBudget != nil {
		plan.validationErr = fmt.Errorf("manual thinking is not supported for this target capability")
		return plan
	}

	if chatReq.ReasoningEffort == "" {
		return plan
	}

	if effort, ok := normalizeDeepSeekOutputConfigEffort(chatReq.ReasoningEffort); ok {
		plan.outputConfig = &OutputConfig{Effort: effort}
	} else {
		plan.unsupportedReasoningEffort = true
	}

	return plan
}

func thinkingTypeFromMetadata(chatReq *llm.Request) string {
	if chatReq.TransformerMetadata == nil {
		return ""
	}

	thinkingType, _ := chatReq.TransformerMetadata[TransformerMetadataKeyThinkingType].(string)
	return thinkingType
}

func outputConfigEffortFromMetadata(chatReq *llm.Request) string {
	if chatReq.TransformerMetadata == nil {
		return ""
	}

	effort, _ := chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfigEffort].(string)
	return effort
}

func manualThinkingPlan(budgetTokens, maxTokens int64) thinkingRequestPlan {
	plan := thinkingRequestPlan{}
	if maxTokens <= minimumManualThinkingBudget {
		plan.validationErr = fmt.Errorf("max_tokens must be greater than %d when manual thinking is enabled", minimumManualThinkingBudget)
		return plan
	}
	if budgetTokens < minimumManualThinkingBudget {
		plan.validationErr = fmt.Errorf("budget_tokens must be at least %d when manual thinking is enabled", minimumManualThinkingBudget)
		return plan
	}
	if budgetTokens >= maxTokens {
		plan.validationErr = fmt.Errorf("budget_tokens must be less than max_tokens when manual thinking is enabled")
		return plan
	}

	plan.thinking = &Thinking{Type: "enabled", BudgetTokens: budgetTokens}
	return plan
}

func resolveThinkingCapability(model string, config *Config) ThinkingCapability {
	if config != nil {
		if config.Type == PlatformDeepSeek {
			return ThinkingCapabilityUnknown
		}
		if isKnownThinkingCapability(config.ThinkingCapabilityOverride) {
			return config.ThinkingCapabilityOverride
		}
		if !isOfficialAnthropicPlatform(config.Type) {
			return ThinkingCapabilityUnknown
		}
	}

	return defaultClaudeThinkingCapability(model)
}

func isKnownThinkingCapability(capability ThinkingCapability) bool {
	switch capability {
	case ThinkingCapabilityAdaptiveOnly, ThinkingCapabilityAdaptivePreferred, ThinkingCapabilityManualSupported:
		return true
	default:
		return false
	}
}

func isOfficialAnthropicPlatform(platform PlatformType) bool {
	switch platform {
	case "", PlatformDirect, PlatformClaudeCode, PlatformBedrock, PlatformVertex:
		return true
	default:
		return false
	}
}

// defaultClaudeThinkingCapability is provider data policy. Keep model-family
// matching here; request conversion consumes only its capability result.
func defaultClaudeThinkingCapability(model string) ThinkingCapability {
	model = strings.ToLower(model)

	switch {
	case strings.Contains(model, "claude-fable-5"),
		strings.Contains(model, "claude-mythos-5"),
		strings.Contains(model, "claude-opus-4-8"),
		strings.Contains(model, "claude-opus-4-7"),
		strings.Contains(model, "claude-sonnet-5"):
		return ThinkingCapabilityAdaptiveOnly
	case strings.Contains(model, "claude-mythos-preview"),
		strings.Contains(model, "claude-opus-4-6"),
		strings.Contains(model, "claude-sonnet-4-6"):
		return ThinkingCapabilityAdaptivePreferred
	case strings.Contains(model, "claude-opus-4-5"),
		strings.Contains(model, "claude-haiku-4-5"),
		strings.Contains(model, "claude-sonnet-4-5"),
		strings.Contains(model, "claude-opus-4-1"),
		strings.Contains(model, "claude-opus-4"),
		strings.Contains(model, "claude-sonnet-4"),
		strings.Contains(model, "claude-3"):
		return ThinkingCapabilityManualSupported
	default:
		return ThinkingCapabilityUnknown
	}
}

// supportsOutputConfig returns true when the target's configured capability is
// known to accept output_config.effort. DeepSeek supports this field through its
// own platform policy but does not support Anthropic adaptive thinking.
func supportsOutputConfig(config *Config) bool {
	if config == nil {
		return true
	}
	if config.Type == PlatformDeepSeek {
		return true
	}
	if config.ThinkingCapabilityOverride == ThinkingCapabilityAdaptiveOnly ||
		config.ThinkingCapabilityOverride == ThinkingCapabilityAdaptivePreferred {
		return true
	}

	return isOfficialAnthropicPlatform(config.Type)
}

func normalizeAnthropicEffort(reasoningEffort string) (string, bool) {
	switch reasoningEffort {
	case "minimal", "low":
		return "low", true
	case "medium":
		return "medium", true
	case "high", "xhigh", "max":
		return "max", true
	default:
		return "", false
	}
}

// normalizeDeepSeekOutputConfigEffort intentionally does not reuse Anthropic's
// adaptive-thinking mapping. DeepSeek accepts Anthropic-format output_config,
// but its platform policy has no adaptive thinking wire mode and keeps its own
// effort values. OpenAI-only "minimal" has no DeepSeek wire equivalent, so it
// is the one controlled downgrade to "low".
func normalizeDeepSeekOutputConfigEffort(reasoningEffort string) (string, bool) {
	switch reasoningEffort {
	case "minimal":
		return "low", true
	case "low", "medium", "high", "xhigh", "max":
		return reasoningEffort, true
	default:
		return "", false
	}
}

func recordAnthropicThinkingLossyDowngrade(req *llm.Request, plan thinkingRequestPlan) {
	if req == nil || !plan.unsupportedReasoningEffort || req.APIFormat == "" {
		return
	}

	sourceField := "reasoning_effort"
	if req.APIFormat == llm.APIFormatOpenAIResponse {
		sourceField = "reasoning.effort"
	}
	llm.AddLossyDowngradeIfPresent(req, req.APIFormat, sourceField, llm.APIFormatAnthropicMessage, true)
}
