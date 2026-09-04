package anthropic

import (
	"slices"

	"github.com/looplj/axonhub/llm"
)

const (
	// ToolTypeWebSearch20250305 is the native web search tool type for Anthropic (Beta).
	// This tool is only supported by native Anthropic API format channels.
	ToolTypeWebSearch20250305 = "web_search_20250305"

	// WebSearchFunctionName is the standard function name that triggers
	// native Anthropic web search tool transformation.
	WebSearchFunctionName = "web_search"
)

// ContainsAnthropicNativeTools checks if the tools slice contains any Anthropic native tools.
// Currently, this checks for the web_search function which maps to Anthropic's native
// web_search_20250305 tool type.
func ContainsAnthropicNativeTools(tools []llm.Tool) bool {
	return slices.ContainsFunc(tools, IsAnthropicNativeTool)
}

// IsAnthropicNativeTool checks if a single tool is an Anthropic native tool.
// A tool is considered Anthropic native if:
// 1. It's a function tool with name "web_search" (OpenAI format input), OR
// 2. It's already transformed to type "web_search_20250305" (Anthropic native format).
func IsAnthropicNativeTool(tool llm.Tool) bool {
	// Match already-transformed Anthropic native tool type
	if tool.Type == llm.ToolTypeWebSearch || tool.Type == ToolTypeWebSearch20250305 {
		return true
	}

	return false
}

// FilterOutAnthropicNativeTools removes Anthropic native tools from the tools slice.
// This is useful as a fallback when routing to channels that don't support native tools.
func FilterOutAnthropicNativeTools(tools []llm.Tool) []llm.Tool {
	if len(tools) == 0 {
		return tools
	}

	filtered := make([]llm.Tool, 0, len(tools))

	for _, tool := range tools {
		if !IsAnthropicNativeTool(tool) {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// supportsAnthropicNativeTools checks if the platform supports Anthropic native tools.
// Only direct Anthropic API, Bedrock, and Claude Code support native tools like web_search.
func supportsAnthropicNativeTools(config *Config) bool {
	if config == nil {
		return true
	}

	//nolint:exhaustive // Only check direct, bedrock, and claude code platforms.
	switch config.Type {
	case PlatformDirect, PlatformBedrock, PlatformClaudeCode:
		return true
	default:
		return false
	}
}
