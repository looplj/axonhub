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
			signature: stringPtr(""),
			expected:  false,
		},
		{
			name:      "valid signature",
			signature: stringPtr(GeminiThoughtSignaturePrefix + "some-signature"),
			expected:  true,
		},
		{
			name:      "valid legacy signature",
			signature: stringPtr(legacyGeminiThoughtSignaturePrefix + "some-signature"),
			expected:  true,
		},
		{
			name:      "invalid prefix",
			signature: stringPtr("some-signature"),
			expected:  false,
		},
		{
			name:      "only prefix",
			signature: stringPtr(GeminiThoughtSignaturePrefix),
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
			signature: stringPtr(""),
			expected:  nil,
		},
		{
			name:      "valid signature",
			signature: stringPtr(GeminiThoughtSignaturePrefix + "some-signature"),
			expected:  stringPtr("some-signature"),
		},
		{
			name:      "valid legacy signature",
			signature: stringPtr(legacyGeminiThoughtSignaturePrefix + "some-signature"),
			expected:  stringPtr("some-signature"),
		},
		{
			name:      "invalid prefix",
			signature: stringPtr("some-signature"),
			expected:  nil,
		},
		{
			name:      "only prefix returns empty string",
			signature: stringPtr(GeminiThoughtSignaturePrefix),
			expected:  stringPtr(""),
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
			signature: stringPtr(""),
			expected:  stringPtr(GeminiThoughtSignaturePrefix),
		},
		{
			name:      "valid signature",
			signature: stringPtr("some-signature"),
			expected:  stringPtr(GeminiThoughtSignaturePrefix + "some-signature"),
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

func TestNormalizeGeminiThoughtSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		expected  *string
	}{
		{
			name:      "empty signature",
			signature: "",
			expected:  nil,
		},
		{
			name:      "already prefixed signature",
			signature: GeminiThoughtSignaturePrefix + "normalized",
			expected:  stringPtr(GeminiThoughtSignaturePrefix + "normalized"),
		},
		{
			name:      "already legacy prefixed signature should normalize to new prefix",
			signature: legacyGeminiThoughtSignaturePrefix + "normalized",
			expected:  stringPtr(GeminiThoughtSignaturePrefix + "normalized"),
		},
		{
			name:      "plain signature",
			signature: "normalized",
			expected:  stringPtr(GeminiThoughtSignaturePrefix + "normalized"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeGeminiThoughtSignature(tt.signature)
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
			name:      "legacy prefixed signature",
			signature: legacyGeminiThoughtSignaturePrefix + "stripped",
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
	original := stringPtr("some-random-signature-data")

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
	signature := stringPtr("YWJjZA==")

	encoded := EncodeGeminiThoughtSignature(signature)
	require.NotNil(t, encoded)
	_, err := base64.StdEncoding.DecodeString(*encoded)
	require.NoError(t, err)

	legacy := legacyGeminiThoughtSignaturePrefix + *signature
	_, err = base64.StdEncoding.DecodeString(legacy)
	require.Error(t, err)
}
