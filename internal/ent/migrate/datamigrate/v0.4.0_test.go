package datamigrate_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestV0_4_0_CreatePrimaryDataStorage(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Run migration
	err := datamigrate.NewV0_4_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify primary data storage was created
	ds, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Primary", ds.Name)
	assert.Equal(t, "Primary database storage", ds.Description)
	assert.True(t, ds.Primary)
	assert.Equal(t, datastorage.TypeDatabase, ds.Type)
	assert.Equal(t, datastorage.StatusActive, ds.Status)
	assert.NotNil(t, ds.Settings)

	// Verify default data storage system setting was created
	sys, err := client.System.Query().
		Where(system.KeyEQ(biz.SystemKeyDefaultDataStorage)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, biz.SystemKeyDefaultDataStorage, sys.Key)
	assert.Equal(t, fmt.Sprintf("%d", ds.ID), sys.Value)
}

func TestV0_4_0_PrimaryDataStorageAlreadyExists(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a primary data storage before migration
	existingDS, err := client.DataStorage.Create().
		SetName("existing-primary").
		SetDescription("Existing primary storage").
		SetPrimary(true).
		SetType(datastorage.TypeDatabase).
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Run migration
	err = datamigrate.NewV0_4_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify no new primary data storage was created
	count, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify the existing primary data storage is still there
	ds, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, existingDS.ID, ds.ID)
	assert.Equal(t, "existing-primary", ds.Name)
}

func TestV0_4_0_Idempotency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Run migration first time
	err := datamigrate.NewV0_4_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Get the created data storage ID
	ds1, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Only(ctx)
	require.NoError(t, err)

	// Run migration second time - should be idempotent
	err = datamigrate.NewV0_4_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify still only one primary data storage exists
	count, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify it's the same data storage
	ds2, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, ds1.ID, ds2.ID)
}

func TestV0_4_0_VerifyDataStorageFields(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Run migration
	err := datamigrate.NewV0_4_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify all fields of the created data storage
	ds, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Only(ctx)
	require.NoError(t, err)

	// Check all required fields
	assert.NotZero(t, ds.ID)
	assert.NotZero(t, ds.CreatedAt)
	assert.NotZero(t, ds.UpdatedAt)
	assert.Equal(t, "Primary", ds.Name)
	assert.Equal(t, "Primary database storage", ds.Description)
	assert.True(t, ds.Primary)
	assert.Equal(t, datastorage.TypeDatabase, ds.Type)
	assert.Equal(t, datastorage.StatusActive, ds.Status)
	assert.NotNil(t, ds.Settings)
}

func TestV0_4_0_DefaultDataStorageSystemSetting(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Run migration
	err := datamigrate.NewV0_4_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Get the created data storage
	ds, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Only(ctx)
	require.NoError(t, err)

	// Verify system setting was created with correct value
	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	defaultID, err := systemService.DefaultDataStorageID(ctx)
	require.NoError(t, err)
	assert.Equal(t, ds.ID, defaultID)
}

func TestV0_4_0_MultipleNonPrimaryDataStorages(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create multiple non-primary data storages before migration
	_, err := client.DataStorage.Create().
		SetName("storage-1").
		SetDescription("Storage 1").
		SetPrimary(false).
		SetType(datastorage.TypeFs).
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.DataStorage.Create().
		SetName("storage-2").
		SetDescription("Storage 2").
		SetPrimary(false).
		SetType(datastorage.TypeS3).
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Run migration
	err = datamigrate.NewV0_4_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify primary data storage was created
	primaryDS, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Primary", primaryDS.Name)

	// Verify total count is 3 (2 existing + 1 new primary)
	totalCount, err := client.DataStorage.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, totalCount)

	// Verify only one primary exists
	primaryCount, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, primaryCount)
}
