package anthropic

// maxCacheControlBreakpoints is the maximum number of cache_control breakpoints allowed by Anthropic.
// See https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching.
const (
	maxCacheControlBreakpoints      = 4
	adaptiveCacheControlBlockWindow = 20
)

// ensureCacheControl 统一自动修复 cache_control：
//   - strict mode：先清空全部断点，再按固定规划重建，避免历史断点造成抖动
//   - 强制结构锚点：tools(last) + system(last)
//   - 消息锚点策略：短内容 1 个、长内容 2 个（受 4 个上限约束）
//   - 消息首选“最后一条消息的最后一个可缓存块”，更贴近官方示例
//   - 第 2 个消息锚点优先落在“距离末尾约 20 块”的窗口边界
//   - thinking 与空 text 不允许打点
func ensureCacheControl(req *MessageRequest) {
	// 统一归一化：将 Content 字符串形式转为 MultipleContent 数组，
	// 后续所有函数只处理数组格式，消除隐式结构改写副作用。
	normalizeMessageContents(req)
	clearCacheControls(req)

	structural := ensureStructuralCacheControls(req)

	remaining := maxCacheControlBreakpoints - structural
	if remaining <= 0 {
		return
	}

	refs := collectMessageBlockRefs(req)
	messageAnchors := min(desiredMessageCacheAnchors(len(refs)), remaining)
	injectPlannedMessageCacheControls(refs, messageAnchors)

	// 最终安全检查：确保 thinking 和空 text 块上不会被意外注入 cache_control。
	sanitizeUnsupportedCacheControls(req)
}

// normalizeMessageContents 将 Messages 中的纯字符串 Content 统一归一化为 MultipleContent 数组格式。
// 必须在其他操作之前调用一次，避免后续函数产生隐式结构改写副作用。
func normalizeMessageContents(req *MessageRequest) {
	for i := range req.Messages {
		msg := &req.Messages[i]
		if len(msg.Content.MultipleContent) == 0 && msg.Content.Content != nil && *msg.Content.Content != "" {
			text := *msg.Content.Content
			msg.Content.Content = nil
			msg.Content.MultipleContent = []MessageContentBlock{{
				Type: "text",
				Text: &text,
			}}
		}
	}
}

// ensureStructuralCacheControls 确保 tools 和 system 的最后一个元素有 cache_control，
// 返回实际注入的结构锚点数量。
// 这些位置内容稳定、每次请求重复发送，是 Anthropic 推荐的缓存锚点。
func ensureStructuralCacheControls(req *MessageRequest) int {
	count := 0

	if len(req.Tools) > 0 {
		req.Tools[len(req.Tools)-1].CacheControl = &CacheControl{Type: "ephemeral"}
		count++
	}

	if req.System == nil {
		return count
	}

	if len(req.System.MultiplePrompts) > 0 {
		last := len(req.System.MultiplePrompts) - 1
		req.System.MultiplePrompts[last].CacheControl = &CacheControl{Type: "ephemeral"}
		count++

		return count
	}

	// system 是字符串形式时归一化为 MultiplePrompts 数组格式，
	// 以便 cache_control 正确注入到数组元素上。
	if req.System.Prompt != nil && *req.System.Prompt != "" {
		text := *req.System.Prompt
		req.System.Prompt = nil
		req.System.MultiplePrompts = []SystemPromptPart{{
			Type:         "text",
			Text:         text,
			CacheControl: &CacheControl{Type: "ephemeral"},
		}}
		count++
	}

	return count
}

// sanitizeUnsupportedCacheControls 清理不允许设置 cache_control 的内容块。
// Anthropic 不允许在 thinking 类型和空 text 块上设置 cache_control。
// 在 strict mode 下用作最终安全检查，防止注入逻辑意外命中不可缓存块。
func sanitizeUnsupportedCacheControls(req *MessageRequest) {
	for i := range req.Messages {
		for j := range req.Messages[i].Content.MultipleContent {
			block := &req.Messages[i].Content.MultipleContent[j]
			if !isCacheableMessageBlock(*block) && block.CacheControl != nil {
				block.CacheControl = nil
			}
		}
	}
}

func desiredMessageCacheAnchors(cacheableBlocks int) int {
	if cacheableBlocks == 0 {
		return 0
	}

	if cacheableBlocks >= adaptiveCacheControlBlockWindow {
		return 2
	}

	return 1
}

func injectPlannedMessageCacheControls(refs []**CacheControl, target int) {
	if target <= 0 || len(refs) == 0 {
		return
	}

	// 第一优先级：最后一个可缓存块（官方推荐会话末尾断点）。
	*refs[len(refs)-1] = &CacheControl{Type: "ephemeral"}

	// 第二优先级（仅需要多个消息锚点时）：末尾前 20 blocks 的窗口边界。
	if target > 1 {
		idx := pickWindowAnchorIndex(refs, adaptiveCacheControlBlockWindow)
		if idx >= 0 {
			*refs[idx] = &CacheControl{Type: "ephemeral"}
		}
	}
}

// pickWindowAnchorIndex 在 refs 中选择距离末尾 window 个位置的未标记锚点。
// 优先选择目标位置左侧（更靠近稳定前缀），找不到时向右回退。
func pickWindowAnchorIndex(refs []**CacheControl, window int) int {
	if len(refs) == 0 {
		return -1
	}

	if window < 0 {
		window = 0
	}

	target := max(len(refs)-1-window, 0)

	// 优先选择目标窗口左侧（更靠近稳定前缀）
	for i := target; i >= 0; i-- {
		if *refs[i] != nil {
			continue
		}

		return i
	}

	for i := target + 1; i < len(refs); i++ {
		if *refs[i] != nil {
			continue
		}

		return i
	}

	return -1
}

// collectMessageBlockRefs 收集所有消息中可缓存块的 CacheControl 指针引用。
// 调用前必须先执行 normalizeMessageContents，本函数不做结构改写。
func collectMessageBlockRefs(req *MessageRequest) []**CacheControl {
	refs := make([]**CacheControl, 0)

	for i := range req.Messages {
		for j := range req.Messages[i].Content.MultipleContent {
			if !isCacheableMessageBlock(req.Messages[i].Content.MultipleContent[j]) {
				continue
			}

			refs = append(refs, &req.Messages[i].Content.MultipleContent[j].CacheControl)
		}
	}

	return refs
}

// clearCacheControls removes all cache_control breakpoints from tools/system/messages.
func clearCacheControls(req *MessageRequest) {
	for i := range req.Tools {
		req.Tools[i].CacheControl = nil
	}

	if req.System != nil {
		for i := range req.System.MultiplePrompts {
			req.System.MultiplePrompts[i].CacheControl = nil
		}
	}

	for i := range req.Messages {
		msg := &req.Messages[i]
		for j := range msg.Content.MultipleContent {
			msg.Content.MultipleContent[j].CacheControl = nil
		}
	}
}

// countCacheControls counts all cache_control breakpoints in tools/system/messages.
func countCacheControls(req *MessageRequest) int {
	count := 0

	// Count tools.
	for i := range req.Tools {
		if req.Tools[i].CacheControl != nil {
			count++
		}
	}

	// Count system prompts.
	if req.System != nil {
		for i := range req.System.MultiplePrompts {
			if req.System.MultiplePrompts[i].CacheControl != nil {
				count++
			}
		}
	}

	// Count message content blocks.
	for i := range req.Messages {
		msg := &req.Messages[i]
		for j := range msg.Content.MultipleContent {
			if isCacheableMessageBlock(msg.Content.MultipleContent[j]) && msg.Content.MultipleContent[j].CacheControl != nil {
				count++
			}
		}
	}

	return count
}

func isCacheableMessageBlock(block MessageContentBlock) bool {
	switch block.Type {
	case "thinking", "redacted_thinking":
		return false
	case "text":
		return block.Text != nil && *block.Text != ""
	default:
		return true
	}
}
