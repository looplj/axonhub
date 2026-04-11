package datamigrate

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
)

type V0_9_0 struct{}

func NewV0_9_0() DataMigrator {
	return &V0_9_0{}
}

func (v *V0_9_0) Version() string {
	return "v0.9.0"
}

func (v *V0_9_0) Migrate(ctx context.Context, client *ent.Client) (err error) {
	ctx = authz.WithSystemBypass(context.Background(), "database-migrate")

	nanogptChannels, err := client.Channel.Query().
		Where(channel.TypeEQ(channel.TypeNanogpt)).
		All(ctx)
	if err != nil {
		return err
	}

	if len(nanogptChannels) == 0 {
		log.Info(ctx, "no channels with type 'nanogpt' found, skipping migration")
		return nil
	}

	log.Info(ctx, "found channels with type 'nanogpt' to migrate", log.Int("count", len(nanogptChannels)))

	ctx, tx, err := client.OpenTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	affected, err := ent.FromContext(ctx).Channel.Update().
		Where(channel.TypeEQ(channel.TypeNanogpt)).
		SetType(channel.TypeNanogptChat).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Info(ctx, "migrated nanogpt channels to nanogpt_chat", log.Int("affected", affected))

	return tx.Commit()
}
