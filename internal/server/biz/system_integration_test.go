package biz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestSystemService_Initialize(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	migrator := datamigrate.NewMigrator(client)
	err := migrator.Run(t.Context())
	require.NoError(t, err)

	service := biz.NewSystemService(biz.SystemServiceParams{})
	ctx := t.Context()
	ctx = ent.NewContext(ctx, client)

	// Test system initialization with auto-generated secret key
	err = service.Initialize(ctx, &biz.InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
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

	// Verify owner user is created
	owner, err := client.User.Query().Where().First(ctx)
	require.NoError(t, err)
	require.Equal(t, "owner@example.com", owner.Email)
	require.True(t, owner.IsOwner)

	// Verify default project is created
	project, err := client.Project.Query().Where().First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Default", project.Name)

	// Verify owner is assigned to the project
	userProject, err := client.UserProject.Query().Where().First(ctx)
	require.NoError(t, err)
	require.Equal(t, owner.ID, userProject.UserID)
	require.Equal(t, project.ID, userProject.ProjectID)
	require.True(t, userProject.IsOwner)

	// Verify default roles are created (admin, developer, viewer)
	roles, err := client.Role.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, roles, 3)

	// Test idempotency - calling Initialize again should not error
	// but should not change the existing secret key or create duplicate projects
	originalKey := secretKey
	err = service.Initialize(ctx, &biz.InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Secret key should remain the same after second initialization
	secretKey2, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.Equal(t, originalKey, secretKey2)

	// Should still have only one project
	projectCount, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, projectCount)
}
