package biz

import (
	"context"
	"testing"

	"entgo.io/ent/privacy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
	"github.com/looplj/axonhub/ent/enttest"
	_ "github.com/looplj/axonhub/ent/runtime"
)

func TestSystemService_Initialize(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{Ent: client})
	ctx := context.Background()

	// Test system initialization with auto-generated secret key
	err := service.Initialize(ctx, &InitializeSystemArgs{
		OwnerEmail:    "owner@example.com",
		OwnerPassword: "password123",
	})
	require.NoError(t, err)

	// Verify system is initialized
	isInitialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	assert.True(t, isInitialized)

	// Verify secret key is set
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	secretKey, err := service.GetSecretKey(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, secretKey)
	assert.Len(t, secretKey, 64) // Should be 64 hex characters (32 bytes)

	// Test idempotency - calling Initialize again should not error
	// but should not change the existing secret key
	originalKey := secretKey
	err = service.Initialize(ctx, &InitializeSystemArgs{
		OwnerEmail:    "owner@example.com",
		OwnerPassword: "password123",
	})
	require.NoError(t, err)

	// Secret key should remain the same after second initialization
	secretKey2, err := service.GetSecretKey(ctx)
	require.NoError(t, err)
	assert.Equal(t, originalKey, secretKey2)
}

func TestSystemService_GetSecretKey_NotInitialized(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{Ent: client})
	ctx := context.Background()

	// Getting secret key before initialization should return error
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	_, err := service.GetSecretKey(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret key not found")
}
