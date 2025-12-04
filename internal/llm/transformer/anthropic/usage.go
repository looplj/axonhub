package anthropic

import "github.com/looplj/axonhub/internal/llm"

// Usage represents usage information in Anthropic format.
type Usage struct {
	// The number of input tokens which were used to bill.
	InputTokens int64 `json:"input_tokens,omitempty"`

	// The number of output tokens which were used.
	OutputTokens int64 `json:"output_tokens,omitempty"`

	// The number of input tokens used to create the cache entry.
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`

	// The number of input tokens read from the cache.
	CacheReadInputTokens int64 `json:"cache_read_input_tokens,omitempty"`

	// Available options: standard, priority, batch
	ServiceTier string `json:"service_tier,omitempty"`

	// For moonshot anthropic endpoint, it uses cached tokens instead of cache read input tokens.
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

// https://docs.claude.com/en/api/messages#response-usage
// convertToLlmUsage converts Anthropic Usage to unified Usage format.
// The platformType parameter determines how cache tokens are calculated:
// - For Anthropic official (direct, bedrock, vertex): input_tokens does NOT include cached tokens
// - For Moonshot: input_tokens INCLUDES cached tokens.
func convertToLlmUsage(usage *Usage, platformType PlatformType) *llm.Usage {
	if usage == nil {
		return nil
	}

	// Handle moonshot's cached_tokens field
	if usage.CachedTokens > 0 && usage.CacheCreationInputTokens == 0 {
		usage.CacheReadInputTokens = usage.CachedTokens
	}

	var promptTokens int64

	// Different calculation logic based on platform type
	//nolint:exhaustive
	switch platformType {
	case PlatformMoonshot:
		// For Moonshot: InputTokens already includes cached tokens
		// So we don't add cache tokens again
		promptTokens = usage.InputTokens
	default:
		// For Anthropic official (direct, bedrock, vertex) or other platform: InputTokens does NOT include cached tokens
		// Total input tokens = input_tokens + cache_creation_input_tokens + cache_read_input_tokens
		promptTokens = usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
	}

	u := llm.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      promptTokens + usage.OutputTokens,
	}

	// Map detailed token information from Anthropic format to unified model
	if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 {
		u.PromptTokensDetails = &llm.PromptTokensDetails{
			CachedTokens: usage.CacheReadInputTokens + usage.CacheCreationInputTokens,
		}
	}

	return &u
}
