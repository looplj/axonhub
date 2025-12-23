package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// setupTagsTest 创建测试环境和测试用的 channels.
func setupTagsTest(t *testing.T) (context.Context, *ent.Client, []*biz.Channel) {
	t.Helper()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx = ent.NewContext(ctx, client)

	// 创建测试渠道
	// Channel 1: 有 tag1 和 tag2
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel with tag1 and tag2").
		SetBaseURL("https://api.example.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{"tag1", "tag2"}).
		Save(ctx)
	require.NoError(t, err)

	// Channel 2: 只有 tag2
	ch2, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel with tag2 only").
		SetBaseURL("https://api.example.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-2"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{"tag2"}).
		Save(ctx)
	require.NoError(t, err)

	// Channel 3: 只有 tag3
	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel with tag3 only").
		SetBaseURL("https://api.example.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-3"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{"tag3"}).
		Save(ctx)
	require.NoError(t, err)

	// Channel 4: 没有任何 tags (空数组)
	ch4, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel without tags").
		SetBaseURL("https://api.example.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-4"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{}).
		Save(ctx)
	require.NoError(t, err)

	// Channel 5: tags 未设置 (nil)
	ch5, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel with nil tags").
		SetBaseURL("https://api.example.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-5"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels := []*biz.Channel{
		{Channel: ch1},
		{Channel: ch2},
		{Channel: ch3},
		{Channel: ch4},
		{Channel: ch5},
	}

	return ctx, client, channels
}

// TestTagsFilterSelector_EmptyAllowedTags 测试当 allowedTags 为空时返回所有渠道.
func TestTagsFilterSelector_EmptyAllowedTags(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	// 创建返回所有 channels 的 mock selector
	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}

	// 创建 TagsFilterSelector with empty allowedTags
	selector := WithTagsFilterSelector(mockSelector, []string{})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// 应该返回所有 5 个渠道
	assert.Len(t, result, 5)
}

// TestTagsFilterSelector_NilAllowedTags 测试当 allowedTags 为 nil 时返回所有渠道.
func TestTagsFilterSelector_NilAllowedTags(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}
	selector := WithTagsFilterSelector(mockSelector, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// 应该返回所有 5 个渠道
	assert.Len(t, result, 5)
}

// TestTagsFilterSelector_SingleMatchingTag 测试单个匹配标签.
func TestTagsFilterSelector_SingleMatchingTag(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}
	selector := WithTagsFilterSelector(mockSelector, []string{"tag1"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// 只有 Channel 1 有 tag1
	assert.Len(t, result, 1)
	assert.Equal(t, "Channel with tag1 and tag2", result[0].Channel.Name)
}

// TestTagsFilterSelector_MultipleMatchingTags 测试多个匹配标签 (OR 逻辑).
func TestTagsFilterSelector_MultipleMatchingTags(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}
	// 允许 tag1 或 tag2
	selector := WithTagsFilterSelector(mockSelector, []string{"tag1", "tag2"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Channel 1 (tag1, tag2) 和 Channel 2 (tag2) 应该被选中
	assert.Len(t, result, 2)

	names := []string{result[0].Channel.Name, result[1].Channel.Name}
	assert.Contains(t, names, "Channel with tag1 and tag2")
	assert.Contains(t, names, "Channel with tag2 only")
}

// TestTagsFilterSelector_NoMatchingTags 测试没有匹配的标签.
func TestTagsFilterSelector_NoMatchingTags(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}
	// 使用不存在的标签
	selector := WithTagsFilterSelector(mockSelector, []string{"nonexistent-tag"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// 没有任何渠道匹配
	assert.Len(t, result, 0)
}

// TestTagsFilterSelector_ChannelsWithoutTags 测试没有标签的渠道不会被匹配.
func TestTagsFilterSelector_ChannelsWithoutTags(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	// 只保留没有标签的渠道
	noTagChannels := []*biz.Channel{
		channels[3], // Channel 4: 空数组
		channels[4], // Channel 5: nil
	}

	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(noTagChannels, req.Model), nil
		},
	}
	selector := WithTagsFilterSelector(mockSelector, []string{"tag1", "tag2", "tag3"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// 没有标签的渠道不应该匹配任何 tag filter
	assert.Len(t, result, 0)
}

// TestTagsFilterSelector_ORLogic 明确测试 OR 逻辑.
func TestTagsFilterSelector_ORLogic(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}
	// 允许 tag1 或 tag3
	selector := WithTagsFilterSelector(mockSelector, []string{"tag1", "tag3"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Channel 1 (有 tag1) 和 Channel 3 (有 tag3) 应该被选中
	// Channel 2 (只有 tag2) 不应该被选中
	assert.Len(t, result, 2)

	names := []string{result[0].Channel.Name, result[1].Channel.Name}
	assert.Contains(t, names, "Channel with tag1 and tag2")
	assert.Contains(t, names, "Channel with tag3 only")
}

// TestTagsFilterSelector_WithSelectedChannelsSelector 测试与 SelectedChannelsSelector 的集成 (交集逻辑).
func TestTagsFilterSelector_WithSelectedChannelsSelector(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	// 先创建一个 mock selector 返回所有渠道
	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}

	// 然后用 SelectedChannelsSelector 过滤，只允许 Channel 1 和 Channel 2
	allowedIDs := []int{channels[0].ID, channels[1].ID}
	channelIDSelector := WithSelectedChannelsSelector(mockSelector, allowedIDs)

	// 最后用 TagsFilterSelector 过滤，只允许 tag2
	// Channel 1 有 tag2, Channel 2 也有 tag2
	// 但 SelectedChannelsSelector 已经只允许这两个，所以应该都被选中
	tagsSelector := WithTagsFilterSelector(channelIDSelector, []string{"tag2"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := tagsSelector.Select(ctx, req)
	require.NoError(t, err)

	// 两个 selector 的交集：Channel 1 和 Channel 2 都有 tag2 且都在 allowedIDs 中
	assert.Len(t, result, 2)
}

// TestTagsFilterSelector_WithSelectedChannelsSelector_NoIntersection 测试 tags 和 IDs 没有交集.
func TestTagsFilterSelector_WithSelectedChannelsSelector_NoIntersection(t *testing.T) {
	ctx, _, channels := setupTagsTest(t)

	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}

	// 只允许 Channel 1 (有 tag1, tag2)
	allowedIDs := []int{channels[0].ID}
	channelIDSelector := WithSelectedChannelsSelector(mockSelector, allowedIDs)

	// 但 tags filter 只允许 tag3 (Channel 1 没有 tag3)
	tagsSelector := WithTagsFilterSelector(channelIDSelector, []string{"tag3"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := tagsSelector.Select(ctx, req)
	require.NoError(t, err)

	// 交集为空
	assert.Len(t, result, 0)
}

// TestTagsFilterSelector_ErrorPropagation 测试错误传播.
func TestTagsFilterSelector_ErrorPropagation(t *testing.T) {
	ctx := context.Background()

	// 创建一个会返回错误的 mock selector
	expectedErr := assert.AnError
	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return nil, expectedErr
		},
	}

	selector := WithTagsFilterSelector(mockSelector, []string{"tag1"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)

	// 错误应该被传播
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
}

// TestTagsFilterSelector_CaseSensitive 测试标签是否大小写敏感.
func TestTagsFilterSelector_CaseSensitive(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// 创建一个带有大写标签的渠道
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel with TAG1").
		SetBaseURL("https://api.example.com/v1").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{"TAG1"}).
		Save(ctx)
	require.NoError(t, err)

	channels := []*biz.Channel{
		{Channel: ch},
	}

	mockSelector := &mockChannelSelector{
		selectFunc: func(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
			return channelsToCandidates(channels, req.Model), nil
		},
	}

	// 用小写的 tag1 来过滤
	selector := WithTagsFilterSelector(mockSelector, []string{"tag1"})

	req := &llm.Request{
		Model: "gpt-4",
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// 标签应该是大小写敏感的，所以不会匹配
	assert.Len(t, result, 0)
}

// channelsToCandidates 辅助函数，将 []*biz.Channel 转换为 []*ChannelModelCandidate.
func channelsToCandidates(channels []*biz.Channel, model string) []*ChannelModelCandidate {
	candidates := make([]*ChannelModelCandidate, 0, len(channels))
	for _, ch := range channels {
		entries := ch.GetModelEntries()

		entry, ok := entries[model]
		if !ok {
			continue
		}

		candidates = append(candidates, &ChannelModelCandidate{
			Channel:      ch,
			RequestModel: entry.RequestModel,
			ActualModel:  entry.ActualModel,
			Priority:     0,
		})
	}

	return candidates
}
