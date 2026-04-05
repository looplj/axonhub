package shared

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

const testFootprint = "a1b2c3"

func TestDecodeAnthropicSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature *string
		footprint string
		expected  *string
	}{
		{
			name:      "nil signature",
			signature: nil,
			footprint: testFootprint,
			expected:  nil,
		},
		{
			name:      "empty string",
			signature: new(""),
			footprint: testFootprint,
			expected:  nil,
		},
		{
			name:      "valid encoded signature",
			signature: EncodeAnthropicSignature(new("some-signature"), testFootprint),
			footprint: testFootprint,
			expected:  new(EnsureBase64Encoding("some-signature")),
		},
		{
			name:      "wrong footprint",
			signature: EncodeAnthropicSignature(new("some-signature"), testFootprint),
			footprint: "ffffff",
			expected:  nil,
		},
		{
			name:      "plain text",
			signature: new("just-a-plain-signature"),
			footprint: testFootprint,
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeAnthropicSignature(tt.signature, tt.footprint)
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
		footprint string
		expected  *string
	}{
		{
			name:      "nil signature",
			signature: nil,
			footprint: testFootprint,
			expected:  nil,
		},
		{
			name:      "valid signature with footprint",
			signature: new("some-signature"),
			footprint: testFootprint,
			expected: new(
				base64.StdEncoding.EncodeToString([]byte("AXN101")) +
					base64.StdEncoding.EncodeToString([]byte(testFootprint)) +
					EnsureBase64Encoding("some-signature"),
			),
		},
		{
			name:      "valid signature without footprint returns raw base64 value",
			signature: new("some-signature"),
			footprint: "",
			expected: new(
				EnsureBase64Encoding("some-signature"),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeAnthropicSignature(tt.signature, tt.footprint)
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

	encoded := EncodeAnthropicSignature(original, testFootprint)
	require.NotNil(t, encoded)
	require.NotNil(t, DecodeAnthropicSignature(encoded, testFootprint))
	require.NotNil(t, DecodeAnthropicSignature(encoded, testFootprint))

	decoded := DecodeAnthropicSignature(encoded, testFootprint)
	require.NotNil(t, decoded)
	require.Equal(t, EnsureBase64Encoding(*original), *decoded)
}

func TestAnthropicSignature_Base64Validity(t *testing.T) {
	sig := new("test-signature-data")
	encoded := EncodeAnthropicSignature(sig, testFootprint)
	require.NotNil(t, encoded)

	// The entire encoded string should be valid base64
	_, err := base64.StdEncoding.DecodeString(*encoded)
	require.NoError(t, err, "encoded signature should be valid base64")
}

func TestAnthropicSignature_CrossFootprintRejection(t *testing.T) {
	sig := new("original-provider-signature")
	fpA := ComputeFootprint("https://api.anthropic.com", "channel-a")
	fpB := ComputeFootprint("https://aihubmix.com", "channel-b")

	encoded := EncodeAnthropicSignature(sig, fpA)
	require.NotNil(t, encoded)

	// Same channel: accepted
	decoded := DecodeAnthropicSignature(encoded, fpA)
	require.NotNil(t, decoded)

	// Different channel: rejected
	decoded = DecodeAnthropicSignature(encoded, fpB)
	require.Nil(t, decoded)
}
