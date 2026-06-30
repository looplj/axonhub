package shared

// TransformerMetadataKeyTopK is the neutral key for storing the top_k sampling
// parameter in TransformerMetadata. canonical llm.Request has no TopK field, so
// top_k is carried here to survive cross-format conversion (chat/responses/anthropic).
// Anthropic previously used the legacy key "anthropic_top_k"; outbound readers
// should fall back to it for backward compatibility with persisted metadata.
const TransformerMetadataKeyTopK = "top_k"

// TransformerMetadataKeyRepetitionPenalty is the neutral key for the repetition_penalty
// sampling parameter (OpenRouter extension, chat-only). canonical llm.Request has no
// corresponding field, so it is carried through TransformerMetadata.
const TransformerMetadataKeyRepetitionPenalty = "repetition_penalty"

// TransformerMetadataKeyMinP is the neutral key for the min_p sampling parameter.
const TransformerMetadataKeyMinP = "min_p"

// TransformerMetadataKeyTopA is the neutral key for the top_a sampling parameter.
const TransformerMetadataKeyTopA = "top_a"
