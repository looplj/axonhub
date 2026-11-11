package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestMaskAndKeepSecrets(t *testing.T) {
	schema := &objects.PluginConfigSchema{
		Type: "object",
		Properties: map[string]objects.PluginConfigProperty{
			"threshold": {Type: objects.ConfigTypeInteger, Default: float64(10)},
			"target":    {Type: objects.ConfigTypeString, Title: "Webhook target"},
			"token":     {Type: objects.ConfigTypeString, Format: objects.ConfigFormatSecret},
			"mode": {
				Type: objects.ConfigTypeString,
				Enum: []any{"allowed", "other"},
			},
		},
	}

	masked := maskConfig(schema, map[string]any{"token": "s3cret-value", "threshold": float64(4)})
	require.Equal(t, SecretMask, masked["token"])
	require.Equal(t, float64(4), masked["threshold"])
	require.NotContains(t, masked, "target")
	require.NotContains(t, masked, "s3cret-value")
	for _, value := range masked {
		require.NotEqual(t, "s3cret-value", value)
	}

	withDefault := maskConfig(schema, map[string]any{})
	require.Equal(t, float64(10), withDefault["threshold"])
	require.NotContains(t, withDefault, "token")

	require.Empty(t, maskConfig(nil, map[string]any{"token": "s3cret-value"}))
	require.Empty(t, maskConfig(&objects.PluginConfigSchema{Type: "object"}, map[string]any{"token": "s3cret-value"}))

	kept := keepSecretValues(schema, map[string]any{"token": "s3cret-value"}, map[string]any{
		"token":     SecretMask,
		"threshold": float64(9),
	})
	require.Equal(t, "s3cret-value", kept["token"])
	require.Equal(t, float64(9), kept["threshold"])

	replaced := keepSecretValues(schema, map[string]any{"token": "s3cret-value"}, map[string]any{
		"token": "next-secret",
	})
	require.Equal(t, "next-secret", replaced["token"])

	unset := keepSecretValues(schema, map[string]any{}, map[string]any{"token": SecretMask})
	require.NotContains(t, unset, "token")

	require.Nil(t, keepSecretValues(schema, nil, nil))

	secretSchema := &objects.PluginConfigSchema{
		Type: "object",
		Properties: map[string]objects.PluginConfigProperty{
			"token": {
				Type:   objects.ConfigTypeString,
				Format: objects.ConfigFormatSecret,
				Enum:   []any{"allowed"},
			},
		},
	}
	err := validateStoredConfig(secretSchema, map[string]any{"token": "s3cret-value"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cret-value")
	require.ErrorContains(t, err, "token")
}

func TestUpdateSettingsKeepsMaskedSecret(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))

	require.NoError(t, manager.UpdateSettings(ctx, installed.ID, 12, map[string]any{
		"token":     "s3cret-value",
		"threshold": float64(4),
	}))
	err := manager.UpdateSettings(ctx, installed.ID, 12, map[string]any{
		"token":     "s3cret-value",
		"threshold": "nope",
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cret-value")
	require.Equal(t, "s3cret-value", manager.Current().ByID(installed.ID).Config["token"])

	require.NoError(t, manager.UpdateSettings(ctx, installed.ID, 15, map[string]any{
		"token":     SecretMask,
		"threshold": float64(6),
	}))
	current := manager.Current().ByID(installed.ID)
	require.Equal(t, "s3cret-value", current.Config["token"])
	require.Equal(t, float64(6), configNumber(t, current.Config["threshold"]))
	require.Equal(t, 15, current.Priority)

	listed, err := manager.List(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, SecretMask, listed[0].Config["token"])
	require.Equal(t, float64(6), configNumber(t, listed[0].Config["threshold"]))
	require.Equal(t, "10", configField(t, listed[0], "threshold").DefaultJSON)
	require.Equal(t, "", configField(t, listed[0], "token").DefaultJSON)
	require.Equal(t, objects.ConfigFormatSecret, configField(t, listed[0], "token").Format)
	require.Zero(t, manager.PackageReads())
	require.NotContains(t, listed[0].Config, "s3cret-value")
}

func configField(t *testing.T, plugin *InstalledPlugin, name string) ConfigField {
	t.Helper()
	for _, field := range plugin.ConfigFields {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("config field %s not found", name)
	return ConfigField{}
}
