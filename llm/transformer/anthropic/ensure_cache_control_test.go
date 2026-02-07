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

		// 倒数第二个 user turn（索引 0）被转为数组格式并注入
		assert.Len(t, req.Messages[0].Content.MultipleContent, 1)
		assert.NotNil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
		assert.Equal(t, "ephemeral", req.Messages[0].Content.MultipleContent[0].CacheControl.Type)

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

	t.Run("只有 1 个 user turn 时不注入 messages 断点", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{{Name: "t1"}},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("only one")}},
			},
		}

		ensureCacheControl(req)

		// 只有 tools 被注入
		assert.Equal(t, 1, countCacheControls(req))
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

		// 倒数第二个 user turn 的最后一个内容块被注入
		blocks := req.Messages[0].Content.MultipleContent
		assert.Nil(t, blocks[0].CacheControl)
		assert.NotNil(t, blocks[1].CacheControl)
		assert.Equal(t, "ephemeral", blocks[1].CacheControl.Type)
	})
}

func TestEnsureCacheControl_WithinLimit_NoModification(t *testing.T) {
	t.Run("客户端设了 1 个断点不做修改", func(t *testing.T) {
		req := &MessageRequest{
			Tools: []Tool{
				{Name: "t1", CacheControl: &CacheControl{Type: "ephemeral"}},
			},
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("hello")}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 1, countCacheControls(req))
	})

	t.Run("客户端设了 4 个断点不做修改", func(t *testing.T) {
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
		assert.Equal(t, 4, countCacheControls(req))
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

		// 保留从后往前最近的 4 个：应删除最早的 tool_a 断点
		assert.Nil(t, req.Tools[0].CacheControl)
		assert.NotNil(t, req.Tools[1].CacheControl)

		assert.NotNil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.Nil(t, req.System.MultiplePrompts[1].CacheControl)

		assert.NotNil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
		assert.Nil(t, req.Messages[0].Content.MultipleContent[1].CacheControl)
		assert.NotNil(t, req.Messages[2].Content.MultipleContent[0].CacheControl)

		assert.Equal(t, 4, countCacheControls(req))
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
		assert.Equal(t, 4, countCacheControls(req))
		assert.Nil(t, req.Tools[0].CacheControl)
		assert.Nil(t, req.Tools[1].CacheControl)
		assert.NotNil(t, req.System.MultiplePrompts[0].CacheControl)
		assert.NotNil(t, req.System.MultiplePrompts[1].CacheControl)
		assert.NotNil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
		assert.NotNil(t, req.Messages[0].Content.MultipleContent[1].CacheControl)
	})
}

func TestEnsureCacheControl_AdaptiveBreakpoints_ByBlockDensity(t *testing.T) {
	t.Run(">20 blocks 时会按密度补充断点（上限 4）", func(t *testing.T) {
		blocks := make([]MessageContentBlock, 0, 65)
		for i := 0; i < 65; i++ {
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

		assert.Equal(t, 4, countCacheControls(req))

		marked := 0
		for i := range req.Messages[0].Content.MultipleContent {
			if req.Messages[0].Content.MultipleContent[i].CacheControl != nil {
				marked++
			}
		}
		assert.GreaterOrEqual(t, marked, 3)
	})

	t.Run("单个 user turn 的长内容也能补齐到 4 个断点", func(t *testing.T) {
		blocks := make([]MessageContentBlock, 0, 65)
		for i := 0; i < 65; i++ {
			text := "chunk"
			blocks = append(blocks, MessageContentBlock{Type: "text", Text: &text})
		}

		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{MultipleContent: blocks}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 4, countCacheControls(req))
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
	t.Run("user turn 内容为空字符串时不注入 messages 断点", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("")}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("second")}},
			},
		}

		ensureCacheControl(req)
		// 空字符串内容不会被转为数组格式，所以倒数第二个 user turn 不注入
		assert.Equal(t, 0, countCacheControls(req))
	})

	t.Run("user turn Content 为 nil 且 MultipleContent 为空时不注入", func(t *testing.T) {
		req := &MessageRequest{
			Messages: []MessageParam{
				{Role: "user", Content: MessageContent{}},
				{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("resp")}},
				{Role: "user", Content: MessageContent{Content: lo.ToPtr("second")}},
			},
		}

		ensureCacheControl(req)
		assert.Equal(t, 0, countCacheControls(req))
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
		assert.Equal(t, 4, countCacheControls(req))
	})
}
