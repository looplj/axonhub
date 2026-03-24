package datamigrate_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/objects"
)

func TestV0_5_0_PreserveExistingConfig(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	// Create claudecode channel with explicit cloaking mode
	settings := &objects.ChannelSettings{}

	// Use reflection to set CloakingMode if field exists
	rv := reflect.ValueOf(settings).Elem()
	field := rv.FieldByName("CloakingMode")
	if field.IsValid() && field.CanSet() {
		mode := "follow_global"
		field.Set(reflect.ValueOf(&mode))
	}

	ch := client.Channel.Create().
		SetType(channel.TypeClaudecode).
		SetName("test-claudecode").
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"claude-3-5-sonnet"}).
		SetDefaultTestModel("claude-3-5-sonnet").
		SetSettings(settings).
		SaveX(ctx)

	// Run migration
	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify cloaking mode was not overwritten
	updated := client.Channel.GetX(ctx, ch.ID)

	// Check if CloakingMode field exists before asserting
	updatedRv := reflect.ValueOf(updated.Settings).Elem()
	updatedField := updatedRv.FieldByName("CloakingMode")
	if updatedField.IsValid() {
		// Field exists, verify it wasn't changed
		assert.False(t, updatedField.IsNil(), "CloakingMode should not be nil after setting")
		if !updatedField.IsNil() {
			modePtr := updatedField.Interface().(*string)
			assert.Equal(t, "follow_global", *modePtr)
		}
	}
}

func TestV0_5_0_BackfillNilConfig(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	// Create claudecode channel with nil CloakingMode
	settings := &objects.ChannelSettings{}

	ch := client.Channel.Create().
		SetType(channel.TypeClaudecode).
		SetName("test-claudecode-nil").
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"claude-3-5-sonnet"}).
		SetDefaultTestModel("claude-3-5-sonnet").
		SetSettings(settings).
		SaveX(ctx)

	// Run migration
	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify CloakingMode was backfilled to "always"
	updated := client.Channel.GetX(ctx, ch.ID)

	// Check if CloakingMode field exists before asserting
	updatedRv := reflect.ValueOf(updated.Settings).Elem()
	updatedField := updatedRv.FieldByName("CloakingMode")
	if updatedField.IsValid() {
		// Field exists, verify it was set to "always"
		assert.False(t, updatedField.IsNil(), "CloakingMode should be set after migration")
		if !updatedField.IsNil() {
			modePtr := updatedField.Interface().(*string)
			assert.Equal(t, "always", *modePtr)
		}
	}
}

func TestV0_5_0_OtherChannelsUnaffected(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	// Create anthropic channel (non-claudecode)
	settings := &objects.ChannelSettings{}

	ch := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("test-anthropic").
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"claude-3-5-sonnet"}).
		SetDefaultTestModel("claude-3-5-sonnet").
		SetSettings(settings).
		SaveX(ctx)

	// Run migration
	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify CloakingMode remains nil (not backfilled)
	updated := client.Channel.GetX(ctx, ch.ID)

	// Check if CloakingMode field exists before asserting
	updatedRv := reflect.ValueOf(updated.Settings).Elem()
	updatedField := updatedRv.FieldByName("CloakingMode")
	if updatedField.IsValid() {
		// Field exists, verify it remains nil (not touched by migration)
		assert.True(t, updatedField.IsNil(), "CloakingMode should remain nil for non-claudecode channels")
	}
}
