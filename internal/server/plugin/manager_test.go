package plugin

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestMetadataColumnsOmitPackage(t *testing.T) {
	require.NotContains(t, pluginMetaColumns, entplugin.FieldPackage)
}

func TestPacketTooLargeError(t *testing.T) {
	err := packetTooLargeError(errors.New("Error 1153: Got a packet bigger than 'max_allowed_packet' bytes"))
	require.ErrorContains(t, err, "插件包超过数据库允许的单次写入大小（max_allowed_packet）")

	other := packetTooLargeError(errors.New("constraint failed"))
	require.EqualError(t, other, "constraint failed")
}

func TestManagerRefreshAfterCommit(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx, cancel := context.WithCancel(pluginCtx())
	defer cancel()
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	txCtx := ent.NewContext(ent.NewTxContext(ctx, tx), tx.Client())
	result, err := manager.Install(txCtx, uiZip(t, nil, nil), nil, nil)
	require.NoError(t, err)
	require.Nil(t, manager.Current().ByID(result.Plugin.ID))
	require.NoError(t, tx.Commit())
	cancel()
	// The refresh must survive request cancellation and read outside the closed transaction.
	require.NotNil(t, manager.Current().ByID(result.Plugin.ID))
	require.NoError(t, manager.sync(context.WithoutCancel(txCtx)))
	require.Equal(t, RuntimeStatusDisabled, manager.Current().ByID(result.Plugin.ID).Status)
}

func TestManagerStopWaitsForExtraction(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, nil))
	require.NoError(t, client.Plugin.UpdateOneID(installed.ID).SetEnabled(true).Exec(ctx))
	started := make(chan struct{})
	release := make(chan struct{})
	manager.extract = func(data []byte, dest string) ([]string, error) {
		close(started)
		<-release
		return ExtractPackageUI(data, dest)
	}
	refreshed := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("refresh panicked: %v", recovered)
			}
		}()
		refreshed <- manager.ForceRefresh(ctx)
	}()
	<-started
	stopped := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("stop panicked: %v", recovered)
			}
		}()
		stopped <- manager.Stop(ctx)
	}()
	require.Eventually(t, func() bool {
		manager.callsMu.Lock()
		defer manager.callsMu.Unlock()
		return manager.stopping
	}, time.Second, time.Millisecond)
	select {
	case err := <-stopped:
		t.Errorf("stop finished during extraction: %v", err)
	default:
	}
	close(release)
	require.NoError(t, <-refreshed)
	require.NoError(t, <-stopped)
	_, err := os.Stat(manager.tempRoot)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.ErrorIs(t, manager.ForceRefresh(ctx), ErrGenerationClosed)
}

func TestManagerInstallDisabled(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()

	_, err := manager.Install(ctx, []byte("not a zip"), &[]string{"read_channels"}, nil)
	require.Error(t, err)
	count, err := client.Plugin.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	reserved := uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "axonhub.webhook"
	}, nil)
	_, err = manager.Install(ctx, reserved, nil, nil)
	require.ErrorContains(t, err, "reserved")
	count, err = client.Plugin.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	submitted := []string{"read_channels", "webhook"}
	result, err := manager.Install(ctx, uiZip(t, nil, nil), &submitted, nil)
	require.NoError(t, err)
	require.Nil(t, result.Preview)
	require.NotNil(t, result.Plugin)
	require.False(t, result.Plugin.Enabled)
	require.Empty(t, result.Plugin.GrantedPermissions)
	require.Empty(t, result.Plugin.Config)
	require.Empty(t, result.Plugin.Package)
	require.Equal(t, "acme.failover-guard", result.Plugin.PluginID)
	require.Equal(t, "1.0.0", result.Plugin.Version)

	require.Zero(t, manager.PackageReads())
	require.NoError(t, manager.Refresh(ctx))
	require.Zero(t, manager.PackageReads())

	current := manager.Current()
	require.Equal(t, RuntimeStatusDisabled, current.ByID(result.Plugin.ID).Status)
	require.Empty(t, current.ByID(result.Plugin.ID).ResourceDir())
	info, err := os.Stat(manager.tempRoot)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestManagerEnableAndRefresh(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	html := "<html>v1</html>"

	installed := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": html}))
	require.Zero(t, manager.PackageReads())

	err := manager.Enable(ctx, installed.ID, []string{"write_channels"}, nil)
	require.ErrorContains(t, err, "write_channels")
	require.Zero(t, manager.PackageReads())
	require.Equal(t, RuntimeStatusDisabled, manager.Current().ByID(installed.ID).Status)

	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"read_channels", "webhook"}, nil))
	require.Equal(t, int64(1), manager.PackageReads())

	current := manager.Current().ByID(installed.ID)
	require.Equal(t, RuntimeStatusActive, current.Status)
	require.Equal(t, []string{"read_channels", "webhook"}, current.GrantedPermissions)
	body, err := current.ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, html, string(body))
	assertNoPayload(t, manager.Current())
	assertNoRuntime(t, manager.Current())
	require.Equal(t, 0, manager.VMCount())

	before := manager.Current()
	require.NoError(t, manager.Refresh(ctx))
	require.Same(t, before, manager.Current())
	require.Equal(t, int64(1), manager.PackageReads())
	require.Equal(t, RuntimeStatusActive, manager.Current().ByID(installed.ID).Status)
}

func TestManagerDisableAndSettings(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	html := "<html>kept</html>"
	installed := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": html}))
	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"read_channels"}, nil))
	reads := manager.PackageReads()
	dir := manager.Current().ByID(installed.ID).ResourceDir()
	require.NotEmpty(t, dir)
	_, err := os.Stat(dir)
	require.NoError(t, err)

	require.NoError(t, manager.Disable(ctx, installed.ID))
	require.Equal(t, reads, manager.PackageReads())
	_, err = os.Stat(dir)
	require.ErrorIs(t, err, os.ErrNotExist)
	disabled := manager.Current().ByID(installed.ID)
	require.Equal(t, RuntimeStatusDisabled, disabled.Status)
	require.Empty(t, disabled.ResourceDir())

	require.NoError(t, manager.UpdateSettings(ctx, installed.ID, 7, map[string]any{"threshold": 4}))
	require.Equal(t, reads, manager.PackageReads())
	updated := manager.Current().ByID(installed.ID)
	require.Equal(t, 7, updated.Priority)
	require.Equal(t, float64(4), configNumber(t, updated.Config["threshold"]))
	require.Equal(t, RuntimeStatusDisabled, updated.Status)

	require.Error(t, manager.UpdateSettings(ctx, installed.ID, 7, map[string]any{"threshold": "4"}))
	require.Equal(t, reads, manager.PackageReads())
	require.Equal(t, float64(4), configNumber(t, manager.Current().ByID(installed.ID).Config["threshold"]))

	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"read_channels"}, nil))
	require.Equal(t, reads+1, manager.PackageReads())
	restoredPlugin := manager.Current().ByID(installed.ID)
	require.Equal(t, dir, restoredPlugin.ResourceDir())
	_, err = os.Stat(restoredPlugin.ResourceDir())
	require.NoError(t, err)
	restored, err := restoredPlugin.ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, html, string(restored))
}

func TestManagerEnableRequiresConfig(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["configSchema"].(map[string]any)["required"] = []any{"token", "threshold"}
	}, nil))

	view, err := manager.Get(ctx, installed.ID)
	require.NoError(t, err)
	required := map[string]bool{}
	for _, field := range view.ConfigFields {
		required[field.Name] = field.Required
	}
	require.Equal(t, map[string]bool{"threshold": true, "target": false, "token": true, "my.custom-prop": false}, required)

	require.ErrorContains(t, manager.Enable(ctx, installed.ID, nil, nil), "required config fields are missing: token")
	require.False(t, manager.Current().ByID(installed.ID).Enabled)

	// A disabled plugin may save partial config; threshold still uses its default.
	require.NoError(t, manager.UpdateSettings(ctx, installed.ID, 100, map[string]any{"target": "https://example.com"}))
	require.ErrorContains(t, manager.Enable(ctx, installed.ID, nil, nil), "token")

	require.NoError(t, manager.UpdateSettings(ctx, installed.ID, 100, map[string]any{"token": "s3cret"}))
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))
	require.True(t, manager.Current().ByID(installed.ID).Enabled)

	// An enabled plugin keeps its stored secret through the mask and cannot drop a required field.
	require.NoError(t, manager.UpdateSettings(ctx, installed.ID, 100, map[string]any{"token": SecretMask}))
	err = manager.UpdateSettings(ctx, installed.ID, 100, map[string]any{"token": ""})
	require.ErrorContains(t, err, "required config fields are missing: token")
	err = manager.UpdateSettings(ctx, installed.ID, 100, map[string]any{})
	require.ErrorContains(t, err, "token")
	require.Equal(t, "s3cret", manager.Current().ByID(installed.ID).Config["token"])

	// A new version that requires config the enabled plugin lacks is refused until it is disabled.
	nextZip := uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["version"] = "1.2.0"
		manifest["configSchema"].(map[string]any)["required"] = []any{"token", "target"}
	}, nil)
	_, err = manager.Install(ctx, nextZip, nil, nil)
	require.ErrorContains(t, err, "required config fields are missing: target")
	require.NoError(t, manager.Disable(ctx, installed.ID))
	result, err := manager.Install(ctx, nextZip, nil, nil)
	require.NoError(t, err)
	require.False(t, result.Plugin.Enabled)
	require.Equal(t, "1.2.0", result.Plugin.Version)
}

func TestManagerConfigValidation(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()

	installed := installUI(t, manager, uiZip(t, nil, nil))
	require.ErrorContains(t, manager.UpdateSettings(ctx, installed.ID, 100, nil), "object")
	require.ErrorContains(t, manager.UpdateSettings(ctx, installed.ID, 100, map[string]any{"threshold": 1.5}), "integer")
	require.ErrorContains(t, manager.UpdateSettings(ctx, installed.ID, 100, map[string]any{"missing": true}), "not declared")

	huge := map[string]any{"token": strings.Repeat("x", MaxConfigBytes)}
	require.ErrorContains(t, manager.UpdateSettings(ctx, installed.ID, 100, huge), "64 KiB")
	require.Empty(t, manager.Current().ByID(installed.ID).Config)

	emptySchema := uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.empty-schema"
		manifest["name"] = "Empty Schema"
		delete(manifest, "configSchema")
	}, map[string]string{"ui/page.html": "<html></html>"})
	other := installUI(t, manager, emptySchema)
	require.ErrorContains(t, manager.UpdateSettings(ctx, other.ID, 3, map[string]any{"threshold": 1}), "empty object")
	require.NoError(t, manager.UpdateSettings(ctx, other.ID, 3, map[string]any{}))
	require.Empty(t, manager.Current().ByID(other.ID).Config)
	require.Equal(t, 3, manager.Current().ByID(other.ID).Priority)
	require.Zero(t, manager.PackageReads())
}

func TestManagerUpgradeVersion(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()

	first := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": "<html>v1</html>"}))
	second := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.other-tool"
		manifest["name"] = "Other"
	}, map[string]string{"ui/page.html": "<html>other</html>"}))
	require.NoError(t, manager.Enable(ctx, first.ID, []string{"webhook"}, nil))
	require.NoError(t, manager.Enable(ctx, second.ID, nil, nil))
	require.Equal(t, int64(2), manager.PackageReads())

	plugins := manager.Current().Plugins()
	require.Len(t, plugins, 2)
	require.Equal(t, first.ID, plugins[0].ID)
	require.Equal(t, second.ID, plugins[1].ID)

	result, err := manager.Install(ctx, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["version"] = "1.2.0"
		manifest["name"] = "Failover Guard 2"
	}, map[string]string{"ui/page.html": "<html>v2</html>"}), nil, nil)
	require.NoError(t, err)
	require.Nil(t, result.Preview)
	require.True(t, result.Plugin.Enabled)
	require.Equal(t, "1.2.0", result.Plugin.Version)
	require.Equal(t, first.ID, result.Plugin.ID)
	require.Equal(t, int64(3), manager.PackageReads())

	current := manager.Current().ByID(first.ID)
	require.Equal(t, RuntimeStatusActive, current.Status)
	require.Equal(t, "1.2.0", current.Version)
	body, err := current.ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>v2</html>", string(body))
	otherBody, err := manager.Current().ByID(second.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>other</html>", string(otherBody))

	require.NoError(t, manager.UpdateSettings(ctx, second.ID, 50, map[string]any{}))
	require.Equal(t, int64(3), manager.PackageReads())
	plugins = manager.Current().Plugins()
	require.Equal(t, second.ID, plugins[0].ID)
	require.Equal(t, first.ID, plugins[1].ID)
}

func TestManagerUpgradePreview(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": "<html>old</html>"}))
	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"read_channels", "webhook"}, nil))

	next := uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["version"] = "1.1.0"
		manifest["permissions"] = []any{"read_channels", "webhook", "write_channels"}
	}, map[string]string{"ui/page.html": "<html>new</html>"})
	reads := manager.PackageReads()
	preview, err := manager.Install(ctx, next, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, preview.Preview)
	require.Equal(t, "1.0.0", preview.Preview.CurrentVersion)
	require.Equal(t, "1.1.0", preview.Preview.NewVersion)
	require.Equal(t, []string{"write_channels"}, preview.Preview.Permissions)
	require.Empty(t, preview.Preview.Events)
	require.Empty(t, preview.Preview.Hooks)
	require.Empty(t, preview.Preview.Pages)
	require.Equal(t, reads, manager.PackageReads())

	row, err := client.Plugin.Get(ctx, installed.ID)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", row.Version)
	require.Equal(t, []string{"read_channels", "webhook"}, row.Manifest.Permissions)
	body, err := manager.Current().ByID(installed.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>old</html>", string(body))

	grant := []string{"webhook", "write_channels", "read_channels"}
	upgraded, err := manager.Install(ctx, next, &grant, nil)
	require.NoError(t, err)
	require.Nil(t, upgraded.Preview)
	require.True(t, upgraded.Plugin.Enabled)
	require.Equal(t, "1.1.0", upgraded.Plugin.Version)
	require.Equal(t, grant, upgraded.Plugin.GrantedPermissions)
	require.Equal(t, reads+1, manager.PackageReads())
	body, err = manager.Current().ByID(installed.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>new</html>", string(body))

	disabled := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.other-tool"
		manifest["name"] = "Other"
	}, map[string]string{"ui/page.html": "<html>other</html>"}))
	direct, err := manager.Install(ctx, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.other-tool"
		manifest["name"] = "Other"
		manifest["version"] = "2.0.0"
		manifest["permissions"] = []any{"read_channels", "write_channels"}
	}, map[string]string{"ui/page.html": "<html>other2</html>"}), nil, nil)
	require.NoError(t, err)
	require.Nil(t, direct.Preview)
	require.False(t, direct.Plugin.Enabled)
	require.Equal(t, disabled.ID, direct.Plugin.ID)
	require.Equal(t, "2.0.0", direct.Plugin.Version)
}

func TestManagerNetworkGrant(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	const exact = "https://example.com/v1/check"
	const added = "https://example.com/v1/added"

	installed := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["network"] = []any{exact}
	}, map[string]string{"ui/page.html": "<html>net</html>"}))
	require.Empty(t, installed.GrantedNetwork)
	require.Empty(t, installed.GrantedPermissions)

	explicit := []string{exact}
	created, err := manager.Install(ctx, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.net-explicit"
		manifest["name"] = "Explicit"
		manifest["network"] = []any{exact}
	}, map[string]string{"ui/page.html": "<html>explicit</html>"}), nil, &explicit)
	require.NoError(t, err)
	require.Equal(t, explicit, created.Plugin.GrantedNetwork)
	require.Empty(t, created.Plugin.GrantedPermissions)

	outside := []string{"https://evil.example/nope"}
	before, err := client.Plugin.Query().Count(ctx)
	require.NoError(t, err)
	_, err = manager.Install(ctx, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.net-rejected"
		manifest["name"] = "Rejected"
		manifest["network"] = []any{exact}
	}, map[string]string{"ui/page.html": "<html>no</html>"}), nil, &outside)
	require.ErrorContains(t, err, "evil.example")
	after, err := client.Plugin.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, before, after)

	require.ErrorContains(t, manager.Enable(ctx, installed.ID, nil, outside), "evil.example")
	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"read_channels"}, []string{exact}))
	require.Equal(t, []string{exact}, manager.Current().ByID(installed.ID).GrantedNetwork)

	next := uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["version"] = "1.1.0"
		manifest["network"] = []any{exact, added}
	}, map[string]string{"ui/page.html": "<html>new</html>"})
	preview, err := manager.Install(ctx, next, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, preview.Preview)
	require.Equal(t, []string{added}, preview.Preview.Network)
	row, err := client.Plugin.Get(ctx, installed.ID)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", row.Version)
	require.Equal(t, []string{exact}, row.GrantedNetwork)

	grant := []string{"read_channels", "webhook"}
	upgraded, err := manager.Install(ctx, next, &grant, nil)
	require.NoError(t, err)
	require.Nil(t, upgraded.Preview)
	require.Equal(t, "1.1.0", upgraded.Plugin.Version)
	require.Equal(t, []string{exact}, upgraded.Plugin.GrantedNetwork)

	onlyAdded := []string{added}
	confirmed, err := manager.Install(ctx, next, nil, &onlyAdded)
	require.NoError(t, err)
	require.Nil(t, confirmed.Preview)
	require.Equal(t, onlyAdded, confirmed.Plugin.GrantedNetwork)

	badGrant := []string{"https://evil.example/nope"}
	_, err = manager.Install(ctx, next, nil, &badGrant)
	require.ErrorContains(t, err, "evil.example")
	row, err = client.Plugin.Get(ctx, installed.ID)
	require.NoError(t, err)
	require.Equal(t, onlyAdded, row.GrantedNetwork)
	require.Equal(t, "1.1.0", row.Version)

	require.NoError(t, manager.Disable(ctx, installed.ID))
	direct, err := manager.Install(ctx, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["version"] = "2.0.0"
		manifest["network"] = []any{exact}
	}, map[string]string{"ui/page.html": "<html>direct</html>"}), nil, nil)
	require.NoError(t, err)
	require.Nil(t, direct.Preview)
	require.False(t, direct.Plugin.Enabled)
	require.Empty(t, direct.Plugin.GrantedNetwork)
	require.Equal(t, "2.0.0", direct.Plugin.Version)
}

func TestManagerSecretDowngrade(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	const secret = "top-secret-token-value"

	installed := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": "<html>secret</html>"}))
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))
	require.NoError(t, manager.UpdateSettings(ctx, installed.ID, 100, map[string]any{
		"token":     secret,
		"threshold": 3,
	}))
	reads := manager.PackageReads()

	_, err := manager.Install(ctx, uiZip(t, func(t *testing.T, manifest map[string]any) {
		manifest["version"] = "1.3.0"
		token := propertiesOf(t, manifest)["token"].(map[string]any)
		delete(token, "format")
	}, map[string]string{"ui/page.html": "<html>downgrade</html>"}), nil, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "token")
	require.NotContains(t, err.Error(), secret)
	require.Equal(t, reads, manager.PackageReads())

	row, err := client.Plugin.Get(ctx, installed.ID)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", row.Version)
	require.Equal(t, secret, row.Config["token"])
	body, err := manager.Current().ByID(installed.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>secret</html>", string(body))
}

func TestManagerDelete(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()

	installed := installUI(t, manager, uiZip(t, nil, nil))
	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"read_channels"}, nil))
	require.NoError(t, manager.UpdateSettings(ctx, installed.ID, 12, map[string]any{"threshold": 9, "token": "old-token"}))
	dir := manager.Current().ByID(installed.ID).ResourceDir()

	require.NoError(t, manager.Delete(ctx, installed.ID))
	_, err := os.Stat(dir)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, manager.Current().ByID(installed.ID))

	deleted, err := client.Plugin.Query().Where(entplugin.IDEQ(installed.ID)).Only(schematype.SkipSoftDelete(ctx))
	require.NoError(t, err)
	require.NotZero(t, deleted.DeletedAt)
	require.Empty(t, deleted.Package)
	require.NotEmpty(t, deleted.PackageSha256)

	again := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": "<html>reinstall</html>"}))
	require.NotEqual(t, installed.ID, again.ID)
	require.False(t, again.Enabled)
	require.Empty(t, again.GrantedPermissions)
	require.Empty(t, again.Config)
	require.Equal(t, RuntimeStatusDisabled, manager.Current().ByID(again.ID).Status)
	require.Nil(t, manager.Current().ByPluginID("acme.failover-guard").Config["token"])
}

func TestManagerTwoInstances(t *testing.T) {
	client := newPluginDB(t)
	managerA := newTestManager(t, client)
	managerB := newTestManager(t, client)
	ctx := pluginCtx()
	require.NotEqual(t, managerA.tempRoot, managerB.tempRoot)
	require.NotEqual(t, pointerString(managerA.notifier), pointerString(managerB.notifier))

	first := installUI(t, managerA, uiZip(t, nil, map[string]string{"ui/page.html": "<html>a</html>"}))
	second := installUI(t, managerA, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.other-tool"
		manifest["name"] = "Other"
	}, map[string]string{"ui/page.html": "<html>b</html>"}))
	require.Nil(t, managerB.Current().ByID(first.ID))

	require.NoError(t, managerB.Refresh(ctx))
	require.Equal(t, RuntimeStatusDisabled, managerB.Current().ByID(first.ID).Status)
	require.Zero(t, managerB.PackageReads())

	require.NoError(t, managerA.Enable(ctx, first.ID, []string{"read_channels"}, nil))
	require.NoError(t, managerA.Enable(ctx, second.ID, nil, nil))
	require.Equal(t, RuntimeStatusActive, managerA.Current().ByID(first.ID).Status)
	require.Equal(t, RuntimeStatusDisabled, managerB.Current().ByID(first.ID).Status)

	before := managerB.PackageReads()
	require.NoError(t, managerB.Refresh(ctx))
	require.Equal(t, RuntimeStatusActive, managerB.Current().ByID(first.ID).Status)
	require.Equal(t, RuntimeStatusActive, managerB.Current().ByID(second.ID).Status)
	require.Equal(t, before+2, managerB.PackageReads())
	body, err := managerB.Current().ByID(first.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>a</html>", string(body))

	require.NoError(t, managerA.UpdateSettings(ctx, second.ID, 1, map[string]any{}))
	require.Equal(t, 100, managerB.Current().ByID(second.ID).Priority)
	require.Equal(t, 1, managerA.Current().ByID(second.ID).Priority)
	require.NoError(t, managerB.Refresh(ctx))
	require.Equal(t, 1, managerB.Current().ByID(second.ID).Priority)

	require.NoError(t, managerA.Delete(ctx, first.ID))
	require.Nil(t, managerA.Current().ByID(first.ID))
	require.NotNil(t, managerB.Current().ByID(first.ID))
	require.NoError(t, managerB.Refresh(ctx))
	require.Nil(t, managerB.Current().ByID(first.ID))
	require.NotNil(t, managerB.Current().ByID(second.ID))
}

func TestManagerInFlightGeneration(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": "<html>old</html>"}))
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))

	old := manager.Current()
	require.NoError(t, old.Acquire())
	oldDir := old.ByID(installed.ID).ResourceDir()
	body, err := old.ByID(installed.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>old</html>", string(body))

	_, err = manager.Install(ctx, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["version"] = "1.2.0"
	}, map[string]string{"ui/page.html": "<html>new</html>"}), nil, nil)
	require.NoError(t, err)
	require.ErrorIs(t, old.Acquire(), ErrGenerationClosed)

	body, err = old.ByID(installed.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>old</html>", string(body))
	_, err = os.Stat(oldDir)
	require.NoError(t, err)

	next := manager.Current()
	require.NotEqual(t, old, next)
	newDir := next.ByID(installed.ID).ResourceDir()
	require.NotEqual(t, oldDir, newDir)
	body, err = next.ByID(installed.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>new</html>", string(body))

	old.Release()
	_, err = os.Stat(oldDir)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(newDir)
	require.NoError(t, err)
	body, err = next.ByID(installed.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>new</html>", string(body))
}

func TestManagerRebuildTempDir(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	html := "<html>rebuild</html>"
	installed := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": html}))
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))
	dir := manager.Current().ByID(installed.ID).ResourceDir()
	require.NoError(t, os.RemoveAll(dir))

	_, err := manager.Current().ByID(installed.ID).ReadFile("ui/page.html")
	require.ErrorIs(t, err, ErrResourceUnavailable)

	reads := manager.PackageReads()
	require.NoError(t, manager.Refresh(ctx))
	require.Equal(t, reads+1, manager.PackageReads())
	body, err := manager.Current().ByID(installed.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, html, string(body))
	require.NoError(t, manager.Refresh(ctx))
	require.Equal(t, reads+1, manager.PackageReads())
}

func TestManagerExtractFailure(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	html := "<html>ok</html>"
	healthy := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": html}))
	require.NoError(t, manager.Enable(ctx, healthy.ID, nil, nil))

	manager.extract = func([]byte, string) ([]string, error) {
		return nil, errors.New("disk full")
	}
	brokenZip := uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.other-tool"
		manifest["name"] = "Other"
	}, map[string]string{"ui/page.html": "<html>broken</html>"})
	broken := installUI(t, manager, brokenZip)
	reads := manager.PackageReads()
	require.NoError(t, manager.Enable(ctx, broken.ID, nil, nil))
	require.Equal(t, reads+1, manager.PackageReads())

	statuses := map[int]RuntimeStatusEntry{}
	for _, status := range manager.RuntimeStatuses() {
		statuses[status.ID] = status
	}
	require.Equal(t, RuntimeStatusActive, statuses[healthy.ID].Status)
	require.Equal(t, RuntimeStatusError, statuses[broken.ID].Status)
	require.Contains(t, statuses[broken.ID].Reason, "disk full")
	require.Empty(t, manager.Current().ByID(broken.ID).ResourceDir())
	body, err := manager.Current().ByID(healthy.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, html, string(body))
	assertNoPayload(t, manager.Current())
	require.Equal(t, 0, manager.VMCount())
	require.Equal(t, 0, manager.Current().VMCount())

	require.NoError(t, manager.Refresh(ctx))
	require.Equal(t, reads+2, manager.PackageReads())
	require.Equal(t, RuntimeStatusError, manager.Current().ByID(broken.ID).Status)
	require.Equal(t, RuntimeStatusActive, manager.Current().ByID(healthy.ID).Status)
}

func TestManagerUIOnlyNoVM(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	installed := installUI(t, manager, uiZip(t, nil, map[string]string{
		"ui/page.html": "<html>ui</html>",
		"ui/app.js":    "console.log('browser')",
	}))
	require.NoError(t, manager.Enable(ctx, installed.ID, nil, nil))
	require.NoError(t, manager.Refresh(ctx))
	require.Equal(t, int64(1), manager.PackageReads())
	assertNoRuntime(t, manager.Current())
	assertNoPayload(t, manager.Current())
	require.Equal(t, 0, manager.VMCount())
	require.Nil(t, manager.Current().ByID(installed.ID).resource.compiledModule())
	require.False(t, manager.Current().ByID(installed.ID).resource.hasPluginJS)
	script, err := manager.Current().ByID(installed.ID).ReadFile("ui/app.js")
	require.NoError(t, err)
	require.Equal(t, "console.log('browser')", string(script))
}

func TestManagerJSCompileReuse(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	compiles := 0
	manager.compileModule = func(data []byte) (*moejs.Module, error) {
		compiles++
		return compilePackageModule(data)
	}

	uiPlugin := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": "<html>ui</html>"}))
	require.NoError(t, manager.Enable(ctx, uiPlugin.ID, nil, nil))
	require.Equal(t, 0, compiles)
	require.Nil(t, manager.Current().ByID(uiPlugin.ID).resource.compiledModule())
	manager.compileModule = nil

	jsPlugin := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.js-tool"
		manifest["name"] = "JS Tool"
	}, map[string]string{
		"ui/page.html": "<html>js</html>",
		"plugin.js":    "import { label } from \"./lib/label.js\"\nexport function onEvent() { return label }\n",
		"lib/label.js": "export const label = \"ok\"\n",
	}))
	reads := manager.PackageReads()
	require.NoError(t, manager.Enable(ctx, jsPlugin.ID, nil, nil))
	require.Equal(t, 0, compiles)
	require.Equal(t, reads+1, manager.PackageReads())

	current := manager.Current().ByID(jsPlugin.ID)
	require.Equal(t, RuntimeStatusActive, current.Status)
	module := current.resource.compiledModule()
	require.NotNil(t, module)
	require.Contains(t, module.Exports(), "onEvent")
	require.Equal(t, 0, manager.VMCount())
	require.Equal(t, 0, manager.Current().VMCount())
	body, err := current.ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>js</html>", string(body))

	require.NoError(t, manager.Refresh(ctx))
	require.Equal(t, 0, compiles)
	require.Equal(t, reads+1, manager.PackageReads())
	require.Same(t, module, manager.Current().ByID(jsPlugin.ID).resource.compiledModule())

	require.NoError(t, manager.Disable(ctx, jsPlugin.ID))
	require.Equal(t, RuntimeStatusDisabled, manager.Current().ByID(jsPlugin.ID).Status)
	require.Nil(t, manager.Current().ByID(jsPlugin.ID).resource)
	manager.resourcesMu.Lock()
	_, resident := manager.resources[resourceKey{id: jsPlugin.ID, sha: jsPlugin.PackageSha256}]
	uiResource := manager.resources[resourceKey{id: uiPlugin.ID, sha: uiPlugin.PackageSha256}]
	manager.resourcesMu.Unlock()
	require.False(t, resident)
	require.NotNil(t, uiResource)
	require.Nil(t, uiResource.compiledModule())
}

func TestManagerJSCompileFailure(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	healthy := installUI(t, manager, uiZip(t, nil, map[string]string{"ui/page.html": "<html>ok</html>"}))
	require.NoError(t, manager.Enable(ctx, healthy.ID, nil, nil))

	manager.compileModule = func([]byte) (*moejs.Module, error) {
		return nil, errors.New("compile failed")
	}
	broken := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.js-broken"
		manifest["name"] = "Broken"
	}, map[string]string{
		"ui/page.html": "<html>broken</html>",
		"plugin.js":    "export {}",
	}))
	reads := manager.PackageReads()
	require.NoError(t, manager.Enable(ctx, broken.ID, nil, nil))
	require.Equal(t, reads+1, manager.PackageReads())

	require.Equal(t, RuntimeStatusActive, manager.Current().ByID(healthy.ID).Status)
	brokenPlugin := manager.Current().ByID(broken.ID)
	require.Equal(t, RuntimeStatusError, brokenPlugin.Status)
	require.Contains(t, brokenPlugin.Reason, "compile failed")
	require.Nil(t, brokenPlugin.resource)
	body, err := manager.Current().ByID(healthy.ID).ReadFile("ui/page.html")
	require.NoError(t, err)
	require.Equal(t, "<html>ok</html>", string(body))

	require.NoError(t, manager.Refresh(ctx))
	require.Equal(t, reads+2, manager.PackageReads())
	require.Equal(t, RuntimeStatusError, manager.Current().ByID(broken.ID).Status)
	require.Equal(t, RuntimeStatusActive, manager.Current().ByID(healthy.ID).Status)
}

func newPluginDB(t *testing.T) *ent.Client {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := "file:" + name + "?mode=memory&cache=shared&_fk=0"
	client := enttest.NewEntClient(t, "sqlite3", dsn)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	return client
}

func newTestManager(t *testing.T, client *ent.Client) *Manager {
	t.Helper()
	manager, err := newManager(ManagerParams{
		Client: client,
		Cache:  xcache.Config{Mode: xcache.ModeMemory},
	}, managerConfig{
		refreshInterval: time.Hour,
		debounce:        time.Hour,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(context.Background()))
	})
	return manager
}

func pluginCtx() context.Context {
	return authz.WithSystemBypass(context.Background(), "plugin-manager-test")
}

func installUI(t *testing.T, manager *Manager, zip []byte) *ent.Plugin {
	t.Helper()
	result, err := manager.Install(pluginCtx(), zip, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Preview)
	require.NotNil(t, result.Plugin)
	require.Empty(t, result.Plugin.Package)
	return result.Plugin
}

func uiZip(t *testing.T, mutate func(*testing.T, map[string]any), files map[string]string) []byte {
	t.Helper()
	manifest := validManifestJSON
	if mutate != nil {
		manifest = string(mutateManifest(t, mutate))
	}
	if files == nil {
		files = map[string]string{"ui/page.html": "<html>v1</html>"}
	}
	copied := make(map[string]string, len(files)+1)
	maps.Copy(copied, files)
	copied["manifest.json"] = manifest
	return mustZip(t, copied)
}

func configNumber(t *testing.T, value any) float64 {
	t.Helper()
	switch number := value.(type) {
	case float64:
		return number
	case int:
		return float64(number)
	case int64:
		return float64(number)
	default:
		t.Fatalf("config value %T is not a number", value)
		return 0
	}
}

func pointerString(value any) string {
	return fmt.Sprintf("%p", value)
}

func assertNoRuntime(t *testing.T, gen *Generation) {
	t.Helper()
	require.Equal(t, 0, gen.VMCount())
	typ := reflect.TypeOf(*gen)
	for field := range typ.Fields() {
		name := strings.ToLower(field.Name)
		require.NotContains(t, name, "runtime")
		require.NotContains(t, name, "engine")
		require.NotContains(t, name, "vm")
	}
}

func assertNoPayload(t *testing.T, gen *Generation) {
	t.Helper()
	seen := map[uintptr]struct{}{}
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Pointer, reflect.Interface:
			if value.IsNil() {
				return
			}
			if value.Kind() == reflect.Pointer {
				addr := value.Pointer()
				if _, ok := seen[addr]; ok {
					return
				}
				seen[addr] = struct{}{}
			}
			walk(value.Elem())
		case reflect.Slice:
			if value.Type().Elem().Kind() == reflect.Uint8 {
				require.Zero(t, value.Len())
				return
			}
			for i := 0; i < value.Len(); i++ {
				walk(value.Index(i))
			}
		case reflect.Struct:
			for i := 0; i < value.NumField(); i++ {
				switch value.Type().Field(i).Name {
				case "manager", "generation", "cache", "db", "notifier":
					continue
				}
				walk(value.Field(i))
			}
		case reflect.Map:
			iter := value.MapRange()
			for iter.Next() {
				walk(iter.Key())
				walk(iter.Value())
			}
		}
	}
	walk(reflect.ValueOf(gen))
}
