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
// 还将单个 text block 的 MultipleContent 归一化为 Content 字符串形式。
// 仅在 ensureCacheControl 可能转换 Content→MultipleContent 的测试场景中使用。
var ignoreCacheControlWithNormalize = append(
	ignoreCacheControlOpts,
	cmp.Transformer("normalizeMessageContent", func(mc MessageContent) MessageContent {
		if mc.Content == nil && len(mc.MultipleContent) == 1 &&
			mc.MultipleContent[0].Type == "text" && mc.MultipleContent[0].Text != nil {
			return MessageContent{Content: mc.MultipleContent[0].Text}
		}
		return mc
	}),
)
