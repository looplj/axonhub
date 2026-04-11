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

// Migrate adds support for new nanogpt channel types (nanogpt_chat, nanogpt_responses).
// Note: Existing 'nanogpt' channels are NOT migrated to preserve backward compatibility.
// The 'nanogpt' type is deprecated but remains functional for existing channels.
func (v *V0_9_0) Migrate(ctx context.Context, client *ent.Client) (err error) {
	// Use the passed context with system bypass for authorization
	ctx = authz.WithSystemBypass(ctx, "database-migrate")

	// Check for existing nanogpt channels to log deprecation warning
	nanogptCount, err := client.Channel.Query().
		Where(channel.TypeEQ(channel.TypeNanogpt)).
		Count(ctx)
	if err != nil {
		return err
	}

	if nanogptCount > 0 {
		log.Warn(ctx, "found channels using deprecated 'nanogpt' type - these will continue to work but new channels should use 'nanogpt_chat' or 'nanogpt_responses'",
			log.Int("count", nanogptCount),
			log.String("migration", "v0.9.0"),
		)
	}

	return nil
}
