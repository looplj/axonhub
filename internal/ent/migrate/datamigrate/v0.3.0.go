package datamigrate

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
)

// V0_3_0 creates a default project if it doesn't exist.
func V0_3_0(ctx context.Context, client *ent.Client) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	exists, err := tx.Project.Query().Limit(1).Exist(ctx)
	if err != nil {
		return err
	}

	if exists {
		log.Info(ctx, "existed project found, skip migration")
		return nil
	}

	err = tx.Project.Create().
		SetSlug("default").
		SetName("Default").
		SetDescription("Default project").
		SetStatus("active").
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		return err
	}

	err = tx.UserProject.Create().
		SetUserID(1).
		SetProjectID(1).
		SetIsOwner(true).
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		return err
	}

	return tx.Commit()
}
