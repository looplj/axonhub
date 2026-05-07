package shared

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeGeminiThoughtSignature(t *testing.T) {
	protoBytes := []byte{0x0a, 0x04, 0x74, 0x65, 0x73, 0x74}
	protoB64 := base64.StdEncoding.EncodeToString(protoBytes)

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
			name:      "protobuf-like base64 (gemini)",
			signature: new(protoB64),
			expected:  new(protoB64),
		},
		{
			name:      "openai-like signature (gAAA prefix) - rejected",
			signature: new("gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0"),
			expected:  nil,
		},
		{
			name:      "anthropic-like signature (Eq prefix) - rejected",
			signature: new("EqQBCAEDEgQIAhAEGAAgAigBMOzOAg=="),
			expected:  nil,
		},
		{
			name:      "unknown standard base64 with + char",
			signature: new("+AA="),
			expected:  nil,
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
			name:      "valid signature",
			signature: new("some-signature"),
			expected:  new("some-signature"),
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

func TestGeminiEncodeDecodeRoundTrip(t *testing.T) {
	protoBytes := []byte{0x0a, 0x04, 0x74, 0x65, 0x73, 0x74}
	original := new(base64.StdEncoding.EncodeToString(protoBytes))

	encoded := EncodeGeminiThoughtSignature(original)
	require.NotNil(t, encoded)
	require.Equal(t, *original, *encoded)

	decoded := DecodeGeminiThoughtSignature(encoded)
	require.NotNil(t, decoded)
	require.Equal(t, *original, *decoded)
}
