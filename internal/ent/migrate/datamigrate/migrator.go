package datamigrate

import (
	"context"

	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// DataMigrator is an interface for data migration operations.
type DataMigrator interface {
	Version() string
	Migrate(ctx context.Context, client *ent.Client) error
}

// Migrator manages the execution of data migrations with version checking.
type Migrator struct {
	client        *ent.Client
	systemService *biz.SystemService
	migrations    []DataMigrator
}

// NewMigrator creates a new Migrator instance with all registered migrations.
func NewMigrator(client *ent.Client) *Migrator {
	migrator := NewMigratorWithoutRegistrations(client)
	migrator.Register(NewV0_3_0())
	migrator.Register(NewV0_4_0())

	return migrator
}

// NewMigratorWithoutRegistrations creates a new Migrator instance without any pre-registered migrations.
// This is useful for testing.
func NewMigratorWithoutRegistrations(client *ent.Client) *Migrator {
	return &Migrator{
		client:        client,
		systemService: biz.NewSystemService(biz.SystemServiceParams{}),
		migrations:    []DataMigrator{},
	}
}

// Register registers a data migrator to be executed.
func (m *Migrator) Register(migrator DataMigrator) *Migrator {
	m.migrations = append(m.migrations, migrator)
	return m
}

// shouldRunMigration checks if migration should be executed based on system version.
// Returns true if:
// - System version is not set (fresh install)
// - System version <= migration version
// Returns false if system version > migration version.
func (m *Migrator) shouldRunMigration(ctx context.Context, migrationVersion string) bool {
	ctx = ent.NewContext(ctx, m.client)

	systemVersion, err := m.systemService.Version(ctx)
	if err != nil {
		log.Warn(ctx, "failed to get system version, will run migration", log.Cause(err))
		return true
	}

	// System initialized, but not set version, set to v0.2.1
	if systemVersion == "" {
		systemVersion = "v0.2.1"
	}

	// Compare versions: if system version >= migration version, skip migration
	// This means the migration has already been applied or a newer version is installed
	if systemVersion >= migrationVersion {
		log.Info(ctx, "skipping migration: system version is equal or newer than migration version",
			log.String("system_version", systemVersion),
			log.String("migration_version", migrationVersion))

		return false
	}

	// System version < migration version, run the migration
	log.Info(ctx, "running migration",
		log.String("system_version", systemVersion),
		log.String("migration_version", migrationVersion))

	return true
}

// Run executes all registered migrations in order, checking versions before each migration.
func (m *Migrator) Run(ctx context.Context) error {
	ctx = ent.NewContext(ctx, m.client)

	inited, err := m.systemService.IsInitialized(ctx)
	if err != nil {
		return err
	}

	if !inited {
		log.Info(ctx, "system not initialized, skipping migration")
		return nil
	}

	for _, migration := range m.migrations {
		version := migration.Version()

		if !m.shouldRunMigration(ctx, version) {
			continue
		}

		log.Info(ctx, "executing migration", log.String("version", version))

		if err := migration.Migrate(ctx, m.client); err != nil {
			log.Error(ctx, "migration failed", log.String("version", version), log.Cause(err))
			return err
		}

		log.Info(ctx, "completed migration", log.String("version", version))
	}

	// Set system version if newer or unset.
	currentVersion, err := m.systemService.Version(ctx)
	if err != nil {
		return err
	}

	if currentVersion == "" || currentVersion < build.Version {
		if err := m.systemService.SetVersion(ctx, build.Version); err != nil {
			return err
		}
	}

	return nil
}
