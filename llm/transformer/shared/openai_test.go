package shared

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsOpenAIEncryptedContent(t *testing.T) {
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
			name:     "empty string",
			content:  stringPtr(""),
			expected: false,
		},
		{
			name:     "valid encrypted content",
			content:  stringPtr(OpenAIEncryptedContentPrefix + "gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
			expected: true,
		},
		{
			name:     "invalid prefix",
			content:  stringPtr("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
			expected: false,
		},
		{
			name:     "only prefix",
			content:  stringPtr(OpenAIEncryptedContentPrefix),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOpenAIEncryptedContent(tt.content)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDecodeOpenAIEncryptedContent(t *testing.T) {
	tests := []struct {
		name     string
		content  *string
		expected *string
	}{
		{
			name:     "nil content",
			content:  nil,
			expected: nil,
		},
		{
			name:     "empty string",
			content:  stringPtr(""),
			expected: nil,
		},
		{
			name:     "valid encrypted content",
			content:  stringPtr(OpenAIEncryptedContentPrefix + "gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
			expected: stringPtr("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
		},
		{
			name:     "invalid prefix",
			content:  stringPtr("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
			expected: nil,
		},
		{
			name:     "only prefix returns empty string",
			content:  stringPtr(OpenAIEncryptedContentPrefix),
			expected: stringPtr(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeOpenAIEncryptedContent(tt.content)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestEncodeOpenAIEncryptedContent(t *testing.T) {
	tests := []struct {
		name     string
		content  *string
		expected *string
	}{
		{
			name:     "nil content",
			content:  nil,
			expected: nil,
		},
		{
			name:     "only prefix",
			content:  stringPtr(""),
			expected: stringPtr(OpenAIEncryptedContentPrefix),
		},
		{
			name:     "valid encrypted content",
			content:  stringPtr("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
			expected: stringPtr(OpenAIEncryptedContentPrefix + "gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeOpenAIEncryptedContent(tt.content)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := stringPtr("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp")

	// Encode
	encoded := EncodeOpenAIEncryptedContent(original)
	require.NotNil(t, encoded)
	require.True(t, IsOpenAIEncryptedContent(encoded))

	// Decode
	decoded := DecodeOpenAIEncryptedContent(encoded)
	require.NotNil(t, decoded)
	require.Equal(t, *original, *decoded)
}

func stringPtr(s string) *string {
	return &s
}
