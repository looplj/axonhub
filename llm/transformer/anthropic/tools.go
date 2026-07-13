package anthropic

import (
	"encoding/json"
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


// UnmarshalJSON captures the original tool JSON for same-protocol replay while
// still decoding known function/web_search fields into typed members.
func (t *Tool) UnmarshalJSON(data []byte) error {
	type toolAlias Tool
	var decoded toolAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = Tool(decoded)

	// Exclusive native/adapter variants always keep Raw.
	// Function/web_search keep Raw only when Anthropic-only children are present
	// so ensureCacheControl can still inject cache_control on plain tools.
	if toolNeedsRawPreservation(data) {
		t.Raw = append(json.RawMessage(nil), data...)
	}
	return nil
}

func toolNeedsRawPreservation(data []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	typ := ""
	if rawType, ok := probe["type"]; ok {
		_ = json.Unmarshal(rawType, &typ)
	}
	switch typ {
	case "", "custom", "function":
		// Known typed function fields. Anything else is Anthropic-only and must
		// round-trip via Raw without widening llm.Tool.
		known := map[string]struct{}{
			"type": {}, "name": {}, "description": {}, "input_schema": {}, "cache_control": {},
		}
		for key := range probe {
			if _, ok := known[key]; !ok {
				return true
			}
		}
		return false
	case ToolTypeWebSearch20250305, WebSearchFunctionName:
		known := map[string]struct{}{
			"type": {}, "name": {}, "max_uses": {}, "strict": {},
			"allowed_domains": {}, "blocked_domains": {}, "user_location": {},
			"cache_control": {},
		}
		for key := range probe {
			if _, ok := known[key]; !ok {
				return true
			}
		}
		return false
	default:
		// mcp_toolset and other native/server tool declarations.
		return true
	}
}

// MarshalJSON emits opaque Raw tool variants verbatim when present.
func (t Tool) MarshalJSON() ([]byte, error) {
	if len(t.Raw) > 0 {
		return t.Raw, nil
	}
	type toolAlias Tool
	return json.Marshal(toolAlias(t))
}

// isExclusiveAnthropicRawTool reports tools that must not be flattened into llm.Tool.
// web_search remains a typed common tool; mcp_toolset and other native declarations
// stay Anthropic-adapter raw fragments.
func isExclusiveAnthropicRawTool(tool Tool) bool {
	if len(tool.Raw) == 0 {
		return false
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(tool.Raw, &probe); err != nil {
		return false
	}
	switch probe.Type {
	case "", "custom", "function", ToolTypeWebSearch20250305, WebSearchFunctionName:
		return false
	default:
		return true
	}
}
