package datamigrate

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// V0_4_0 implements DataMigrator for version 0.4.0 migration.
type V0_4_0 struct{}

// NewV0_4_0 creates a new V0_4_0 data migrator.
func NewV0_4_0() DataMigrator {
	return &V0_4_0{}
}

// Migrate performs the version 0.4.0 data migration.
// Creates a primary data storage if it doesn't exist.
func (v *V0_4_0) Migrate(ctx context.Context, client *ent.Client) (err error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Check if a primary data storage already exists
	exists, err := client.DataStorage.Query().
		Where(datastorage.Primary(true)).
		Exist(ctx)
	if err != nil {
		return err
	}

	if exists {
		log.Info(ctx, "primary data storage already exists, skip migration")
		return nil
	}

	ctx, tx, err := client.OpenTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Create primary data storage
	primaryDataStorage, err := ent.FromContext(ctx).DataStorage.Create().
		SetName("primary").
		SetDescription("Primary database storage").
		SetPrimary(true).
		SetType(datastorage.TypeDatabase).
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Info(ctx, "created primary data storage", log.Int("data_storage_id", primaryDataStorage.ID))

	// Set default data storage ID.
	_, err = ent.FromContext(ctx).System.Create().
		SetKey(biz.SystemKeyDefaultDataStorage).
		SetValue(fmt.Sprintf("%d", primaryDataStorage.ID)).
		Save(ctx)
	if err != nil {
		return err
	}

	// Set initialized flag to true.
	log.Info(ctx, "saved the default data storage ID")

	return tx.Commit()
}
