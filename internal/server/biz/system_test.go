package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	_ "github.com/looplj/axonhub/internal/ent/runtime"
)

func TestSystemService_Initialize(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// Test system initialization with auto-generated secret key
	err := service.Initialize(ctx, &InitializeSystemArgs{
		OwnerEmail:    "owner@example.com",
		OwnerPassword: "password123",
	})
	require.NoError(t, err)

	// Verify system is initialized
	isInitialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	require.True(t, isInitialized)

	// Verify secret key is set
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	secretKey, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, secretKey)
	require.Len(t, secretKey, 64) // Should be 64 hex characters (32 bytes)

	// Test idempotency - calling Initialize again should not error
	// but should not change the existing secret key
	originalKey := secretKey
	err = service.Initialize(ctx, &InitializeSystemArgs{
		OwnerEmail:    "owner@example.com",
		OwnerPassword: "password123",
	})
	require.NoError(t, err)

	// Secret key should remain the same after second initialization
	secretKey2, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.Equal(t, originalKey, secretKey2)
}

func TestSystemService_GetSecretKey_NotInitialized(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	service := NewSystemService(SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// Getting secret key before initialization should return error
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	_, err := service.SecretKey(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "secret key not found")
}
