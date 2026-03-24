package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/enttest"
)

func TestChannelClientIDService_GetOrCreateClientID(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	svc := NewChannelClientIDService(client)

	t.Run("creates new client ID on first call", func(t *testing.T) {
		channelID := 1
		principalKind := "api_key"
		principalHash := "api_key:abc123hash"

		clientIDHex, err := svc.GetOrCreateClientID(ctx, channelID, principalKind, principalHash)
		require.NoError(t, err)
		assert.NotEmpty(t, clientIDHex)
		assert.Len(t, clientIDHex, 64) // SHA256 hex = 64 chars
	})

	t.Run("returns same client ID for same channel and principal", func(t *testing.T) {
		channelID := 2
		principalKind := "api_key"
		principalHash := "api_key:stable_hash"

		clientID1, err := svc.GetOrCreateClientID(ctx, channelID, principalKind, principalHash)
		require.NoError(t, err)

		clientID2, err := svc.GetOrCreateClientID(ctx, channelID, principalKind, principalHash)
		require.NoError(t, err)

		assert.Equal(t, clientID1, clientID2, "should return same client ID for same inputs")
	})

	t.Run("different API keys produce different client IDs", func(t *testing.T) {
		channelID := 3
		principalKind := "api_key"

		hash1 := ComputePrincipalHash(principalKind, "key_aaa")
		hash2 := ComputePrincipalHash(principalKind, "key_bbb")

		clientID1, err := svc.GetOrCreateClientID(ctx, channelID, principalKind, hash1)
		require.NoError(t, err)

		clientID2, err := svc.GetOrCreateClientID(ctx, channelID, principalKind, hash2)
		require.NoError(t, err)

		assert.NotEqual(t, clientID1, clientID2, "different API keys should produce different client IDs")
	})

	t.Run("OAuth uses special principal hash", func(t *testing.T) {
		channelID := 4
		principalKind := "oauth"
		principalHash := ComputePrincipalHash(principalKind, "")

		assert.Equal(t, "__oauth__", principalHash)

		clientIDHex, err := svc.GetOrCreateClientID(ctx, channelID, principalKind, principalHash)
		require.NoError(t, err)
		assert.NotEmpty(t, clientIDHex)
	})

	t.Run("same channel different principals produce different IDs", func(t *testing.T) {
		channelID := 5

		oauthHash := ComputePrincipalHash("oauth", "")
		apiKeyHash := ComputePrincipalHash("api_key", "some_key")

		oauthID, err := svc.GetOrCreateClientID(ctx, channelID, "oauth", oauthHash)
		require.NoError(t, err)

		apiKeyID, err := svc.GetOrCreateClientID(ctx, channelID, "api_key", apiKeyHash)
		require.NoError(t, err)

		assert.NotEqual(t, oauthID, apiKeyID, "OAuth and API key should have different client IDs")
	})
}

func TestComputePrincipalHash(t *testing.T) {
	t.Run("OAuth returns special constant", func(t *testing.T) {
		hash := ComputePrincipalHash("oauth", "")
		assert.Equal(t, "__oauth__", hash)

		hash = ComputePrincipalHash("oauth", "ignored_value")
		assert.Equal(t, "__oauth__", hash)
	})

	t.Run("API key returns prefixed SHA256", func(t *testing.T) {
		hash := ComputePrincipalHash("api_key", "test_key_123")
		assert.True(t, len(hash) > len("api_key:"))
		assert.Contains(t, hash, "api_key:")

		// Same key produces same hash
		hash2 := ComputePrincipalHash("api_key", "test_key_123")
		assert.Equal(t, hash, hash2)

		// Different key produces different hash
		hash3 := ComputePrincipalHash("api_key", "different_key")
		assert.NotEqual(t, hash, hash3)
	})

	t.Run("empty API key produces valid hash", func(t *testing.T) {
		hash := ComputePrincipalHash("api_key", "")
		assert.Contains(t, hash, "api_key:")
		assert.True(t, len(hash) > len("api_key:"))
	})
}
