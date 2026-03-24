package datamigrate

import (
	"context"
	"reflect"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

// V0_5_0 implements DataMigrator for version 0.5.0 migration.
type V0_5_0 struct{}

// NewV0_5_0 creates a new V0_5_0 data migrator.
func NewV0_5_0() DataMigrator {
	return &V0_5_0{}
}

// Version returns the version of this migrator.
func (v *V0_5_0) Version() string {
	return "v0.5.0"
}

// Migrate performs the version 0.5.0 data migration.
// Backfills CloakingMode to "always" for claudecode channels where it is nil.
func (v *V0_5_0) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(context.Background(), "database-migrate")

	channels, err := client.Channel.Query().
		Where(channel.TypeEQ(channel.TypeClaudecode)).
		All(ctx)
	if err != nil {
		return err
	}

	if len(channels) == 0 {
		log.Info(ctx, "no claudecode channels found, skip migration")
		return nil
	}

	updated := 0
	for _, ch := range channels {
		if ch.Settings == nil {
			ch.Settings = &objects.ChannelSettings{}
		}

		if setCloakingModeIfNil(ch.Settings) {
			err = client.Channel.UpdateOneID(ch.ID).
				SetSettings(ch.Settings).
				Exec(ctx)
			if err != nil {
				return err
			}
			updated++
		}
	}

	log.Info(ctx, "backfilled CloakingMode for claudecode channels", log.Int("updated", updated))
	return nil
}

// setCloakingModeIfNil uses reflection to set CloakingMode field to "always" if field exists and is nil.
func setCloakingModeIfNil(settings *objects.ChannelSettings) bool {
	if settings == nil {
		return false
	}

	rv := reflect.ValueOf(settings).Elem()
	field := rv.FieldByName("CloakingMode")

	if !field.IsValid() {
		// Field doesn't exist yet in struct
		return false
	}

	if field.Kind() != reflect.Ptr || field.Type().Elem().Kind() != reflect.String {
		// Field exists but wrong type
		return false
	}

	if !field.IsNil() {
		// Already has a value
		return false
	}

	// Set to "always"
	mode := "always"
	field.Set(reflect.ValueOf(&mode))
	return true
}
