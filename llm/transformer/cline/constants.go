package cline

// DefaultModels returns a static list of known Cline model IDs.
func DefaultModels() []string {
	return []string{
		"cline-pass/deepseek-v4-flash",
		"cline-pass/deepseek-v4-pro",
		"cline-pass/qwen3.7-plus",
		"cline-pass/qwen3.7-max",
		"cline-pass/kimi-k2.7-code",
		"cline-pass/kimi-k2.6",
		"cline-pass/glm-5.2",
		"cline-pass/mimo-v2.5",
		"cline-pass/mimo-v2.5-pro",
		"cline-pass/minimax-m3",
	}
}
