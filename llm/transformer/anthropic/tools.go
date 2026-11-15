package anthropic

import (
	"fmt"
	"slices"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

const (
	// ToolTypeWebSearch20250305 is the original native web search tool type for Anthropic.
	// This tool is only supported by native Anthropic API format channels.
	ToolTypeWebSearch20250305 = "web_search_20250305"

	// ToolTypeWebSearch20260209 is the native web search tool type that adds
	// dynamic filtering. Upstream executes that filtering through code
	// execution and provisions it automatically; callers must not add a code
	// execution tool. Its allowed_callers defaults to the code execution
	// caller, so an explicit "direct" value is what keeps the old behavior.
	ToolTypeWebSearch20260209 = "web_search_20260209"

	// WebSearchFunctionName is the standard function name that triggers
	// native Anthropic web search tool transformation.
	WebSearchFunctionName = "web_search"
)

// nativeWebSearchToolTypes lists the versions this package understands. Inbound
// drops an unknown version instead of inferring it from a "web_search_" prefix,
// and outbound rejects a unified request carrying one instead of downgrading it.
var nativeWebSearchToolTypes = []string{
	ToolTypeWebSearch20250305,
	ToolTypeWebSearch20260209,
}

// isNativeWebSearchToolType reports whether the tool type is a native web search
// version this package supports.
func isNativeWebSearchToolType(toolType string) bool {
	return slices.Contains(nativeWebSearchToolTypes, toolType)
}

// nativeWebSearchToolType resolves the wire type for a unified web search tool.
// An empty version keeps the original tool. A non-empty version is passed
// through verbatim so that an unsupported value is rejected at the transformer
// entry instead of being silently downgraded to a different API contract here.
func nativeWebSearchToolType(webSearch *llm.WebSearch) string {
	if webSearch == nil || webSearch.Version == "" {
		return ToolTypeWebSearch20250305
	}

	return webSearch.Version
}

// validateNativeWebSearchToolVersions rejects a non-empty web search version this
// package does not understand. The unified request can be populated from public
// JSON (for example a plugin hook), so an unknown version must fail loudly
// rather than be downgraded or dropped on the way out.
func validateNativeWebSearchToolVersions(tools []llm.Tool) error {
	for _, tool := range tools {
		if tool.Type != llm.ToolTypeWebSearch || tool.WebSearch == nil {
			continue
		}

		version := tool.WebSearch.Version
		if version == "" || isNativeWebSearchToolType(version) {
			continue
		}

		return fmt.Errorf("%w: unsupported web search tool version %q (supported: %s)",
			transformer.ErrInvalidRequest, version, strings.Join(nativeWebSearchToolTypes, ", "))
	}

	return nil
}

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
// Direct Anthropic API, Bedrock, Claude Code, and DeepSeek's Anthropic-compatible API
// support native tools like web_search.
func supportsAnthropicNativeTools(config *Config) bool {
	if config == nil {
		return true
	}

	//nolint:exhaustive // Only check platforms with verified native tool support.
	switch config.Type {
	case PlatformDirect, PlatformBedrock, PlatformClaudeCode, PlatformDeepSeek:
		return true
	default:
		return false
	}
}
