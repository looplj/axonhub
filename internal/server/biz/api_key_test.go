package biz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	apiKey, err := GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEmpty(t, apiKey)
	assert.True(t, len(apiKey) > 3)
	assert.Equal(t, "ah-", apiKey[:3])

	// Test that multiple calls produce different keys
	apiKey2, err := GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEqual(t, apiKey, apiKey2)
}
