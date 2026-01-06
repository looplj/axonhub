package anthropic

import (
	"github.com/looplj/axonhub/llm"
)

func convertToAnthropicCacheControl(cacheControl *llm.CacheControl) *CacheControl {
	if cacheControl == nil {
		return nil
	}

	return &CacheControl{
		Type: cacheControl.Type,
		TTL:  cacheControl.TTL,
	}
}

func convertToLLMCacheControl(c *CacheControl) *llm.CacheControl {
	if c == nil {
		return nil
	}

	return &llm.CacheControl{
		Type: c.Type,
		TTL:  c.TTL,
	}
}
