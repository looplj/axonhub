package shared

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
			name:     "empty string - rejected",
			content:  new(""),
			expected: nil,
		},
		{
			name:     "openai-like encrypted content (gAAAA prefix)",
			content:  new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8"),
			expected: new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8"),
		},
		{
			name:     "anthropic-like signature (Eq prefix) - rejected",
			content:  new("EqQBCAEDEgQIAhAEGAAgAigBMOzOAg=="),
			expected: nil,
		},
		{
			name:     "gemini-like protobuf base64 - rejected",
			content:  new("CgNmb28="),
			expected: nil,
		},
		{
			name:     "unknown standard base64 - rejected",
			content:  new("SGVsbG8="),
			expected: nil,
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
			name:     "valid content",
			content:  new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0"),
			expected: new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0"),
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

func TestOpenAIEncodeDecodeRoundTrip(t *testing.T) {
	original := new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0")

	encoded := EncodeOpenAIEncryptedContent(original)
	require.NotNil(t, encoded)
	require.Equal(t, *original, *encoded)

	decoded := DecodeOpenAIEncryptedContent(encoded)
	require.NotNil(t, decoded)
	require.Equal(t, *original, *decoded)
}
