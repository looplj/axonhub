package shared

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

const realAnthropicSignature = "Eq0CCokBCBAYAipAmkim+S4ApjNpcVSh82hYj016e9aYlvNfdj8ZaVbASj64fkCHtgDxjvumIhTpVr6WsoYoGyBtZOuoFPg7JUV7vjIPY2xhdWRlLXNvbm5ldC01OABCCHRoaW5raW5nWiRjYTEwYTFhOS03ZWFmLTRiZDUtYWFkMy1iY2MyY2Q1MWQ1MDgSDETIROQHvz1/jQbeLxIMwjZzeFruDsqTqYxwGgy4ekwdZi3oeDEsWGsiMD3w0HGjBb28dNuTqZE1X2zCSndpSwOWYwRhbrXFV8RIg6jFiS+MSo6Gt0QUFWKh4CpD8q8wDmAKZYQ45z+1rFBwX7SWdXo02qQNUkGIwm1fTFf/GIRTRwIUTNdG35tcDHWh6pJ/if5LjcPdJTMiiw+bFgPTCBgB"

func TestGuessSignatureProvider(t *testing.T) {
	// Mirrors a current Anthropic signature: the encoded value starts with
	// "Eq0C" and its opaque decoded payload contains a Claude model name.
	anthropicBinarySignature := make([]byte, 306)
	copy(anthropicBinarySignature, []byte{0x12, 0xad, 0x02, 0x0a, 0x89, 0x01})
	copy(anthropicBinarySignature[6:], "claude-sonnet-5 thinking ca10a1a9-7eaf-4bd5-aad3-bcc2cd51d508")

	tests := []struct {
		name             string
		raw              string
		expectedProvider SignatureProvider
		expectReasons    bool
	}{
		{
			name:             "OpenAI gAAAA prefix",
			raw:              "gAAAAABpg2hk4yLqQUPBKlNLPwYE5lSfBmhv0",
			expectedProvider: ProviderOpenAI,
			expectReasons:    true,
		},
		{
			name:             "OpenAI gAAA prefix",
			raw:              "gAAAxxxxxxxx",
			expectedProvider: ProviderOpenAI,
			expectReasons:    true,
		},
		{
			name:             "EqQ prefix without decoded model marker",
			raw:              "EqQBCAEDEgQIAhAEGAAgAigBMOzOAg==",
			expectedProvider: ProviderUnknown,
			expectReasons:    true,
		},
		{
			name:             "Eqo prefix without decoded model marker",
			raw:              "EqoBxxxxxxxx",
			expectedProvider: ProviderUnknown,
			expectReasons:    true,
		},
		{
			name:             "Eqr prefix without decoded model marker",
			raw:              "EqrBxxxxxxxx",
			expectedProvider: ProviderUnknown,
			expectReasons:    true,
		},
		{
			name:             "Anthropic decoded payload with Claude model and Eq0 prefix",
			raw:              base64.StdEncoding.EncodeToString(anthropicBinarySignature),
			expectedProvider: ProviderAnthropic,
			expectReasons:    true,
		},
		{
			name:             "real Anthropic claude-sonnet-5 signature",
			raw:              realAnthropicSignature,
			expectedProvider: ProviderAnthropic,
			expectReasons:    true,
		},
		{
			name:             "Gemini protobuf-like",
			raw:              base64.StdEncoding.EncodeToString([]byte{0x0a, 0x04, 0x74, 0x65, 0x73, 0x74}),
			expectedProvider: ProviderGemini,
			expectReasons:    true,
		},
		{
			name:             "Unknown standard base64",
			raw:              "SGVsbG8=",
			expectedProvider: ProviderUnknown,
			expectReasons:    true,
		},
		{
			name:             "Unknown non-base64",
			raw:              "not-base64!!!",
			expectedProvider: ProviderUnknown,
			expectReasons:    true,
		},
		{
			name:             "Quoted string",
			raw:              `"gAAAAABpg2hk"`,
			expectedProvider: ProviderOpenAI,
			expectReasons:    true,
		},
		{
			name:             "Empty string",
			raw:              "",
			expectedProvider: ProviderUnknown,
			expectReasons:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GuessSignatureProvider(tt.raw)
			require.Equal(t, tt.expectedProvider, result.Provider)
			if tt.expectReasons {
				require.NotEmpty(t, result.Reasons)
			}
		})
	}
}

func TestContainsAnthropicModel(t *testing.T) {
	tests := []struct {
		name     string
		buf      []byte
		expected bool
	}{
		{
			name:     "Claude model marker in binary payload",
			buf:      []byte{0x12, 0x03, 0x00, 'C', 'l', 'a', 'u', 'd', 'e'},
			expected: true,
		},
		{
			name:     "Anthropic marker in binary payload",
			buf:      []byte{0x00, 0x01, 'a', 'n', 't', 'h', 'r', 'o', 'p', 'i', 'c'},
			expected: true,
		},
		{
			name:     "generic protobuf without model marker",
			buf:      []byte{0x0a, 0x02, 0x08, 0x01},
			expected: false,
		},
		{
			name:     "case insensitive marker",
			buf:      []byte("CLAUDE-OPUS"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, containsAnthropicModel(tt.buf))
		})
	}
}

func TestIsStdBase64String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid no padding", "SGVsbG8", true},
		{"valid one padding", "SGVsbG8=", true},
		{"valid two padding", "SGVsbG8s", true},
		{"empty string", "", false},
		{"invalid char", "SGVs-bG8=", false},
		{"padding in middle", "SGV=sbG8=", false},
		{"non-padding after padding", "SGVsbG8=abc", false},
		{"three padding chars", "SGVsbG8===", false},
		{"only padding", "===", false},
		{"valid with + and /", "+/+/", true},
		{"valid complex", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3ODkwKysvLw==", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStdBase64String(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestLooksLikeProto(t *testing.T) {
	tests := []struct {
		name     string
		buf      []byte
		expected bool
	}{
		{
			name:     "empty buffer",
			buf:      []byte{},
			expected: false,
		},
		{
			name:     "valid single varint field",
			buf:      []byte{0x08, 0x01}, // field 1, wire type 0, varint value 1
			expected: true,
		},
		{
			name:     "valid length-delimited field",
			buf:      []byte{0x0a, 0x04, 0x74, 0x65, 0x73, 0x74}, // field 1, wire type 2, length 4, "test"
			expected: true,
		},
		{
			name:     "valid multiple fields",
			buf:      []byte{0x08, 0x01, 0x12, 0x02, 0x68, 0x69}, // field1 varint 1, field2 length-delimited "hi"
			expected: true,
		},
		{
			name:     "valid 64-bit field",
			buf:      []byte{0x09, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, // field 1, wire type 1, 8 bytes
			expected: true,
		},
		{
			name:     "valid 32-bit field",
			buf:      []byte{0x0d, 0x01, 0x02, 0x03, 0x04}, // field 1, wire type 5, 4 bytes
			expected: true,
		},
		{
			name:     "field number 0 rejected",
			buf:      []byte{0x00, 0x01}, // field 0, wire type 0
			expected: false,
		},
		{
			name:     "deprecated start group rejected",
			buf:      []byte{0x0b, 0x00}, // field 1, wire type 3
			expected: false,
		},
		{
			name:     "deprecated end group rejected",
			buf:      []byte{0x0c, 0x00}, // field 1, wire type 4
			expected: false,
		},
		{
			name:     "unknown wire type rejected",
			buf:      []byte{0x0e, 0x00}, // field 1, wire type 6
			expected: false,
		},
		{
			name:     "incomplete varint",
			buf:      []byte{0x08, 0x80}, // field 1, wire type 0, incomplete varint (no terminating byte)
			expected: false,
		},
		{
			name:     "length-delimited exceeds buffer",
			buf:      []byte{0x0a, 0x10, 0x01}, // field 1, wire type 2, claims length 16 but only 1 byte follows
			expected: false,
		},
		{
			name:     "64-bit field truncated",
			buf:      []byte{0x09, 0x01, 0x02}, // field 1, wire type 1, only 2 bytes
			expected: false,
		},
		{
			name:     "32-bit field truncated",
			buf:      []byte{0x0d, 0x01, 0x02}, // field 1, wire type 5, only 2 bytes
			expected: false,
		},
		{
			name:     "real gemini-like protobuf",
			buf:      []byte{0x0a, 0x04, 0x74, 0x65, 0x73, 0x74},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeProto(tt.buf)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestReadVarint64(t *testing.T) {
	tests := []struct {
		name        string
		buf         []byte
		expectValue uint64
		expectN     int
	}{
		{
			name:        "single byte",
			buf:         []byte{0x01},
			expectValue: 1,
			expectN:     1,
		},
		{
			name:        "multi-byte",
			buf:         []byte{0x80, 0x01},
			expectValue: 128,
			expectN:     2,
		},
		{
			name:        "max uint32 varint",
			buf:         []byte{0xff, 0xff, 0xff, 0xff, 0x0f},
			expectValue: 0xffffffff,
			expectN:     5,
		},
		{
			name:        "max uint64 varint",
			buf:         []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01},
			expectValue: 0xffffffffffffffff,
			expectN:     10,
		},
		{
			name:        "empty buffer",
			buf:         []byte{},
			expectValue: 0,
			expectN:     0,
		},
		{
			name:        "incomplete (no termination)",
			buf:         []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
			expectValue: 0,
			expectN:     0,
		},
		{
			name:        "incomplete short",
			buf:         []byte{0x80},
			expectValue: 0,
			expectN:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, n := readVarint64(tt.buf)
			require.Equal(t, tt.expectValue, val)
			require.Equal(t, tt.expectN, n)
		})
	}
}

func TestGuessSignatureProviderEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		raw              string
		expectedProvider SignatureProvider
	}{
		{
			name:             "OpenAI prefix exact match gAAAA",
			raw:              "gAAAA",
			expectedProvider: ProviderOpenAI,
		},
		{
			name:             "OpenAI prefix exact match gAAA",
			raw:              "gAAA",
			expectedProvider: ProviderOpenAI,
		},
		{
			name:             "EqQ prefix exact match is unknown",
			raw:              "EqQ",
			expectedProvider: ProviderUnknown,
		},
		{
			name:             "Gemini empty protobuf",
			raw:              base64.StdEncoding.EncodeToString([]byte{}),
			expectedProvider: ProviderUnknown,
		},
		{
			name:             "Gemini invalid protobuf (field 0)",
			raw:              base64.StdEncoding.EncodeToString([]byte{0x00, 0x01}),
			expectedProvider: ProviderUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GuessSignatureProvider(tt.raw)
			require.Equal(t, tt.expectedProvider, result.Provider)
		})
	}
}
