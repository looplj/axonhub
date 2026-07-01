package shared

// TransformerMetadata keys shared across transformer packages. Defining these
// as exported constants (instead of bare string literals scattered across
// packages) prevents silent data loss from key typos — a map read with a
// misspelled key returns a zero value with no error.
const (
	// MetadataKeyModel carries the original request model name so outbound
	// response builders can backfill it (used by image/video/embedding outbounds).
	MetadataKeyModel = "model"
	// MetadataKeyInclude carries the responses API "include" directive
	// (e.g. "reasoning.encrypted_content"). Shared between responses and codex.
	MetadataKeyInclude = "include"
	// MetadataKeyResponsesWebSearchCalls carries responses API web_search call
	// results. Shared between responses (writer) and anthropic (reader).
	MetadataKeyResponsesWebSearchCalls = "openai_responses_web_search_calls"
)
