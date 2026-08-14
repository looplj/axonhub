package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCodeVerifier_returns_url_safe_value_with_requested_entropy(t *testing.T) {
	// Given
	const byteLength = 32

	// When
	verifier, err := GenerateCodeVerifier(byteLength)

	// Then
	require.NoError(t, err)
	decoded, err := base64.RawURLEncoding.DecodeString(verifier)
	require.NoError(t, err)
	require.Len(t, decoded, byteLength)
}

func TestGenerateCodeChallenge_returns_S256_base64url_value(t *testing.T) {
	// Given
	const verifier = "synthetic-pkce-verifier"
	expectedHash := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(expectedHash[:])

	// When
	challenge := GenerateCodeChallenge(verifier)

	// Then
	require.Equal(t, expected, challenge)
}

func TestGenerateState_returns_url_safe_value_with_requested_entropy(t *testing.T) {
	// Given
	const byteLength = 32

	// When
	state, err := GenerateState(byteLength)

	// Then
	require.NoError(t, err)
	decoded, err := base64.RawURLEncoding.DecodeString(state)
	require.NoError(t, err)
	require.Len(t, decoded, byteLength)
}

func TestGenerateCodeVerifier_enforces_rfc7636_byte_length_bounds(t *testing.T) {
	// Given
	invalid := []int{0, 8, 31, 97, 1024}

	// When
	for _, byteLength := range invalid {
		_, err := GenerateCodeVerifier(byteLength)

		// Then
		require.Error(t, err)
	}
	for _, byteLength := range []int{minCodeVerifierBytes, 64, maxCodeVerifierBytes} {
		verifier, err := GenerateCodeVerifier(byteLength)
		require.NoError(t, err)
		decoded, err := base64.RawURLEncoding.DecodeString(verifier)
		require.NoError(t, err)
		require.Len(t, decoded, byteLength)
	}
}

func TestGenerateState_enforces_byte_length_bounds(t *testing.T) {
	// Given
	for _, byteLength := range []int{0, 8, 15, 97} {
		_, err := GenerateState(byteLength)

		// Then
		require.Error(t, err)
	}
	for _, byteLength := range []int{minStateBytes, 32, maxStateBytes} {
		state, err := GenerateState(byteLength)
		require.NoError(t, err)
		decoded, err := base64.RawURLEncoding.DecodeString(state)
		require.NoError(t, err)
		require.Len(t, decoded, byteLength)
	}
}
