package datamigrate_test

import (
	"context"
	"database/sql"
	"testing"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/objects"
)

func extractSettingsJSONField(t *testing.T, driver *entsql.Driver, id int, path string) []byte {
	t.Helper()
	var null sql.NullString
	err := driver.DB().QueryRowContext(context.Background(),
		"SELECT json_extract(settings, ?) FROM channels WHERE id = ?",
		path, id).Scan(&null)
	require.NoError(t, err)
	if !null.Valid {
		return nil
	}
	return []byte(null.String)
}

func TestV1_0_0_Beta9_StripsProviderQuotaFromSettings(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:beta9-providerquota?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	ch := client.Channel.Create().
		SetName("opencode-legacy").
		SetType(channel.TypeOpencodeGo).
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-test"}).SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
		SetSettings(&objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{RPM: int64Ptr(10)},
		}).
		SaveX(ctx)

	// Simulate a legacy row: inject the obsolete providerQuota (incl. auth cookie).
	driver := client.Driver().(*entsql.Driver)
	legacySettings := `{"rateLimit":{"rpm":10},"providerQuota":{"opencodeGo":{"workspaceId":"wk_1","authCookie":"auth=live-session-cookie"}}}`
	_, err := driver.ExecContext(ctx,
		"UPDATE channels SET settings = ? WHERE id = ?", legacySettings, ch.ID)
	require.NoError(t, err)

	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))

	require.Nil(t, extractSettingsJSONField(t, driver, ch.ID, "$.providerQuota"))
	require.JSONEq(t, `{"rpm":10}`, string(extractSettingsJSONField(t, driver, ch.ID, "$.rateLimit")))
}

func TestV1_0_0_Beta9_IsIdempotent(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:beta9-idempotent?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	ch := client.Channel.Create().
		SetName("opencode-legacy-2").
		SetType(channel.TypeOpencodeGo).
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-test"}).SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
		SetSettings(&objects.ChannelSettings{}).
		SaveX(ctx)

	driver := client.Driver().(*entsql.Driver)
	_, err := driver.ExecContext(ctx,
		"UPDATE channels SET settings = ? WHERE id = ?",
		`{"providerQuota":{"opencodeGo":{"workspaceId":"wk_2"}}}`, ch.ID)
	require.NoError(t, err)

	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))
	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))

	require.Nil(t, extractSettingsJSONField(t, driver, ch.ID, "$.providerQuota"))
}

func TestV1_0_0_Beta9_StripsJsonNullProviderQuota(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:beta9-jsonnull?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	ch := client.Channel.Create().
		SetName("opencode-json-null").
		SetType(channel.TypeOpencodeGo).
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-test"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetSettings(&objects.ChannelSettings{}).
		SaveX(ctx)

	driver := client.Driver().(*entsql.Driver)
	// JSON-null providerQuota is a present key that json_extract maps to SQL NULL.
	_, err := driver.ExecContext(ctx,
		"UPDATE channels SET settings = ? WHERE id = ?",
		`{"providerQuota":null}`, ch.ID)
	require.NoError(t, err)

	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))

	require.Nil(t, extractSettingsJSONField(t, driver, ch.ID, "$.providerQuota"))
}

func TestV1_0_0_Beta9_LeavesUntouchedChannelsAlone(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:beta9-untouched?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	ch := client.Channel.Create().
		SetName("opencode-clean").
		SetType(channel.TypeOpencodeGo).
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-test"}).SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
		SetSettings(&objects.ChannelSettings{
			RateLimit: &objects.ChannelRateLimit{RPM: int64Ptr(10)},
		}).
		SaveX(ctx)

	driver := client.Driver().(*entsql.Driver)
	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))

	require.JSONEq(t, `{"rpm":10}`, string(extractSettingsJSONField(t, driver, ch.ID, "$.rateLimit")))
	require.Nil(t, extractSettingsJSONField(t, driver, ch.ID, "$.providerQuota"))
}

func int64Ptr(v int64) *int64 { return &v }
