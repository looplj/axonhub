package shared

// TransformerMetadataKeyCacheControl is the neutral key for storing the top-level
// cache_control value (OpenRouter/Anthropic prompt-caching marker) in
// TransformerMetadata. canonical llm.Request has no CacheControl field, so
// cache_control is carried here as opaque json.RawMessage to survive
// cross-format conversion (chat/responses/anthropic).
//
// The value is retained as "anthropic_cache_control" for backward compatibility
// with persisted metadata written by the legacy Anthropic-only key.
const TransformerMetadataKeyCacheControl = "anthropic_cache_control"
