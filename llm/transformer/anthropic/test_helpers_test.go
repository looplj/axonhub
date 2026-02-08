package anthropic

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// ignoreCacheControlOpts 只忽略 cache_control 字段差异，保留结构严格对比。
// ensureCacheControl 注入行为已在 ensure_cache_control_test.go 中专门覆盖。
var ignoreCacheControlOpts = []cmp.Option{
	cmpopts.IgnoreFields(Tool{}, "CacheControl"),
	cmpopts.IgnoreFields(MessageContentBlock{}, "CacheControl"),
	cmpopts.IgnoreFields(SystemPromptPart{}, "CacheControl"),
}

// ignoreCacheControlWithNormalize 在忽略 cache_control 的基础上，
// 还将 ensureCacheControl 可能产生的结构变化归一化：
// 1. 单个 text block 的 MultipleContent → Content 字符串形式（MessageContent 归一化）
// 2. System.MultiplePrompts 单条文本 → System.Prompt 字符串形式（SystemPrompt 归一化）.
var ignoreCacheControlWithNormalize = append(
	ignoreCacheControlOpts,
	cmp.Transformer("normalizeMessageContent", func(mc MessageContent) MessageContent {
		if mc.Content == nil && len(mc.MultipleContent) == 1 &&
			mc.MultipleContent[0].Type == "text" && mc.MultipleContent[0].Text != nil {
			return MessageContent{Content: mc.MultipleContent[0].Text}
		}
		return mc
	}),
	cmp.Transformer("normalizeSystemPrompt", func(sp *SystemPrompt) *SystemPrompt {
		if sp == nil {
			return nil
		}
		// ensureStructuralCacheControls 会把 System.Prompt 归一化为 MultiplePrompts，
		// 这里反向还原：如果 MultiplePrompts 只有一条 text 且 Prompt 为空，还原回 Prompt 形式。
		if sp.Prompt == nil && len(sp.MultiplePrompts) == 1 && sp.MultiplePrompts[0].Type == "text" {
			text := sp.MultiplePrompts[0].Text
			return &SystemPrompt{Prompt: &text}
		}
		return sp
	}),
)
