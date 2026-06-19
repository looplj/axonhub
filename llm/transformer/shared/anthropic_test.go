package shared

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

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
			name:      "empty string - rejected",
			signature: new(""),
			expected:  nil,
		},
		{
			name:      "anthropic-like signature (Eq prefix)",
			signature: new("EqQBCAEDEgQIAhAEGAAgAigBMOzOAg=="),
			expected:  new("EqQBCAEDEgQIAhAEGAAgAigBMOzOAg=="),
		},
		{
			name:      "openai-like signature (gAAA prefix) - rejected",
			signature: new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0"),
			expected:  nil,
		},
		{
			name:      "gemini-like protobuf base64 - rejected",
			signature: new(base64.StdEncoding.EncodeToString([]byte{0x0a, 0x04, 0x74, 0x65, 0x73, 0x74})),
			expected:  nil,
		},
		{
			name:      "unknown standard base64 - rejected",
			signature: new("SGVsbG8="),
			expected:  nil,
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
			name:      "valid signature - base64 encodes if needed",
			signature: new("some-signature"),
			expected:  new(EnsureBase64Encoding("some-signature")),
		},
		{
			name:      "already base64 signature",
			signature: new("YWxyZWFkeS1iYXNlNjQtZW5jb2RlZA=="),
			expected:  new("YWxyZWFkeS1iYXNlNjQtZW5jb2RlZA=="),
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
	original := new("EqQBCAEDEgQIAhAEGAAgAigBMOzOAg==")

	encoded := EncodeAnthropicSignature(original)
	require.NotNil(t, encoded)

	decoded := DecodeAnthropicSignature(encoded)
	require.NotNil(t, decoded)
	require.Equal(t, *original, *decoded)
}
