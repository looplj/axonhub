package shared

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

const testGeminiFootprint = "a1b2c3"

func TestDecodeGeminiThoughtSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature *string
		footprint string
		expected  *string
	}{
		{
			name:      "nil signature",
			signature: nil,
			footprint: "",
			expected:  nil,
		},
		{
			name:      "empty string",
			signature: new(""),
			footprint: "",
			expected:  nil,
		},
		{
			name:      "valid signature with footprint",
			signature: EncodeGeminiThoughtSignature(new("some-signature"), testGeminiFootprint),
			footprint: testGeminiFootprint,
			expected:  new("some-signature"),
		},
		{
			name:      "invalid prefix",
			signature: new("some-signature"),
			footprint: "",
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeGeminiThoughtSignature(tt.signature, tt.footprint)
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
			name:      "empty signature remains raw",
			signature: new(""),
			expected:  new(""),
		},
		{
			name:      "valid signature remains raw without footprint",
			signature: new("some-signature"),
			expected:  new("some-signature"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeGeminiThoughtSignature(tt.signature, "")
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestEncodeDecodeGeminiThoughtSignature(t *testing.T) {
	sig := new("some-random-signature-data")
	encoded := EncodeGeminiThoughtSignature(sig, testGeminiFootprint)
	require.NotNil(t, encoded)
	require.NotNil(t, DecodeGeminiThoughtSignature(encoded, testGeminiFootprint))
	require.NotNil(t, DecodeGeminiThoughtSignature(encoded, testGeminiFootprint))

	decoded := DecodeGeminiThoughtSignature(encoded, testGeminiFootprint)
	require.NotNil(t, decoded)
	require.Equal(t, *sig, *decoded)
	require.Nil(t, DecodeGeminiThoughtSignature(encoded, "ffffff"))
}

func TestGeminiEncodeDecodeRoundTrip(t *testing.T) {
	original := new("some-random-signature-data")

	// Encode
	encoded := EncodeGeminiThoughtSignature(original, "")
	require.NotNil(t, encoded)

	// Decode
	decoded := DecodeGeminiThoughtSignature(encoded, "")
	require.Nil(t, decoded)
}

func TestGeminiThoughtSignatureWholeValueCanDecodeAsBase64(t *testing.T) {
	signature := new("YWJjZA==")

	encoded := EncodeGeminiThoughtSignature(signature, "")
	require.NotNil(t, encoded)
	_, err := base64.StdEncoding.DecodeString(*encoded)
	require.NoError(t, err)

	encodedWithFootprint := EncodeGeminiThoughtSignature(signature, testGeminiFootprint)
	require.NotNil(t, encodedWithFootprint)
	_, err = base64.StdEncoding.DecodeString(*encodedWithFootprint)
	require.NoError(t, err)
}
