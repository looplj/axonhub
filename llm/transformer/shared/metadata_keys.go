package shared

// TransformerMetadata keys shared across transformer packages. Defining these
// as exported constants (instead of bare string literals scattered across
// packages) prevents silent data loss from key typos — a map read with a
// misspelled key returns a zero value with no error.
const (
	// MetadataKeyModel carries the original request model name so outbound
	// response builders can backfill it (used by image/video/embedding outbounds).
	MetadataKeyModel = "model"
	// MetadataKeyInclude is deprecated for Responses body ownership.
	// Responses "include" is stored on ProviderExtensions.OpenAIResponses.Request.Include.
	// Kept only so old docs/tests that mention the symbol can be grepped; do not write this key.
	MetadataKeyInclude = "include"
	// MetadataKeyResponsesWebSearchCalls carries responses API web_search call
	// results. Shared between responses (writer) and anthropic (reader).
	MetadataKeyResponsesWebSearchCalls = "openai_responses_web_search_calls"
)
