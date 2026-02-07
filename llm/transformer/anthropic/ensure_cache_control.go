package anthropic

// maxCacheControlBreakpoints is the maximum number of cache_control breakpoints allowed by Anthropic.
// See https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching.
const maxCacheControlBreakpoints = 4
const adaptiveCacheControlBlockWindow = 20

// ensureCacheControl 统一自动修复 cache_control：
//   - count == 0: 自动注入推荐断点
//   - count > 4: 仅保留最近 4 个断点（从后往前）
//   - 长内容按 block 密度补充断点（仍受 4 个上限约束）
func ensureCacheControl(req *MessageRequest) {
	if countCacheControls(req) == 0 {
		injectOptimalCacheControls(req)
	}

	trimCacheControlsToLastN(req, maxCacheControlBreakpoints)
	injectAdaptiveCacheControls(req)
	trimCacheControlsToLastN(req, maxCacheControlBreakpoints)
}

func injectAdaptiveCacheControls(req *MessageRequest) {
	target := desiredCacheControlBreakpoints(req)
	current := countCacheControls(req)
	if current >= target || current >= maxCacheControlBreakpoints {
		return
	}

	need := min(target-current, maxCacheControlBreakpoints-current)
	if need <= 0 {
		return
	}

	candidates := collectMessageBlockRefs(req)
	if len(candidates) == 0 {
		return
	}

	for boundary := len(candidates) - 1; boundary >= 0 && need > 0; boundary -= adaptiveCacheControlBlockWindow {
		lower := max(boundary-adaptiveCacheControlBlockWindow+1, 0)
		for i := boundary; i >= lower; i-- {
			if *candidates[i] == nil {
				*candidates[i] = &CacheControl{Type: "ephemeral"}
				need--
				break
			}
		}
	}
}

func desiredCacheControlBreakpoints(req *MessageRequest) int {
	totalBlocks := countCacheableBlocks(req)
	if totalBlocks <= adaptiveCacheControlBlockWindow {
		return 0
	}

	target := (totalBlocks + adaptiveCacheControlBlockWindow - 1) / adaptiveCacheControlBlockWindow
	if target > maxCacheControlBreakpoints {
		target = maxCacheControlBreakpoints
	}
	return target
}

func countCacheableBlocks(req *MessageRequest) int {
	total := len(req.Tools)

	if req.System != nil {
		switch {
		case len(req.System.MultiplePrompts) > 0:
			total += len(req.System.MultiplePrompts)
		case req.System.Prompt != nil && *req.System.Prompt != "":
			total++
		}
	}

	for i := range req.Messages {
		msg := &req.Messages[i]
		if len(msg.Content.MultipleContent) > 0 {
			total += len(msg.Content.MultipleContent)
			continue
		}
		if msg.Content.Content != nil && *msg.Content.Content != "" {
			total++
		}
	}

	return total
}

func trimCacheControlsToLastN(req *MessageRequest, n int) {
	if n <= 0 {
		clearCacheControls(req)
		return
	}

	all := collectCacheControlRefs(req)
	if len(all) <= n {
		return
	}

	for i := 0; i < len(all)-n; i++ {
		*all[i] = nil
	}
}

func collectCacheControlRefs(req *MessageRequest) []**CacheControl {
	all := make([]**CacheControl, 0, countCacheControls(req))

	for i := range req.Tools {
		if req.Tools[i].CacheControl != nil {
			all = append(all, &req.Tools[i].CacheControl)
		}
	}

	if req.System != nil {
		for i := range req.System.MultiplePrompts {
			if req.System.MultiplePrompts[i].CacheControl != nil {
				all = append(all, &req.System.MultiplePrompts[i].CacheControl)
			}
		}
	}

	for i := range req.Messages {
		for j := range req.Messages[i].Content.MultipleContent {
			if req.Messages[i].Content.MultipleContent[j].CacheControl != nil {
				all = append(all, &req.Messages[i].Content.MultipleContent[j].CacheControl)
			}
		}
	}

	return all
}

func collectMessageBlockRefs(req *MessageRequest) []**CacheControl {
	refs := make([]**CacheControl, 0)

	for i := range req.Messages {
		msg := &req.Messages[i]
		if len(msg.Content.MultipleContent) > 0 {
			for j := range msg.Content.MultipleContent {
				refs = append(refs, &msg.Content.MultipleContent[j].CacheControl)
			}
			continue
		}

		if msg.Content.Content != nil && *msg.Content.Content != "" {
			text := *msg.Content.Content
			msg.Content.Content = nil
			msg.Content.MultipleContent = []MessageContentBlock{{
				Type: "text",
				Text: &text,
			}}
			refs = append(refs, &msg.Content.MultipleContent[0].CacheControl)
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
			if msg.Content.MultipleContent[j].CacheControl != nil {
				count++
			}
		}
	}

	return count
}

// injectOptimalCacheControls injects Anthropic-recommended breakpoints:
//  1. last tool
//  2. last system prompt
//  3. last content block of the second-to-last user turn
func injectOptimalCacheControls(req *MessageRequest) {
	// 1. Last tool.
	if len(req.Tools) > 0 {
		req.Tools[len(req.Tools)-1].CacheControl = &CacheControl{Type: "ephemeral"}
	}

	// 2. Last system prompt.
	if req.System != nil {
		// 如果 system 是纯字符串形式，先归一化为 MultiplePrompts 数组格式，
		// 这样 cache_control 才能正确注入到数组元素上。
		if req.System.Prompt != nil && len(req.System.MultiplePrompts) == 0 {
			text := *req.System.Prompt
			req.System.Prompt = nil
			req.System.MultiplePrompts = []SystemPromptPart{
				{Type: "text", Text: text},
			}
		}

		if len(req.System.MultiplePrompts) > 0 {
			last := len(req.System.MultiplePrompts) - 1
			req.System.MultiplePrompts[last].CacheControl = &CacheControl{Type: "ephemeral"}
		}
	}

	// 3. Last content block of second-to-last user turn.
	injectUserTurnCacheControl(req, &CacheControl{Type: "ephemeral"})
}

// injectUserTurnCacheControl injects cache_control into the last block of the second-to-last user turn.
// At least two user turns are required.
func injectUserTurnCacheControl(req *MessageRequest, cc *CacheControl) {
	// Collect all user turn indices.
	var userIndices []int
	for i := range req.Messages {
		if req.Messages[i].Role == "user" {
			userIndices = append(userIndices, i)
		}
	}

	// Need at least two user turns.
	if len(userIndices) < 2 {
		return
	}

	secondToLastIdx := userIndices[len(userIndices)-2]
	msg := &req.Messages[secondToLastIdx]

	// If content is array-based, set cache_control on the last block.
	if len(msg.Content.MultipleContent) > 0 {
		last := len(msg.Content.MultipleContent) - 1
		msg.Content.MultipleContent[last].CacheControl = cc
		return
	}

	// If content is plain text, convert to block format first.
	if msg.Content.Content != nil && *msg.Content.Content != "" {
		text := *msg.Content.Content
		msg.Content.Content = nil
		msg.Content.MultipleContent = []MessageContentBlock{
			{
				Type:         "text",
				Text:         &text,
				CacheControl: cc,
			},
		}
	}
}
