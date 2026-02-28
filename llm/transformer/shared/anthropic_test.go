package shared

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAnthropicSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature *string
		expected  bool
	}{
		{
			name:      "nil signature",
			signature: nil,
			expected:  false,
		},
		{
			name:      "empty string",
			signature: new(""),
			expected:  false,
		},
		{
			name:      "valid anthropic signature",
			signature: new(AnthropicSignaturePrefix + "some-signature"),
			expected:  true,
		},
		{
			name:      "gemini signature",
			signature: new(GeminiThoughtSignaturePrefix + "some-signature"),
			expected:  false,
		},
		{
			name:      "openai encrypted content",
			signature: new(OpenAIEncryptedContentPrefix + "some-content"),
			expected:  false,
		},
		{
			name:      "plain text",
			signature: new("just-a-plain-signature"),
			expected:  false,
		},
		{
			name:      "only prefix",
			signature: new(AnthropicSignaturePrefix),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAnthropicSignature(tt.signature)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDecodeAnthropicSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature *string
		expected  *string
	}{
		{
			name:      "nil signature",
			signature: nil,
			expected:  nil,
		},
		{
			name:      "empty string",
			signature: new(""),
			expected:  nil,
		},
		{
			name:      "valid anthropic signature",
			signature: new(AnthropicSignaturePrefix + "some-signature"),
			expected:  new("some-signature"),
		},
		{
			name:      "plain text",
			signature: new("just-a-plain-signature"),
			expected:  nil,
		},
		{
			name:      "only prefix returns empty string",
			signature: new(AnthropicSignaturePrefix),
			expected:  new(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeAnthropicSignature(tt.signature)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestEncodeAnthropicSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature *string
		expected  *string
	}{
		{
			name:      "nil signature",
			signature: nil,
			expected:  nil,
		},
		{
			name:      "empty string",
			signature: new(""),
			expected:  new(AnthropicSignaturePrefix),
		},
		{
			name:      "valid signature",
			signature: new("some-signature"),
			expected:  new(AnthropicSignaturePrefix + EnsureBase64Encoding("some-signature")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeAnthropicSignature(tt.signature)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestAnthropicEncodeDecodeRoundTrip(t *testing.T) {
	original := new("some-random-anthropic-signature-data")

	// Encode
	encoded := EncodeAnthropicSignature(original)
	require.NotNil(t, encoded)
	require.True(t, IsAnthropicSignature(encoded))

	// Decode
	decoded := DecodeAnthropicSignature(encoded)
	require.NotNil(t, decoded)
	require.Equal(t, EnsureBase64Encoding(*original), *decoded)
}

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
			content:  new("this is normal text"),
			expected: true,
		},
		{
			name:     "gemini signature",
			content:  new(GeminiThoughtSignaturePrefix + "signature"),
			expected: false,
		},
		{
			name:     "openai encrypted",
			content:  new(OpenAIEncryptedContentPrefix + "encrypted"),
			expected: false,
		},
		{
			name:     "anthropic signature",
			content:  new(AnthropicSignaturePrefix + "signature"),
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
