package subscription

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSSOToken_extracts_sso_cookie(t *testing.T) {
	// Given
	input := "cookie: other=value; sso=synthetic-sso-token; theme=dark"

	// When
	token := NormalizeSSOToken(input)

	// Then
	require.Equal(t, "synthetic-sso-token", token)
}

func TestNormalizeSSOToken_sanitizes_header_controls(t *testing.T) {
	// Given
	input := "sso=synthetic\r\nInjected: value"

	// When
	token := NormalizeSSOToken(input)

	// Then
	require.Equal(t, "syntheticInjected: value", token)
}

func TestNormalizeSSOToken_rejects_oversized_token(t *testing.T) {
	// Given
	input := make([]byte, maxSSOTokenLength+1)
	for index := range input {
		input[index] = 'a'
	}

	// When
	token := NormalizeSSOToken(string(input))

	// Then
	require.Empty(t, token)
}

func TestNormalizeSSOToken_skips_empty_cookie_and_uses_next(t *testing.T) {
	// Given
	input := "sso=; sso-rw=valid-token"

	// When
	result := NormalizeSSOToken(input)

	// Then
	require.Equal(t, "valid-token", result)
}

func TestNormalizeSSOToken_empty_supported_cookies_yield_empty(t *testing.T) {
	// Given
	input := "sso=; sso-rw="

	// When
	result := NormalizeSSOToken(input)

	// Then
	require.Empty(t, result)
}
