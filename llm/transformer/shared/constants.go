package shared

import "encoding/base64"

// AnthropicSignaturePrefix is the prefix used for Anthropic thinking signatures.
// This is used to preserve Anthropic-specific signatures when converting between different providers.
// By marking these signatures with a unique prefix, AxonHub can ensure that Anthropic signatures
// are correctly identified and preserved during the transformation pipeline.
//
// Background (same-session channel/model switching):
// In a single user session, AxonHub may route consecutive requests to different channels/providers/models
// (e.g. due to load balancing, failover, or a user manually switching the channel). Some providers emit
// provider-specific "signature" blocks (thinking/reasoning/encrypted payloads). If these blocks are not
// tagged and preserved in a provider-agnostic way, they may be dropped or mis-parsed when the session
// later switches back to the original provider, causing context loss and degraded model behavior.
//
// Design:
// We wrap such provider-specific blocks with a stable base64 prefix ("AXN10x") so the transformation
// pipeline can safely carry them across provider boundaries and restore them when applicable.
var AnthropicSignaturePrefix = base64.StdEncoding.EncodeToString([]byte("AXN101"))

// GeminiThoughtSignaturePrefix is the prefix used for Gemini thought/reasoning signatures.
// In models like Gemini 2.0, reasoning process is a first-class citizen.
// This signature allows AxonHub to "wrap" and preserve these reasoning blocks in the internal
// message structure. This ensures that when switching between different providers (e.g., Gemini -> OpenAI -> Gemini),
// the original reasoning context is maintained and can be restored, preventing model performance degradation.
var GeminiThoughtSignaturePrefix = base64.StdEncoding.EncodeToString([]byte("AXN102"))

// OpenAIEncryptedContentPrefix is the prefix used for OpenAI encrypted content.
// This is used to preserve OpenAI-specific encrypted blocks when converting between different providers.
// By marking these blocks with a unique signature, AxonHub can ensure that encrypted data
// is not lost or corrupted during the transformation pipeline, allowing it to be restored
// when the conversation returns to an OpenAI-compatible model.
//
// This is especially important for same-session switching like:
// OpenAI (encrypted_content emitted) -> Gemini/Anthropic -> OpenAI,
// where the encrypted payload must remain intact even if intermediate providers can't interpret it.
var OpenAIEncryptedContentPrefix = base64.StdEncoding.EncodeToString([]byte("AXN103"))
