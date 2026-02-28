package shared

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsGeminiThoughtSignature(t *testing.T) {
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
			name:      "valid signature",
			signature: new(GeminiThoughtSignaturePrefix + "some-signature"),
			expected:  true,
		},
		{
			name:      "invalid prefix",
			signature: new("some-signature"),
			expected:  false,
		},
		{
			name:      "only prefix",
			signature: new(GeminiThoughtSignaturePrefix),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGeminiThoughtSignature(tt.signature)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDecodeGeminiThoughtSignature(t *testing.T) {
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
			name:      "valid signature",
			signature: new(GeminiThoughtSignaturePrefix + "some-signature"),
			expected:  new("some-signature"),
		},
		{
			name:      "invalid prefix",
			signature: new("some-signature"),
			expected:  nil,
		},
		{
			name:      "only prefix returns empty string",
			signature: new(GeminiThoughtSignaturePrefix),
			expected:  new(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeGeminiThoughtSignature(tt.signature)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestEncodeGeminiThoughtSignature(t *testing.T) {
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
			name:      "only prefix",
			signature: new(""),
			expected:  new(GeminiThoughtSignaturePrefix),
		},
		{
			name:      "valid signature",
			signature: new("some-signature"),
			expected:  new(GeminiThoughtSignaturePrefix + "some-signature"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeGeminiThoughtSignature(tt.signature)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestStripGeminiThoughtSignaturePrefix(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		expected  string
	}{
		{
			name:      "prefixed signature",
			signature: GeminiThoughtSignaturePrefix + "stripped",
			expected:  "stripped",
		},
		{
			name:      "plain signature",
			signature: "plain",
			expected:  "plain",
		},
		{
			name:      "prefix only",
			signature: GeminiThoughtSignaturePrefix,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripGeminiThoughtSignaturePrefix(tt.signature)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGeminiEncodeDecodeRoundTrip(t *testing.T) {
	original := new("some-random-signature-data")

	// Encode
	encoded := EncodeGeminiThoughtSignature(original)
	require.NotNil(t, encoded)
	require.True(t, IsGeminiThoughtSignature(encoded))

	// Decode
	decoded := DecodeGeminiThoughtSignature(encoded)
	require.NotNil(t, decoded)
	require.Equal(t, *original, *decoded)
}

func TestGeminiThoughtSignatureWholeValueCanDecodeAsBase64(t *testing.T) {
	signature := new("YWJjZA==")

	encoded := EncodeGeminiThoughtSignature(signature)
	require.NotNil(t, encoded)
	_, err := base64.StdEncoding.DecodeString(*encoded)
	require.NoError(t, err)
}
