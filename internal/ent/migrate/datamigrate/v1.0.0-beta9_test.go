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

// dirtyUpdatedAt mimics the corrupted value channel_price.go used to persist:
// time.Now() (monotonic suffix "m=+..." plus local timezone) serialized by the
// SQLite driver via time.Time.String().
const dirtyUpdatedAt = "2026-08-18 00:13:11.396794292 +0800 CST m=+0.000011483"

// cleanedUpdatedAt is dirtyUpdatedAt with the monotonic suffix stripped, i.e.
// the exact string the driver produces after a scan round-trip. The optimistic
// lock in UpdateChannel compares against this value via updated_at = ?.
const cleanedUpdatedAt = "2026-08-18 00:13:11.396794292 +0800 CST"

// updatedAtMatches simulates the optimistic-lock comparison UpdateChannel
// performs: WHERE updated_at = <snapshot>. Returns how many rows match.
func updatedAtMatches(t *testing.T, driver *entsql.Driver, id int, updatedAt string) int {
	t.Helper()
	var n int
	require.NoError(t, driver.DB().QueryRowContext(context.Background(),
		"SELECT count(*) FROM channels WHERE id = ? AND updated_at = ?", id, updatedAt).Scan(&n))
	return n
}

func TestV1_0_0_Beta9_StripsMonotonicSuffixFromUpdatedAt(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:beta9-updated-at?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	ch := client.Channel.Create().
		SetName("dirty-updated-at").
		SetType(channel.TypeOpenai).
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-test"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SaveX(ctx)

	driver := client.Driver().(*entsql.Driver)
	_, err := driver.ExecContext(ctx,
		"UPDATE channels SET updated_at = ? WHERE id = ?", dirtyUpdatedAt, ch.ID)
	require.NoError(t, err)

	// Before migration the optimistic lock can never match: the stored value
	// still carries the "m=+..." suffix.
	require.Equal(t, 0, updatedAtMatches(t, driver, ch.ID, cleanedUpdatedAt))

	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))

	// After migration the cleaned snapshot matches, so UpdateChannel recovers.
	require.Equal(t, 1, updatedAtMatches(t, driver, ch.ID, cleanedUpdatedAt))
	// The raw dirty value is gone.
	require.Equal(t, 0, updatedAtMatches(t, driver, ch.ID, dirtyUpdatedAt))
}

func TestV1_0_0_Beta9_PreservesCleanUpdatedAt(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:beta9-clean-updated-at?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	ch := client.Channel.Create().
		SetName("clean-updated-at").
		SetType(channel.TypeOpenai).
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-test"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SaveX(ctx)

	clean := "2026-08-18 00:13:11.396794292 +0000 UTC"
	driver := client.Driver().(*entsql.Driver)
	_, err := driver.ExecContext(ctx,
		"UPDATE channels SET updated_at = ? WHERE id = ?", clean, ch.ID)
	require.NoError(t, err)

	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))

	// Unaffected rows still satisfy the optimistic lock.
	require.Equal(t, 1, updatedAtMatches(t, driver, ch.ID, clean))
}

func TestV1_0_0_Beta9_StripsMonotonicSuffixIsIdempotent(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:beta9-updated-at-idem?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	ch := client.Channel.Create().
		SetName("dirty-updated-at-idem").
		SetType(channel.TypeOpenai).
		SetCredentials(objects.ChannelCredentials{APIKey: "sk-test"}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SaveX(ctx)

	driver := client.Driver().(*entsql.Driver)
	_, err := driver.ExecContext(ctx,
		"UPDATE channels SET updated_at = ? WHERE id = ?", dirtyUpdatedAt, ch.ID)
	require.NoError(t, err)

	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))
	require.Equal(t, 1, updatedAtMatches(t, driver, ch.ID, cleanedUpdatedAt))

	// Running again must not corrupt anything further.
	require.NoError(t, datamigrate.NewV1_0_0_Beta9().Migrate(ctx, client))
	require.Equal(t, 1, updatedAtMatches(t, driver, ch.ID, cleanedUpdatedAt))
	require.Equal(t, 0, updatedAtMatches(t, driver, ch.ID, dirtyUpdatedAt))
}
