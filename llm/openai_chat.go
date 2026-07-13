package llm

import "encoding/json"

// OpenAIChatCustomTool carries the Chat Completions custom-tool declaration.
// It is intentionally distinct from OpenAI Responses custom-tool shapes; only
// the OpenAI Chat adapter reads or emits it.
type OpenAIChatCustomTool struct {
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Format      json.RawMessage `json:"format,omitempty"`
}

// OpenAIChatCustomToolChoice carries a named Chat custom-tool choice.
type OpenAIChatCustomToolChoice struct {
	Name string `json:"name"`
}

// OpenAIChatAllowedToolsChoice carries the Chat-only allowed_tools choice.
// Tool references remain raw because the official Chat shape is an array of
// maps and is not a cross-protocol llm.Tool semantic.
type OpenAIChatAllowedToolsChoice struct {
	Mode  string            `json:"mode"`
	Tools []json.RawMessage `json:"tools"`
}

// OpenAIChatCustomToolCall carries a Chat custom-tool call. Index is optional
// because Chat completion responses omit it while stream deltas use it.
type OpenAIChatCustomToolCall struct {
	Name  string `json:"name"`
	Input string `json:"input"`
	Index *int   `json:"index,omitempty"`
}

// OpenAIChatFileContentPart carries the native Chat file part payload. The
// pointer fields preserve the distinction between omission and an empty value.
type OpenAIChatFileContentPart struct {
	FileData *string `json:"file_data,omitempty"`
	FileID   *string `json:"file_id,omitempty"`
	Filename *string `json:"filename,omitempty"`
}
