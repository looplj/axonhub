package anthropic

import "github.com/looplj/axonhub/llm"

// Usage represents usage information in Anthropic format.
// Total input tokens in a request is the summation of input_tokens, cache_creation_input_tokens, and cache_read_input_tokens.
type Usage struct {
	// The number of input tokens which were used to bill.
	InputTokens int64 `json:"input_tokens"`

	// The number of output tokens which were used.
	OutputTokens int64 `json:"output_tokens"`

	// The number of input tokens used to create the cache entry.
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`

	// The number of input tokens read from the cache.
	CacheReadInputTokens int64 `json:"cache_read_input_tokens"`

	// CacheCreation is the breakdown of cached tokens by TTL
	CacheCreation CacheCreation `json:"cache_creation"`

	// Available options: standard, priority, batch
	ServiceTier string `json:"service_tier,omitempty"`

	// For moonshot anthropic endpoint, it uses cached tokens instead of cache read input tokens.
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

type CacheCreation struct {
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
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
		PromptTokens:            promptTokens,
		CompletionTokens:        usage.OutputTokens,
		CompletionTokensDetails: &llm.CompletionTokensDetails{},
		TotalTokens:             promptTokens + usage.OutputTokens,
	}

	if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 ||
		usage.CacheCreation.Ephemeral5mInputTokens > 0 || usage.CacheCreation.Ephemeral1hInputTokens > 0 {
		u.PromptTokensDetails = &llm.PromptTokensDetails{
			CachedTokens:           usage.CacheReadInputTokens,
			WriteCachedTokens:      usage.CacheCreationInputTokens,
			WriteCached5MinTokens:  usage.CacheCreation.Ephemeral5mInputTokens,
			WriteCached1HourTokens: usage.CacheCreation.Ephemeral1hInputTokens,
		}
	}

	return &u
}

func convertToAnthropicUsage(llmUsage *llm.Usage) *Usage {
	usage := &Usage{
		InputTokens:  llmUsage.PromptTokens,
		OutputTokens: llmUsage.CompletionTokens,
	}

	// Map detailed token information from unified model to Anthropic format
	if llmUsage.PromptTokensDetails != nil {
		usage.CacheReadInputTokens = llmUsage.PromptTokensDetails.CachedTokens
		usage.CacheCreationInputTokens = llmUsage.PromptTokensDetails.WriteCachedTokens
		usage.CacheCreation = CacheCreation{
			Ephemeral5mInputTokens: llmUsage.PromptTokensDetails.WriteCached5MinTokens,
			Ephemeral1hInputTokens: llmUsage.PromptTokensDetails.WriteCached1HourTokens,
		}
		usage.InputTokens -= (usage.CacheReadInputTokens + usage.CacheCreationInputTokens)
	}

	// Note: Anthropic doesn't have a direct equivalent for reasoning tokens in their current API
	// but we can store it in cache_creation_input_tokens as a workaround if needed
	if llmUsage.CompletionTokensDetails != nil {
		// For now, we don't map reasoning tokens as Anthropic doesn't have a direct field
		// This could be extended in the future if Anthropic adds support
	}

	return usage
}
