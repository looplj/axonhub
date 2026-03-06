package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateTokens_English(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty", "", 0},
		{"short", "hello", 1},
		{"sentence", "Hello, how are you today?", 5},
		{"longer", "The quick brown fox jumps over the lazy dog.", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			assert.InDelta(t, tt.expected, got, 2, "input: %q", tt.input)
		})
	}
}

func TestEstimateTokens_Chinese(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"short", "你好", 1},
		{"sentence", "今天天气很好", 4},
		{"longer", "人工智能正在改变我们的生活方式", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			assert.InDelta(t, tt.expected, got, 3, "input: %q", tt.input)
		})
	}
}

func TestEstimateTokens_Mixed(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"mixed_cn_en", "Hello 你好 World 世界"},
		{"code_cn", "func main() { // 主函数 }"},
		{"japanese", "こんにちは世界"},
		{"korean", "안녕하세요"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			assert.Greater(t, got, 0, "input: %q should have positive tokens", tt.input)
		})
	}
}

func TestEstimateTokens_CJKMoreTokensThanASCII(t *testing.T) {
	// Same character count, but CJK should estimate more tokens
	ascii := "abcdefghij"       // 10 chars -> ~2.5 tokens
	cjk := "你好世界天气很好今天" // 10 chars -> ~6.7 tokens

	asciiTokens := EstimateTokens(ascii)
	cjkTokens := EstimateTokens(cjk)

	assert.Greater(t, cjkTokens, asciiTokens,
		"CJK text should estimate more tokens than ASCII for same length")
}

func TestEstimateMessagesTokens(t *testing.T) {
	messages := []Message{
		newTextMessage(RoleUser, "Hello, how are you?"),
		newTextMessage(RoleAssistant, "I'm doing well, thank you!"),
	}

	tokens := EstimateMessagesTokens(messages)
	assert.Greater(t, tokens, 0)
	assert.Less(t, tokens, 50)
}

func TestEstimateMessagesTokens_WithToolUse(t *testing.T) {
	messages := []Message{
		{
			Role:    RoleAssistant,
			ToolUse: &ToolUse{Name: "read_file", Input: `{"path": "/test/file.txt"}`},
		},
	}

	tokens := EstimateMessagesTokens(messages)
	assert.Greater(t, tokens, 5)
}

func TestIsCJK(t *testing.T) {
	tests := []struct {
		r        rune
		expected bool
	}{
		{'中', true},
		{'あ', true},
		{'ア', true},
		{'한', true},
		{'a', false},
		{'1', false},
		{' ', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			assert.Equal(t, tt.expected, isCJK(tt.r))
		})
	}
}
