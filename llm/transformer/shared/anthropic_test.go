package shared

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAnthropicRedactedContent(t *testing.T) {
	tests := []struct {
		name     string
		content  *string
		expected bool
	}{
		{
			name:     "nil content",
			content:  nil,
			expected: false,
		},
		{
			name:     "normal text",
			content:  stringPtr("this is normal text"),
			expected: true,
		},
		{
			name:     "gemini signature",
			content:  stringPtr(GeminiThoughtSignaturePrefix + "signature"),
			expected: false,
		},
		{
			name:     "openai encrypted",
			content:  stringPtr(OpenAIEncryptedContentPrefix + "encrypted"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAnthropicRedactedContent(tt.content)
			require.Equal(t, tt.expected, result)
		})
	}
}
