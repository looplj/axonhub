package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestUpgrade(t *testing.T) {
	t.Run("version and metadata are not a preview", func(t *testing.T) {
		old := sampleManifest(t)
		next := sampleManifest(t)
		next.Version = "1.2.0"
		next.Name = "Renamed"
		next.Description = "updated"
		next.AxonhubVersion = ">=1.0.0 <2.0.0"
		page := next.UI.Pages[0]
		page.Entry = "ui/other.html"
		page.Title = mustTitle(t, `{"en":"Other"}`)
		next.UI = &objects.PluginUI{Pages: []objects.PluginUIPage{page}}

		diff := DiffManifest(&old, &next)
		require.False(t, diff.NeedsPreview())
		require.Empty(t, diff.Permissions)
		require.Empty(t, diff.Events)
		require.Empty(t, diff.Hooks)
		require.Empty(t, diff.Pages)
	})

	t.Run("added permission event hook and page are a preview", func(t *testing.T) {
		old := sampleManifest(t)

		withPermission := sampleManifest(t)
		withPermission.Permissions = []string{"read_channels", "write_channels"}
		diff := DiffManifest(&old, &withPermission)
		require.True(t, diff.NeedsPreview())
		require.Equal(t, []string{"write_channels"}, diff.Permissions)

		withEvent := sampleManifest(t)
		withEvent.Events = []string{"request.failover"}
		diff = DiffManifest(&old, &withEvent)
		require.True(t, diff.NeedsPreview())
		require.Equal(t, []string{"request.failover"}, diff.Events)

		withHook := sampleManifest(t)
		withHook.Hooks = &objects.PluginHooksSpec{Request: true}
		diff = DiffManifest(&old, &withHook)
		require.True(t, diff.NeedsPreview())
		require.Equal(t, []string{"request"}, diff.Hooks)

		withPage := sampleManifest(t)
		withPage.UI = &objects.PluginUI{Pages: []objects.PluginUIPage{
			old.UI.Pages[0],
			{Key: "logs", Entry: "ui/logs.html", Title: mustTitle(t, `"Logs"`)},
		}}
		diff = DiffManifest(&old, &withPage)
		require.True(t, diff.NeedsPreview())
		require.Equal(t, []string{"logs"}, diff.Pages)
	})

	t.Run("removed permission is not a preview", func(t *testing.T) {
		old := sampleManifest(t)
		old.Permissions = []string{"read_channels", "write_channels"}
		next := sampleManifest(t)
		next.Permissions = []string{"read_channels"}
		require.False(t, DiffManifest(&old, &next).NeedsPreview())
	})

	t.Run("permission convergence", func(t *testing.T) {
		granted, err := ConvergePermissions(
			[]string{"write_channels", "read_channels"},
			[]string{"read_channels", "webhook"},
			nil,
		)
		require.NoError(t, err)
		require.Equal(t, []string{"read_channels"}, granted)

		submitted := []string{"webhook", "read_channels"}
		granted, err = ConvergePermissions(
			[]string{"read_channels"},
			[]string{"read_channels", "webhook", "write_channels"},
			&submitted,
		)
		require.NoError(t, err)
		require.Equal(t, []string{"webhook", "read_channels"}, granted)

		outside := []string{"write_channels"}
		granted, err = ConvergePermissions(nil, []string{"read_channels"}, &outside)
		require.ErrorContains(t, err, "write_channels")
		require.Nil(t, granted)

		old := sampleManifest(t)
		old.Permissions = []string{"read_channels", "write_channels"}
		next := sampleManifest(t)
		next.Permissions = []string{"read_channels"}
		plan, err := PrepareUpgrade(&old, &next, old.Permissions, map[string]any{}, &outside)
		require.ErrorContains(t, err, "write_channels")
		require.Nil(t, plan)
	})

	t.Run("secret downgrade names the field and not the value", func(t *testing.T) {
		const secret = "top-secret-token-value"
		old := sampleManifest(t)
		next := sampleManifest(t)
		token := next.ConfigSchema.Properties["token"]
		token.Format = ""
		next.ConfigSchema.Properties["token"] = token
		oldConfig := map[string]any{"token": secret, "threshold": 3}

		plan, err := PrepareUpgrade(&old, &next, nil, oldConfig, nil)
		require.Error(t, err)
		require.Nil(t, plan)
		require.ErrorContains(t, err, "token")
		require.ErrorContains(t, err, "secret")
		require.NotContains(t, err.Error(), secret)
		require.Equal(t, secret, oldConfig["token"])
	})

	t.Run("deleted secret is removed from migrated config", func(t *testing.T) {
		const secret = "deleted-secret-value"
		old := sampleManifest(t)
		next := sampleManifest(t)
		delete(next.ConfigSchema.Properties, "token")
		oldConfig := map[string]any{"token": secret, "threshold": 3}

		plan, err := PrepareUpgrade(&old, &next, nil, oldConfig, nil)
		require.NoError(t, err)
		require.NotNil(t, plan)
		require.NotContains(t, plan.Config, "token")
		require.Equal(t, 3, plan.Config["threshold"])
		encoded, err := json.Marshal(plan.Config)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), secret)
		require.Equal(t, secret, oldConfig["token"])
	})

	t.Run("incompatible retained type fails", func(t *testing.T) {
		old := sampleManifest(t)
		next := sampleManifest(t)
		oldConfig := map[string]any{"threshold": "10", "token": "kept"}

		migrated, err := MigrateConfig(oldConfig, next.ConfigSchema)
		require.Error(t, err)
		require.Nil(t, migrated)
		require.ErrorContains(t, err, "threshold")
		require.Equal(t, "10", oldConfig["threshold"])

		plan, err := PrepareUpgrade(&old, &next, nil, oldConfig, nil)
		require.Error(t, err)
		require.Nil(t, plan)
		require.Equal(t, "10", oldConfig["threshold"])
	})

	t.Run("number is not coerced into integer", func(t *testing.T) {
		schema := &objects.PluginConfigSchema{
			Type: "object",
			Properties: map[string]objects.PluginConfigProperty{
				"threshold": {Type: objects.ConfigTypeInteger},
			},
		}
		migrated, err := MigrateConfig(map[string]any{"threshold": 1.5}, schema)
		require.Error(t, err)
		require.Nil(t, migrated)

		migrated, err = MigrateConfig(map[string]any{"threshold": 2}, schema)
		require.NoError(t, err)
		require.Equal(t, 2, migrated["threshold"])
	})
}

func sampleManifest(t *testing.T) objects.PluginManifest {
	t.Helper()
	return objects.PluginManifest{
		SchemaVersion:       1,
		SchemaVersionSet:    true,
		ID:                  "acme.failover-guard",
		Name:                "Failover Guard",
		Version:             "1.0.0",
		Description:         "Notify and disable channels on failover storms",
		AxonhubVersion:      "^1.0.0",
		PluginAPIVersion:    1,
		PluginAPIVersionSet: true,
		Permissions:         []string{"read_channels"},
		Hooks:               &objects.PluginHooksSpec{},
		ConfigSchema: &objects.PluginConfigSchema{
			Type: "object",
			Properties: map[string]objects.PluginConfigProperty{
				"token":     {Type: objects.ConfigTypeString, Format: objects.ConfigFormatSecret},
				"threshold": {Type: objects.ConfigTypeInteger},
			},
		},
		UI: &objects.PluginUI{Pages: []objects.PluginUIPage{{
			Key:   "main",
			Entry: "ui/page.html",
			Title: mustTitle(t, `"Overview"`),
		}}},
	}
}

func mustTitle(t *testing.T, raw string) objects.PluginTitle {
	t.Helper()
	var title objects.PluginTitle
	require.NoError(t, json.Unmarshal([]byte(raw), &title))
	return title
}
