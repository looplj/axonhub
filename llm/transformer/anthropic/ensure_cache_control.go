package anthropic

import (
	"bytes"
	"encoding/json"
)

// maxCacheControlBreakpoints is the maximum number of cache_control breakpoints allowed by Anthropic.
// See https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching.
const (
	maxCacheControlBreakpoints      = 4
	adaptiveCacheControlBlockWindow = 20
)

const (
	cacheControlSectionTools = iota
	cacheControlSectionSystem
	cacheControlSectionMessages
)

func optimizeCacheControl(req *MessageRequest) {
	normalizeMessageContents(req)
	sanitizeUnsupportedCacheControls(req)
	trimCacheControlsToLimit(req, maxCacheControlBreakpoints)

	remaining := maxCacheControlBreakpoints - countCacheControls(req)
	if remaining <= 0 {
		return
	}

	ensureStructuralCacheControls(req, remaining)
	if countMessageBreakpoints(req) > 0 {
		return
	}

	remaining = maxCacheControlBreakpoints - countCacheControls(req)
	refs := collectMessageBlockRefs(req)
	messageAnchors := min(desiredMessageCacheAnchors(len(refs)), remaining)
	injectPlannedMessageCacheControls(refs, messageAnchors)
}

func moveTopLevelCacheControlToLastBlock(req *MessageRequest) bool {
	cacheControl := req.CacheControl
	req.CacheControl = nil
	if cacheControl == nil {
		return true
	}
	refs := normalizeAndCollectCacheControlRefs(req)
	if len(refs) > 0 {
		last := refs[len(refs)-1].get()
		if last != nil && isOneHourCacheControl(last) == isOneHourCacheControl(cacheControl) {
			return true
		}
	}
	if countCacheControls(req) >= maxCacheControlBreakpoints {
		return false
	}
	if isOneHourCacheControl(cacheControl) {
		for i, ref := range refs {
			if existing := ref.get(); existing != nil && !isOneHourCacheControl(existing) {
				for j := i - 1; j >= 0; j-- {
					if refs[j].get() == nil {
						refs[j].set(cacheControl)
						return true
					}
				}

				return false
			}
		}
	}

	lastOneHour := -1
	for i, ref := range refs {
		if isOneHourCacheControl(ref.get()) {
			lastOneHour = i
		}
	}
	for i := len(refs) - 1; i > lastOneHour; i-- {
		if refs[i].get() == nil {
			refs[i].set(cacheControl)
			return true
		}
	}

	return false
}

func normalizeAndCollectCacheControlRefs(req *MessageRequest) []cacheControlSlot {
	normalizeMessageContents(req)
	refs := make([]cacheControlSlot, 0, len(req.Tools)+len(req.Messages)+1)
	for i := range req.Tools {
		cacheControl := &req.Tools[i].CacheControl
		refs = append(refs, newCacheControlSlot(cacheControl))
	}

	if req.System != nil {
		if len(req.System.MultiplePrompts) > 0 {
			for i := range req.System.MultiplePrompts {
				cacheControl := &req.System.MultiplePrompts[i].CacheControl
				refs = append(refs, newCacheControlSlot(cacheControl))
			}
		}
		if len(req.System.MultiplePrompts) == 0 && req.System.Prompt != nil && *req.System.Prompt != "" {
			text := *req.System.Prompt
			req.System.Prompt = nil
			req.System.MultiplePrompts = []SystemPromptPart{{
				Type: "text",
				Text: text,
			}}
			cacheControl := &req.System.MultiplePrompts[0].CacheControl
			refs = append(refs, newCacheControlSlot(cacheControl))
		}
	}

	for i := range req.Messages {
		refs = appendCacheControlSlots(refs, &req.Messages[i].Content)
	}
	return refs
}

type cacheControlSlot struct {
	ref **CacheControl
	get func() *CacheControl
	set func(*CacheControl)
}

func newCacheControlSlot(ref **CacheControl) cacheControlSlot {
	return cacheControlSlot{
		ref: ref,
		get: func() *CacheControl { return *ref },
		set: func(value *CacheControl) { *ref = value },
	}
}

func appendCacheControlSlots(refs []cacheControlSlot, content *MessageContent) []cacheControlSlot {
	if len(content.Raw) > 0 {
		return appendRawCacheControlSlots(refs, content)
	}

	for i := range content.MultipleContent {
		block := &content.MultipleContent[i]
		if isCacheableMessageBlock(*block) {
			cacheControl := &block.CacheControl
			refs = append(refs, newCacheControlSlot(cacheControl))
		}
		if block.Content != nil {
			refs = appendCacheControlSlots(refs, block.Content)
		}
	}

	return refs
}

func appendRawCacheControlSlots(refs []cacheControlSlot, content *MessageContent) []cacheControlSlot {
	decoded, err := decodeRawContent(content.Raw)
	if err != nil {
		return refs
	}

	walkRawContent(decoded, func(block map[string]any) bool {
		if !isCacheableRawBlock(block) {
			return false
		}
		refs = append(refs, cacheControlSlot{
			get: func() *CacheControl { return rawCacheControl(block["cache_control"]) },
			set: func(value *CacheControl) {
				if value == nil {
					delete(block, "cache_control")
				} else {
					block["cache_control"] = value
				}
				content.Raw, _ = json.Marshal(decoded)
			},
		})
		return false
	})
	return refs
}

func isCacheableRawBlock(block map[string]any) bool {
	typeName, ok := block["type"].(string)
	if !ok || typeName == "thinking" || typeName == "redacted_thinking" {
		return false
	}
	if typeName != "text" {
		return true
	}
	text, ok := block["text"].(string)
	return ok && text != ""
}

func rawCacheControl(value any) *CacheControl {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var cacheControl CacheControl
	if err := json.Unmarshal(data, &cacheControl); err != nil {
		return nil
	}
	return &cacheControl
}

func isOneHourCacheControl(cacheControl *CacheControl) bool {
	return cacheControl != nil && cacheControl.TTL == "1h"
}

// countMessageBreakpoints 统计 messages 上已存在的 cache_control 断点数（不含 tools/system）。
func countMessageBreakpoints(req *MessageRequest) int {
	count := 0

	for i := range req.Messages {
		count += countContentCacheControls(&req.Messages[i].Content)
	}

	return count
}

// trimCacheControlsToLimit trims message breakpoints first to preserve the
// existing policy, then deterministically trims structural-only overflow.
func trimCacheControlsToLimit(req *MessageRequest, limit int) {
	for countCacheControls(req) > limit {
		if !removeEarliestMessageBreakpoint(req) && !removeEarliestStructuralBreakpoint(req) {
			return
		}
	}
}

func removeEarliestMessageBreakpoint(req *MessageRequest) bool {
	for i := range req.Messages {
		if removeEarliestContentBreakpoint(&req.Messages[i].Content) {
			return true
		}
	}

	return false
}

func removeEarliestContentBreakpoint(content *MessageContent) bool {
	if len(content.Raw) > 0 {
		return removeEarliestRawCacheControl(content)
	}

	for i := range content.MultipleContent {
		block := &content.MultipleContent[i]
		if block.CacheControl != nil {
			block.CacheControl = nil
			return true
		}
		if block.Content != nil && removeEarliestContentBreakpoint(block.Content) {
			return true
		}
	}

	return false
}

func removeEarliestStructuralBreakpoint(req *MessageRequest) bool {
	for i := range req.Tools {
		if req.Tools[i].CacheControl != nil {
			req.Tools[i].CacheControl = nil
			return true
		}
	}

	if req.System != nil {
		for i := range req.System.MultiplePrompts {
			if req.System.MultiplePrompts[i].CacheControl != nil {
				req.System.MultiplePrompts[i].CacheControl = nil
				return true
			}
		}
	}

	return false
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

// ensureStructuralCacheControls fills tools and system within the global
// budget. Generated 5m anchors are never placed before a client 1h anchor.
func ensureStructuralCacheControls(req *MessageRequest, remaining int) {
	oneHourSection := lastOneHourCacheControlSection(req)
	if remaining > 0 && oneHourSection <= cacheControlSectionTools && len(req.Tools) > 0 && !anyToolHasCacheControl(req) {
		req.Tools[len(req.Tools)-1].CacheControl = &CacheControl{Type: "ephemeral"}
		remaining--
	}

	if remaining <= 0 || oneHourSection > cacheControlSectionSystem || req.System == nil {
		return
	}

	if len(req.System.MultiplePrompts) > 0 {
		if !anySystemPromptHasCacheControl(req) {
			last := len(req.System.MultiplePrompts) - 1
			req.System.MultiplePrompts[last].CacheControl = &CacheControl{Type: "ephemeral"}
		}

		return
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
	}
}

func anyToolHasCacheControl(req *MessageRequest) bool {
	for i := range req.Tools {
		if req.Tools[i].CacheControl != nil {
			return true
		}
	}

	return false
}

func anySystemPromptHasCacheControl(req *MessageRequest) bool {
	for i := range req.System.MultiplePrompts {
		if req.System.MultiplePrompts[i].CacheControl != nil {
			return true
		}
	}

	return false
}

func lastOneHourCacheControlSection(req *MessageRequest) int {
	section := -1
	for i := range req.Tools {
		if req.Tools[i].CacheControl != nil && req.Tools[i].CacheControl.TTL == "1h" {
			section = cacheControlSectionTools
		}
	}

	if req.System != nil {
		for i := range req.System.MultiplePrompts {
			if req.System.MultiplePrompts[i].CacheControl != nil && req.System.MultiplePrompts[i].CacheControl.TTL == "1h" {
				section = cacheControlSectionSystem
			}
		}
	}

	for i := range req.Messages {
		if contentHasOneHourCacheControl(&req.Messages[i].Content) {
			return cacheControlSectionMessages
		}
	}

	return section
}

// sanitizeUnsupportedCacheControls 清理不允许设置 cache_control 的内容块。
// Anthropic 不允许在 thinking 类型和空 text 块上设置 cache_control。
// 在 strict mode 下用作最终安全检查，防止注入逻辑意外命中不可缓存块。
func sanitizeUnsupportedCacheControls(req *MessageRequest) {
	for i := range req.Messages {
		sanitizeContentCacheControls(&req.Messages[i].Content)
	}
}

func sanitizeContentCacheControls(content *MessageContent) {
	if len(content.Raw) > 0 {
		return
	}

	for i := range content.MultipleContent {
		block := &content.MultipleContent[i]
		if !isCacheableMessageBlock(*block) && block.CacheControl != nil {
			block.CacheControl = nil
		}
		if block.Content != nil {
			sanitizeContentCacheControls(block.Content)
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
func collectMessageBlockRefs(req *MessageRequest) []**CacheControl {
	normalizeMessageContents(req)
	slots := make([]cacheControlSlot, 0, len(req.Messages))
	for i := range req.Messages {
		slots = appendCacheControlSlots(slots, &req.Messages[i].Content)
	}

	refs := make([]**CacheControl, 0)
	for _, slot := range slots {
		if slot.ref != nil {
			refs = append(refs, slot.ref)
		}
	}
	return refs
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
		count += countContentCacheControls(&req.Messages[i].Content)
	}

	return count
}

func countContentCacheControls(content *MessageContent) int {
	if len(content.Raw) > 0 {
		value, err := decodeRawContent(content.Raw)
		if err != nil {
			return 0
		}

		count := 0
		walkRawContent(value, func(block map[string]any) bool {
			if block["cache_control"] != nil {
				count++
			}
			return false
		})

		return count
	}

	count := 0
	for i := range content.MultipleContent {
		block := &content.MultipleContent[i]
		if isCacheableMessageBlock(*block) && block.CacheControl != nil {
			count++
		}
		if block.Content != nil {
			count += countContentCacheControls(block.Content)
		}
	}

	return count
}

func contentHasOneHourCacheControl(content *MessageContent) bool {
	if len(content.Raw) > 0 {
		value, err := decodeRawContent(content.Raw)
		if err != nil {
			return false
		}

		found := false
		walkRawContent(value, func(block map[string]any) bool {
			control, ok := block["cache_control"].(map[string]any)
			if ok && control["ttl"] == "1h" {
				found = true
				return true
			}
			return false
		})

		return found
	}

	for i := range content.MultipleContent {
		block := &content.MultipleContent[i]
		if block.CacheControl != nil && block.CacheControl.TTL == "1h" {
			return true
		}
		if block.Content != nil && contentHasOneHourCacheControl(block.Content) {
			return true
		}
	}

	return false
}

func removeEarliestRawCacheControl(content *MessageContent) bool {
	value, err := decodeRawContent(content.Raw)
	if err != nil {
		return false
	}

	removed := walkRawContent(value, func(block map[string]any) bool {
		if block["cache_control"] == nil {
			return false
		}
		delete(block, "cache_control")
		return true
	})
	if !removed {
		return false
	}

	updated, err := json.Marshal(value)
	if err != nil {
		return false
	}

	var blocks []MessageContentBlock
	if err := json.Unmarshal(updated, &blocks); err != nil {
		return false
	}

	content.Raw = updated
	content.MultipleContent = blocks

	return true
}

func decodeRawContent(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	return value, nil
}

func walkRawContent(value any, visit func(map[string]any) bool) bool {
	switch value := value.(type) {
	case []any:
		for _, item := range value {
			if walkRawContent(item, visit) {
				return true
			}
		}
	case map[string]any:
		if visit(value) {
			return true
		}
		if nested, ok := value["content"]; ok {
			return walkRawContent(nested, visit)
		}
	}

	return false
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
