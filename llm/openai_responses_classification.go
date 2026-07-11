package llm

// IsKnownOpenAIResponsesNativeToolType reports whether a Responses tool type is
// a known OpenAI/Codex native tool shape, even if the common llm.Request model
// cannot structurally represent it yet.
func IsKnownOpenAIResponsesNativeToolType(toolType string) bool {
	switch toolType {
	case "function",
		"image_generation",
		"web_search",
		"custom",
		"namespace",
		"tool_search",
		"mcp",
		"file_search",
		"code_interpreter",
		"computer_use_preview",
		"local_shell",
		"shell",
		"apply_patch":
		return true
	default:
		return false
	}
}

// IsKnownOpenAIResponsesInputItemType reports whether a Responses input item
// type is a known OpenAI/Codex shape. Known raw-only items may still be lossy
// across non-Responses protocols; this only separates official/native shapes
// from future unknown variants for diagnostics.
func IsKnownOpenAIResponsesInputItemType(itemType string) bool {
	switch itemType {
	case "",
		"message",
		"input_text",
		"input_image",
		"input_audio",
		"input_file",
		"function_call",
		"function_call_output",
		"custom_tool_call",
		"custom_tool_call_output",
		"reasoning",
		"compaction",
		"compaction_summary",
		"tool_search_call",
		"tool_search_output",
		"web_search_call",
		"file_search_call",
		"image_generation_call",
		"code_interpreter_call",
		"computer_call",
		"local_shell_call",
		"local_shell_call_output",
		"shell_call",
		"shell_call_output",
		"mcp_list_tools",
		"mcp_approval_request",
		"mcp_approval_response",
		"mcp_call",
		"item_reference",
		"datetime",
		"web_search_server_tool",
		"code_interpreter_server_tool",
		"file_search_server_tool",
		"image_generation_server_tool",
		"browser_use_server_tool",
		"bash_server_tool",
		"text_editor_server_tool",
		"apply_patch_server_tool",
		"web_fetch_server_tool",
		"tool_search_server_tool",
		"memory_server_tool",
		"mcp_server_tool",
		"search_models_server_tool",
		"fusion_server_tool",
		"advisor_server_tool",
		"subagent_server_tool":
		return true
	default:
		return false
	}
}
