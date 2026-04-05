package shared

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

const testOpenAIFootprint = "a1b2c3"

func TestDecodeOpenAIEncryptedContent(t *testing.T) {
	tests := []struct {
		name      string
		content   *string
		footprint string
		expected  *string
	}{
		{
			name:      "nil content",
			content:   nil,
			footprint: "",
			expected:  nil,
		},
		{
			name:      "empty string",
			content:   new(""),
			footprint: "",
			expected:  nil,
		},
		{
			name:      "valid encrypted content with footprint",
			content:   EncodeOpenAIEncryptedContent(new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"), testOpenAIFootprint),
			footprint: testOpenAIFootprint,
			expected:  new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
		},
		{
			name:      "invalid prefix",
			content:   new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
			footprint: "",
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeOpenAIEncryptedContent(tt.content, tt.footprint)
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
			name:     "empty content remains raw",
			content:  new(""),
			expected: new(""),
		},
		{
			name:     "valid encrypted content remains raw without footprint",
			content:  new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
			expected: new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeOpenAIEncryptedContent(tt.content, "")
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
	original := new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp")

	// Encode
	encoded := EncodeOpenAIEncryptedContent(original, "")
	require.NotNil(t, encoded)

	// Decode
	decoded := DecodeOpenAIEncryptedContent(encoded, "")
	require.Nil(t, decoded)
}

func TestEncodeDecodeOpenAIEncryptedContent(t *testing.T) {
	original := new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0P1P10QyeNeFLD2yVYYnLJY8-QnwOjWp")
	encoded := EncodeOpenAIEncryptedContent(original, testOpenAIFootprint)
	require.NotNil(t, encoded)
	require.NotNil(t, DecodeOpenAIEncryptedContent(encoded, testOpenAIFootprint))
	require.NotNil(t, DecodeOpenAIEncryptedContent(encoded, testOpenAIFootprint))

	decoded := DecodeOpenAIEncryptedContent(encoded, testOpenAIFootprint)
	require.NotNil(t, decoded)
	require.Equal(t, *original, *decoded)
	require.Nil(t, DecodeOpenAIEncryptedContent(encoded, "ffffff"))
}

func TestOpenAIEncryptedContentWholeValueCanDecodeAsBase64(t *testing.T) {
	content := new("YWJjZA==")

	encoded := EncodeOpenAIEncryptedContent(content, testOpenAIFootprint)
	require.NotNil(t, encoded)
	_, err := base64.StdEncoding.DecodeString(*encoded)
	require.NoError(t, err)
}
