package biz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	apiKey, err := GenerateAPIKey()
	require.NoError(t, err)
	require.NotEmpty(t, apiKey)
	require.True(t, len(apiKey) > 3)
	require.Equal(t, "ah-", apiKey[:3])

	// Test that multiple calls produce different keys
	apiKey2, err := GenerateAPIKey()
	require.NoError(t, err)
	require.NotEqual(t, apiKey, apiKey2)
}
