package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// TestGenMaskedTag 验证标签生成唯一性和可解析性
func TestGenMaskedTag(t *testing.T) {
	tag1 := genMaskedTagWithReplacement("荣昌", "[REDACTED]")
	tag2 := genMaskedTagWithReplacement("开州", "[REDACTED]")
	tag3 := genMaskedTagWithReplacement("荣昌", "[REDACTED]")

	// 不同原文应生成不同标签
	assert.NotEqual(t, tag1, tag2, "different originals should produce different tags")

	// 同一原文也应生成不同标签（UUID 保证唯一性）
	assert.NotEqual(t, tag1, tag3, "same original should still produce unique tags")

	// 每个标签都应能正确解析回原文
	orig1, ok1 := parseMaskedTag(tag1)
	assert.True(t, ok1)
	assert.Equal(t, "荣昌", orig1)

	orig2, ok2 := parseMaskedTag(tag2)
	assert.True(t, ok2)
	assert.Equal(t, "开州", orig2)

	orig3, ok3 := parseMaskedTag(tag3)
	assert.True(t, ok3)
	assert.Equal(t, "荣昌", orig3)
}

// TestGenMaskedTagReplacementVisible 验证标签中包含用户配置的替换文本
func TestGenMaskedTagReplacementVisible(t *testing.T) {
	tag := genMaskedTagWithReplacement("荣昌", "[REDACTED]")
	assert.Contains(t, tag, "[REDACTED]", "tag should contain the user-configured replacement text")

	tag2 := genMaskedTagWithReplacement("荣昌", "***")
	assert.Contains(t, tag2, "***", "tag should contain the custom replacement")
}

// TestRestoreMaskedContent 验证完整文本中的标签还原
func TestRestoreMaskedContent(t *testing.T) {
	tag1 := genMaskedTagWithReplacement("荣昌", "[REDACTED]")
	tag2 := genMaskedTagWithReplacement("开州", "[REDACTED]")

	text := "请分析" + tag1 + "和" + tag2 + "的数据"
	restored := restoreMaskedContent(text)

	assert.Equal(t, "请分析荣昌和开州的数据", restored)
}

// TestRestoreMaskedContentNoTags 验证无标签时返回原文
func TestRestoreMaskedContentNoTags(t *testing.T) {
	text := "请分析荣昌和开州的数据"
	restored := restoreMaskedContent(text)
	assert.Equal(t, text, restored)
}

// TestRestoreMaskedContentJSON 验证 JSON 中的标签还原
func TestRestoreMaskedContentJSON(t *testing.T) {
	tag := genMaskedTagWithReplacement("荣昌", "[REDACTED]")
	jsonBody := `{"choices":[{"message":{"content":"对` + tag + `的分析结果"}}]}`
	restored := restoreMaskedContent(jsonBody)

	assert.Contains(t, restored, "荣昌")
	assert.NotContains(t, restored, axhTagPrefix)
	assert.True(t, strings.HasPrefix(restored, `{"choices":`))
}

// TestContainsMaskedContent 验证检测函数
func TestContainsMaskedContent(t *testing.T) {
	tag := genMaskedTagWithReplacement("test", "[REDACTED]")
	msgWith := "hello " + tag + " world"
	msgWithout := "hello world"

	msgs := []llm.Message{
		{Role: "user", Content: llm.MessageContent{Content: &msgWith}},
	}
	assert.True(t, containsMaskedContent(msgs))

	msgs2 := []llm.Message{
		{Role: "user", Content: llm.MessageContent{Content: &msgWithout}},
	}
	assert.False(t, containsMaskedContent(msgs2))
}

// TestMaskMessageWithUniqueTags 验证多规则不同匹配的唯一标签
func TestMaskMessageWithUniqueTags(t *testing.T) {
	rules := []*ent.PromptProtectionRule{
		{
			Pattern: `(?:荣昌|开州|梁平|武隆)`,
			Settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionMask,
				Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		},
	}

	content := "请分析荣昌和开州的数据"
	messages := []llm.Message{
		{Role: "user", Content: llm.MessageContent{Content: &content}},
	}

	MaskMessageWithUniqueTags(messages, rules)

	// 两个不同的地名应被替换为不同的唯一标签
	maskedContent := *messages[0].Content.Content
	assert.Contains(t, maskedContent, axhTagPrefix)
	assert.NotContains(t, maskedContent, "荣昌")
	assert.NotContains(t, maskedContent, "开州")

	// 统计标签数量：应有 2 个
	tagCount := strings.Count(maskedContent, axhTagPrefix)
	assert.Equal(t, 2, tagCount, "should have 2 unique tags for 2 matches")

	// 验证还原
	restored := restoreMaskedContent(maskedContent)
	assert.Equal(t, "请分析荣昌和开州的数据", restored)
}

// TestMaskMessageWithUniqueTagsMultipleRules 验证多条规则同时应用
func TestMaskMessageWithUniqueTagsMultipleRules(t *testing.T) {
	rules := []*ent.PromptProtectionRule{
		{
			Pattern: `(?:荣昌|开州)`,
			Settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionMask,
				Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		},
		{
			Pattern: `secret-\d+`,
			Settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionMask,
				Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		},
	}

	content := "荣昌的 secret-12345 开州"
	messages := []llm.Message{
		{Role: "user", Content: llm.MessageContent{Content: &content}},
	}

	MaskMessageWithUniqueTags(messages, rules)

	maskedContent := *messages[0].Content.Content
	assert.NotContains(t, maskedContent, "荣昌")
	assert.NotContains(t, maskedContent, "secret-12345")

	// 还原
	restored := restoreMaskedContent(maskedContent)
	assert.Equal(t, "荣昌的 secret-12345 开州", restored)
}

// TestRestorePromptProtectionNonStream 验证非流式响应还原中间件
func TestRestorePromptProtectionNonStream(t *testing.T) {
	state := &PersistenceState{}
	inbound := &PersistentInboundTransformer{state: state}
	middleware := restorePromptProtection(inbound)

	tag := genMaskedTagWithReplacement("荣昌", "[REDACTED]")
	body := `{"choices":[{"message":{"content":"对` + tag + `的分析结果"}}]}`
	response := &httpclient.Response{
		StatusCode: 200,
		Body:       []byte(body),
	}

	result, err := middleware.OnInboundRawResponse(context.Background(), response)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, string(result.Body), "荣昌")
	assert.NotContains(t, string(result.Body), axhTagPrefix)
}

// TestRestorePromptProtectionNoTags 验证无标签时不修改响应
func TestRestorePromptProtectionNoTags(t *testing.T) {
	state := &PersistenceState{}
	inbound := &PersistentInboundTransformer{state: state}
	middleware := restorePromptProtection(inbound)

	originalBody := `{"choices":[{"message":{"content":"hello"}}]}`
	response := &httpclient.Response{
		StatusCode: 200,
		Body:       []byte(originalBody),
	}

	result, err := middleware.OnInboundRawResponse(context.Background(), response)
	require.NoError(t, err)
	assert.Equal(t, originalBody, string(result.Body))
}

// TestRestorePromptProtectionStream 验证流式响应逐块还原
func TestRestorePromptProtectionStream(t *testing.T) {
	state := &PersistenceState{}
	inbound := &PersistentInboundTransformer{state: state}
	middleware := restorePromptProtection(inbound)

	tag := genMaskedTagWithReplacement("荣昌", "[REDACTED]")
	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"choices":[{"delta":{"content":"分析结果 "}}]}`)},
		{Data: []byte(`{"choices":[{"delta":{"content":"` + tag + ` 数据"}}]}`)},
		{Data: []byte(`{"choices":[{"delta":{"content":" end"}}]}`)},
	}
	src := streams.SliceStream(events)

	wrapped, err := middleware.OnInboundRawStream(context.Background(), src)
	require.NoError(t, err)

	var results []string
	for wrapped.Next() {
		event := wrapped.Current()
		if event != nil {
			results = append(results, string(event.Data))
		}
	}
	wrapped.Close()

	require.Len(t, results, 3)

	// 包含标签的 chunk 应被还原
	assert.Contains(t, results[1], "荣昌")
	assert.NotContains(t, results[1], axhTagPrefix)

	// 不含标签的 chunk 应不变
	assert.Contains(t, results[0], "分析结果")
	assert.Contains(t, results[2], " end")
}
