package anthropic

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- countCacheControls ---

func TestCountCacheControls(t *testing.T) {
	t.Run("空请求返回 0", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
			},
		}
		assert.Equal(t, 0, countCacheControls(req))
	})

	t.Run("tools 中有断点", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "a"},
				{Name: "b", CacheControl: &CacheControl{Type: "ephemeral"}},
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
			},
		}
		assert.Equal(t, 1, countCacheControls(req))
	})

	t.Run("system 中有断点", func(t *testing.T) {
		req := &MessageRequest{
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "a", CacheControl: &CacheControl{Type: "ephemeral"}},
					{Type: "text", Text: "b", CacheControl: &CacheControl{Type: "ephemeral"}},
				},
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
			},
		}
		assert.Equal(t, 2, countCacheControls(req))
	})

	t.Run("messages 内容块中有断点", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("a"), CacheControl: &CacheControl{Type: "ephemeral"}},
							{Type: "text", Text: lo.ToPtr("b")},
						},
					},
				},
			},
		}
		assert.Equal(t, 1, countCacheControls(req))
	})

	t.Run("混合场景统计所有断点", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "t1", CacheControl: &CacheControl{Type: "ephemeral"}},
			},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "sys", CacheControl: &CacheControl{Type: "ephemeral"}},
				},
			},
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("msg"), CacheControl: &CacheControl{Type: "ephemeral"}},
						},
					},
				},
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "tool_result", ToolUseID: lo.ToPtr("id1"), CacheControl: &CacheControl{Type: "ephemeral"}},
						},
					},
				},
			},
		}
		assert.Equal(t, 4, countCacheControls(req))
	})
}

// --- ensureCacheControl ---

func TestEnsureCacheControl_ZeroBreakpoints_AutoInjects(t *testing.T) {
	t.Run("有 tools + system + 多个 user turn 时注入 3 个断点", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "tool_a"},
				{Name: "tool_b"},
			},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "prompt_1"},
					{Type: "text", Text: "prompt_2"},
				},
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("first user msg")}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("second user msg")}},
			},
		}

		ensureCacheControl(req)

		// tools 最后一个被注入
		assert.Nil(t, req.Tools[0].CacheControl)
		assert.NotNil(t, req.Tools[1].CacheControl)
		assert.Equal(t, "ephemeral", req.Tools[1].CacheControl.Type)

		// system 最后一个被注入
		assert.Nil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.NotNil(t, req.System.MultiplePrompts[1].CacheControl)
		assert.Equal(t, "ephemeral", req.System.MultiplePrompts[1].CacheControl.Type)

		// 会话末尾（最后一条消息）被转为数组格式并注入
		assert.Len(t, req.Messages[2].Content.MultipleContent, 1)
		assert.NotNil(t, req.Messages[2].Content.MultipleContent[0].CacheControl)
		assert.Equal(t, "ephemeral", req.Messages[2].Content.MultipleContent[0].CacheControl.Type)

		// 总计 3 个断点
		assert.Equal(t, 3, countCacheControls(req))
	})

	t.Run("没有 tools 和 system 时只注入 messages 断点", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("first")}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("second")}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 1, countCacheControls(req))
	})

	t.Run("只有 1 个 user turn 时会注入 1 个消息断点", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{{Name: "t1"}},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("only one")}},
			},
		}

		ensureCacheControl(req)

		// tools 结构锚点 + 1 个消息锚点
		assert.Equal(t, 2, countCacheControls(req))
		assert.NotNil(t, req.Tools[0].CacheControl)
	})

	t.Run("user turn 是数组格式内容时在最后一个块上注入", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("part1")},
							{Type: "text", Text: lo.ToPtr("part2")},
						},
					},
				},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("latest")}},
			},
		}

		ensureCacheControl(req)

		// 会话末尾（最后一条消息）优先注入
		require.Len(t, req.Messages[2].Content.MultipleContent, 1)
		assert.NotNil(t, req.Messages[2].Content.MultipleContent[0].CacheControl)
		assert.Equal(t, "ephemeral", req.Messages[2].Content.MultipleContent[0].CacheControl.Type)
	})
}

func TestEnsureCacheControl_WithinLimit_NoModification(t *testing.T) {
	t.Run("客户端设了 1 个断点时会补齐策略性断点", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "t1", CacheControl: &CacheControl{Type: "ephemeral"}},
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 2, countCacheControls(req))
	})

	t.Run("客户端设了 4 个断点时会按策略重排并去重", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "t1", CacheControl: &CacheControl{Type: "ephemeral"}},
			},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "s1", CacheControl: &CacheControl{Type: "ephemeral"}},
				},
			},
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("a"), CacheControl: &CacheControl{Type: "ephemeral"}},
							{Type: "text", Text: lo.ToPtr("b"), CacheControl: &CacheControl{Type: "ephemeral"}},
						},
					},
				},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 3, countCacheControls(req))
	})
}

func TestEnsureCacheControl_ExistingBreakpoints_KeepAsIs(t *testing.T) {
	req := &MessageRequest{
		Messages: []MessageParam{
			{Role: "user", Content: MessageContent{Content: lo.ToPtr("first")}},
			{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
			{Role: "user", Content: MessageContent{Content: lo.ToPtr("second")}},
		},
	}

	ensureCacheControl(req)
	assert.Equal(t, 1, countCacheControls(req))
}

func TestEnsureCacheControl_ExceedsLimit_TrimToLastFour(t *testing.T) {
	t.Run("5 个断点会自动裁剪为最近 4 个", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "tool_a", CacheControl: &CacheControl{Type: "ephemeral"}},
				{Name: "tool_b", CacheControl: &CacheControl{Type: "ephemeral"}},
			},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "sys1", CacheControl: &CacheControl{Type: "ephemeral"}},
					{Type: "text", Text: "sys2"},
				},
			},
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("turn1-a"), CacheControl: &CacheControl{Type: "ephemeral"}},
							{Type: "text", Text: lo.ToPtr("turn1-b")},
						},
					},
				},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("turn2"), CacheControl: &CacheControl{Type: "ephemeral"}},
						},
					},
				},
			},
		}

		ensureCacheControl(req)

		// strict 模式按新策略重建：结构锚点 + 会话末尾消息锚点。
		assert.Nil(t, req.Tools[0].CacheControl)
		assert.NotNil(t, req.Tools[1].CacheControl)

		assert.Nil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.NotNil(t, req.System.MultiplePrompts[1].CacheControl)

		assert.Nil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
		assert.Nil(t, req.Messages[0].Content.MultipleContent[1].CacheControl)
		assert.NotNil(t, req.Messages[2].Content.MultipleContent[0].CacheControl)

		assert.Equal(t, 3, countCacheControls(req))
	})

	t.Run("6 个断点也会自动裁剪为最近 4 个", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "tool_a", CacheControl: &CacheControl{Type: "ephemeral"}},
				{Name: "tool_b", CacheControl: &CacheControl{Type: "ephemeral"}},
			},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "sys1", CacheControl: &CacheControl{Type: "ephemeral"}},
					{Type: "text", Text: "sys2", CacheControl: &CacheControl{Type: "ephemeral"}},
				},
			},
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("turn1"), CacheControl: &CacheControl{Type: "ephemeral"}},
							{Type: "text", Text: lo.ToPtr("turn1-b"), CacheControl: &CacheControl{Type: "ephemeral"}},
						},
					},
				},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("turn2")}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 3, countCacheControls(req))
		assert.Nil(t, req.Tools[0].CacheControl)
		assert.NotNil(t, req.Tools[1].CacheControl)
		assert.Nil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.NotNil(t, req.System.MultiplePrompts[1].CacheControl)
		assert.Nil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
		assert.Nil(t, req.Messages[0].Content.MultipleContent[1].CacheControl)
		require.Len(t, req.Messages[2].Content.MultipleContent, 1)
		assert.NotNil(t, req.Messages[2].Content.MultipleContent[0].CacheControl)
	})
}

func TestEnsureCacheControl_AdaptiveBreakpoints_ByBlockDensity(t *testing.T) {
	t.Run(">20 blocks 时会按密度补充断点（上限 4）", func(t *testing.T) {
		blocks := make([]MessageContentBlock, 0, 65)

		for range 65 {
			text := "chunk"
			blocks = append(blocks, MessageContentBlock{Type: "text", Text: &text})
		}

		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{MultipleContent: blocks}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("latest")}},
			},
		}

		ensureCacheControl(req)

		assert.Equal(t, 2, countCacheControls(req))

		marked := 0

		for i := range req.Messages[0].Content.MultipleContent {
			if req.Messages[0].Content.MultipleContent[i].CacheControl != nil {
				marked++
			}
		}

		assert.GreaterOrEqual(t, marked, 1)
	})

	t.Run("单个 user turn 的长内容会补齐到 2 个消息断点", func(t *testing.T) {
		blocks := make([]MessageContentBlock, 0, 65)

		for range 65 {
			text := "chunk"
			blocks = append(blocks, MessageContentBlock{Type: "text", Text: &text})
		}

		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{MultipleContent: blocks}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 2, countCacheControls(req))
	})

	t.Run(">40 blocks 时第二个消息锚点应落在末尾前20窗口边界", func(t *testing.T) {
		blocks := make([]MessageContentBlock, 0, 50)

		for range 50 {
			text := "chunk"
			blocks = append(blocks, MessageContentBlock{Type: "text", Text: &text})
		}

		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{MultipleContent: blocks}},
			},
		}

		ensureCacheControl(req)

		assert.Equal(t, 2, countCacheControls(req))

		marked := make([]int, 0, 2)

		for i := range req.Messages[0].Content.MultipleContent {
			if req.Messages[0].Content.MultipleContent[i].CacheControl != nil {
				marked = append(marked, i)
			}
		}

		require.Equal(t, []int{29, 49}, marked)
	})
}

// --- System.Prompt 字符串形式归一化 ---

func TestEnsureCacheControl_SystemPromptStringForm(t *testing.T) {
	t.Run("System.Prompt 字符串形式也能被注入 cache_control", func(t *testing.T) {
		req := &MessageRequest{
			System: &SystemPrompt{
				Prompt: lo.ToPtr("You are a helpful assistant."),
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
			},
		}

		ensureCacheControl(req)

		// System.Prompt 应被归一化为 MultiplePrompts，并注入 cache_control
		assert.Nil(t, req.System.Prompt)
		require.Len(t, req.System.MultiplePrompts, 1)
		assert.Equal(t, "You are a helpful assistant.", req.System.MultiplePrompts[0].Text)
		assert.NotNil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.Equal(t, "ephemeral", req.System.MultiplePrompts[0].CacheControl.Type)
	})

	t.Run("已有断点时也能处理 System.Prompt 字符串形式", func(t *testing.T) {
		req := &MessageRequest{
			System: &SystemPrompt{
				Prompt: lo.ToPtr("System prompt text"),
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
			},
		}

		ensureCacheControl(req)

		assert.Nil(t, req.System.Prompt)
		require.Len(t, req.System.MultiplePrompts, 1)
		assert.NotNil(t, req.System.MultiplePrompts[0].CacheControl)
	})
}

// --- 边界用例 ---

func TestEnsureCacheControl_EdgeCases(t *testing.T) {
	t.Run("user turn 内容为空字符串时仍可在后续可缓存块注入 1 个消息断点", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("")}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("second")}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 1, countCacheControls(req))
	})

	t.Run("user turn Content 为 nil 且 MultipleContent 为空时仍可在后续可缓存块注入", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("second")}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 1, countCacheControls(req))
	})
}

// --- 结构锚点补齐 ---

func TestEnsureCacheControl_StructuralAnchors(t *testing.T) {
	t.Run("客户端仅在 messages 设了断点时 tools 和 system 应被补齐", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "bash"},
				{Name: "edit"},
			},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "You are helpful."},
				},
			},
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("context"), CacheControl: &CacheControl{Type: "ephemeral"}},
						},
					},
				},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("ok")}},
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("question"), CacheControl: &CacheControl{Type: "ephemeral"}},
						},
					},
				},
			},
		}

		ensureCacheControl(req)

		// tools 最后一个应被补齐
		assert.NotNil(t, req.Tools[1].CacheControl)
		assert.Equal(t, "ephemeral", req.Tools[1].CacheControl.Type)

		// system 最后一个应被补齐
		assert.NotNil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.Equal(t, "ephemeral", req.System.MultiplePrompts[0].CacheControl.Type)

		// 总数应 <= 4
		assert.LessOrEqual(t, countCacheControls(req), 4)
	})

	t.Run("客户端已在 tools 设断点时不会重复注入", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "bash"},
				{Name: "edit", CacheControl: &CacheControl{Type: "ephemeral"}},
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, "ephemeral", req.Tools[1].CacheControl.Type)
		assert.Nil(t, req.Tools[0].CacheControl)
	})

	t.Run("满额消息断点场景仍优先保留 tools/system 锚点", func(t *testing.T) {
		req := &MessageRequest{
			Tools:  []Tool{{Name: "bash"}, {Name: "edit"}},
			System: &SystemPrompt{Prompt: lo.ToPtr("You are helpful")},
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{MultipleContent: []MessageContentBlock{
						{Type: "text", Text: lo.ToPtr("m1"), CacheControl: &CacheControl{Type: "ephemeral"}},
						{Type: "text", Text: lo.ToPtr("m2"), CacheControl: &CacheControl{Type: "ephemeral"}},
						{Type: "text", Text: lo.ToPtr("m3"), CacheControl: &CacheControl{Type: "ephemeral"}},
						{Type: "text", Text: lo.ToPtr("m4"), CacheControl: &CacheControl{Type: "ephemeral"}},
					}},
				},
			},
		}

		ensureCacheControl(req)

		assert.NotNil(t, req.Tools[1].CacheControl)
		require.NotNil(t, req.System)
		require.Len(t, req.System.MultiplePrompts, 1)
		assert.NotNil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.LessOrEqual(t, countCacheControls(req), 4)
	})
}

// --- thinking 块安全处理 ---

func TestEnsureCacheControl_ThinkingBlocks(t *testing.T) {
	t.Run("Text 为 nil 的 text 块不可缓存", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{MultipleContent: []MessageContentBlock{
						{Type: "text"},
						{Type: "text", Text: lo.ToPtr("valid")},
					}},
				},
			},
		}

		ensureCacheControl(req)
		assert.Nil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
		assert.NotNil(t, req.Messages[0].Content.MultipleContent[1].CacheControl)
		assert.Equal(t, 1, countCacheControls(req))
	})

	t.Run("空 text 块上的 cache_control 会被清理", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{MultipleContent: []MessageContentBlock{
						{Type: "text", Text: lo.ToPtr(""), CacheControl: &CacheControl{Type: "ephemeral"}},
						{Type: "text", Text: lo.ToPtr("valid")},
					}},
				},
			},
		}

		ensureCacheControl(req)
		assert.Nil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
	})

	t.Run("thinking 块不会被注入 cache_control", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("question")},
						},
					},
				},
				{
					Role: "assistant",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "thinking", Thinking: lo.ToPtr("let me think...")},
							{Type: "text", Text: lo.ToPtr("answer")},
						},
					},
				},
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("follow up")},
						},
					},
				},
			},
		}

		ensureCacheControl(req)

		// thinking 块绝不能有 cache_control
		thinkingBlock := req.Messages[1].Content.MultipleContent[0]
		assert.Equal(t, "thinking", thinkingBlock.Type)
		assert.Nil(t, thinkingBlock.CacheControl)
	})

	t.Run("已有 cache_control 的 thinking 块会被清理", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "assistant",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "thinking", Thinking: lo.ToPtr("thought"), CacheControl: &CacheControl{Type: "ephemeral"}},
							{Type: "text", Text: lo.ToPtr("reply")},
						},
					},
				},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("msg1")}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("msg2")}},
			},
		}

		ensureCacheControl(req)

		// thinking 块上的 cache_control 应被清理
		assert.Nil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
	})

	t.Run("大量 thinking 块不影响 adaptive 密度计算", func(t *testing.T) {
		blocks := make([]MessageContentBlock, 0, 70)
		// 40 个 thinking 块 + 30 个 text 块
		for range 40 {
			blocks = append(blocks, MessageContentBlock{Type: "thinking", Thinking: lo.ToPtr("thought")})
		}

		for range 30 {
			text := "chunk"
			blocks = append(blocks, MessageContentBlock{Type: "text", Text: &text})
		}

		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{MultipleContent: blocks}},
			},
		}

		ensureCacheControl(req)

		// thinking 块不计入可缓存密度，30 个 text 块 → desired = 2
		// 所有 cache_control 应只在 text 块上
		for _, block := range req.Messages[0].Content.MultipleContent {
			if block.Type == "thinking" {
				assert.Nil(t, block.CacheControl)
			}
		}
	})

	t.Run("redacted_thinking 块不会被注入 cache_control", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("question")},
						},
					},
				},
				{
					Role: "assistant",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "thinking", Thinking: lo.ToPtr("let me think"), Signature: lo.ToPtr("sig")},
							{Type: "redacted_thinking", Data: "encrypted-data"},
							{Type: "text", Text: lo.ToPtr("answer")},
						},
					},
				},
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("follow up")},
						},
					},
				},
			},
		}

		ensureCacheControl(req)

		// thinking 和 redacted_thinking 块都不能有 cache_control
		assert.Nil(t, req.Messages[1].Content.MultipleContent[0].CacheControl)
		assert.Equal(t, "thinking", req.Messages[1].Content.MultipleContent[0].Type)
		assert.Nil(t, req.Messages[1].Content.MultipleContent[1].CacheControl)
		assert.Equal(t, "redacted_thinking", req.Messages[1].Content.MultipleContent[1].Type)
	})

	t.Run("已有 cache_control 的 redacted_thinking 块会被清理", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "assistant",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "redacted_thinking", Data: "encrypted", CacheControl: &CacheControl{Type: "ephemeral"}},
							{Type: "text", Text: lo.ToPtr("reply")},
						},
					},
				},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("msg")}},
			},
		}

		ensureCacheControl(req)

		// redacted_thinking 块上的 cache_control 应被清理
		assert.Nil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
		assert.Equal(t, "redacted_thinking", req.Messages[0].Content.MultipleContent[0].Type)
	})

	t.Run("大量 redacted_thinking 块不影响 adaptive 密度计算", func(t *testing.T) {
		blocks := make([]MessageContentBlock, 0, 70)
		// 40 个 redacted_thinking 块 + 30 个 text 块
		for range 40 {
			blocks = append(blocks, MessageContentBlock{Type: "redacted_thinking", Data: "encrypted"})
		}

		for range 30 {
			text := "chunk"
			blocks = append(blocks, MessageContentBlock{Type: "text", Text: &text})
		}

		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{MultipleContent: blocks}},
			},
		}

		ensureCacheControl(req)

		// redacted_thinking 块不计入可缓存密度，30 个 text 块 → desired = 2
		for _, block := range req.Messages[0].Content.MultipleContent {
			if block.Type == "redacted_thinking" {
				assert.Nil(t, block.CacheControl)
			}
		}

		assert.Equal(t, 2, countCacheControls(req))
	})

	t.Run("thinking 断点不会占用预算并阻断结构锚点补齐", func(t *testing.T) {
		req := &MessageRequest{
			Tools:  []Tool{{Name: "bash"}, {Name: "edit"}},
			System: &SystemPrompt{Prompt: lo.ToPtr("You are helpful")},
			Messages: []MessageParam{
				{
					Role: "assistant",
					Content: MessageContent{MultipleContent: []MessageContentBlock{
						{Type: "thinking", Thinking: lo.ToPtr("t1"), CacheControl: &CacheControl{Type: "ephemeral"}},
						{Type: "thinking", Thinking: lo.ToPtr("t2"), CacheControl: &CacheControl{Type: "ephemeral"}},
						{Type: "thinking", Thinking: lo.ToPtr("t3"), CacheControl: &CacheControl{Type: "ephemeral"}},
						{Type: "thinking", Thinking: lo.ToPtr("t4"), CacheControl: &CacheControl{Type: "ephemeral"}},
					}},
				},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("u1")}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("a1")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("u2")}},
			},
		}

		ensureCacheControl(req)

		for _, block := range req.Messages[0].Content.MultipleContent {
			if block.Type == "thinking" {
				assert.Nil(t, block.CacheControl)
			}
		}

		assert.NotNil(t, req.Tools[1].CacheControl)
		require.NotNil(t, req.System)
		require.Len(t, req.System.MultiplePrompts, 1)
		assert.NotNil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.LessOrEqual(t, countCacheControls(req), 4)
	})
}

// --- 回归测试 ---

func TestEnsureCacheControl_Regression(t *testing.T) {
	t.Run("尾部消息全为 non-cacheable 时向前扫描注入 final 锚点", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("question")},
						},
					},
				},
				{
					Role: "assistant",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "thinking", Thinking: lo.ToPtr("let me think")},
						},
					},
				},
			},
		}

		ensureCacheControl(req)

		// thinking 块不应有 cache_control
		assert.Nil(t, req.Messages[1].Content.MultipleContent[0].CacheControl)
		// final 锚点应落在 msg[0] 的 "question" 上
		assert.NotNil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
		assert.Equal(t, "ephemeral", req.Messages[0].Content.MultipleContent[0].CacheControl.Type)
		assert.Equal(t, 1, countCacheControls(req))
	})

	t.Run("断点总数不变量：任何场景下不超过 maxCacheControlBreakpoints", func(t *testing.T) {
		// 构造压力场景：大量工具、多条系统提示、大量消息块
		blocks := make([]MessageContentBlock, 0, 100)

		for range 100 {
			text := "chunk"
			blocks = append(blocks, MessageContentBlock{Type: "text", Text: &text})
		}

		req := &MessageRequest{
			Tools: []Tool{
				{Name: "t1"},
				{Name: "t2"},
				{Name: "t3"},
			},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "s1"},
					{Type: "text", Text: "s2"},
				},
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{MultipleContent: blocks}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("response")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("follow up")}},
			},
		}

		ensureCacheControl(req)
		assert.LessOrEqual(t, countCacheControls(req), maxCacheControlBreakpoints)
	})

	t.Run("多次调用 ensureCacheControl 结果幂等", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{{Name: "bash"}, {Name: "edit"}},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "You are helpful."},
				},
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("hi")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("question")}},
			},
		}

		ensureCacheControl(req)
		firstCount := countCacheControls(req)

		ensureCacheControl(req)
		secondCount := countCacheControls(req)

		assert.Equal(t, firstCount, secondCount)
		assert.LessOrEqual(t, secondCount, maxCacheControlBreakpoints)
	})
}

// --- OpenCode 插件场景复现 ---

func TestEnsureCacheControl_OpenCodePluginScenario(t *testing.T) {
	t.Run("客户端已设置 4 个断点（模拟 opencode-dynamic-context-pruning 插件）不会超限", func(t *testing.T) {
		// 模拟插件设置的 4 个断点：tools 末尾 + system 末尾 + 2 个 message 内容块
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "bash"},
				{Name: "edit", CacheControl: &CacheControl{Type: "ephemeral"}}, // 断点 1
			},
			System: &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{
					{Type: "text", Text: "You are Claude Code."},
					{Type: "text", Text: "System instructions.", CacheControl: &CacheControl{Type: "ephemeral"}}, // 断点 2
				},
			},
			Messages: []MessageParam{
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "text", Text: lo.ToPtr("context data"), CacheControl: &CacheControl{Type: "ephemeral"}}, // 断点 3
						},
					},
				},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("ok")}},
				{
					Role: "user",
					Content: MessageContent{
						MultipleContent: []MessageContentBlock{
							{Type: "tool_result", ToolUseID: lo.ToPtr("id1"), CacheControl: &CacheControl{Type: "ephemeral"}}, // 断点 4
						},
					},
				},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 3, countCacheControls(req))
	})
}
