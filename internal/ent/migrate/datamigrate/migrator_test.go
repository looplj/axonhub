package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/server/biz"
)

// mockMigrator is a mock implementation of DataMigrator for testing.
type mockMigrator struct {
	version      string
	migrateCalls int
	migrateError error
}

func (m *mockMigrator) Version() string {
	return m.version
}

func (m *mockMigrator) Migrate(ctx context.Context, client *ent.Client) error {
	m.migrateCalls++
	return m.migrateError
}

func TestMigrator_Run_SystemNotInitialized(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create migrator with mock migrations
	migrator := datamigrate.NewMigratorWithoutRegistrations(client)
	mock := &mockMigrator{version: "v0.3.0"}
	migrator.Register(mock)

	// Run migration when system is not initialized
	err := migrator.Run(ctx)
	require.NoError(t, err)

	// Verify migration was not executed
	assert.Equal(t, 0, mock.migrateCalls)
}

func TestMigrator_Run_WithInitializedSystem(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Initialize system
	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	err := systemService.Initialize(ctx, &biz.InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Set system version to v0.2.1 (older than migration)
	err = systemService.SetVersion(ctx, "v0.2.1")
	require.NoError(t, err)

	// Create migrator with mock migration
	migrator := datamigrate.NewMigratorWithoutRegistrations(client)
	mock := &mockMigrator{version: "v0.3.0"}
	migrator.Register(mock)

	// Run migration
	err = migrator.Run(ctx)
	require.NoError(t, err)

	// Verify migration was executed
	assert.Equal(t, 1, mock.migrateCalls)
}

func TestMigrator_Run_SkipNewerVersion(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Initialize system
	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	err := systemService.Initialize(ctx, &biz.InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Set system version to v0.5.0 (newer than migration)
	err = systemService.SetVersion(ctx, "v0.5.0")
	require.NoError(t, err)

	// Create migrator with mock migration
	migrator := datamigrate.NewMigratorWithoutRegistrations(client)
	mock := &mockMigrator{version: "v0.3.0"}
	migrator.Register(mock)

	// Run migration
	err = migrator.Run(ctx)
	require.NoError(t, err)

	// Verify migration was not executed (system version is newer)
	assert.Equal(t, 0, mock.migrateCalls)
}

func TestMigrator_Run_SkipEqualVersion(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Initialize system
	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	err := systemService.Initialize(ctx, &biz.InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Set system version to v0.3.0 (equal to migration)
	err = systemService.SetVersion(ctx, "v0.3.0")
	require.NoError(t, err)

	// Create migrator with mock migration
	migrator := datamigrate.NewMigratorWithoutRegistrations(client)
	mock := &mockMigrator{version: "v0.3.0"}
	migrator.Register(mock)

	// Run migration
	err = migrator.Run(ctx)
	require.NoError(t, err)

	// Verify migration was not executed (system version is equal)
	assert.Equal(t, 0, mock.migrateCalls)
}

func TestMigrator_Run_MultipleMigrations(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Initialize system
	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	err := systemService.Initialize(ctx, &biz.InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Set system version to v0.2.1 (older than all migrations)
	err = systemService.SetVersion(ctx, "v0.2.1")
	require.NoError(t, err)

	// Create migrator with multiple mock migrations
	migrator := datamigrate.NewMigratorWithoutRegistrations(client)
	mock1 := &mockMigrator{version: "v0.3.0"}
	mock2 := &mockMigrator{version: "v0.4.0"}
	mock3 := &mockMigrator{version: "v0.5.0"}
	migrator.Register(mock1).Register(mock2).Register(mock3)

	// Run migrations
	err = migrator.Run(ctx)
	require.NoError(t, err)

	// Verify all migrations were executed in order
	assert.Equal(t, 1, mock1.migrateCalls)
	assert.Equal(t, 1, mock2.migrateCalls)
	assert.Equal(t, 1, mock3.migrateCalls)
}

func TestMigrator_Run_PartialMigrations(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Initialize system
	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	err := systemService.Initialize(ctx, &biz.InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Set system version to v0.3.5 (between migrations)
	err = systemService.SetVersion(ctx, "v0.3.5")
	require.NoError(t, err)

	// Create migrator with multiple mock migrations
	migrator := datamigrate.NewMigratorWithoutRegistrations(client)
	mock1 := &mockMigrator{version: "v0.3.0"} // Should be skipped
	mock2 := &mockMigrator{version: "v0.4.0"} // Should run
	mock3 := &mockMigrator{version: "v0.5.0"} // Should run
	migrator.Register(mock1).Register(mock2).Register(mock3)

	// Run migrations
	err = migrator.Run(ctx)
	require.NoError(t, err)

	// Verify only newer migrations were executed
	assert.Equal(t, 0, mock1.migrateCalls, "v0.3.0 should be skipped")
	assert.Equal(t, 1, mock2.migrateCalls, "v0.4.0 should run")
	assert.Equal(t, 1, mock3.migrateCalls, "v0.5.0 should run")
}

func TestMigrator_Run_EmptySystemVersion(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Manually create an initialized system without version (simulating old system)
	_, err := client.System.Create().
		SetKey(biz.SystemKeyInitialized).
		SetValue("true").
		Save(ctx)
	require.NoError(t, err)

	// Create migrator with mock migration
	migrator := datamigrate.NewMigratorWithoutRegistrations(client)
	mock := &mockMigrator{version: "v0.3.0"}
	migrator.Register(mock)

	// Run migration
	err = migrator.Run(ctx)
	require.NoError(t, err)

	// Verify migration was executed (empty version defaults to v0.2.1)
	assert.Equal(t, 1, mock.migrateCalls)
}

func TestMigrator_Register(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	// Create migrator
	migrator := datamigrate.NewMigratorWithoutRegistrations(client)

	// Register migrations
	mock1 := &mockMigrator{version: "v0.3.0"}
	mock2 := &mockMigrator{version: "v0.4.0"}

	result := migrator.Register(mock1).Register(mock2)

	// Verify chaining works
	assert.NotNil(t, result)
}

func TestMigrator_NewMigrator(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	// Create migrator with default registrations
	migrator := datamigrate.NewMigrator(client)

	// Verify migrator is created
	assert.NotNil(t, migrator)
}

func TestMigrator_IntegrationTest(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Initialize system
	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	err := systemService.Initialize(ctx, &biz.InitializeSystemArgs{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Set system version to v0.2.1 to trigger all migrations
	err = systemService.SetVersion(ctx, "v0.2.1")
	require.NoError(t, err)

	// Run all registered migrations
	migrator := datamigrate.NewMigrator(client)
	err = migrator.Run(ctx)
	require.NoError(t, err)

	// Verify system is still initialized
	isInitialized, err := systemService.IsInitialized(ctx)
	require.NoError(t, err)
	assert.True(t, isInitialized)
}

func TestMigrator_UpgradeFromV0_3_0(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Simulate an already initialized system on v0.3.0 without primary data storage
	_, err := client.System.Create().
		SetKey(biz.SystemKeyInitialized).
		SetValue("true").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.System.Create().
		SetKey(biz.SystemKeyVersion).
		SetValue("v0.3.0").
		Save(ctx)
	require.NoError(t, err)

	migrator := datamigrate.NewMigrator(client)
	err = migrator.Run(ctx)
	require.NoError(t, err)

	primaryCount, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, primaryCount)

	primaryDS, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Primary", primaryDS.Name)

	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	defaultID, err := systemService.DefaultDataStorageID(ctx)
	require.NoError(t, err)
	assert.Equal(t, primaryDS.ID, defaultID)

	version, err := systemService.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, build.Version, version)
}
