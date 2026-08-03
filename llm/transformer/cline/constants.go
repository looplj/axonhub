package cline

// RecommendedModelsURL is Cline's public model discovery endpoint.
const RecommendedModelsURL = "https://api.cline.bot/api/v1/ai/cline/recommended-models"

// DefaultModels returns known Cline Pass models used when dynamic discovery is unavailable.
func DefaultModels() []string {
	return []string{
		"cline-pass/deepseek-v4-flash",
		"cline-pass/deepseek-v4-pro",
		"cline-pass/qwen3.7-plus",
		"cline-pass/qwen3.7-max",
		"cline-pass/kimi-k3",
		"cline-pass/kimi-k2.7-code",
		"cline-pass/kimi-k2.6",
		"cline-pass/glm-5.2",
		"cline-pass/mimo-v2.5",
		"cline-pass/mimo-v2.5-pro",
		"cline-pass/minimax-m3",
	}
}
