package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/ent/enttest"
	_ "github.com/looplj/axonhub/ent/runtime"
	_ "github.com/mattn/go-sqlite3"
)

func TestSystemService_Initialize(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{Ent: client})
	ctx := context.Background()

	// Test system initialization with auto-generated secret key
	err := service.Initialize(ctx, "")
	require.NoError(t, err)

	// Verify system is initialized
	isInitialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	assert.True(t, isInitialized)

	// Verify secret key is set
	secretKey, err := service.GetSecretKey(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, secretKey)
	assert.Len(t, secretKey, 64) // Should be 64 hex characters (32 bytes)

	// Test idempotency - calling Initialize again should not error
	// but should not change the existing secret key
	originalKey := secretKey
	err = service.Initialize(ctx, "")
	require.NoError(t, err)

	// Secret key should remain the same after second initialization
	secretKey2, err := service.GetSecretKey(ctx)
	require.NoError(t, err)
	assert.Equal(t, originalKey, secretKey2)
}

func TestSystemService_Initialize_WithCustomKey(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{Ent: client})
	ctx := context.Background()

	customKey := "my-custom-secret-key-for-testing"

	// Test system initialization with custom secret key
	err := service.Initialize(ctx, customKey)
	require.NoError(t, err)

	// Verify secret key is set to custom value
	secretKey, err := service.GetSecretKey(ctx)
	require.NoError(t, err)
	assert.Equal(t, customKey, secretKey)
}

func TestSystemService_GetSecretKey_NotInitialized(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{Ent: client})
	ctx := context.Background()

	// Getting secret key before initialization should return error
	_, err := service.GetSecretKey(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret key not found")
}