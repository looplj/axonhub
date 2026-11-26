package anthropic

import (
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
)

// Usage represents usage information in Anthropic format.
// Total input tokens in a request is the summation of input_tokens, cache_creation_input_tokens, and cache_read_input_tokens.
type Usage struct {
	CacheReadStatus llm.CacheReadStatus `json:"-"`
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

	// Cost is the request cost calculated by AxonHub from channel model prices.
	Cost *float64 `json:"cost,omitempty"`
}

type CacheCreation struct {
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
}

func (u *Usage) cacheReadTokens() int64 {
	// Moonshot 兼容字段保留其现有优先级，不改写原生用量。
	if u.CachedTokens != 0 && (u.CacheCreationInputTokens == 0 || u.CachedTokens < 0) {
		return u.CachedTokens
	}
	return u.CacheReadInputTokens
}

// mergeStreamUsage 先合并原生累计字段，再由调用方计算统一用量。省略字段沿用旧值，显式零覆盖旧值。
func mergeStreamUsage(previous, update *Usage, event []byte) *Usage {
	if previous == nil {
		return update
	}
	next := *previous
	// 已解析的数字给出值；仅检查 delta 中字段是否存在且非 null，以区别省略和显式零。
	fields := gjson.GetBytes(event, "usage")
	if fields.Get("input_tokens").Type == gjson.Number {
		next.InputTokens = update.InputTokens
	}
	if fields.Get("output_tokens").Type == gjson.Number {
		next.OutputTokens = update.OutputTokens
	}
	if update.CacheReadStatus == llm.CacheReadReported {
		next.CacheReadStatus = update.CacheReadStatus
		next.CacheReadInputTokens = update.CacheReadInputTokens
		next.CachedTokens = update.CachedTokens
	}
	if fields.Get("cache_creation_input_tokens").Type == gjson.Number {
		next.CacheCreationInputTokens = update.CacheCreationInputTokens
	}
	cacheCreation := fields.Get("cache_creation")
	if cacheCreation.Get("ephemeral_5m_input_tokens").Type == gjson.Number {
		next.CacheCreation.Ephemeral5mInputTokens = update.CacheCreation.Ephemeral5mInputTokens
	}
	if cacheCreation.Get("ephemeral_1h_input_tokens").Type == gjson.Number {
		next.CacheCreation.Ephemeral1hInputTokens = update.CacheCreation.Ephemeral1hInputTokens
	}
	return &next
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

	cacheRead := usage.cacheReadTokens()

	var promptTokens int64

	// Different calculation logic based on platform type
	//nolint:exhaustive
	switch platformType {
	case PlatformMoonshot:
		// Moonshot may return InputTokens as a net billed amount (can be negative with cache discounts),
		// or as the standard positive count. We handle both formats.
		if usage.InputTokens < 0 && cacheRead > 0 {
			promptTokens = usage.InputTokens + 2*cacheRead
		} else if usage.InputTokens >= 0 && cacheRead > 0 && usage.InputTokens < cacheRead {
			promptTokens = usage.InputTokens + cacheRead
		} else {
			promptTokens = usage.InputTokens
		}
	default:
		// For Anthropic official (direct, bedrock, vertex) or other platform: InputTokens does NOT include cached tokens
		// Total input tokens = input_tokens + cache_creation_input_tokens + cache_read_input_tokens
		promptTokens = usage.InputTokens + usage.CacheCreationInputTokens + cacheRead
	}

	u := llm.Usage{
		CacheReadStatus:         usage.CacheReadStatus,
		PromptTokens:            promptTokens,
		CompletionTokens:        usage.OutputTokens,
		CompletionTokensDetails: &llm.CompletionTokensDetails{},
		TotalTokens:             promptTokens + usage.OutputTokens,
	}

	// 保留负缓存读数，让统一验证层识别异常，而不是将其折叠成可信零值。
	if cacheRead != 0 || usage.CacheCreationInputTokens > 0 ||
		usage.CacheCreation.Ephemeral5mInputTokens > 0 || usage.CacheCreation.Ephemeral1hInputTokens > 0 {
		u.PromptTokensDetails = &llm.PromptTokensDetails{
			CachedTokens:           cacheRead,
			WriteCachedTokens:      usage.CacheCreationInputTokens,
			WriteCached5MinTokens:  usage.CacheCreation.Ephemeral5mInputTokens,
			WriteCached1HourTokens: usage.CacheCreation.Ephemeral1hInputTokens,
		}
	}

	return &u
}

func convertToAnthropicUsage(llmUsage *llm.Usage) *Usage {
	usage := &Usage{
		CacheReadStatus: llmUsage.CacheReadStatus,
		InputTokens:     llmUsage.PromptTokens,
		OutputTokens:    llmUsage.CompletionTokens,
		Cost:            llmUsage.Cost,
	}

	// Map detailed token information from unified model to Anthropic format
	if llmUsage.PromptTokensDetails != nil {
		if usage.CacheReadStatus == llm.CacheReadReported {
			usage.CacheReadInputTokens = llmUsage.PromptTokensDetails.CachedTokens
		}
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
