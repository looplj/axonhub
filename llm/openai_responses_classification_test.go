package llm

import "testing"

func TestIsKnownOpenAIResponsesNativeToolType(t *testing.T) {
	known := []string{
		"function", "image_generation", "web_search", "custom", "namespace",
		"tool_search", "mcp", "file_search", "code_interpreter",
		"computer_use_preview", "local_shell", "shell", "apply_patch",
	}
	for _, toolType := range known {
		if !IsKnownOpenAIResponsesNativeToolType(toolType) {
			t.Fatalf("expected %q to be a known OpenAI Responses native tool type", toolType)
		}
	}

	if IsKnownOpenAIResponsesNativeToolType("future_tool") {
		t.Fatal("future_tool must remain unknown")
	}
}

func TestIsKnownOpenAIResponsesInputItemType(t *testing.T) {
	known := []string{
		"", "message", "input_text", "input_image", "input_audio", "input_file",
		"function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output",
		"reasoning", "compaction", "compaction_summary",
		"tool_search_call", "tool_search_output", "web_search_call", "file_search_call",
		"image_generation_call", "code_interpreter_call", "computer_call",
		"local_shell_call", "local_shell_call_output", "shell_call", "shell_call_output",
		"mcp_list_tools", "mcp_approval_request", "mcp_approval_response", "mcp_call",
		"item_reference", "datetime", "web_search_server_tool", "code_interpreter_server_tool",
		"file_search_server_tool", "image_generation_server_tool", "browser_use_server_tool",
		"bash_server_tool", "text_editor_server_tool", "apply_patch_server_tool",
		"web_fetch_server_tool", "tool_search_server_tool", "memory_server_tool",
		"mcp_server_tool", "search_models_server_tool", "fusion_server_tool",
		"advisor_server_tool", "subagent_server_tool",
	}
	for _, itemType := range known {
		if !IsKnownOpenAIResponsesInputItemType(itemType) {
			t.Fatalf("expected %q to be a known OpenAI Responses input item type", itemType)
		}
	}

	if IsKnownOpenAIResponsesInputItemType("future_input_item") {
		t.Fatal("future_input_item must remain unknown")
	}
}
